// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package fspath_test

import (
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/fspath"
)

func TestInspectWritableTarget_NonRegularSpecialFiles(t *testing.T) {
	t.Parallel()

	t.Run("direct_fifo_fails_closed", func(t *testing.T) {
		t.Parallel()

		fifoPath := filepath.Join(t.TempDir(), "pipe.fifo")
		if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
			t.Fatalf("Mkfifo() error = %v", err)
		}

		_, err := fspath.InspectWritableTarget(fifoPath)
		if !errors.Is(err, fspath.ErrNonRegularTarget) {
			t.Fatalf("InspectWritableTarget() error = %v, want ErrNonRegularTarget", err)
		}
	})

	t.Run("symlink_to_fifo_fails_closed", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)

		root := t.TempDir()
		fifoPath := filepath.Join(root, "pipe.fifo")
		if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
			t.Fatalf("Mkfifo() error = %v", err)
		}
		linkPath := filepath.Join(root, "link.cue")
		fstree.MustSymlink(t, fifoPath, linkPath)

		_, err := fspath.InspectWritableTarget(linkPath)
		if !errors.Is(err, fspath.ErrNonRegularTarget) {
			t.Fatalf("InspectWritableTarget() error = %v, want ErrNonRegularTarget", err)
		}
	})
}
