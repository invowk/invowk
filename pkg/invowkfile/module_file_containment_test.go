// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/fstree"
)

// containmentFixture builds base/{outside, mod/{script.sh, sub/inner.sh}} and
// returns base and the module root, both with ancestor aliases resolved.
func containmentFixture(t *testing.T) (base, mod string) {
	t.Helper()
	base = fstree.TempRoot(t)
	mod = filepath.Join(base, "mod")
	testutil.MustMkdirAll(t, filepath.Join(mod, "sub"), 0o755)
	mustWrite(t, filepath.Join(mod, "script.sh"), "echo inside")
	mustWrite(t, filepath.Join(mod, "sub", "inner.sh"), "echo inner")
	mustWrite(t, filepath.Join(base, "outside"), "OUTSIDE")
	return base, mod
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestCheckModuleFilePhysicalContainment covers every result of the helper
// (design D2): contained, absent, escaping (with its three messages) and
// denied. Each case asserts both results, so a mutant that swaps any two
// classes fails.
func TestCheckModuleFilePhysicalContainment(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	type want struct {
		absent    bool
		traversal bool
		message   string // substring of the traversal error
	}
	tests := []struct {
		name  string
		setup func(t fstree.Fataler, base, mod string) (rel string)
		want  want
	}{
		{"regular file", func(fstree.Fataler, string, string) string { return "script.sh" }, want{}},
		{"file in subdirectory", func(fstree.Fataler, string, string) string { return "sub/inner.sh" }, want{}},
		{"relative in-module link to a file", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, "script.sh", filepath.Join(mod, "l"))
			return "l"
		}, want{}},
		{"relative in-module link to a directory", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, "sub", filepath.Join(mod, "l"))
			return "l/inner.sh"
		}, want{}},
		{"relative link to the root", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, "..", filepath.Join(mod, "sub", "up"))
			return "sub/up/script.sh"
		}, want{}},
		{"absolute in-module link", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, filepath.Join(mod, "script.sh"), filepath.Join(mod, "l"))
			return "l"
		}, want{traversal: true, message: "resolves outside module"}},
		{"relative escaping link", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, filepath.Join("..", "outside"), filepath.Join(mod, "l"))
			return "l"
		}, want{traversal: true, message: "resolves outside module"}},
		{"absolute escaping link", func(t fstree.Fataler, base, mod string) string {
			fstree.MustSymlink(t, filepath.Join(base, "outside"), filepath.Join(mod, "l"))
			return "l"
		}, want{traversal: true, message: "resolves outside module"}},
		{"path below an escaping link", func(t fstree.Fataler, base, mod string) string {
			fstree.MustSymlink(t, base, filepath.Join(mod, "l"))
			return "l/outside"
		}, want{traversal: true, message: "resolves outside module"}},
		{"dangling relative link", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, "missing", filepath.Join(mod, "l"))
			return "l"
		}, want{traversal: true, message: "dangling link"}},
		{"self link", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, "l", filepath.Join(mod, "l"))
			return "l"
		}, want{traversal: true, message: "link does not resolve"}},
		{"two-link cycle", func(t fstree.Fataler, _, mod string) string {
			fstree.MustSymlink(t, "b", filepath.Join(mod, "a"))
			fstree.MustSymlink(t, "a", filepath.Join(mod, "b"))
			return "a"
		}, want{traversal: true, message: "link does not resolve"}},
		{"missing file", func(fstree.Fataler, string, string) string { return "missing.sh" }, want{absent: true}},
		{"missing file in missing directory", func(fstree.Fataler, string, string) string { return "nodir/missing.sh" }, want{absent: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base, mod := containmentFixture(t)
			rel := tt.setup(t, base, mod)
			scriptPath := FilesystemPath(filepath.Join(mod, filepath.FromSlash(rel)))
			absent, err := checkModuleFilePhysicalContainment(scriptPath, FilesystemPath(mod))
			if absent != tt.want.absent {
				t.Errorf("absent = %v, want %v (err %v)", absent, tt.want.absent, err)
			}
			if !tt.want.traversal {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, ErrScriptPathTraversal) {
				t.Fatalf("error = %v, want ErrScriptPathTraversal", err)
			}
			msg := err.Error()
			if !strings.Contains(msg, tt.want.message) {
				t.Errorf("error %q does not name %q", msg, tt.want.message)
			}
			if !strings.Contains(msg, string(scriptPath)) || !strings.Contains(msg, mod) {
				t.Errorf("error %q does not name the path and the module", msg)
			}
			for _, other := range []string{"resolves outside module", "dangling link", "link does not resolve"} {
				if other != tt.want.message && strings.Contains(msg, other) {
					t.Errorf("error %q also names %q", msg, other)
				}
			}
		})
	}
}

