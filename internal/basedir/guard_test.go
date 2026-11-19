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
		"internal/testutil/home.go":                             "SetHomeDir writes HOME/USERPROFILE so tests can point the process home at a temp dir",
		"cmd/invowk/config_paths_help.go":                       "user-facing help text that names the per-OS user config paths ($XDG_CONFIG_HOME, %APPDATA%)",
		"specs/003-test-suite-audit/contracts/testutil_home.go": "historical spec contract for internal/testutil.SetHomeDir; never called by production code",
	}

	// guardSkipDirs are directory names the scan never enters.
	guardSkipDirs = []string{"testdata", "vendor", "node_modules"}
	// guardLookups are os functions that read a base directory, or expand
	// any variable named in a string (ExpandEnv, Expand).
	guardLookups = []string{"UserHomeDir", "UserConfigDir", "UserCacheDir", "ExpandEnv", "Expand"}
	// guardHelperImports are third-party home and XDG helpers that would
	// bypass the resolver.
	guardHelperImports = []string{
		"github.com/mitchellh/go-homedir",
		"github.com/adrg/xdg",
		"github.com/OpenPeeDeeP/xdg",
		"github.com/kirsle/configdir",
		"github.com/shibukawa/configdir",
		"github.com/muesli/go-app-paths",
	}
	// guardWindowsNames match case-insensitively, as Windows looks them up.
	guardWindowsNames = []string{"USERPROFILE", "APPDATA", "LOCALAPPDATA"}
	guardXDGName      = regexp.MustCompile(`^XDG_($|[A-Z])`)
	guardBracedRef    = regexp.MustCompile(`\$\{(HOME|USERPROFILE|APPDATA|LOCALAPPDATA|XDG_[A-Z0-9_]*)\}`)
	guardShellRef     = regexp.MustCompile(`\$(HOME|USERPROFILE|APPDATA|LOCALAPPDATA|XDG_[A-Z0-9_]*)`)
	guardWindowsRef   = regexp.MustCompile(`(?i)%(HOME|USERPROFILE|APPDATA|LOCALAPPDATA|XDG_[A-Z0-9_]*)%`)
	guardWantMarker   = regexp.MustCompile(`// want: (.+)$`)
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
// and nothing else (the skipped and allowlisted fixture files also contain
// violations).
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
		"os.UserHomeDir", "os.UserConfigDir", "os.UserCacheDir", "os.ExpandEnv", "os.Expand",
		"os/user import", "home-helper import",
		"literal HOME", "literal USERPROFILE", "literal APPDATA", "literal LOCALAPPDATA",
		"literal HOME=", "literal USERPROFILE=", "literal APPDATA=", "literal LOCALAPPDATA=",
		"literal XDG_*", "literal $HOME", "literal ${HOME}", "literal $XDG_*", "literal ${XDG_*}",
		"literal $USERPROFILE", "literal ${APPDATA}", "literal $LOCALAPPDATA",
		"literal %APPDATA%", "literal %USERPROFILE%", "literal %LOCALAPPDATA%", "literal %HOME%", "literal %XDG_*%",
	} {
		if !kinds[kind] {
			t.Errorf("fixture plants no %q violation", kind)
		}
	}
	if n := countFindings(want, "internal/planted/helpers.go"); n != len(guardHelperImports) {
		t.Errorf("helpers.go plants %d helper imports, want %d", n, len(guardHelperImports))
	}
	for _, file := range []string{"internal/planted/plain.go", "internal/planted/alias.go", "internal/planted/dot.go"} {
		if n := countFindings(want, file); n != len(guardLookups) {
			t.Errorf("%s plants %d lookups, want %d", file, n, len(guardLookups))
		}
	}
}

func countFindings(findings []guardFinding, file string) int {
	n := 0
	for _, f := range findings {
		if f.file == file {
			n++
		}
	}
	return n
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

// scanBaseDirLookups parses every non-test .go file of the module below
// root and reports direct base-directory lookups. It skips guardSkipDirs,
// hidden directories, directories with their own go.mod (other modules) and
// internal/basedir (the owner), all matched on paths relative to root, and
// the allowlisted files.
func scanBaseDirLookups(root string, allowlist map[string]string) ([]guardFinding, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == "." {
				return nil
			}
			if slices.Contains(guardSkipDirs, d.Name()) || strings.HasPrefix(d.Name(), ".") || rel == "internal/basedir" {
				return filepath.SkipDir
			}
			if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
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
		return nil, fmt.Errorf("walk %s: %w", root, err)
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
		switch {
		case path == "os/user":
			report(imp.Pos(), "os/user import")
		case slices.Contains(guardHelperImports, path):
			report(imp.Pos(), "home-helper import")
		case path == "os":
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
// or "" when it is allowed. XDG names are reported as XDG_* and Windows
// names in upper case.
func forbiddenLiteral(quoted string) string {
	value, err := strconv.Unquote(quoted)
	if err != nil {
		return ""
	}
	if name := exactBaseName(value); name != "" {
		return "literal " + name
	}
	if name, _, found := strings.Cut(value, "="); found {
		if base := exactBaseName(name); base != "" {
			return "literal " + base + "="
		}
	}
	if guardXDGName.MatchString(value) {
		return "literal XDG_*"
	}
	if m := guardBracedRef.FindStringSubmatch(value); m != nil {
		return "literal ${" + refName(m[1]) + "}"
	}
	if m := guardShellRef.FindStringSubmatch(value); m != nil {
		return "literal $" + refName(m[1])
	}
	if m := guardWindowsRef.FindStringSubmatch(value); m != nil {
		return "literal %" + refName(strings.ToUpper(m[1])) + "%"
	}
	return ""
}

// exactBaseName returns the canonical base-variable name that value is, or
// "". HOME and XDG names match exactly; Windows names in any case.
func exactBaseName(value string) string {
	if value == "HOME" {
		return value
	}
	for _, name := range guardWindowsNames {
		if strings.EqualFold(value, name) {
			return name
		}
	}
	return ""
}

func refName(name string) string {
	if strings.HasPrefix(name, "XDG_") {
		return "XDG_*"
	}
	return name
}

func sortFindings(findings []guardFinding) {
	slices.SortFunc(findings, func(a, b guardFinding) int {
		if c := strings.Compare(a.file, b.file); c != 0 {
			return c
		}
		return a.line - b.line
	})
}
