// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// These tests replay the counterexamples formal/tla/LockIntegrity.tla records
// for findings F4 and F6 against the real code. They characterise current
// behaviour: when a fix lands, each test must be inverted with it.

func writeVendoredModule(t *testing.T, moduleID ModuleID, body string) *Module {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return &Module{Metadata: &Invowkmod{Module: moduleID}, Path: types.FilesystemPath(dir)}
}

// TestLockIntegrity_FindingF4HashlessEntryAcceptsAnyContent: a v1.0 lock entry
// has no content hash, and vendored verification accepts whatever content the
// vendored directory holds (LockIntegrity_F4_Current).
func TestLockIntegrity_FindingF4HashlessEntryAcceptsAnyContent(t *testing.T) {
	t.Parallel()

	module := writeVendoredModule(t, "io.example.tools", "cmds: {} // tampered")
	legacy := LockedModule{ModuleID: "io.example.tools", Namespace: "io.example.tools@1.0.0"}

	if err := VerifyLockedVendoredModuleHash("https://example.com/tools.git", legacy, module); err != nil {
		t.Fatalf("VerifyLockedVendoredModuleHash() = %v; F4 records that a hashless entry accepts any content. "+
			"If F4 was fixed, invert this test and LockIntegrity_F4_Current together", err)
	}
}

// TestLockIntegrity_FindingF6AdmissionIgnoresCallerHash: command-scope
// admission matches the caller's lock entry by module identity and command
// source only, so a sibling's vendored copy with different content is admitted
// although the caller locked another hash (LockIntegrity_F6_Current).
func TestLockIntegrity_FindingF6AdmissionIgnoresCallerHash(t *testing.T) {
	t.Parallel()

	req := ModuleRequirement{GitURL: "https://example.com/tools.git", Version: "^1.0.0"}
	callerLock := &LockFile{Modules: map[ModuleRefKey]LockedModule{
		ModuleRef(req).Key(): {
			GitURL:          req.GitURL,
			ModuleID:        "io.example.tools",
			CommandSourceID: "io.example.tools",
			ContentHash:     "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		},
	}}
	sibling := writeVendoredModule(t, "io.example.tools", "cmds: {} // the sibling's pinned version")
	siblingHash, err := ComputeModuleHash(string(sibling.Path))
	if err != nil {
		t.Fatalf("ComputeModuleHash() error = %v", err)
	}
	if siblingHash == callerLock.Modules[ModuleRef(req).Key()].ContentHash {
		t.Fatal("fixture error: the sibling copy must differ from the caller's locked content")
	}

	if !IsDeclaredLockedCommandSource([]ModuleRequirement{req}, callerLock, "io.example.tools", "io.example.tools") {
		t.Fatal("IsDeclaredLockedCommandSource() = false; F6 records that admission ignores the caller's hash. " +
			"If F6 was fixed, invert this test and LockIntegrity_F6_Current together")
	}
}
