// SPDX-License-Identifier: MPL-2.0

package runtime

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

// TestLoadEnvFile_ModuleNoSearchPermission: Stat and Lstat both fail with
// EACCES, which is a read error (not absent, so an optional file fails too).
func TestLoadEnvFile_ModuleNoSearchPermission(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits do not deny directory search on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	_, mod := moduleEnvFixture(t)
	locked := filepath.Join(mod, "sub")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	err := LoadEnvFile(map[string]string{}, "sub/inner.env?", mod, mod)
	if !errors.Is(err, fs.ErrPermission) || errors.Is(err, ErrEnvFileOutsideModule) {
		t.Fatalf("err = %v, want a permission read error", err)
	}
}
