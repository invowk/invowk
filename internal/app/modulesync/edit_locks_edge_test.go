// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/types"
)

// TestModuleEditLockFileNotOpenable checks design D5 for the lock file
// itself: when locks/ exists but is not a directory, so the lock file cannot
// be opened, add, remove and sync fail with the path and the
// INVOWK_MODULES_PATH hint, and leave both files untouched.
func TestModuleEditLockFileNotOpenable(t *testing.T) {
	t.Parallel()

	runs := map[string]func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error{
		"add": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := AddModuleDependency(ctx, locks, path, ModuleRef{GitURL: concurrentURLA, Version: "^1.0.0"})
			return err
		},
		"remove": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := RemoveModuleDependency(ctx, locks, path, string(concurrentURLA))
			return err
		},
		"sync": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := SyncModule(ctx, locks, path)
			return err
		},
	}
	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := t.TempDir()
			writeSourceModule(t, source, "io.example.a")
			workDir := t.TempDir()
			before := buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{concurrentURLA})
			if err := writeLockWindowFiles(workDir, before); err != nil {
				t.Fatalf("write files: %v", err)
			}
			cacheRoot := t.TempDir()
			if err := os.WriteFile(filepath.Join(cacheRoot, locksDirName), []byte("not a directory"), 0o644); err != nil {
				t.Fatalf("WriteFile(locks) error = %v", err)
			}

			err := run(t.Context(), EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheRoot)}, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")))
			if err == nil || !strings.Contains(err.Error(), "open lock file "+filepath.Join(cacheRoot, locksDirName)) || !strings.Contains(err.Error(), "set INVOWK_MODULES_PATH to a writable directory") {
				t.Fatalf("%s error = %v, want the lock file path and the INVOWK_MODULES_PATH hint", name, err)
			}
			assertLockWindowFiles(t, "files", readLockWindowFiles(t, workDir), before)
		})
	}
}

// TestLockFileErrorHintsOnlyOpenFailures checks that only an open failure
// (a *fs.PathError) gets the INVOWK_MODULES_PATH hint: a timeout or a
// cancelled wait passes through unchanged.
func TestLockFileErrorHintsOnlyOpenFailures(t *testing.T) {
	t.Parallel()

	openErr := &fs.PathError{Op: "open", Path: "x.lock", Err: fs.ErrPermission}
	if got := lockFileError(openErr); !errors.Is(got, fs.ErrPermission) || !strings.HasSuffix(got.Error(), "; set INVOWK_MODULES_PATH to a writable directory") {
		t.Fatalf("lockFileError(open) = %v, want the cause and the hint", got)
	}
	for _, err := range []error{&modulelock.TimeoutError{Path: "x.lock"}, context.Canceled} {
		if got := lockFileError(err); got != err { //nolint:errorlint // identity is the contract: the error passes through unwrapped.
			t.Fatalf("lockFileError(%v) = %v, want it unchanged", err, got)
		}
	}
}

// TestLockProjectSecondCallRefused checks design D4's held-lock record: a
// second LockProject on a resolver that holds its project lock fails with
// errLockOrder without reaching the locker, and the first lock stays
// recorded until its release, so a source lock is still granted under it.
func TestLockProjectSecondCallRefused(t *testing.T) {
	t.Parallel()

	locker := &recordingLocker{}
	r := newLockWindowResolver(t, t.TempDir(), t.TempDir(), locker)
	unlockProject, err := r.LockProject(t.Context())
	if err != nil {
		t.Fatalf("LockProject() error = %v", err)
	}
	if _, secondErr := r.LockProject(t.Context()); !errors.Is(secondErr, errLockOrder) {
		t.Fatalf("second LockProject() error = %v, want errLockOrder", secondErr)
	}
	r.locks.mu.Lock()
	held := r.locks.project
	r.locks.mu.Unlock()
	if !held {
		t.Fatal("the refused second LockProject() cleared the first lock's record")
	}
	unlockSource, err := r.lockSource(t.Context(), concurrentURLA)
	if err != nil {
		t.Fatalf("lockSource() under the project lock error = %v", err)
	}
	if err := errors.Join(unlockSource(), unlockProject()); err != nil {
		t.Fatalf("unlock error = %v", err)
	}
	if events, _ := locker.recorded(); strings.Join(events, ",") != "lock project,lock source,unlock source,unlock project" {
		t.Fatalf("lock events = %v, want only the granted locks", events)
	}
}

