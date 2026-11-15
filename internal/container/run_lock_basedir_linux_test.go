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