// TestCheckModuleFilePhysicalContainment_MissingRoot: a module directory that
// does not exist is absent (the injected reader then reports the read error).
func TestCheckModuleFilePhysicalContainment_MissingRoot(t *testing.T) {
	t.Parallel()
	mod := filepath.Join(t.TempDir(), "gone")
	absent, err := checkModuleFilePhysicalContainment(FilesystemPath(filepath.Join(mod, "s.sh")), FilesystemPath(mod))
	if err != nil || !absent {
		t.Fatalf("missing root: absent=%v err=%v, want absent and no error", absent, err)
	}
}

// TestCheckModuleFilePhysicalContainment_RootIsFile: OpenRoot on a regular
// file fails with an OS error that is not not-exist, which is denied.
func TestCheckModuleFilePhysicalContainment_RootIsFile(t *testing.T) {
	t.Parallel()
	_, mod := containmentFixture(t)
	notDir := filepath.Join(mod, "script.sh")
	absent, err := checkModuleFilePhysicalContainment(FilesystemPath(filepath.Join(notDir, "x")), FilesystemPath(notDir))
	if absent || err == nil {
		t.Fatalf("root is a file: absent=%v err=%v, want a denial", absent, err)
	}
	if errors.Is(err, ErrScriptPathTraversal) {
		t.Fatalf("root is a file: got a traversal error, want the OS error: %v", err)
	}
}

// TestCheckModuleFilePhysicalContainment_RelFailure: a script path that
// filepath.Rel cannot relate to the module is a traversal error.
func TestCheckModuleFilePhysicalContainment_RelFailure(t *testing.T) {
	t.Parallel()
	_, mod := containmentFixture(t)
	absent, err := checkModuleFilePhysicalContainment("relative/script.sh", FilesystemPath(mod))
	if absent || !errors.Is(err, ErrScriptPathTraversal) {
		t.Fatalf("Rel failure: absent=%v err=%v, want ErrScriptPathTraversal", absent, err)
	}
}

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

func TestIsOSError(t *testing.T) {
	t.Parallel()
	if isOSError(errors.New("path escapes from parent")) {
		t.Error("a plain error was reported as an OS error")
	}
	if !isOSError(&fs.PathError{Op: "stat", Path: "x", Err: syscall.ENOENT}) {
		t.Error("a wrapped errno was not reported as an OS error")
	}
	if isOSError(fmt.Errorf("wrapped: %w", &fs.PathError{Op: "stat", Path: "x", Err: errors.New("escape")})) {
		t.Error("a wrapped plain error was reported as an OS error")
	}
}

// TestContainModuleScriptRead_ErrorShapes: escapes pass through as traversal
// errors; other OS errors become script read errors naming the selection.
func TestContainModuleScriptRead_ErrorShapes(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base, mod := containmentFixture(t)
	fstree.MustSymlink(t, filepath.Join(base, "outside"), filepath.Join(mod, "l"))

	err := containModuleScriptRead("l", FilesystemPath(filepath.Join(mod, "l")), FilesystemPath(mod))
	if !errors.Is(err, ErrScriptPathTraversal) || strings.Contains(err.Error(), "failed to read script file") {
		t.Fatalf("escape: err = %v, want a bare traversal error", err)
	}
	if err = containModuleScriptRead("script.sh", FilesystemPath(filepath.Join(mod, "script.sh")), FilesystemPath(mod)); err != nil {
		t.Fatalf("contained: unexpected error %v", err)
	}
	if err = containModuleScriptRead("missing.sh", FilesystemPath(filepath.Join(mod, "missing.sh")), FilesystemPath(mod)); err != nil {
		t.Fatalf("absent: unexpected error %v", err)
	}
	notDir := filepath.Join(mod, "script.sh")
	err = containModuleScriptRead("x", FilesystemPath(filepath.Join(notDir, "x")), FilesystemPath(notDir))
	if err == nil || errors.Is(err, ErrScriptPathTraversal) || !strings.Contains(err.Error(), "failed to read script file") {
		t.Fatalf("denied: err = %v, want a script read error", err)
	}
}
