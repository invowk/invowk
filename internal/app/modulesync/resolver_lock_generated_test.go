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
// that instant instead of keeping the loaded one. Each path must also report
// that it wrote the lock (spec "Saving the lock reports whether it wrote").
func TestResolverContentChangeAdvancesGenerated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		change func(*testing.T, *integrityFixture) (lockWritten bool)
	}{
		{name: "update to a newer version", change: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			f.fetcher.listVersions = []SemVer{"1.2.3", "1.3.0"}
			updated, lockWritten, err := f.resolver.Update(t.Context(), "")
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			if len(updated) != 1 || updated[0].ResolvedVersion != "1.3.0" {
				t.Fatalf("Update() = %v, want one module at 1.3.0", updated)
			}
			return lockWritten
		}},
		{name: "add a second module", change: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			_, lockWritten, err := f.resolver.Add(t.Context(), ModuleRef{GitURL: "https://github.com/user/other.git", Version: "^1.0.0"})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			return lockWritten
		}},
		{name: "remove the module", change: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			_, lockWritten, err := f.resolver.Remove(t.Context(), integrityTestGitURL)
			if err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			return lockWritten
		}},
		{name: "sync to a newer version", change: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			f.fetcher.listVersions = []SemVer{"1.2.3", "1.3.0"}
			second, lockWritten := f.mustSyncReportingWrite(t)
			if second.ResolvedVersion != "1.3.0" {
				t.Fatalf("Sync() resolved %s, want 1.3.0", second.ResolvedVersion)
			}
			return lockWritten
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

			if !tt.change(t, f) {
				t.Fatal("lockWritten = false after a content change, want true")
			}

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
// generated: included, and report that it skipped the write.
func TestResolverUnchangedContentKeepsLockBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		resave func(*testing.T, *integrityFixture) (lockWritten bool)
	}{
		{name: "add the already-locked module", resave: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			_, lockWritten, err := f.resolver.Add(t.Context(), ModuleRef{GitURL: integrityTestGitURL, Version: "^1.0.0"})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			return lockWritten
		}},
		{name: "update all with no newer version", resave: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			_, lockWritten, err := f.resolver.Update(t.Context(), "")
			if err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			return lockWritten
		}},
		{name: "update one with no newer version", resave: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			_, lockWritten, err := f.resolver.Update(t.Context(), integrityTestGitURL)
			if err != nil {
				t.Fatalf("Update(%s) error = %v", integrityTestGitURL, err)
			}
			return lockWritten
		}},
		{name: "sync the same requirement", resave: func(t *testing.T, f *integrityFixture) bool {
			t.Helper()
			_, lockWritten := f.mustSyncReportingWrite(t)
			return lockWritten
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
			f.mustSync(t)
			lockBefore := f.backdateLock(t)

			if tt.resave(t, f) {
				t.Fatal("lockWritten = true for unchanged content, want false")
			}

			if got := f.readLock(t); !bytes.Equal(got, lockBefore) {
				t.Fatalf("lock file changed although its content did not:\n got: %s\nwant: %s", got, lockBefore)
			}
		})
	}
}

// TestResolverRepeatedSyncReportsWriteThenSkip covers the spec scenario
// "Repeated sync" at the resolver level, where it runs without a network: the
// first sync from an empty working directory creates the lock and reports a
// write, and a second sync with the same requirements reports a skip and
// leaves the lock byte-identical.
func TestResolverRepeatedSyncReportsWriteThenSkip(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	assertPathAbsent(t, filepath.Join(f.workDir, LockFileName))

	if _, lockWritten := f.mustSyncReportingWrite(t); !lockWritten {
		t.Fatal("first Sync() lockWritten = false, want true: it created the lock")
	}
	first := f.readLock(t)

	if _, lockWritten := f.mustSyncReportingWrite(t); lockWritten {
		t.Fatal("second Sync() lockWritten = true, want false: the content did not change")
	}
	if second := f.readLock(t); !bytes.Equal(second, first) {
		t.Fatalf("second Sync() changed the lock:\n got: %s\nwant: %s", second, first)
	}
}

// TestResolverSyncWithoutRequirementsReportsNoWrite covers "A sync that finds
// no requirements saves nothing": Sync returns lockWritten == false and
// creates no lock file.
func TestResolverSyncWithoutRequirementsReportsNoWrite(t *testing.T) {
	t.Parallel()

	f := newIntegrityFixture(t, t.TempDir(), "1.2.3")
	resolved, lockWritten, err := f.resolver.Sync(t.Context(), nil)
	if err != nil {
		t.Fatalf("Sync(nil) error = %v", err)
	}
	if resolved != nil || lockWritten {
		t.Fatalf("Sync(nil) = %v, %t, want nil, false", resolved, lockWritten)
	}
	assertPathAbsent(t, filepath.Join(f.workDir, LockFileName))
}
