// SPDX-License-Identifier: MPL-2.0

package basedir

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var (
	// guardAllowlist lists files, relative to the module root, that may name
	// a base-directory variable, each with its reason. Review additions like
	// any lint exception.
	guardAllowlist = map[string]string{
		"internal/testutil/home.go": "SetHomeDir writes HOME/USERPROFILE so tests can point the process home at a temp dir",
	}

	guardScanDirs   = []string{"cmd", "internal", "pkg", "tests"}
	guardLookups    = []string{"UserHomeDir", "UserConfigDir", "UserCacheDir"}
	guardExactNames = []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA"}
	guardXDGName    = regexp.MustCompile(`^XDG_[A-Z_]+$`)
	guardWantMarker = regexp.MustCompile(`// want: (.+)$`)
)

type guardFinding struct {
	file string // slash-separated, relative to the scan root
	line int
	kind string
}

func (f guardFinding) String() string { return fmt.Sprintf("%s:%d: %s", f.file, f.line, f.kind) }

// TestBaseDirGuard is the repository gate of base-directory-resolution:
// non-test code outside internal/basedir must not look up per-user base
// directories directly.
func TestBaseDirGuard(t *testing.T) {
	t.Parallel()

	root := moduleRoot(t)
	findings, err := scanBaseDirLookups(root, guardAllowlist)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, f := range findings {
		t.Errorf("%s: use internal/basedir instead (or add a reasoned guardAllowlist entry)", f)
	}
	for file := range guardAllowlist {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(file))); statErr != nil {
			t.Errorf("stale guardAllowlist entry %s: %v", file, statErr)
		}
	}
}

// TestBaseDirGuardPlantedViolations proves the guard is not vacuous: every
// line of the committed fixture marked "// want: <kind>" must be reported,
// and nothing else (the skipped, allowlisted and out-of-scope fixture files
// also contain violations).
func TestBaseDirGuardPlantedViolations(t *testing.T) {
	t.Parallel()

	root := filepath.Join("testdata", "guardfixture")
	got, err := scanBaseDirLookups(root, guardAllowlist)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := plantedMarkers(t, root)
	if len(want) == 0 {
		t.Fatal("fixture has no // want markers")
	}
	if !slices.Equal(got, want) {
		t.Errorf("findings differ from the planted markers\n got: %v\nwant: %v", got, want)
	}
	kinds := map[string]bool{}
	for _, f := range want {
		kinds[f.kind] = true
	}
	for _, kind := range []string{
		"os.UserHomeDir", "os.UserConfigDir", "os.UserCacheDir", "os/user import",
		"literal HOME", "literal USERPROFILE", "literal APPDATA", "literal LOCALAPPDATA",
		"literal XDG_*", "literal $HOME", "literal ${HOME}",
	} {
		if !kinds[kind] {
			t.Errorf("fixture plants no %q violation", kind)
		}
	}
	for _, file := range []string{"internal/planted/plain.go", "internal/planted/alias.go", "internal/planted/dot.go"} {
		n := 0
		for _, f := range want {
			if f.file == file {
				n++
			}
		}
		if n != len(guardLookups) {
			t.Errorf("%s plants %d lookups, want %d", file, n, len(guardLookups))
		}
	}
}

// plantedMarkers reads the expected findings from the fixture's markers.
func plantedMarkers(t *testing.T, root string) []guardFinding {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return walkErr
	})
	if err != nil {
		t.Fatalf("walk fixture: %v", err)
	}
	var want []guardFinding
	for _, path := range files {
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("rel %s: %v", path, relErr)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for line := 1; scanner.Scan(); line++ {
			if m := guardWantMarker.FindStringSubmatch(scanner.Text()); m != nil {
				want = append(want, guardFinding{file: filepath.ToSlash(rel), line: line, kind: m[1]})
			}
		}
	}
	sortFindings(want)
	return want
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// scanBaseDirLookups parses every non-test .go file under cmd/, internal/,
// pkg/ and tests/ below root, plus root/main.go, and reports direct
// base-directory lookups. testdata directories and internal/basedir (the
// owner) are skipped, both matched on paths relative to root, and so are the
// allowlisted files.
func scanBaseDirLookups(root string, allowlist map[string]string) ([]guardFinding, error) {
	var files []string
	for _, dir := range guardScanDirs {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); err != nil {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return fmt.Errorf("relative path of %s: %w", path, err)
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "testdata" || rel == "internal/basedir" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") {
				if _, allowed := allowlist[rel]; !allowed {
					files = append(files, rel)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", base, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "main.go")); err == nil {
		files = append(files, "main.go")
	}

	var findings []guardFinding
	fset := token.NewFileSet()
	for _, rel := range files {
		parsed, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		for _, f := range scanFile(fset, parsed) {
			f.file = rel
			findings = append(findings, f)
		}
	}
	sortFindings(findings)
	return findings, nil
}

// scanFile reports the forbidden lookups and literals in one parsed file.
func scanFile(fset *token.FileSet, file *ast.File) []guardFinding {
	var findings []guardFinding
	report := func(pos token.Pos, kind string) {
		findings = append(findings, guardFinding{line: fset.Position(pos).Line, kind: kind})
	}

	osNames := map[string]bool{}
	dotOS := false
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		switch path {
		case "os/user":
			report(imp.Pos(), "os/user import")
		case "os":
			switch {
			case imp.Name == nil:
				osNames["os"] = true
			case imp.Name.Name == ".":
				dotOS = true
			case imp.Name.Name != "_":
				osNames[imp.Name.Name] = true
			}
		}
	}

	selectors := map[*ast.Ident]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			selectors[node.Sel] = true
			if x, ok := node.X.(*ast.Ident); ok && osNames[x.Name] && slices.Contains(guardLookups, node.Sel.Name) {
				report(node.Pos(), "os."+node.Sel.Name)
			}
		case *ast.Ident:
			if dotOS && !selectors[node] && slices.Contains(guardLookups, node.Name) {
				report(node.Pos(), "os."+node.Name)
			}
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				if kind := forbiddenLiteral(node.Value); kind != "" {
					report(node.Pos(), kind)
				}
			}
		}
		return true
	})
	return findings
}

// forbiddenLiteral returns the finding kind for a quoted Go string literal,
// or "" when it is allowed.
func forbiddenLiteral(quoted string) string {
	value, err := strconv.Unquote(quoted)
	if err != nil {
		return ""
	}
	switch {
	case slices.Contains(guardExactNames, value):
		return "literal " + value
	case guardXDGName.MatchString(value):
		return "literal XDG_*"
	case strings.Contains(value, "${HOME}"):
		return "literal ${HOME}"
	case strings.Contains(value, "$HOME"):
		return "literal $HOME"
	default:
		return ""
	}
}

func sortFindings(findings []guardFinding) {
	slices.SortFunc(findings, func(a, b guardFinding) int {
		if c := strings.Compare(a.file, b.file); c != 0 {
			return c
		}
		return a.line - b.line
	})
}
