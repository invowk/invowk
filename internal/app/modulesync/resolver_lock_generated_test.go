// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/invowk/invowk/pkg/invowkmod"
)

// TestResolverContentChangeAdvancesGenerated covers the spec requirement
// "Changed lock gets a fresh generation time" for every resolver path that
// saves the lock: each starts from a lock backdated to a fixed past instant,
// changes its module entries, and must write a generated: value later than
// that instant instead of keeping the loaded one.
func TestResolverContentChangeAdvancesGenerated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change func(*testing.T, *integrityFixture)
	}{
		{name: "update to a newer version", change: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			f.fetcher.listVersions = []SemVer{"1.2.3", "1.3.0"}
			updated, err := f.resolver.Update(t.Context(), "")
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if len(updated) != 1 || updated[0].ResolvedVersion != "1.3.0" {
				t.Fatalf("Update() = %v, want one module at 1.3.0", updated)
			}
		}},
		{name: "add a second module", change: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			if _, err := f.resolver.Add(t.Context(), ModuleRef{GitURL: "https://github.com/user/other.git", Version: "^1.0.0"}); err != nil {
				t.Fatalf("Add() error = %v", err)
			}
		}},
		{name: "remove the module", change: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			if _, err := f.resolver.Remove(t.Context(), integrityTestGitURL); err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
		}},
		{name: "sync to a newer version", change: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			f.fetcher.listVersions = []SemVer{"1.2.3", "1.3.0"}
			if second := f.mustSync(t); second.ResolvedVersion != "1.3.0" {
				t.Fatalf("Sync() resolved %s, want 1.3.0", second.ResolvedVersion)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
			f.mustSync(t)
			backdated, err := time.Parse(time.RFC3339, integrityTestBackdatedGenerated)
			if err != nil {
				t.Fatalf("fixture: parse backdated instant: %v", err)
			}
			f.backdateLock(t)

			tt.change(t, f)

			lock, err := invowkmod.LoadLockFile(filepath.Join(f.workDir, LockFileName))
			if err != nil {
				t.Fatalf("LoadLockFile() error = %v", err)
			}
			if !lock.Generated.After(backdated) {
				t.Fatalf("generated: = %s after a content change, want later than the loaded %s", lock.Generated, backdated)
			}
		})
	}
}

// TestResolverUnchangedContentKeepsLockBytes covers the spec requirement
// "Unchanged lock is not rewritten" for every resolver path that saves a
// non-empty lock: each starts from a lock backdated to a fixed past instant,
// saves the same module entries again, and must leave the file byte-identical,
// generated: included.
func TestResolverUnchangedContentKeepsLockBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		resave func(*testing.T, *integrityFixture)
	}{
		{name: "add the already-locked module", resave: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			if _, err := f.resolver.Add(t.Context(), ModuleRef{GitURL: integrityTestGitURL, Version: "^1.0.0"}); err != nil {
				t.Fatalf("Add() error = %v", err)
			}
		}},
		{name: "update all with no newer version", resave: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			if _, err := f.resolver.Update(t.Context(), ""); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
		}},
		{name: "update one with no newer version", resave: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			if _, err := f.resolver.Update(t.Context(), integrityTestGitURL); err != nil {
				t.Fatalf("Update(%s) error = %v", integrityTestGitURL, err)
			}
		}},
		{name: "sync the same requirement", resave: func(t *testing.T, f *integrityFixture) {
			t.Helper()
			f.mustSync(t)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
			f.mustSync(t)
			lockBefore := f.backdateLock(t)

			tt.resave(t, f)

			if got := f.readLock(t); !bytes.Equal(got, lockBefore) {
				t.Fatalf("lock file changed although its content did not:\n got: %s\nwant: %s", got, lockBefore)
			}
		})
	}
}
