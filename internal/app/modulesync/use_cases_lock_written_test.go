// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

const lockWrittenRootInvowkmod = `module: "io.example.root"
version: "1.0.0"
`

// writeLockWrittenRootInvowkmod writes a root invowkmod.cue without requires
// into dir and returns its path.
func writeLockWrittenRootInvowkmod(t *testing.T, dir string) types.FilesystemPath {
	t.Helper()
	path := filepath.Join(dir, "invowkmod.cue")
	if err := os.WriteFile(path, []byte(lockWrittenRootInvowkmod), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	return types.FilesystemPath(path)
}

// TestAddModuleDependencyReportsLockAndDeclarationSeparately covers the spec
// requirement "Add and remove report each file truthfully" for add: the lock
// write is reported from the save itself, independent of the invowkmod.cue
// edit.
func TestAddModuleDependencyReportsLockAndDeclarationSeparately(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		lockBefore      bool
		wantLockWritten bool
	}{
		{name: "module already locked", lockBefore: true, wantLockWritten: false},
		{name: "new module", lockBefore: false, wantLockWritten: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
			if tt.lockBefore {
				f.mustSync(t)
			}
			invowkmodPath := writeLockWrittenRootInvowkmod(t, f.workDir)
			lockBefore, lockReadErr := os.ReadFile(filepath.Join(f.workDir, LockFileName))

			result, err := f.resolver.AddModuleDependency(t.Context(), invowkmodPath, ModuleRef{
				GitURL:  integrityTestGitURL,
				Version: "^1.0.0",
			})
			if err != nil {
				t.Fatalf("AddModuleDependency() error = %v", err)
			}
			if got := result.LockWritten(); got != tt.wantLockWritten {
				t.Fatalf("LockWritten() = %t, want %t", got, tt.wantLockWritten)
			}
			if !result.Declaration().Updated() || result.Declaration().Err() != nil {
				t.Fatalf("Declaration() = updated %t, err %v; want updated without error",
					result.Declaration().Updated(), result.Declaration().Err())
			}
			if tt.lockBefore {
				if lockReadErr != nil {
					t.Fatalf("fixture: ReadFile(lock) error = %v", lockReadErr)
				}
				if got := f.readLock(t); !bytes.Equal(got, lockBefore) {
					t.Fatalf("lock changed although the save reported a skip:\n got: %s\nwant: %s", got, lockBefore)
				}
			}
		})
	}
}

// TestRemoveModuleDependencyReportsLockWritten covers "Remove that changes
// the lock": deleting a locked entry writes the lock, and the result says so.
func TestRemoveModuleDependencyReportsLockWritten(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	f.mustSync(t)
	invowkmodPath := filepath.Join(f.workDir, "invowkmod.cue")
	if err := os.WriteFile(invowkmodPath, []byte(lockWrittenRootInvowkmod+`requires: [
	{git_url: "`+integrityTestGitURL+`", version: "^1.0.0"},
]
`), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}

	result, err := f.resolver.RemoveModuleDependency(t.Context(), types.FilesystemPath(invowkmodPath), integrityTestGitURL)
	if err != nil {
		t.Fatalf("RemoveModuleDependency() error = %v", err)
	}
	if !result.LockWritten() {
		t.Fatal("LockWritten() = false, want true: the lock lost an entry")
	}
	if len(result.Removed()) != 1 {
		t.Fatalf("Removed() = %d entries, want 1", len(result.Removed()))
	}
	if !moduleDeclarationsUpdated(result.Declarations()) {
		t.Fatalf("Declarations() = %+v, want every edit updated without error", result.Declarations())
	}
}

// TestSyncModuleWithoutRequirementsReportsNoWrite covers "A sync that finds no
// requirements saves nothing" at the service level.
func TestSyncModuleWithoutRequirementsReportsNoWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invowkmodPath := writeLockWrittenRootInvowkmod(t, dir)

	result, err := SyncModule(t.Context(), invowkmodPath)
	if err != nil {
		t.Fatalf("SyncModule() error = %v", err)
	}
	if len(result.Requirements()) != 0 || len(result.Resolved()) != 0 {
		t.Fatalf("SyncModule() = %d requirements, %d resolved; want none",
			len(result.Requirements()), len(result.Resolved()))
	}
	if result.LockWritten() {
		t.Fatal("LockWritten() = true, want false: nothing was saved")
	}
	assertPathAbsent(t, filepath.Join(dir, LockFileName))
}

func TestSyncAndUpdateModuleResultsCarryLockWritten(t *testing.T) {
	t.Parallel()

	resolved := []*ResolvedModule{nil}
	reqs := []ModuleRef{{GitURL: integrityTestGitURL, Version: "^1.0.0"}}
	for _, lockWritten := range []bool{true, false} {
		syncResult, err := NewSyncModuleResult(reqs, resolved, lockWritten)
		if err != nil {
			t.Fatalf("NewSyncModuleResult() error = %v", err)
		}
		if syncResult.LockWritten() != lockWritten || len(syncResult.Requirements()) != 1 || len(syncResult.Resolved()) != 1 {
			t.Fatalf("SyncModuleResult = %+v, want the constructor's values (lockWritten %t)", syncResult, lockWritten)
		}
		updateResult, err := NewUpdateModuleResult(resolved, lockWritten)
		if err != nil {
			t.Fatalf("NewUpdateModuleResult() error = %v", err)
		}
		if updateResult.LockWritten() != lockWritten || len(updateResult.Updated()) != 1 {
			t.Fatalf("UpdateModuleResult = %+v, want the constructor's values (lockWritten %t)", updateResult, lockWritten)
		}
	}
}

func moduleDeclarationsUpdated(declarations []DeclarationEditResult) bool {
	if len(declarations) == 0 {
		return false
	}
	for i := range declarations {
		if !declarations[i].Updated() || declarations[i].Err() != nil {
			return false
		}
	}
	return true
}
