// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package invowkmod

import (
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

func TestInvowkmodEditSymlinkToFIFOFailsClosed(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	root := t.TempDir()
	fifoPath := filepath.Join(root, "pipe.fifo")
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("Mkfifo() error = %v", err)
	}
	linkPath := filepath.Join(root, "invowkmod.cue")
	fstree.MustSymlink(t, fifoPath, linkPath)

	addErr := AddRequirement(types.FilesystemPath(linkPath), testEditModuleRef())
	if !errors.Is(addErr, fspath.ErrNonRegularTarget) {
		t.Fatalf("AddRequirement() error = %v, want fspath.ErrNonRegularTarget", addErr)
	}

	written, rmErr := RemoveRequirementIfPresent(types.FilesystemPath(linkPath), "https://github.com/user/tools.git", "")
	if !errors.Is(rmErr, fspath.ErrNonRegularTarget) {
		t.Fatalf("RemoveRequirementIfPresent() error = %v, want fspath.ErrNonRegularTarget", rmErr)
	}
	if written {
		t.Fatal("RemoveRequirementIfPresent() written = true on FIFO symlink")
	}
	assertSymlinkPreserved(t, linkPath, fifoPath)
}
