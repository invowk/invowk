// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package uroot

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func mkfifoTest(t *testing.T, path string) {
	t.Helper()
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatalf("mkfifo %s: %v", path, err)
	}
}

func TestUrootDerivedPathsCpNamedPipe(t *testing.T) {
	t.Parallel()
	runDerivedCases(t, []derivedCase{
		{
			name: "named pipe in the source tree",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				mkfifoTest(t, tr.in("src", "p"))
			},
			args: []string{"cp", "-r", "src", "dst"},
			check: func(t *testing.T, _ derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if run.err == nil || !strings.Contains(run.err.Error(), "unsupported file mode") {
					t.Fatalf("error = %v, want u-root's unsupported-mode error", run.err)
				}
			},
		},
		{
			name: "entries after a named pipe are still checked", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				mkfifoTest(t, tr.in("src", "p"))
				writeTestFile(t, tr.in("src", "q"), "q")
				mkdirTest(t, tr.in("dst", "src"))
				symlinkTest(t, tr.out("t"), tr.in("dst", "src", "q"))
			},
			args: []string{"cp", "-r", "src", "dst"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if run.err == nil {
					t.Fatal("cp succeeded; want u-root's unsupported-mode error")
				}
				assertRegularFile(t, tr.out("t"), "outside-content")
			},
		},
	})
}

// TestUrootDerivedPathsTarNamedPipeArchive runs only under restricted access:
// under full access u-root would open the pipe and wait for a writer.
func TestUrootDerivedPathsTarNamedPipeArchive(t *testing.T) {
	t.Parallel()
	tr := newDerivedTestTree(t)
	mkfifoTest(t, tr.in("p"))
	mkdirTest(t, tr.in("ex"))
	before := derivedSnapshot(t, tr.base)
	run := tr.run(t, tr.root, false, "tar", "-x", "-f", "p", "ex")
	assertDerivedRefusal(t, run, errTarArchiveNotRegular, before, derivedSnapshot(t, tr.base))
	if !strings.Contains(run.err.Error(), tr.in("p")) {
		t.Fatalf("error %q does not name the archive %s", run.err, tr.in("p"))
	}
	if errors.Is(run.err, errDerivedTestDenied) {
		t.Fatalf("error %v comes from the validator, want the archive refusal", run.err)
	}
}
