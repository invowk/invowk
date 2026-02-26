// SPDX-License-Identifier: MPL-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type runtimeMirrorExemptions struct {
	Exact  map[string]string `json:"exact"`
	Prefix map[string]string `json:"prefix"`
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

	return exemptions
}

// TestVirtualRuntimeMirrorCoverage enforces that non-exempt virtual-runtime
// feature tests have a native-runtime mirror.
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

	// Keep exact exemptions fresh and typo-free.
	for exemptFile, reason := range exemptions.Exact {
		if !virtualSet[exemptFile] {
			t.Errorf("stale exact mirror exemption %q (%s): file not found in %s", exemptFile, reason, filepath.Join("tests", "cli", "testdata"))
		}
	}

	// Keep prefix exemptions fresh and typo-free.
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
}
