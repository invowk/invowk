// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"
)

func TestLockedModuleExpectedContentHash(t *testing.T) {
	t.Parallel()

	const (
		key        = ModuleRefKey("https://github.com/org/tools.git")
		lockedHash = ContentHash("sha256:1111111111111111111111111111111111111111111111111111111111111111")
		commitA    = GitCommit("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		commitB    = GitCommit("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	)
	locked := LockedModule{ResolvedVersion: "1.2.3", GitCommit: commitA, ContentHash: lockedHash}
	lockedV10 := LockedModule{ResolvedVersion: "1.2.3", GitCommit: commitA}

	tests := []struct {
		name       string
		entry      LockedModule
		version    SemVer
		commit     GitCommit
		wantHash   ContentHash
		wantCommit bool
	}{
		{name: "same version reproduces commit", entry: locked, version: "1.2.3", commit: commitA, wantHash: lockedHash},
		{name: "same version at another commit", entry: locked, version: "1.2.3", commit: commitB, wantCommit: true},
		{name: "version change carries no expectation", entry: locked, version: "1.3.0", commit: commitB},
		{name: "v1.0 entry without hash checks commit", entry: lockedV10, version: "1.2.3", commit: commitA},
		{name: "v1.0 entry without hash rejects another commit", entry: lockedV10, version: "1.2.3", commit: commitB, wantCommit: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hash, err := tt.entry.ExpectedContentHash(key, tt.version, tt.commit)
			if tt.wantCommit {
				mismatch, ok := errors.AsType[*LockedCommitMismatchError](err)
				if !ok || !errors.Is(err, ErrLockedCommitMismatch) {
					t.Fatalf("error = %v, want LockedCommitMismatchError", err)
				}
				if mismatch.ModuleKey != key || mismatch.Locked != commitA || mismatch.Fetched != commitB {
					t.Fatalf("mismatch = %+v", mismatch)
				}
				return
			}
			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if hash != tt.wantHash {
				t.Fatalf("hash = %q, want %q", hash, tt.wantHash)
			}
		})
	}
}

func TestLockedCommitMismatchErrorMessage(t *testing.T) {
	t.Parallel()

	msg := (&LockedCommitMismatchError{
		ModuleKey: "https://github.com/org/tools.git",
		Version:   "1.2.3",
		Locked:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Fetched:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}).Error()
	for _, want := range []string{"https://github.com/org/tools.git", "1.2.3", "aaaaaaaa", "bbbbbbbb", LockFileName} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, missing %q", msg, want)
		}
	}
}

func TestContentHashMismatchErrorIncludesVersion(t *testing.T) {
	t.Parallel()

	withVersion := (&ContentHashMismatchError{ModuleKey: "k", Version: "1.2.3", Expected: "sha256:a", Actual: "sha256:b"}).Error()
	if !strings.Contains(withVersion, "at 1.2.3") {
		t.Fatalf("Error() = %q, want version context", withVersion)
	}
	withoutVersion := (&ContentHashMismatchError{ModuleKey: "k", Expected: "sha256:a", Actual: "sha256:b"}).Error()
	if strings.Contains(withoutVersion, " at ") {
		t.Fatalf("Error() = %q, want no version context", withoutVersion)
	}
}
