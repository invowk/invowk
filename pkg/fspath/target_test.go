// SPDX-License-Identifier: MPL-2.0

package fspath_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/fspath"
)

func TestInspectWritableTarget(t *testing.T) {
	t.Parallel()

	t.Run("regular_file_preserves_perm", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "manifest.cue")
		if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}

		got, err := fspath.InspectWritableTarget(path)
		if err != nil {
			t.Fatalf("InspectWritableTarget() error = %v", err)
		}
		if got.Path != path || !got.Exists || got.WasSymlink {
			t.Fatalf("InspectWritableTarget() = %+v, want Path=%q Exists=true WasSymlink=false", got, path)
		}
		if runtime.GOOS != "windows" && got.Perm != 0o600 {
			t.Fatalf("Perm = %v, want 0600", got.Perm)
		}
	})

	t.Run("absent_path", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "missing.cue")
		got, err := fspath.InspectWritableTarget(path)
		if err != nil {
			t.Fatalf("InspectWritableTarget() error = %v", err)
		}
		want := fspath.FileTarget{Path: path}
		if got != want {
			t.Fatalf("InspectWritableTarget() = %+v, want %+v", got, want)
		}
	})

	t.Run("direct_directory_fails_closed", func(t *testing.T) {
		t.Parallel()

		dir := filepath.Join(t.TempDir(), "dir")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}

		_, err := fspath.InspectWritableTarget(dir)
		if !errors.Is(err, fspath.ErrNonRegularTarget) {
			t.Fatalf("InspectWritableTarget() error = %v, want ErrNonRegularTarget", err)
		}
	})

	t.Run("symlink_to_regular_file", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)

		root := t.TempDir()
		realPath := filepath.Join(root, "real.cue")
		if err := os.WriteFile(realPath, []byte("ok"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		linkPath := filepath.Join(root, "link.cue")
		fstree.MustSymlink(t, realPath, linkPath)

		wantResolved, err := filepath.EvalSymlinks(realPath)
		if err != nil {
			t.Fatalf("EvalSymlinks() error = %v", err)
		}

		got, err := fspath.InspectWritableTarget(linkPath)
		if err != nil {
			t.Fatalf("InspectWritableTarget() error = %v", err)
		}
		if got.Path != wantResolved || !got.Exists || !got.WasSymlink {
			t.Fatalf("InspectWritableTarget() = %+v, want Path=%q Exists=true WasSymlink=true", got, wantResolved)
		}
		if runtime.GOOS != "windows" && got.Perm != 0o600 {
			t.Fatalf("Perm = %v, want 0600", got.Perm)
		}
	})

	t.Run("multi_hop_symlink_to_regular_file", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)

		root := t.TempDir()
		realPath := filepath.Join(root, "real.cue")
		if err := os.WriteFile(realPath, []byte("ok"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		hop1 := filepath.Join(root, "hop1.cue")
		hop2 := filepath.Join(root, "hop2.cue")
		fstree.MustSymlink(t, realPath, hop1)
		fstree.MustSymlink(t, hop1, hop2)

		wantResolved, err := filepath.EvalSymlinks(realPath)
		if err != nil {
			t.Fatalf("EvalSymlinks() error = %v", err)
		}

		got, err := fspath.InspectWritableTarget(hop2)
		if err != nil {
			t.Fatalf("InspectWritableTarget() error = %v", err)
		}
		if got.Path != wantResolved || !got.Exists || !got.WasSymlink {
			t.Fatalf("InspectWritableTarget() = %+v, want Path=%q Exists=true WasSymlink=true", got, wantResolved)
		}
		if runtime.GOOS != "windows" && got.Perm != 0o600 {
			t.Fatalf("Perm = %v, want 0600", got.Perm)
		}
	})

	t.Run("dangling_symlink_fails_with_not_exist", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)

		root := t.TempDir()
		linkPath := filepath.Join(root, "dangling.cue")
		fstree.MustSymlink(t, filepath.Join(root, "missing.cue"), linkPath)

		_, err := fspath.InspectWritableTarget(linkPath)
		if err == nil {
			t.Fatal("InspectWritableTarget() error = nil, want dangling symlink error")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("InspectWritableTarget() error = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("circular_symlink_fails_closed", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)

		root := t.TempDir()
		linkA := filepath.Join(root, "a.cue")
		linkB := filepath.Join(root, "b.cue")
		fstree.MustSymlink(t, linkB, linkA)
		fstree.MustSymlink(t, linkA, linkB)

		_, err := fspath.InspectWritableTarget(linkA)
		if err == nil {
			t.Fatal("InspectWritableTarget() error = nil, want circular symlink error")
		}
	})

	t.Run("symlink_to_directory_fails_with_non_regular", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)

		root := t.TempDir()
		dir := filepath.Join(root, "targetdir")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir() error = %v", err)
		}
		linkPath := filepath.Join(root, "dirlink")
		fstree.MustSymlink(t, dir, linkPath)

		_, err := fspath.InspectWritableTarget(linkPath)
		if !errors.Is(err, fspath.ErrNonRegularTarget) {
			t.Fatalf("InspectWritableTarget() error = %v, want ErrNonRegularTarget", err)
		}
	})
}
