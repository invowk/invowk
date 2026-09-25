// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
)

const (
	lockedCommitMismatchErrMsg = "locked commit mismatch"
	lockEntryWithoutHashErrMsg = "lock entry has no content hash"
)

var (
	// ErrLockedCommitMismatch is the sentinel error wrapped by LockedCommitMismatchError.
	ErrLockedCommitMismatch = errors.New(lockedCommitMismatchErrMsg)
	// ErrLockEntryWithoutContentHash is the sentinel error wrapped by
	// LockEntryWithoutContentHashError.
	ErrLockEntryWithoutContentHash = errors.New(lockEntryWithoutHashErrMsg)
)

type (
	// LockedCommitMismatchError is returned when re-resolving a locked version
	// fetches a different commit than the lock file records, for example after
	// an upstream tag was re-pointed.
	LockedCommitMismatchError struct {
		ModuleKey ModuleRefKey
		Version   SemVer
		Locked    GitCommit
		Fetched   GitCommit
	}

	// LockEntryWithoutContentHashError is returned when vendored content would
	// be trusted through a lock entry that records no content hash, as v1.0
	// lock files do. Such an entry cannot detect tampering, so it vouches for
	// nothing (formal/tla/LockIntegrity.tla, finding F4).
	LockEntryWithoutContentHashError struct {
		ModuleKey ModuleRefKey
	}
)

// Error implements the error interface.
func (e *LockedCommitMismatchError) Error() string {
	return fmt.Sprintf("%s for module %q at %s: %s records commit %s but the remote serves %s",
		lockedCommitMismatchErrMsg, e.ModuleKey, e.Version, LockFileName, e.Locked, e.Fetched)
}

// Unwrap returns ErrLockedCommitMismatch for errors.Is() compatibility.
func (e *LockedCommitMismatchError) Unwrap() error { return ErrLockedCommitMismatch }

// Error implements the error interface.
func (e *LockEntryWithoutContentHashError) Error() string {
	return fmt.Sprintf("%s for module %q (v1.0 lock file); run `invowk module sync` to upgrade %s to v2.0",
		lockEntryWithoutHashErrMsg, e.ModuleKey, LockFileName)
}

// Unwrap returns ErrLockEntryWithoutContentHash for errors.Is() compatibility.
func (e *LockEntryWithoutContentHashError) Unwrap() error { return ErrLockEntryWithoutContentHash }

// ExpectedContentHash returns the content hash that a module freshly resolved
// at version with commit must reproduce under this lock entry.
//
// The entry constrains only its own resolved version. A different version (a
// changed constraint, or a newer match found by update) is the user's intent,
// so the entry's commit and hash are not carried over and the result is empty.
// For the same version, a different fetched commit is a re-pointed tag and
// returns a LockedCommitMismatchError. The hash is empty for v1.0 entries that
// recorded none; their commit is still checked.
func (m LockedModule) ExpectedContentHash(key ModuleRefKey, version SemVer, commit GitCommit) (ContentHash, error) {
	if m.ResolvedVersion != version {
		return "", nil
	}
	if m.GitCommit != "" && commit != m.GitCommit {
		return "", &LockedCommitMismatchError{ModuleKey: key, Version: version, Locked: m.GitCommit, Fetched: commit}
	}
	return m.ContentHash, nil
}
