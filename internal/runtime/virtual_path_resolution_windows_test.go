// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestVirtualPathWindowsRemoveJunctionRemovesJunction: rm acts on its operand
// itself and does not follow its last component (design D1-D3 of
// virtual-path-no-follow-operands), so `rm -rf j` on a junction removes the
// junction entry and leaves its target tree, wherever the target lies: under
// restricted access with the target inside the root, under restricted access
// with the target outside every root (the junction itself lies inside the
// root, so its removal is allowed), and under full access with the target
// outside the roots.
func TestVirtualPathWindowsRemoveJunctionRemovesJunction(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name          string
		access        invowkfile.VirtualFilesystemAccess
		targetOutside bool
	}{
		{name: "restricted target inside", access: invowkfile.VirtualFilesystemAccessRestricted},
		{name: "restricted target outside", access: invowkfile.VirtualFilesystemAccessRestricted, targetOutside: true},
		{name: "full target outside", access: invowkfile.VirtualFilesystemAccessFull, targetOutside: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fstree.Require(t, fstree.CapJunction)
			work := mustMkdir(t, filepath.Join(fstree.TempRoot(t), "work"))
			targetBase := work
			if tt.targetOutside {
				targetBase = outsideVirtualRootsDir(t)
			}
			target := mustMkdir(t, filepath.Join(targetBase, "target"))
			file := filepath.Join(target, "file.txt")
			if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
				t.Fatalf("write file: %v", err)
			}
			junction := filepath.Join(work, "j")
			fstree.MustJunction(t, target, junction)

			run := runVirtualShTest(t, work, "rm -rf j", tt.access)
			if run.exit != 0 {
				t.Fatalf("rm -rf j = %+v", run)
			}
			if _, err := os.Lstat(junction); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the junction entry is still there (lstat error %v)", err)
			}
			if _, err := os.Stat(file); err != nil {
				t.Errorf("the junction's target tree was changed: %v", err)
			}
		})
	}
}

