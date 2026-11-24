// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runtime

import (
	"bytes"
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

// TestVirtualPathWindowsRemoveJunctionActsOnTarget: an operation runs on the
// validated path, and a junction validates to its target, as a symbolic link
// does on every platform. `rm -rf` on a junction whose target is inside the
// allowed root therefore removes the target directory and leaves the junction
// entry in place (design D3).
func TestVirtualPathWindowsRemoveJunctionActsOnTarget(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)
	work := fstree.TempRoot(t)
	target := mustMkdir(t, filepath.Join(work, "target"))
	if err := os.WriteFile(filepath.Join(target, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	junction := filepath.Join(work, "j")
	fstree.MustJunction(t, target, junction)

	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(work, "invowkfile.cue"))}
	cmd := testCommandWithScript("rm-junction", "rm -rf j", invowkfile.RuntimeVirtualSh)
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}
	if result := NewShRuntime(true).Execute(ctx); !result.Success() {
		t.Fatalf("rm -rf j: %#v", result)
	}

	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the junction's target directory still exists (stat error %v)", err)
	}
	if _, err := os.Lstat(junction); err != nil {
		t.Errorf("the junction entry was removed: %v", err)
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
	fake.entries[fakeRoot()] = fakePathEntry{mode: fs.ModeDir, lstatErr: errorBadNetName}
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
