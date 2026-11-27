// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package agentcmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/fspath"
)

func TestModuleFilesSymlinkToFIFOFailsClosed(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	root := t.TempDir()
	modulePath := filepath.Join(root, "mod.invowkmod")
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, invowkmodFileName), []byte(moduleCUE("io.example.tools")), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	fifoPath := filepath.Join(root, "pipe.fifo")
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("Mkfifo() error = %v", err)
	}
	linkPath := filepath.Join(modulePath, invowkfileFileName)
	fstree.MustSymlink(t, fifoPath, linkPath)

	if _, _, err := readModuleFiles(modulePath); !errors.Is(err, fspath.ErrNonRegularTarget) {
		t.Fatalf("readModuleFiles() error = %v, want ErrNonRegularTarget", err)
	}
	if err := writeModuleFiles(modulePath, moduleCUE("io.example.tools"), defaultInvowkfileContent); !errors.Is(err, fspath.ErrNonRegularTarget) {
		t.Fatalf("writeModuleFiles() error = %v, want ErrNonRegularTarget", err)
	}
	assertModuleSymlink(t, linkPath, fifoPath)
}