// TestVirtualPathWindowsMissingDrive: a path on a drive that does not exist
// has no existing ancestor and is refused with an error that says so.
func TestVirtualPathWindowsMissingDrive(t *testing.T) {
	t.Parallel()
	root := fstree.TempRoot(t)
	drive := ""
	for letter := 'Z'; letter >= 'D'; letter-- {
		candidate := string(letter) + `:\`
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			drive = candidate
			break
		}
	}
	if drive == "" {
		t.Fatal("every drive letter from D to Z exists; no missing drive to test with")
	}
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, root)
	request := drive + `dir\file.txt`
	_, err := validator.validate(root, request)
	if !errors.Is(err, errVirtualPathNoExistingAncestor) || errors.Is(err, errVirtualPathDenied) {
		t.Fatalf("validate(%s) error = %v, want errVirtualPathNoExistingAncestor", request, err)
	}
	if !strings.Contains(err.Error(), request) {
		t.Errorf("the error does not name the path %s: %v", request, err)
	}
	if roots := normalizedRoots([]string{drive + "root"}); len(roots) != 0 {
		t.Errorf("a root on the missing drive was kept: %v", roots)
	}
}

// TestVirtualPathWindowsMissingShare: a share that does not exist on a server
// that answers (ERROR_BAD_NET_NAME, which Go does not map to fs.ErrNotExist)
// is a missing volume too, refused with the error that says so.
func TestVirtualPathWindowsMissingShare(t *testing.T) {
	t.Parallel()
	fake := newFakePathFS(t, nil, nil)
	fake.entries[fakeRoot()] = fakePathEntry{mode: fs.ModeDir, lstatErr: errBadNetName}
	_, err := walkVirtualPath(fake.vfs(), fakePath("a"))
	if !errors.Is(err, errVirtualPathNoExistingAncestor) {
		t.Fatalf("error = %v, want errVirtualPathNoExistingAncestor", err)
	}
}

// legacyNormalizeExistingOrParent is a frozen copy of normalizeExistingOrParent
// as it was before virtual-path-physical-normalisation (main at c2fe96ff). It
// is the oracle for Windows device names, which cross no link and so must keep
// their pre-change verdict (design D2). Do not update it.
func legacyNormalizeExistingOrParent(path, cwd string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path must not be empty")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		if cwd == "" {
			var err error
			cwd, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("get working directory: %w", err)
			}
		}
		cleaned = filepath.Join(cwd, cleaned)
	}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return resolved, nil
	}
	parent := filepath.Dir(cleaned)
	if resolvedParent, err := filepath.EvalSymlinks(parent); err == nil {
		return filepath.Join(resolvedParent, filepath.Base(cleaned)), nil
	}
	return cleaned, nil
}

// TestVirtualPathWindowsDeviceNames: for Windows device names the verdict
// under restricted access equals the pre-change verdict, and under full access
// NUL validates to a path that opens.
func TestVirtualPathWindowsDeviceNames(t *testing.T) {
	t.Parallel()
	root := fstree.TempRoot(t)
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, root)
	legacyRoot, err := legacyNormalizeExistingOrParent(root, "")
	if err != nil {
		t.Fatalf("legacy root: %v", err)
	}

	for _, name := range []string{"NUL", "CON", `\\.\NUL`, `\\.\pipe\invowk-path-test`} {
		legacy, legacyErr := legacyNormalizeExistingOrParent(name, root)
		wantAccept := legacyErr == nil && pathWithin(legacyRoot, legacy)
		validated, validateErr := validator.validate(root, name)
		if gotAccept := validateErr == nil; gotAccept != wantAccept {
			t.Errorf("validate(%s) accepted=%v (%q, %v), pre-change accepted=%v (%q, %v)",
				name, gotAccept, validated, validateErr, wantAccept, legacy, legacyErr)
		}
	}

	full := pathTestValidator(invowkfile.VirtualFilesystemAccessFull)
	validated, err := full.validate(root, "NUL")
	if err != nil {
		t.Fatalf("validate(NUL) under full access: %v", err)
	}
	file, err := os.OpenFile(validated, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open the validated path %s: %v", validated, err)
	}
	if closeErr := file.Close(); closeErr != nil {
		t.Errorf("close %s: %v", validated, closeErr)
	}
}

// TestVirtualPathWindowsDriveLessRootedPath verifies that a separator-led
// path with no drive volume ("/shared", "\shared") resolves via filepath.Abs
// against the current drive root rather than joining under cwd.
func TestVirtualPathWindowsDriveLessRootedPath(t *testing.T) {
	t.Parallel()
	someCwd := filepath.Join(t.TempDir(), "nested", "work")
	if err := os.MkdirAll(someCwd, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", someCwd, err)
	}
	wantAbs, err := filepath.Abs("/shared")
	if err != nil {
		t.Fatalf("filepath.Abs(/shared): %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
		cwd  string
	}{
		{name: "forward slash empty cwd", path: "/shared", cwd: ""},
		{name: "backslash empty cwd", path: `\shared`, cwd: ""},
		{name: "forward slash explicit cwd", path: "/shared", cwd: someCwd},
		{name: "backslash explicit cwd", path: `\shared`, cwd: someCwd},
	} {
		got, err := absoluteVirtualPath(tc.path, tc.cwd)
		if err != nil {
			t.Fatalf("%s: absoluteVirtualPath(%q, %q) error = %v", tc.name, tc.path, tc.cwd, err)
		}
		if got != wantAbs {
			t.Fatalf("%s: absoluteVirtualPath(%q, %q) = %q, want %q", tc.name, tc.path, tc.cwd, got, wantAbs)
		}
	}

	// Trusted workdir "/invowk-nonexistent-drive-root" must not add <cwd>\... as an allowed root.
	relWork := filepath.Join(someCwd, "invowk-nonexistent-drive-root")
	for _, root := range normalizedRoots([]string{"/invowk-nonexistent-drive-root"}) {
		if strings.EqualFold(root, relWork) {
			t.Fatalf("normalizedRoots(/invowk-nonexistent-drive-root) unexpectedly added cwd-relative root %q", root)
		}
	}
}
