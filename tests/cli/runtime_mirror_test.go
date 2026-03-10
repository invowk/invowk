// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type runtimeMirrorExemptions struct {
	Exact             map[string]string `json:"exact"`
	Prefix            map[string]string `json:"prefix"`
	CommandPathExempt map[string]string `json:"command_path_exempt"`
}

func (e runtimeMirrorExemptions) isExempt(fileName string) (exempt bool, reason string) {
	if reason, ok := e.Exact[fileName]; ok {
		return true, reason
	}
	for prefix, reason := range e.Prefix {
		if strings.HasPrefix(fileName, prefix) {
			return true, reason
		}
	}
	return false, ""
}

func loadRuntimeMirrorExemptions(t *testing.T) runtimeMirrorExemptions {
	t.Helper()

	raw, err := os.ReadFile("runtime_mirror_exemptions.json")
	if err != nil {
		t.Fatalf("failed to read runtime mirror exemptions: %v", err)
	}

	var exemptions runtimeMirrorExemptions
	if err := json.Unmarshal(raw, &exemptions); err != nil {
		t.Fatalf("failed to parse runtime mirror exemptions: %v", err)
	}

	if exemptions.Exact == nil {
		exemptions.Exact = make(map[string]string)
	}
	if exemptions.Prefix == nil {
		exemptions.Prefix = make(map[string]string)
	}
	if exemptions.CommandPathExempt == nil {
		exemptions.CommandPathExempt = make(map[string]string)
	}

	return exemptions
}

// TestVirtualRuntimeMirrorCoverage enforces that non-exempt virtual-runtime
// feature tests have a native-runtime mirror. It also detects stale and
// superseded exemptions, and orphan native mirrors without a virtual counterpart.
func TestVirtualRuntimeMirrorCoverage(t *testing.T) {
	t.Parallel()

	exemptions := loadRuntimeMirrorExemptions(t)

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}

	virtualSet := make(map[string]bool)
	nativeSet := make(map[string]bool)
	var virtualFiles []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}
		name := entry.Name()
		switch {
		case strings.HasPrefix(name, "virtual_"):
			virtualSet[name] = true
			virtualFiles = append(virtualFiles, name)
		case strings.HasPrefix(name, "native_"):
			nativeSet[name] = true
		}
	}

	if len(virtualFiles) == 0 {
		t.Fatal("no virtual_*.txtar files found in tests/cli/testdata")
	}
	slices.Sort(virtualFiles)

	for _, virtualFile := range virtualFiles {
		expectedNative := "native_" + strings.TrimPrefix(virtualFile, "virtual_")
		if nativeSet[expectedNative] {
			continue
		}
		if exempt, _ := exemptions.isExempt(virtualFile); exempt {
			continue
		}

		t.Errorf("missing native runtime mirror for %q (expected %q)", virtualFile, expectedNative)
	}

	// Keep exact exemptions fresh: detect entries pointing to deleted virtual files.
	for exemptFile, reason := range exemptions.Exact {
		if !virtualSet[exemptFile] {
			t.Errorf("stale exact mirror exemption %q (%s): file not found in %s", exemptFile, reason, filepath.Join("tests", "cli", "testdata"))
		}
	}

	// Detect exact exemptions superseded by a later-created native mirror.
	// The main loop short-circuits when nativeSet has the mirror, so a superseded
	// exemption never triggers the "missing mirror" error — it just silently persists.
	for exemptFile, reason := range exemptions.Exact {
		if !virtualSet[exemptFile] {
			continue // Already reported as stale (file not found) above.
		}
		expectedNative := "native_" + strings.TrimPrefix(exemptFile, "virtual_")
		if nativeSet[expectedNative] {
			t.Errorf("superseded exact mirror exemption %q (%s): native mirror %q now exists — remove from exemptions",
				exemptFile, reason, expectedNative)
		}
	}

	// Keep prefix exemptions fresh: detect prefixes matching no virtual files.
	for prefix, reason := range exemptions.Prefix {
		matched := false
		for _, virtualFile := range virtualFiles {
			if strings.HasPrefix(virtualFile, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("stale prefix mirror exemption %q (%s): no matching virtual_*.txtar files found", prefix, reason)
		}
	}

	// Detect orphan native mirrors: native files without a corresponding virtual file.
	// An orphan is always a cleanup mistake (deleted virtual, forgot to delete native).
	for nativeName := range nativeSet {
		expectedVirtual := "virtual_" + strings.TrimPrefix(nativeName, "native_")
		if !virtualSet[expectedVirtual] {
			t.Errorf("orphan native mirror %q: expected virtual file %q not found in testdata", nativeName, expectedVirtual)
		}
	}
}

