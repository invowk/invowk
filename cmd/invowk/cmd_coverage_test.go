// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var (
	txtarArchiveHeaderRE = regexp.MustCompile(`^-- .+ --$`)

	// builtinTxtarCoverageExemptions lists leaf built-in commands that are
	// intentionally exempt from txtar coverage checks.
	builtinTxtarCoverageExemptions = map[string]string{
		"tui input":   "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui choose":  "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui confirm": "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui write":   "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui filter":  "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui file":    "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui table":   "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui spin":    "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
		"tui pager":   "interactive TTY required; unit-tested in internal/tui/; E2E via tmux in tests/cli/tui_tmux_test.go",
	}
)

// TestBuiltinCommandTxtarCoverage verifies that every non-hidden, runnable,
// leaf built-in command has at least one testscript (.txtar) file exercising it
// in tests/cli/testdata/. This guards the constitution's 100% CLI integration
// test coverage mandate.
//
// The test builds the static Cobra command tree (no dynamic command registration),
// collects all leaf commands (non-hidden, with RunE/Run, no visible children),
// then scans txtar files for `exec invowk <path>` patterns to determine coverage.
//
// Exemptions are documented inline for commands that require interactive TTY
// input and are tested via tmux/VHS instead (see exemptions map below).
// The test enforces two-way exemption verification: stale exemptions
// (command no longer exists) and unnecessary exemptions (command is actually
// covered) both cause failures.
func TestBuiltinCommandTxtarCoverage(t *testing.T) {
	t.Parallel()

	// Build the static Cobra command tree (no dynamic command registration).
	app, err := NewApp(Dependencies{})
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}
	rootCmd := NewRootCommand(app)

	// Collect all leaf, non-hidden, runnable commands and their aliases.
	commands, aliasMap := collectLeafCommands(rootCmd)

	// Two-way exemption verification: detect stale exemptions.
	for exemptCmd, reason := range builtinTxtarCoverageExemptions {
		if !commands[exemptCmd] {
			t.Errorf("stale exemption: %q does not exist in Cobra tree (reason was: %s)", exemptCmd, reason)
		}
	}

	// Locate the testdata directory relative to this test file.
	// This follows the same goruntime.Caller pattern as TestNoGlobalConfigAccess.
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path via goruntime.Caller")
	}
	// cmd/invowk/coverage_test.go → cmd/invowk/ → cmd/ → project root
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	// Sanity check: verify we computed the correct project root by looking for go.mod.
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err != nil {
		t.Fatalf("project root detection failed: go.mod not found at %s", projectRoot)
	}
	testdataDir := filepath.Join(projectRoot, "tests", "cli", "testdata")

	// Scan all txtar files for `exec invowk ...` lines.
	covered := scanTxtarCoverage(t, testdataDir, commands, aliasMap)

	// Two-way exemption verification: detect unnecessary exemptions.
	for exemptCmd, reason := range builtinTxtarCoverageExemptions {
		if covered[exemptCmd] {
			t.Errorf("unnecessary exemption: %q is covered by txtar tests — remove from exemptions (reason was: %s)", exemptCmd, reason)
		}
	}

	// Report uncovered commands (sorted for deterministic output).
	var uncovered []string
	for cmdPath := range commands {
		if builtinTxtarCoverageExemptions[cmdPath] != "" {
			continue
		}
		if !covered[cmdPath] {
			uncovered = append(uncovered, cmdPath)
		}
	}

	sort.Strings(uncovered)
	for _, cmdPath := range uncovered {
		t.Errorf("uncovered command: %q has no txtar test in %s", cmdPath, testdataDir)
	}
}

// TestTUIExemptionTmuxCoverage verifies that every TUI txtar exemption has an
// explicit tmux e2e marker, preventing silent loss of e2e coverage.
func TestTUIExemptionTmuxCoverage(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path via goruntime.Caller")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	tmuxTestPath := filepath.Join(projectRoot, "tests", "cli", "tui_tmux_test.go")

	content, err := os.ReadFile(tmuxTestPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", tmuxTestPath, err)
	}

	covered, err := extractTmuxTUICommands(tmuxTestPath, content)
	if err != nil {
		t.Fatalf("extract tmux TUI commands: %v", err)
	}
	for _, issue := range tuiTmuxCoverageIssues(builtinTxtarCoverageExemptions, covered) {
		t.Error(issue)
	}
}

