// SPDX-License-Identifier: MPL-2.0

//go:build linux

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// pipeDescriptorPath returns /proc/self/fd/<write end> of a new pipe. Such a
// link's target text ("pipe:[N]") names no file, although the kernel follows
// the link: it is what /dev/stdout and /dev/stderr are when the stream is a
// pipe.
func pipeDescriptorPath(t *testing.T) (path string, reader *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	return fmt.Sprintf("/proc/self/fd/%d", writer.Fd()), reader
}

// TestVirtualPathFullAccessPipeDescriptor: under full access a link whose
// target does not resolve keeps its own path, so the system can follow it
// (design D2). A write to the validated path arrives at the pipe's read end.
func TestVirtualPathFullAccessPipeDescriptor(t *testing.T) {
	t.Parallel()
	path, reader := pipeDescriptorPath(t)
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessFull)

	validated, err := validator.validate("/", path)
	if err != nil {
		t.Fatalf("validate(%s): %v", path, err)
	}
	file, err := os.OpenFile(validated, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open the validated path %s: %v", validated, err)
	}
	if _, writeErr := file.WriteString("ping"); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		t.Fatalf("close: %v", closeErr)
	}
	buf := make([]byte, 4)
	if _, readErr := reader.Read(buf); readErr != nil || string(buf) != "ping" {
		t.Errorf("read end = (%q, %v), want ping", buf, readErr)
	}
}

// TestVirtualPathRestrictedPipeLinkDenied: under restricted access a link is
// judged by its target, so a link to a pipe descriptor inside a declared
// directory is denied. The pre-change code accepted the link's own path
// through its parent fallback.
func TestVirtualPathRestrictedPipeLinkDenied(t *testing.T) {
	t.Parallel()
	path, _ := pipeDescriptorPath(t)
	declared := mustMkdir(t, filepath.Join(fstree.TempRoot(t), "declared"))
	link := filepath.Join(declared, "out")
	fstree.MustSymlink(t, path, link)
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, declared)

	if validated, err := validator.validate(declared, link); !errors.Is(err, errVirtualPathDenied) {
		t.Errorf("validate(%s) = (%q, %v), want errVirtualPathDenied", link, validated, err)
	}
}