// TestVirtualNativeCommandPathAlignment verifies that each virtual/native mirror
// pair exercises the same set of invowk command paths. This catches content drift
// where a native mirror exists but tests completely different commands.
func TestVirtualNativeCommandPathAlignment(t *testing.T) {
	t.Parallel()

	exemptions := loadRuntimeMirrorExemptions(t)

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatalf("failed to read testdata directory: %v", err)
	}

	var virtualFiles []string
	nativeSet := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}
		name := entry.Name()
		switch {
		case strings.HasPrefix(name, "virtual_"):
			virtualFiles = append(virtualFiles, name)
		case strings.HasPrefix(name, "native_"):
			nativeSet[name] = true
		}
	}
	slices.Sort(virtualFiles)

	for _, virtualFile := range virtualFiles {
		nativeFile := "native_" + strings.TrimPrefix(virtualFile, "virtual_")
		if !nativeSet[nativeFile] {
			continue // No native mirror; existence is checked by TestVirtualRuntimeMirrorCoverage.
		}
		if _, ok := exemptions.CommandPathExempt[virtualFile]; ok {
			continue // Command-path divergence is intentional for this pair.
		}

		virtualPaths := extractCommandPaths(t, filepath.Join("testdata", virtualFile))
		nativePaths := extractCommandPaths(t, filepath.Join("testdata", nativeFile))

		if !slices.Equal(virtualPaths, nativePaths) {
			t.Errorf("command-path mismatch between %q and %q:\n  virtual: %v\n  native:  %v",
				virtualFile, nativeFile, virtualPaths, nativePaths)
		}
	}

	// Two-way: detect stale command-path exemptions referencing non-existent pairs.
	for exemptFile, reason := range exemptions.CommandPathExempt {
		nativeFile := "native_" + strings.TrimPrefix(exemptFile, "virtual_")
		if !nativeSet[nativeFile] {
			t.Errorf("stale command-path exemption %q (%s): native mirror %q not found",
				exemptFile, reason, nativeFile)
		}
	}
}

// extractCommandPaths reads the script section of a txtar file (lines before the
// first "-- filename --" marker) and returns the sorted, deduplicated set of
// invowk command paths found in exec lines.
func extractCommandPaths(t *testing.T, filePath string) []string {
	t.Helper()

	f, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("failed to open %s: %v", filePath, err)
	}
	defer func() { _ = f.Close() }() // Read-only file; close error non-critical.

	execRe := regexp.MustCompile(`^!?\s*exec\s+invowk\s+(.+)`)
	sectionRe := regexp.MustCompile(`^-- .+ --$`)

	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Stop at the first file section marker (txtar archive boundary).
		if sectionRe.MatchString(line) {
			break
		}
		m := execRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		cmdPath := normalizeCommandPath(m[1])
		if cmdPath != "" {
			seen[cmdPath] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("error scanning %s: %v", filePath, err)
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return paths
}

// normalizeCommandPath extracts the bare command path from the token string
// after "exec invowk". It strips --ivk-* flags (and their values), the -r
// short alias for --ivk-runtime, and stops at the "--" separator.
func normalizeCommandPath(tokenStr string) string {
	tokens := tokenizeExecLine(tokenStr)

	var cmdParts []string
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		// Stop at "--" separator (everything after is passed to the command).
		if tok == "--" {
			break
		}
		// Skip --ivk-* long flags. If not using "=" form, the next non-flag
		// token is the value.
		if strings.HasPrefix(tok, "--ivk-") {
			if !strings.Contains(tok, "=") && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++ // Skip the value token.
			}
			continue
		}
		// Skip -r (short alias for --ivk-runtime).
		if tok == "-r" {
			if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				i++ // Skip the value token.
			}
			continue
		}
		cmdParts = append(cmdParts, tok)
	}
	return strings.Join(cmdParts, " ")
}

// tokenizeExecLine splits a command-line string into tokens, respecting
// single-quoted strings (the quoting convention used by testscript).
func tokenizeExecLine(s string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '\'' && !inQuote:
			inQuote = true
		case r == '\'' && inQuote:
			inQuote = false
		case r == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
