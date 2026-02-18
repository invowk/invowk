// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

	// Exemptions: commands that require interactive TTY input and are tested
	// via tmux/VHS instead of testscript. Each entry requires a documented reason.
	exemptions := map[string]string{
		"tui input":   "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui write":   "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui choose":  "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui confirm": "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui filter":  "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui file":    "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui table":   "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui spin":    "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
		"tui pager":   "interactive TTY required; unit-tested in internal/tui/; E2E pending tmux infrastructure",
	}

	// Build the static Cobra command tree (no dynamic command registration).
	app, err := NewApp(Dependencies{})
	if err != nil {
		t.Fatalf("NewApp() failed: %v", err)
	}
	rootCmd := NewRootCommand(app)

	// Collect all leaf, non-hidden, runnable commands and their aliases.
	commands, aliasMap := collectLeafCommands(rootCmd)

	// Two-way exemption verification: detect stale exemptions.
	for exemptCmd, reason := range exemptions {
		if !commands[exemptCmd] {
			t.Errorf("stale exemption: %q does not exist in Cobra tree (reason was: %s)", exemptCmd, reason)
		}
	}

	// Locate the testdata directory relative to this test file.
	// This follows the same runtime.Caller pattern as TestNoGlobalConfigAccess.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file path via runtime.Caller")
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
	for exemptCmd, reason := range exemptions {
		if covered[exemptCmd] {
			t.Errorf("unnecessary exemption: %q is covered by txtar tests — remove from exemptions (reason was: %s)", exemptCmd, reason)
		}
	}

	// Report uncovered commands (sorted for deterministic output).
	var uncovered []string
	for cmdPath := range commands {
		if exemptions[cmdPath] != "" {
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
		line := strings.TrimSpace(scanner.Text())
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
			tokens: []string{"mod", "validate"},
			want:   []string{"module", "validate"},
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
		"config show":     true,
		"module validate": true,
		"init":            true,
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
			tokens: []string{"mod", "validate"},
			want:   "module validate",
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
