// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// These tests replay the counterexamples formal/tla/LockIntegrity.tla records
// for findings F4, F6, and F12 against the real code. F4 and F6 are fixed, and
// each test was inverted together with its fix; F12 is open.

func writeVendoredModule(t *testing.T, moduleID ModuleID, body string) *Module {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return &Module{Metadata: &Invowkmod{Module: moduleID}, Path: types.FilesystemPath(dir)}
}

// TestLockIntegrity_HashlessEntryIsRejected replays the counterexample of
// finding F4 (now fixed): a v1.0 lock entry without a content hash no longer
// vouches for whatever the vendored directory holds.
func TestLockIntegrity_HashlessEntryIsRejected(t *testing.T) {
	t.Parallel()

	module := writeVendoredModule(t, "io.example.tools", "cmds: {} // tampered")
	legacy := LockedModule{ModuleID: "io.example.tools", Namespace: "io.example.tools@1.0.0"}

	err := VerifyLockedVendoredModuleHash("https://example.com/tools.git", legacy, module)
	if !errors.Is(err, ErrLockEntryWithoutContentHash) {
		t.Fatalf("VerifyLockedVendoredModuleHash() = %v, want ErrLockEntryWithoutContentHash", err)
	}
	if !strings.Contains(err.Error(), "invowk module sync") {
		t.Fatalf("error %q should tell the user to upgrade the lock with invowk module sync", err)
	}
}

// TestLockIntegrity_SiblingCopyMustMatchCallerHash: command-scope admission
// compares the caller's locked hash with the discovered copy, so a sibling's
// vendored copy of another version is rejected (LockIntegrity.f6CheckCallerHash,
// the fix for finding F6) and a copy of the locked content is admitted.
func TestLockIntegrity_SiblingCopyMustMatchCallerHash(t *testing.T) {
	t.Parallel()

	req := ModuleRequirement{GitURL: "https://example.com/tools.git", Version: "^1.0.0"}
	sibling := writeVendoredModule(t, "io.example.tools", "cmds: {} // the sibling's pinned version")
	siblingHash, err := ComputeModuleHash(string(sibling.Path))
	if err != nil {
		t.Fatalf("ComputeModuleHash() error = %v", err)
	}
	callerLock := func(hash ContentHash) *LockFile {
		return &LockFile{Modules: map[ModuleRefKey]LockedModule{
			ModuleRef(req).Key(): {
				GitURL:          req.GitURL,
				ModuleID:        "io.example.tools",
				CommandSourceID: "io.example.tools",
				ContentHash:     hash,
			},
		}}
	}
	admit := func(lock *LockFile) bool {
		return IsDeclaredLockedCommandSource([]ModuleRequirement{req}, lock, "io.example.tools", "io.example.tools", sibling.Path)
	}

	other := ContentHash("sha256:1111111111111111111111111111111111111111111111111111111111111111")
	if admit(callerLock(other)) {
		t.Fatal("IsDeclaredLockedCommandSource() admitted a sibling copy whose content differs from the caller's lock (F6)")
	}
	if !admit(callerLock(siblingHash)) {
		t.Fatal("IsDeclaredLockedCommandSource() rejected a copy of exactly the content the caller locked")
	}
}

// TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy replays finding F12
// (LockIntegrity.findingF12HashlessSiblingAdmission), which is open: when the
// caller's lock entry records no content hash, as a v1.0 lock loaded for
// command scope does, admission compares only identity and namespace, so a
// sibling's vendored copy of different content is admitted. The fix
// (f12AdmitRequiresCallerHash) inverts this test.
func TestLockIntegrity_HashlessCallerEntryAdmitsSiblingCopy(t *testing.T) {
	t.Parallel()

	req := ModuleRequirement{GitURL: "https://example.com/tools.git", Version: "^1.0.0"}
	callerCopy := writeVendoredModule(t, "io.example.tools", "cmds: {} // the version the caller locked")
	sibling := writeVendoredModule(t, "io.example.tools", "cmds: {} // the sibling's pinned version")
	callerHash, err := ComputeModuleHash(string(callerCopy.Path))
	if err != nil {
		t.Fatalf("ComputeModuleHash(caller) error = %v", err)
	}
	siblingHash, err := ComputeModuleHash(string(sibling.Path))
	if err != nil {
		t.Fatalf("ComputeModuleHash(sibling) error = %v", err)
	}
	if callerHash == siblingHash {
		t.Fatal("fixture: the sibling copy must differ from the caller's locked content")
	}
	hashless := &LockFile{Version: "1.0", Modules: map[ModuleRefKey]LockedModule{
		ModuleRef(req).Key(): {GitURL: req.GitURL, ModuleID: "io.example.tools", CommandSourceID: "io.example.tools"},
	}}
	if !IsDeclaredLockedCommandSource([]ModuleRequirement{req}, hashless, "io.example.tools", "io.example.tools", sibling.Path) {
		t.Fatal("IsDeclaredLockedCommandSource() rejected a sibling copy under a hashless caller entry; F12 is fixed, invert this test")
	}
}
