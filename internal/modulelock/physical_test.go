// SPDX-License-Identifier: MPL-2.0

package modulelock_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/types"
)

func TestTrimFinalPathPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{`\\?\C:\Users\me\proj`, `C:\Users\me\proj`},
		{`\\?\UNC\srv\share\proj`, `\\srv\share\proj`},
		{`C:\Users\me\proj`, `C:\Users\me\proj`},
		{`/home/me/proj`, `/home/me/proj`},
	}
	for _, tt := range tests {
		if got := modulelock.TrimFinalPathPrefix(tt.in); got != tt.want {
			t.Errorf("trimFinalPathPrefix(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestPhysicalDirResolvesSymlink checks that a directory and a symbolic link
// to it give one physical path (spec "One project through two paths").
func TestPhysicalDirResolvesSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows resolves junctions and links with GetFinalPathNameByHandle: TestPhysicalDirWindowsJunctionCaseAndShortName")
	}
	fstree.Require(t, fstree.CapSymlink)

	root := t.TempDir()
	dir := filepath.Join(root, "proj")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}
	link := filepath.Join(root, "link")
	fstree.MustSymlink(t, dir, link)

	locker := modulelock.NewFileLocker()
	viaDir, err := locker.PhysicalDir(types.FilesystemPath(dir))
	if err != nil {
		t.Fatalf("PhysicalDir(dir) error = %v", err)
	}
	viaLink, err := locker.PhysicalDir(types.FilesystemPath(link))
	if err != nil {
		t.Fatalf("PhysicalDir(link) error = %v", err)
	}
	if viaDir != viaLink {
		t.Fatalf("PhysicalDir(dir) = %s, PhysicalDir(link) = %s; want one path", viaDir, viaLink)
	}
	if !filepath.IsAbs(string(viaDir)) {
		t.Fatalf("PhysicalDir(dir) = %s, want an absolute path", viaDir)
	}
}