// extractTmuxTUICommands returns TUI command paths invoked by executable
// tmux sendKeys calls. AST inspection prevents comments and unrelated string
// literals from satisfying the coverage contract.
func extractTmuxTUICommands(filename string, source []byte) (map[string]bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	covered := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		commandVariables := make(map[string]string)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if assignment, ok := node.(*ast.AssignStmt); ok && len(assignment.Lhs) == len(assignment.Rhs) {
				for i, lhs := range assignment.Lhs {
					name, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}
					if command, ok := tmuxCommandLiteral(assignment.Rhs[i]); ok {
						commandVariables[name.Name] = command
					}
				}
			}

			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "sendKeys" {
				return true
			}

			command, ok := tmuxCommandLiteral(call.Args[0])
			if !ok {
				if variable, variableOK := call.Args[0].(*ast.Ident); variableOK {
					command, ok = commandVariables[variable.Name]
				}
			}
			if !ok {
				return true
			}
			fields := strings.Fields(command)
			if len(fields) >= 2 && fields[0] == "tui" {
				covered[strings.Join(fields[:2], " ")] = true
			}
			return true
		})
	}
	return covered, nil
}

func tmuxCommandLiteral(expr ast.Expr) (string, bool) {
	concat, ok := expr.(*ast.BinaryExpr)
	if ok && concat.Op == token.ADD {
		binaryPath, binaryOK := concat.X.(*ast.Ident)
		literal, literalOK := concat.Y.(*ast.BasicLit)
		if binaryOK && binaryPath.Name == "binaryPath" && literalOK && literal.Kind == token.STRING {
			command, err := strconv.Unquote(literal.Value)
			if err == nil {
				return strings.TrimSpace(command), true
			}
		}
	}

	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Sprintf" {
		return "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok || pkg.Name != "fmt" {
		return "", false
	}
	formatLiteral, ok := call.Args[0].(*ast.BasicLit)
	if !ok || formatLiteral.Kind != token.STRING {
		return "", false
	}
	binaryPath, ok := call.Args[1].(*ast.Ident)
	if !ok || binaryPath.Name != "binaryPath" {
		return "", false
	}
	format, err := strconv.Unquote(formatLiteral.Value)
	if err != nil || !strings.HasPrefix(format, "%s ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(format, "%s")), true
}

func tuiTmuxCoverageIssues(exemptions map[string]string, covered map[string]bool) []string {
	var issues []string
	for cmdPath, reason := range exemptions {
		if !strings.HasPrefix(cmdPath, "tui ") {
			continue
		}
		if !strings.Contains(strings.ToLower(reason), "e2e via tmux") {
			issues = append(issues, fmt.Sprintf("exemption reason for %q must mention tmux e2e coverage, got: %q", cmdPath, reason))
		}
		if !covered[cmdPath] {
			issues = append(issues, fmt.Sprintf("missing executable tmux e2e coverage for %q", cmdPath))
		}
	}
	for cmdPath := range covered {
		if _, ok := exemptions[cmdPath]; !ok {
			issues = append(issues, fmt.Sprintf("tmux e2e covers %q, but the command is not in builtin txtar exemptions", cmdPath))
		}
	}
	sort.Strings(issues)
	return issues
}

// collectLeafCommands walks the Cobra tree and returns:
//   - commands: set of leaf (no visible children), non-hidden, runnable command paths
//   - aliasMap: mapping from alias paths to canonical paths (e.g., "mod" -> "module")
func collectLeafCommands(root *cobra.Command) (commands map[string]bool, aliasMap map[string]string) {
	commands = make(map[string]bool)
	aliasMap = make(map[string]string)
	walkCobraTree(root, "", commands, aliasMap)
	return
}

// walkCobraTree recursively visits all commands in the Cobra tree, collecting
// leaf commands and alias mappings.
func walkCobraTree(cmd *cobra.Command, prefix string, commands map[string]bool, aliasMap map[string]string) {
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}

		childPath := child.Name()
		if prefix != "" {
			childPath = prefix + " " + child.Name()
		}

		// Record aliases at this level (e.g., "mod" -> "module").
		for _, alias := range child.Aliases {
			aliasPath := alias
			if prefix != "" {
				aliasPath = prefix + " " + alias
			}
			aliasMap[aliasPath] = childPath
		}

		// Count visible (non-hidden) children to distinguish leaf vs routing nodes.
		visibleChildren := 0
		for _, grandchild := range child.Commands() {
			if !grandchild.Hidden {
				visibleChildren++
			}
		}

		// Include leaf commands that have a handler (RunE or Run).
		// Intentionally excludes runnable routing nodes — commands that have both
		// RunE/Run AND visible children (e.g., bare `invowk config`). These routing
		// nodes delegate to subcommands; their RunE is a help/usage fallback, not a
		// feature that needs independent txtar coverage.
		if visibleChildren == 0 && (child.RunE != nil || child.Run != nil) {
			commands[childPath] = true
		}

		// Recurse into children.
		walkCobraTree(child, childPath, commands, aliasMap)
	}
}

