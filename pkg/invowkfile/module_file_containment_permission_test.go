// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// This file holds the permission tests, which skip directly by platform and
// user; binding-test files may skip only through the fstree helpers
// (scripts/formal.py correspondence).

// TestCheckModuleFilePhysicalContainment_NoSearchPermission: a directory
// without search permission makes Stat and Lstat fail with EACCES; the result
// is the OS error (a read error), not a traversal error and not absent.
func TestCheckModuleFilePhysicalContainment_NoSearchPermission(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not deny directory search on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	_, mod := containmentFixture(t)
	locked := filepath.Join(mod, "sub")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	absent, err := checkModuleFilePhysicalContainment(FilesystemPath(filepath.Join(locked, "inner.sh")), FilesystemPath(mod))
	if absent {
		t.Fatal("no search permission: reported absent")
	}
	if !errors.Is(err, fs.ErrPermission) || errors.Is(err, ErrScriptPathTraversal) {
		t.Fatalf("no search permission: err = %v, want a permission error that is not a traversal error", err)
	}
}