// TestSourceLockKeyFoldsCheckoutSpellings checks design D3: URL spellings
// that share one checkout directory (a trailing slash, letter case on a
// case-insensitive volume) share one source lock, and distinct repositories
// do not.
func TestSourceLockKeyFoldsCheckoutSpellings(t *testing.T) {
	t.Parallel()

	base := sourceLockKey("https://github.com/user/repo.git")
	for _, spelling := range []GitURL{
		"https://github.com/User/Repo.git",
		"https://github.com/user/repo/",
		"git@github.com:user/repo.git",
		"https://github.com//user/repo.git",
	} {
		if got := sourceLockKey(spelling); got != base {
			t.Errorf("sourceLockKey(%s) = %q, want %q", spelling, got, base)
		}
	}
	if got := sourceLockKey("https://github.com/user/other.git"); got == base {
		t.Errorf("sourceLockKey(other) = %q, want a different key", got)
	}
}

// TestNewResolverRejectsConflictingCacheDirs checks that the lock files'
// root has one source: a cacheDir argument that differs from the edit locks'
// CacheDir fails, and an equal one is accepted.
func TestNewResolverRejectsConflictingCacheDirs(t *testing.T) {
	t.Parallel()

	cacheA, cacheB := t.TempDir(), t.TempDir()
	locks := EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheB)}
	if _, err := NewResolver(types.FilesystemPath(t.TempDir()), types.FilesystemPath(cacheA), WithEditLocks(locks)); err == nil || !strings.Contains(err.Error(), "differs from the edit locks' cache directory") {
		t.Fatalf("NewResolver(cacheA, locks at cacheB) error = %v, want the conflict", err)
	}
	r, err := NewResolver(types.FilesystemPath(t.TempDir()), types.FilesystemPath(cacheB), WithEditLocks(locks))
	if err != nil {
		t.Fatalf("NewResolver(cacheB, locks at cacheB) error = %v", err)
	}
	if got := string(r.CacheDir()); got != cacheB {
		t.Fatalf("CacheDir() = %s, want %s", got, cacheB)
	}
}

// TestDeclarationEditResultZeroValidates checks that the zero result an
// error return carries validates, as Rollback() reads it as none.
func TestDeclarationEditResultZeroValidates(t *testing.T) {
	t.Parallel()

	var zero DeclarationEditResult
	if err := zero.Validate(); err != nil {
		t.Fatalf("zero DeclarationEditResult.Validate() = %v, want nil", err)
	}
	if got := zero.Rollback(); got != DeclarationRollbackNone {
		t.Fatalf("zero Rollback() = %q, want none", got)
	}
	if err := (AddModuleDependencyResult{}).Validate(); err != nil {
		t.Fatalf("zero AddModuleDependencyResult.Validate() = %v, want nil", err)
	}
}

// TestSnapshotsBytes checks which modes the rollback snapshots by their
// bytes: regular files everywhere, and on Windows also the non-directory
// reparse points os.Lstat reports as ModeIrregular (cloud-file placeholders).
func TestSnapshotsBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode fs.FileMode
		goos string
		want bool
	}{
		{"regular linux", 0o644, "linux", true},
		{"regular windows", 0o666, "windows", true},
		{"irregular windows", fs.ModeIrregular | 0o666, "windows", true},
		{"irregular linux", fs.ModeIrregular | 0o644, "linux", false},
		{"irregular directory windows", fs.ModeDir | fs.ModeIrregular | 0o777, "windows", false},
		{"symlink windows", fs.ModeSymlink | 0o777, "windows", false},
		{"directory linux", fs.ModeDir | 0o755, "linux", false},
		{"named pipe linux", fs.ModeNamedPipe | 0o644, "linux", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := snapshotsBytes(tt.mode, tt.goos); got != tt.want {
				t.Fatalf("snapshotsBytes(%v, %s) = %v, want %v", tt.mode, tt.goos, got, tt.want)
			}
		})
	}
}