// scanTxtarCoverage reads all .txtar files in testdataDir and extracts
// invowk command paths from `exec invowk ...` and `! exec invowk ...` lines.
// Returns a set of covered canonical command paths.
func scanTxtarCoverage(t *testing.T, testdataDir string, knownCommands map[string]bool, aliasMap map[string]string) map[string]bool {
	t.Helper()
	covered := make(map[string]bool)

	// Match both `exec invowk ...` and `! exec invowk ...`
	execRe := regexp.MustCompile(`^!?\s*exec\s+invowk\s+(.+)`)

	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("failed to read testdata directory %s: %v", testdataDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}

		filePath := filepath.Join(testdataDir, entry.Name())
		scanTxtarFile(t, filePath, entry.Name(), execRe, knownCommands, aliasMap, covered)
	}

	return covered
}

// scanTxtarFile opens a single txtar file, scans for exec invowk lines,
// and records covered commands. Extracted from scanTxtarCoverage to enable
// proper defer-based file close (defer in a loop leaks resources).
func scanTxtarFile(t *testing.T, filePath, name string, execRe *regexp.Regexp, knownCommands map[string]bool, aliasMap map[string]string, covered map[string]bool) {
	t.Helper()

	f, err := os.Open(filePath)
	if err != nil {
		t.Errorf("failed to open %s: %v", name, err)
		return
	}
	defer func() { _ = f.Close() }() // Read-only file; close error non-critical.

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		rawLine := scanner.Text()
		if txtarArchiveHeaderRE.MatchString(rawLine) {
			break
		}
		line := strings.TrimSpace(rawLine)
		m := execRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		tokens := strings.Fields(m[1])
		cmdPath := matchLongestCommand(tokens, knownCommands, aliasMap)
		if cmdPath != "" {
			covered[cmdPath] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Errorf("error scanning %s: %v", name, err)
	}
}

func TestScanTxtarFile_ScriptSectionOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		content    string
		wantConfig bool
		wantAudit  bool
	}{
		{
			name:       "script execution counts",
			content:    "exec invowk config show\n",
			wantConfig: true,
		},
		{
			name:      "archive payload execution does not count",
			content:   "exec invowk audit\n-- helper.sh --\nexec invowk config show\n",
			wantAudit: true,
		},
		{
			name:       "indented archive-like prose is not a header",
			content:    "  -- helper.sh --\nexec invowk config show\n",
			wantConfig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "coverage.txtar")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			known := map[string]bool{"config show": true, "audit": true}
			covered := make(map[string]bool)
			execRE := regexp.MustCompile(`^!?\s*exec\s+invowk\s+(.+)`)
			scanTxtarFile(t, path, filepath.Base(path), execRE, known, nil, covered)

			if covered["config show"] != tt.wantConfig {
				t.Errorf("config coverage = %t, want %t", covered["config show"], tt.wantConfig)
			}
			if covered["audit"] != tt.wantAudit {
				t.Errorf("audit coverage = %t, want %t", covered["audit"], tt.wantAudit)
			}
		})
	}
}

func TestExtractTmuxTUICommands_ExecutableCallsOnly(t *testing.T) {
	t.Parallel()

	source := []byte(`package cli
func test(s *tmuxSession) {
	// binaryPath + " tui pager ignored-comment"
	_ = " tui table ignored-string "
	s.sendKeys(binaryPath + " tui input --header 'Name'", "Enter")
	s.sendKeys(binaryPath + " tui choose one two", "Enter")
	command := fmt.Sprintf("%s tui pager README.md", binaryPath)
	s.sendKeys(command, "Enter")
	s.sendKeys("invowk tui filter alpha beta", "Enter")
}
`)
	got, err := extractTmuxTUICommands("synthetic.go", source)
	if err != nil {
		t.Fatalf("extract commands: %v", err)
	}
	want := map[string]bool{"tui input": true, "tui choose": true, "tui pager": true}
	if !maps.Equal(got, want) {
		t.Errorf("covered commands = %v, want %v", got, want)
	}
}

