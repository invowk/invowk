// SPDX-License-Identifier: MPL-2.0

//go:build windows

package modulelock_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/types"
)

// TestPhysicalDirWindowsJunctionCaseAndShortName checks that a directory, a
// junction to it, another letter case and its 8.3 short name all give the
// directory's long, on-disk-case path. Only the 8.3 case may skip, when the
// volume has short names disabled.
func TestPhysicalDirWindowsJunctionCaseAndShortName(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)

	root := t.TempDir()
	dir := filepath.Join(root, "LongProjectDirectory")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	locker := modulelock.NewFileLocker()
	want, err := locker.PhysicalDir(types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("PhysicalDir(dir) error = %v", err)
	}
	if strings.HasPrefix(string(want), `\\?\`) || strings.Contains(string(want), "~") || filepath.Base(string(want)) != "LongProjectDirectory" {
		t.Fatalf("PhysicalDir(dir) = %s, want the long on-disk-case path without the \\\\?\\ prefix", want)
	}

	check := func(t *testing.T, spelling string) {
		t.Helper()
		got, err := locker.PhysicalDir(types.FilesystemPath(spelling))
		if err != nil {
			t.Fatalf("PhysicalDir(%s) error = %v", spelling, err)
		}
		if got != want {
			t.Fatalf("PhysicalDir(%s) = %s, want %s", spelling, got, want)
		}
	}

	t.Run("junction", func(t *testing.T) {
		t.Parallel()
		junction := filepath.Join(root, "junction")
		fstree.MustJunction(t, dir, junction)
		check(t, junction)
	})
	t.Run("letter case", func(t *testing.T) {
		t.Parallel()
		check(t, strings.ToUpper(string(want)))
	})
	t.Run("short name", func(t *testing.T) {
		t.Parallel()
		long, err := windows.UTF16PtrFromString(string(want))
		if err != nil {
			t.Fatalf("UTF16PtrFromString() error = %v", err)
		}
		buf := make([]uint16, windows.MAX_PATH)
		n, err := windows.GetShortPathName(long, &buf[0], uint32(len(buf)))
		if err != nil {
			t.Fatalf("GetShortPathName() error = %v", err)
		}
		short := windows.UTF16ToString(buf[:n])
		if strings.EqualFold(short, string(want)) {
			t.Skipf("the volume has 8.3 short names disabled: GetShortPathName(%s) = %s", want, short)
		}
		check(t, short)
	})
}
