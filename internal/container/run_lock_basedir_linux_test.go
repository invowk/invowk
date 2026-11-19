// SPDX-License-Identifier: MPL-2.0

//go:build linux

package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestLockFilePath_IgnoresRelativeXDGRuntimeDir(t *testing.T) {
	t.Parallel()

	path := lockFilePathWith(func(key string) string {
		if key == "XDG_RUNTIME_DIR" {
			return "run"
		}
		return ""
	})
	if want := types.FilesystemPath(filepath.Join(os.TempDir(), lockFileName)); path != want {
		t.Errorf("lockFilePathWith(XDG_RUNTIME_DIR=run) = %q, want %q", path, want)
	}
}

// legacyLockFilePathWith is lockFilePathWith as it was on main before
// require-absolute-base-dirs. It is the zero-change oracle (design D5 row b)
// and must not be edited.
func legacyLockFilePathWith(getenv func(string) string) types.FilesystemPath {
	dir := getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return types.FilesystemPath(filepath.Join(dir, lockFileName))
}

// TestLockFilePathMatchesLegacy: with XDG_RUNTIME_DIR absolute or unset, the
// lock path is unchanged.
func TestLockFilePathMatchesLegacy(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]string{"unset": "", "absolute": t.TempDir()} {
		getenv := func(key string) string {
			if key == "XDG_RUNTIME_DIR" {
				return value
			}
			return ""
		}
		if got, want := lockFilePathWith(getenv), legacyLockFilePathWith(getenv); got != want {
			t.Errorf("XDG_RUNTIME_DIR %s: lockFilePathWith() = %q, legacy %q", name, got, want)
		}
	}
}

// TestWithRunLock_RelativeXDGRuntimeDirCreatesNoCwdLock covers
// base-directory-resolution "relative XDG_RUNTIME_DIR" (run lock) through the
// production entry point: with XDG_RUNTIME_DIR=run, WithRunLock takes its lock
// in the OS temp directory and creates nothing under ./run. A CLI test cannot
// show this, because Podman, the only engine that takes the lock, reads
// XDG_RUNTIME_DIR itself (tracked as N9 in .claude/issues/cst-adoption-gaps.md).
// Not parallel: t.Chdir and t.Setenv mutate the process.
func TestWithRunLock_RelativeXDGRuntimeDirCreatesNoCwdLock(t *testing.T) {
	work := t.TempDir()
	tmp := t.TempDir()
	t.Chdir(work)
	t.Setenv("XDG_RUNTIME_DIR", "run")
	t.Setenv("TMPDIR", tmp)

	want := filepath.Join(tmp, lockFileName)
	err := WithRunLock(func() error {
		if _, statErr := os.Stat(want); statErr != nil {
			t.Errorf("lock file not in the temp dir while held: %v", statErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithRunLock() = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(work, "run")); !os.IsNotExist(statErr) {
		t.Errorf("./run exists after WithRunLock (stat error %v); the lock must never land under the current directory", statErr)
	}
}