func TestTUITmuxCoverageIssues_Bidirectional(t *testing.T) {
	t.Parallel()

	exemptions := map[string]string{
		"tui input":   "interactive; E2E via tmux",
		"tui choose":  "interactive without a structured test",
		"config show": "covered elsewhere",
	}
	covered := map[string]bool{
		"tui input": true,
		"tui file":  true,
	}
	got := tuiTmuxCoverageIssues(exemptions, covered)
	want := []string{
		`exemption reason for "tui choose" must mention tmux e2e coverage, got: "interactive without a structured test"`,
		`missing executable tmux e2e coverage for "tui choose"`,
		`tmux e2e covers "tui file", but the command is not in builtin txtar exemptions`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("issues = %v, want %v", got, want)
	}
}

// matchLongestCommand resolves aliases in the token list and returns the
// longest matching command path from knownCommands. Returns empty string
// if no match is found.
func matchLongestCommand(tokens []string, knownCommands map[string]bool, aliasMap map[string]string) string {
	resolved := resolveAliases(tokens, aliasMap)

	// Try progressively longer prefixes; the longest match wins.
	var best string
	for i := 1; i <= len(resolved); i++ {
		candidate := strings.Join(resolved[:i], " ")
		if knownCommands[candidate] {
			best = candidate
		}
	}
	return best
}

// TestResolveAliases verifies alias resolution with synthetic data.
func TestResolveAliases(t *testing.T) {
	t.Parallel()

	aliasMap := map[string]string{
		"mod": "module",
		"cfg": "config",
	}

	tests := []struct {
		name   string
		tokens []string
		want   []string
	}{
		{
			name:   "single alias",
			tokens: []string{"mod", "list"},
			want:   []string{"module", "list"},
		},
		{
			name:   "no alias match",
			tokens: []string{"config", "show"},
			want:   []string{"config", "show"},
		},
		{
			name:   "empty tokens",
			tokens: []string{},
			want:   []string{},
		},
		{
			name:   "alias only",
			tokens: []string{"cfg"},
			want:   []string{"config"},
		},
		{
			name:   "multiple tokens no alias",
			tokens: []string{"tui", "input"},
			want:   []string{"tui", "input"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := resolveAliases(tt.tokens, aliasMap)
			if !slices.Equal(got, tt.want) {
				t.Errorf("resolveAliases(%v) = %v, want %v", tt.tokens, got, tt.want)
			}
		})
	}
}

// TestMatchLongestCommand verifies longest-prefix command matching with synthetic data.
func TestMatchLongestCommand(t *testing.T) {
	t.Parallel()

	knownCommands := map[string]bool{
		"config show": true,
		"module list": true,
		"init":        true,
	}
	aliasMap := map[string]string{
		"mod": "module",
	}

	tests := []struct {
		name   string
		tokens []string
		want   string
	}{
		{
			name:   "exact match",
			tokens: []string{"init"},
			want:   "init",
		},
		{
			name:   "prefix match two tokens",
			tokens: []string{"config", "show"},
			want:   "config show",
		},
		{
			name:   "alias then match",
			tokens: []string{"mod", "list"},
			want:   "module list",
		},
		{
			name:   "no match",
			tokens: []string{"nonexistent"},
			want:   "",
		},
		{
			name:   "empty tokens",
			tokens: []string{},
			want:   "",
		},
		{
			name:   "extra args after match",
			tokens: []string{"init", "myfile.cue"},
			want:   "init",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := matchLongestCommand(tt.tokens, knownCommands, aliasMap)
			if got != tt.want {
				t.Errorf("matchLongestCommand(%v) = %q, want %q", tt.tokens, got, tt.want)
			}
		})
	}
}

// resolveAliases replaces alias tokens with their canonical equivalents.
// For example, ["mod", "validate"] becomes ["module", "validate"] when
// "mod" is an alias for "module".
func resolveAliases(tokens []string, aliasMap map[string]string) []string {
	resolved := make([]string, len(tokens))
	copy(resolved, tokens)

	for i := 0; i < len(resolved); i++ {
		path := strings.Join(resolved[:i+1], " ")
		if canonical, ok := aliasMap[path]; ok {
			canonicalParts := strings.Fields(canonical)
			tail := resolved[i+1:]
			resolved = make([]string, 0, len(canonicalParts)+len(tail))
			resolved = append(resolved, canonicalParts...)
			resolved = append(resolved, tail...)
			// Advance index past the canonical replacement so the next
			// iteration checks the first unresolved token.
			i = len(canonicalParts) - 1
		}
	}

	return resolved
}
