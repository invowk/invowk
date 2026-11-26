// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"archive/tar"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

type (
	// derivedCase is one built-in run, under a restricted context (FullAccess
	// false) and under a full-access context (FullAccess true), each on a
	// fresh tree.
	derivedCase struct {
		name string
		// links requires symbolic links (fstree.CapSymlink).
		links bool
		// restrictedOnly skips the full-access run, for a shape whose u-root
		// run is unbounded (it stops only at the path length limit).
		restrictedOnly bool
		setup          func(t *testing.T, tr derivedTestTree)
		// dir is the handler's working directory; nil means tr.root.
		dir  func(tr derivedTestTree) string
		args []string
		// refused is the error a restricted run must match with errors.Is;
		// nil means the restricted run is not refused.
		refused error
		// names returns a path a refusal must name.
		names func(tr derivedTestTree) string
		// check runs on every run that is not refused.
		check func(t *testing.T, tr derivedTestTree, run derivedTestRun, full bool)
	}
)

func runDerivedCases(t *testing.T, cases []derivedCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.links {
				fstree.Require(t, fstree.CapSymlink)
			}
			for _, full := range []bool{false, true} {
				mode := "restricted"
				if full {
					if tc.restrictedOnly {
						continue
					}
					mode = "full"
				}
				t.Run(mode, func(t *testing.T) {
					tr := newDerivedTestTree(t)
					if tc.setup != nil {
						tc.setup(t, tr)
					}
					dir := tr.root
					if tc.dir != nil {
						dir = tc.dir(tr)
					}
					before := derivedSnapshot(t, tr.base)
					run := tr.run(t, dir, full, tc.args...)
					after := derivedSnapshot(t, tr.base)
					if !full && tc.refused != nil {
						assertDerivedRefusal(t, run, tc.refused, before, after)
						if tc.names != nil && !strings.Contains(run.err.Error(), tc.names(tr)) {
							t.Fatalf("error %q does not name %s", run.err, tc.names(tr))
						}
						return
					}
					assertNotDenied(t, run)
					if tc.check != nil {
						tc.check(t, tr, run, full)
					}
				})
			}
		})
	}
}

func expectNoError(t *testing.T, run derivedTestRun) {
	t.Helper()
	if run.err != nil {
		t.Fatalf("run failed: %v (stderr %q)", run.err, run.stderr)
	}
}

// cpBaseSetup writes root/f, root/g, the directory root/d and the outside
// file outside/t.
func cpBaseSetup(t *testing.T, tr derivedTestTree) {
	t.Helper()
	writeTestFile(t, tr.in("f"), "f-content")
	writeTestFile(t, tr.in("g"), "g-content")
	mkdirTest(t, tr.in("d"))
	writeTestFile(t, tr.out("t"), "outside-content")
	mkdirTest(t, tr.out("dir"))
}

// cpTreeSetup adds root/src with the file x and src/sub/y, and the
// directory root/dst.
func cpTreeSetup(t *testing.T, tr derivedTestTree) {
	t.Helper()
	cpBaseSetup(t, tr)
	writeTestFile(t, tr.in("src", "x"), "x-content")
	writeTestFile(t, tr.in("src", "sub", "y"), "y-content")
	mkdirTest(t, tr.in("dst"))
}

func TestUrootDerivedPathsCpDestinations(t *testing.T) {
	t.Parallel()
	runDerivedCases(t, []derivedCase{
		{
			name: "destination entry that is a link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				symlinkTest(t, tr.out("t"), tr.in("d", "f"))
			},
			args: []string{"cp", "f", "d"}, refused: errDerivedTestDenied,
			names: func(tr derivedTestTree) string { return tr.out("t") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.out("t"), "f-content")
			},
		},
		{
			name: "destination entry that is a dangling link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				symlinkTest(t, tr.out("new"), tr.in("d", "f"))
			},
			args: []string{"cp", "f", "d"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.out("new"), "f-content")
			},
		},
		{
			name: "two sources with a destination entry that is a link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				symlinkTest(t, tr.out("t"), tr.in("d", "g"))
			},
			args: []string{"cp", "f", "g", "d"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.out("t"), "g-content")
			},
		},
		{
			name: "destination directory operand that is a link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				symlinkTest(t, tr.out("dir"), tr.in("dl"))
			},
			args: []string{"cp", "f", "dl"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.out("dir", "f"), "f-content")
			},
		},
		{
			name: "destination entry that is a link inside the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				writeTestFile(t, tr.in("t2"), "old")
				symlinkTest(t, tr.in("t2"), tr.in("d", "f"))
			},
			args: []string{"cp", "f", "d"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("t2"), "f-content")
			},
		},
	})
}

func TestUrootDerivedPathsCpRecursive(t *testing.T) {
	t.Parallel()
	var cases []derivedCase
	for _, flags := range [][]string{{"-r"}, {"-P", "-r"}} {
		label := strings.Join(flags, " ")
		args := func(rest ...string) []string { return slices.Concat([]string{"cp"}, flags, rest) }
		cases = append(cases,
			derivedCase{
				name: label + " destination file that is a link out of the root", links: true,
				setup: func(t *testing.T, tr derivedTestTree) {
					t.Helper()
					cpTreeSetup(t, tr)
					mkdirTest(t, tr.in("dst", "src"))
					symlinkTest(t, tr.out("t"), tr.in("dst", "src", "x"))
				},
				args: args("src", "dst"), refused: errDerivedTestDenied,
				check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
					t.Helper()
					expectNoError(t, run)
					assertRegularFile(t, tr.out("t"), "x-content")
				},
			},
			derivedCase{
				name: label + " destination directory that is a link out of the root", links: true,
				setup: func(t *testing.T, tr derivedTestTree) {
					t.Helper()
					cpTreeSetup(t, tr)
					mkdirTest(t, tr.in("dst", "src"))
					symlinkTest(t, tr.out("dir"), tr.in("dst", "src", "sub"))
				},
				args: args("src", "dst"), refused: errDerivedTestDenied,
				check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
					t.Helper()
					expectNoError(t, run)
					assertRegularFile(t, tr.out("dir", "y"), "y-content")
				},
			},
			derivedCase{
				name: label + " top destination that is a link out of the root", links: true,
				setup: func(t *testing.T, tr derivedTestTree) {
					t.Helper()
					cpTreeSetup(t, tr)
					symlinkTest(t, tr.out("dir"), tr.in("dst", "src"))
				},
				args: args("src", "dst"), refused: errDerivedTestDenied,
				check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
					t.Helper()
					expectNoError(t, run)
					assertRegularFile(t, tr.out("dir", "x"), "x-content")
				},
			},
		)
	}
	cases = append(cases,
		derivedCase{
			name: "destination directory that is a link inside the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				mkdirTest(t, tr.in("in"))
				mkdirTest(t, tr.in("dst", "src"))
				symlinkTest(t, tr.in("in"), tr.in("dst", "src", "sub"))
			},
			args: []string{"cp", "-r", "src", "dst"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("in", "y"), "y-content")
			},
		},
	)
	runDerivedCases(t, cases)
}

func TestUrootDerivedPathsCpSourceLinks(t *testing.T) {
	t.Parallel()
	runDerivedCases(t, []derivedCase{
		{
			name: "source link to a file out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				symlinkTest(t, tr.out("t"), tr.in("src", "l"))
			},
			args: []string{"cp", "-r", "src", "dst"}, refused: errDerivedTestDenied,
			names: func(tr derivedTestTree) string { return tr.out("t") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("dst", "src", "l"), "outside-content")
			},
		},
		{
			name: "source link to a directory out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				writeTestFile(t, tr.out("dir", "z"), "z")
				symlinkTest(t, tr.out("dir"), tr.in("src", "l"))
			},
			args: []string{"cp", "-r", "src", "dst"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				info, err := os.Lstat(tr.in("dst", "src", "l"))
				if err != nil || !info.IsDir() {
					t.Fatalf("dst/src/l = %v, %v; want a directory", info, err)
				}
			},
		},
		{
			name: "dangling source link whose target is out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				symlinkTest(t, tr.out("missing"), tr.in("src", "l"))
			},
			args: []string{"cp", "-r", "src", "dst"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, _ derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if run.err == nil {
					t.Fatal("cp of a dangling source link succeeded; want u-root's stat error")
				}
			},
		},
		{
			name: "source link to a file inside the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				writeTestFile(t, tr.in("t2"), "t2-content")
				symlinkTest(t, tr.in("t2"), tr.in("src", "l"))
			},
			args: []string{"cp", "-r", "src", "dst"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("dst", "src", "l"), "t2-content")
			},
		},
		{
			name: "no-dereference source link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				symlinkTest(t, tr.out("t"), tr.in("src", "l"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"}, refused: errCpLinkWithOtherDestinations,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertLinkTo(t, tr.in("dst", "src", "l"), tr.out("t"))
			},
		},
	})
}

func TestUrootDerivedPathsCpSourceMissing(t *testing.T) {
	t.Parallel()
	runDerivedCases(t, []derivedCase{
		{
			name: "later source missing without -r",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
			},
			args: []string{"cp", "f", "missing", "d"}, refused: errCpSourceMissing,
			names: func(tr derivedTestTree) string { return tr.in("missing") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if !errors.Is(run.err, fs.ErrNotExist) {
					t.Fatalf("error = %v, want u-root's not-exist error", run.err)
				}
				assertRegularFile(t, tr.in("d", "f"), "f-content")
			},
		},
		{
			name: "later source that an earlier source creates",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				writeTestFile(t, tr.in("a", "s", "x", "f"), "af")
			},
			args: []string{"cp", "-r", "a/s", "d/s/x", "d"}, refused: errCpSourceMissing,
			names: func(tr derivedTestTree) string { return tr.in("d", "s", "x") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("d", "x", "f"), "af")
			},
		},
		{
			name: "later no-dereference source below a link an earlier source creates", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				writeTestFile(t, tr.out("dir", "f"), "outside-f")
				symlinkTest(t, tr.out("dir"), tr.in("a", "s", "l"))
			},
			args: []string{"cp", "-P", "-r", "a/s", "d/s/l/f", "d"}, refused: errCpSourceMissing,
		},
		{
			name: "missing only source keeps u-root's error",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
			},
			args: []string{"cp", "missing", "d"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if !errors.Is(run.err, fs.ErrNotExist) || !strings.Contains(run.err.Error(), tr.in("missing")) {
					t.Fatalf("error = %v, want u-root's not-exist error naming missing", run.err)
				}
			},
		},
		{
			name: "missing first source keeps u-root's behaviour",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
			},
			args: []string{"cp", "missing", "f", "d"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("d", "f"), "f-content")
			},
		},
	})
}

func TestUrootDerivedPathsCpIntoSource(t *testing.T) {
	t.Parallel()
	dSetup := func(t *testing.T, tr derivedTestTree) {
		t.Helper()
		writeTestFile(t, tr.in("d", "x"), "x-content")
	}
	runDerivedCases(t, []derivedCase{
		{
			name: "directory copied into itself", setup: dSetup,
			args: []string{"cp", "-r", "d", "d"}, refused: errCpIntoSource,
			names: func(tr derivedTestTree) string { return tr.in("d", "d") },
		},
		{
			name: "directory copied to a new path below itself", setup: dSetup,
			args: []string{"cp", "-r", "d", "d/sub"}, refused: errCpIntoSource,
		},
		{
			name: "directory copied into an existing directory below itself",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				dSetup(t, tr)
				mkdirTest(t, tr.in("d", "sub"))
			},
			args: []string{"cp", "-r", "d", "d/sub"}, refused: errCpIntoSource,
		},
		{
			name: "earlier source copied into a later source",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("a", "s", "x"), "ax")
				writeTestFile(t, tr.in("d", "s", "y"), "dy")
			},
			args: []string{"cp", "-r", "a/s", "d/s", "d"}, refused: errCpIntoSource,
			names: func(tr derivedTestTree) string { return tr.in("d", "s") },
		},
		{
			name: "earlier destination that is a later source operand",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("a", "s", "x", "f"), "axf")
				writeTestFile(t, tr.in("d", "s", "x", "g"), "dxg")
			},
			args: []string{"cp", "-r", "a/s", "d/s/x", "d"}, refused: errCpIntoSource,
			names: func(tr derivedTestTree) string { return tr.in("d", "s", "x") },
		},
		{
			name: "copy onto the same entries", setup: dSetup,
			args: []string{"cp", "-r", "d", "."},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("d", "x"), "x-content")
			},
		},
		{
			name: "copy onto the same entries through a link in the tree", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("a", "b", "x"), "x-content")
				symlinkTest(t, "x", tr.in("a", "b", "l"))
			},
			args: []string{"cp", "-r", "a/b", "a"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("a", "b", "x"), "x-content")
				assertLinkTo(t, tr.in("a", "b", "l"), "x")
			},
		},
		{
			name: "destination that is a hard link of a file source",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpBaseSetup(t, tr)
				if err := os.Link(tr.in("f"), tr.in("d", "g")); err != nil {
					t.Fatal(err)
				}
			},
			args: []string{"cp", "-r", "f", "g", "d"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("d", "f"), "f-content")
				assertRegularFile(t, tr.in("d", "g"), "g-content")
			},
		},
		{
			name: "same-named trees merged",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("a", "s", "x"), "ax")
				writeTestFile(t, tr.in("b", "s", "y"), "by")
				mkdirTest(t, tr.in("d"))
			},
			args: []string{"cp", "-r", "a/s", "b/s", "d"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("d", "s", "x"), "ax")
				assertRegularFile(t, tr.in("d", "s", "y"), "by")
			},
		},
	})
}

func TestUrootDerivedPathsCpLinkCreation(t *testing.T) {
	t.Parallel()
	runDerivedCases(t, []derivedCase{
		{
			name: "several sources one of which holds a link", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("a", "s", "x"), "ax")
				symlinkTest(t, tr.in("a", "s", "x"), tr.in("a", "s", "l"))
				writeTestFile(t, tr.in("b", "s", "f"), "bf")
				mkdirTest(t, tr.in("d"))
			},
			args: []string{"cp", "-P", "-r", "a/s", "b/s", "d"}, refused: errCpLinkWithOtherDestinations,
			names: func(tr derivedTestTree) string { return tr.in("d", "s", "l") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertLinkTo(t, tr.in("d", "s", "l"), tr.in("a", "s", "x"))
				assertRegularFile(t, tr.in("d", "s", "f"), "bf")
			},
		},
		{
			name: "one source tree with a link and a file", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("src", "f"), "sf")
				symlinkTest(t, "f", tr.in("src", "l"))
				mkdirTest(t, tr.in("dst"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"}, refused: errCpLinkWithOtherDestinations,
			names: func(tr derivedTestTree) string { return tr.in("dst", "src", "l") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertLinkTo(t, tr.in("dst", "src", "l"), "f")
			},
		},
		{
			name: "one source tree with only a link", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("real"), "real")
				symlinkTest(t, tr.in("real"), tr.in("src", "l"))
				mkdirTest(t, tr.in("dst"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"}, refused: errCpLinkWithOtherDestinations,
			names: func(tr derivedTestTree) string { return tr.in("dst", "src", "l") },
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertLinkTo(t, tr.in("dst", "src", "l"), tr.in("real"))
			},
		},
		{
			name: "link operand without -r", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("real"), "real-content")
				symlinkTest(t, tr.in("real"), tr.in("lnk"))
				mkdirTest(t, tr.in("d"))
			},
			args: []string{"cp", "-P", "lnk", "d"},
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("d", "real"), "real-content")
				assertNotExist(t, tr.in("d", "lnk"))
			},
		},
	})
}

func TestUrootDerivedPathsCpCopyFailures(t *testing.T) {
	t.Parallel()
	runDerivedCases(t, []derivedCase{
		{
			name: "dangling source link inside the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				symlinkTest(t, tr.in("nothing"), tr.in("src", "a"))
			},
			args: []string{"cp", "-r", "src", "dst"},
			check: func(t *testing.T, _ derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if !errors.Is(run.err, fs.ErrNotExist) {
					t.Fatalf("error = %v, want u-root's stat error", run.err)
				}
			},
		},
		{
			name: "entries after a failing entry are still checked", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
				symlinkTest(t, tr.in("nothing"), tr.in("src", "a"))
				writeTestFile(t, tr.in("src", "b"), "b")
				mkdirTest(t, tr.in("dst", "src"))
				symlinkTest(t, tr.out("t"), tr.in("dst", "src", "b"))
			},
			args: []string{"cp", "-r", "src", "dst"}, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if run.err == nil {
					t.Fatal("cp succeeded; want u-root's stat error")
				}
				assertRegularFile(t, tr.out("t"), "outside-content")
			},
		},
		{
			name: "no-dereference junction in the source tree",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				fstree.Require(t, fstree.CapJunction)
				cpTreeSetup(t, tr)
				mkdirTest(t, tr.in("jt"))
				fstree.MustJunction(t, tr.in("jt"), tr.in("src", "j"))
			},
			args: []string{"cp", "-P", "-r", "src", "dst"},
			check: func(t *testing.T, _ derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if run.err == nil {
					t.Fatal("cp -P of a junction succeeded; want u-root's unsupported-mode error")
				}
			},
		},
	})
}

// TestUrootDerivedPathsCpCaseInsensitive runs only on a volume that treats
// names differing in letter case as one entry.
func TestUrootDerivedPathsCpCaseInsensitive(t *testing.T) {
	t.Parallel()
	probe := t.TempDir()
	writeTestFile(t, filepath.Join(probe, "case"), "x")
	if _, err := os.Stat(filepath.Join(probe, "CASE")); err != nil {
		t.Skip("the temporary directory is on a case-sensitive volume")
	}
	runDerivedCases(t, []derivedCase{
		{
			name: "destination that names a source with another letter case",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("b", "x"), "bx")
			},
			args: []string{"cp", "-r", "b", "B"}, refused: errCpIntoSource,
		},
		{
			name: "no-dereference copy into a source named with another letter case",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTestFile(t, tr.in("a", "s", "x"), "ax")
				writeTestFile(t, tr.in("b", "y"), "by")
			},
			args: []string{"cp", "-P", "-r", "a/s", "b", "B"}, refused: errCpIntoSource,
		},
	})
}

func TestUrootDerivedPathsTar(t *testing.T) {
	t.Parallel()
	exSetup := func(t *testing.T, tr derivedTestTree, entries ...tarTestEntry) {
		t.Helper()
		writeTarArchive(t, tr.in("a.tar"), entries...)
		mkdirTest(t, tr.in("ex"))
		writeTestFile(t, tr.out("t"), "outside-content")
		if err := os.Chmod(tr.out("t"), 0o600); err != nil {
			t.Fatal(err)
		}
		mkdirTest(t, tr.out("dir"))
		if err := os.Chmod(tr.out("dir"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"tar", "-x", "-f", "a.tar", "ex"}
	dirLinkCase := func(name string, entries ...tarTestEntry) derivedCase {
		return derivedCase{
			name: name, links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, entries...)
				symlinkTest(t, tr.out("dir"), tr.in("ex", "l"))
			},
			args: args, refused: errDerivedTestDenied,
		}
	}
	runDerivedCases(t, []derivedCase{
		{
			name: "file entry onto a link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, tarFile("t", "entry", 0o777))
				symlinkTest(t, tr.out("t"), tr.in("ex", "t"))
			},
			args: args, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.out("t"), "entry")
			},
		},
		{
			name: "file entry onto a dangling link out of the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, tarFile("t", "entry", 0o777))
				symlinkTest(t, tr.out("new"), tr.in("ex", "t"))
			},
			args: args, refused: errDerivedTestDenied,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.out("new"), "entry")
			},
		},
		dirLinkCase("directory entry onto a directory link out of the root", tarDir("l", 0o777)),
		dirLinkCase("file entry below a directory link out of the root", tarFile("l/f", "f", 0o644)),
		dirLinkCase("nested entries below a directory link out of the root", tarDir("l/a/b", 0o755), tarFile("l/a/b/f", "f", 0o644)),
		{
			name: "entries that u-root skips stay skipped",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, tarFile("/abs", "abs", 0o644), tarFile("../up", "up", 0o644), tarFile("ok", "ok", 0o644))
			},
			args: args,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("ex", "ok"), "ok")
				assertNotExist(t, tr.in("up"))
			},
		},
		{
			name: "link entry stops the extraction",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, tarFile("a", "a", 0o644),
					tarTestEntry{name: "l", typeflag: tar.TypeSymlink, linkname: "a", mode: 0o777},
					tarFile("b", "b", 0o644))
			},
			args: args,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				if run.err == nil {
					t.Fatal("tar -x of a link entry succeeded; want u-root's error")
				}
				assertRegularFile(t, tr.in("ex", "a"), "a")
				assertNotExist(t, tr.in("ex", "b"))
			},
		},
		{
			name: "hard-link entry is an empty regular file",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, tarFile("a", "a", 0o644), tarTestEntry{name: "h", typeflag: tar.TypeLink, linkname: "a", mode: 0o644})
			},
			args: args,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("ex", "h"), "")
			},
		},
		{
			name: "entries below a directory link inside the root", links: true,
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				exSetup(t, tr, tarFile("l/f", "f", 0o644))
				mkdirTest(t, tr.in("in"))
				symlinkTest(t, tr.in("in"), tr.in("ex", "l"))
			},
			args: args,
			check: func(t *testing.T, tr derivedTestTree, run derivedTestRun, _ bool) {
				t.Helper()
				expectNoError(t, run)
				assertRegularFile(t, tr.in("in", "f"), "f")
			},
		},
	})
}

// TestUrootDerivedPathsGated checks that FullAccess turns the derived-path
// checks off and that its zero value runs them.
func TestUrootDerivedPathsGated(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		setup   func(t *testing.T, tr derivedTestTree)
		args    []string
		derived func(tr derivedTestTree) string
	}{
		{
			name: "cp -r",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				cpTreeSetup(t, tr)
			},
			args:    []string{"cp", "-r", "src", "dst"},
			derived: func(tr derivedTestTree) string { return tr.in("dst", "src", "sub", "y") },
		},
		{
			name: "tar -x",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTarArchive(t, tr.in("a.tar"), tarFile("f", "f", 0o644))
				mkdirTest(t, tr.in("ex"))
			},
			args:    []string{"tar", "-x", "-f", "a.tar", "ex"},
			derived: func(tr derivedTestTree) string { return tr.in("ex", "f") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, full := range []bool{true, false} {
				tr := newDerivedTestTree(t)
				tc.setup(t, tr)
				validator := &derivedTestValidator{root: tr.root}
				expectNoError(t, runDerivedWith(t, validator, tr.root, full, tc.args...))
				calls := validator.recorded()
				operands := slices.DeleteFunc(slices.Clone(tc.args[1:]), func(arg string) bool { return strings.HasPrefix(arg, "-") })
				if full {
					slices.Sort(calls)
					slices.Sort(operands)
					if !slices.Equal(calls, operands) {
						t.Fatalf("full access: validator calls %q, want only the operands %q", calls, operands)
					}
					continue
				}
				if !slices.Contains(calls, tc.derived(tr)) {
					t.Fatalf("zero FullAccess: validator calls %q do not include %s", calls, tc.derived(tr))
				}
			}
		})
	}
}

// TestUrootDerivedPathsTarDashArchive checks that tar -x never reads its
// archive from standard input: u-root's own argument preprocessing drops a
// lone "-", so "-f -" leaves tar without its operand, and "--file=-" names
// the file called "-" in the working directory.
func TestUrootDerivedPathsTarDashArchive(t *testing.T) {
	t.Parallel()
	for _, full := range []bool{false, true} {
		tr := newDerivedTestTree(t)
		mkdirTest(t, tr.in("ex"))
		writeTarArchive(t, tr.in("stdin.tar"), tarFile("from-stdin", "stdin", 0o644))
		writeTarArchive(t, tr.in("-"), tarFile("from-dash", "dash", 0o644))
		stdin, err := os.Open(tr.in("stdin.tar"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = stdin.Close() })
		validator := &derivedTestValidator{root: tr.root}
		var stdout, stderr strings.Builder
		hc := &HandlerContext{
			Stdin: stdin, Stdout: &stdout, Stderr: &stderr, Dir: tr.root, LookupEnv: os.LookupEnv,
			ValidatePath: validator.follow, ValidateNoFollowPath: validator.noFollow, FullAccess: full,
		}
		registry := BuildDefaultRegistry()
		ctx := WithHandlerContext(t.Context(), hc)
		if err := registry.Run(ctx, "tar", []string{"tar", "-x", "-f", "-", "ex"}); err == nil {
			t.Fatalf("full=%v: tar -x -f - ex succeeded; want u-root's usage error", full)
		}
		assertNotExist(t, tr.in("ex", "from-stdin"))
		if err := registry.Run(ctx, "tar", []string{"tar", "-x", "--file=-", "ex"}); err != nil {
			t.Fatalf("full=%v: tar -x --file=- ex: %v (%s)", full, err, stderr.String())
		}
		assertRegularFile(t, tr.in("ex", "from-dash"), "dash")
		assertNotExist(t, tr.in("ex", "from-stdin"))
	}
}

// TestUrootDerivedPathsCancelled checks that a cancelled context ends the
// derived-path enumeration with the context's error, named once by the
// command, and that nothing is written.
func TestUrootDerivedPathsCancelled(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, tr derivedTestTree)
		args  []string
	}{
		{name: "cp -r", setup: cpTreeSetup, args: []string{"cp", "-r", "src", "dst"}},
		{
			name: "tar -x",
			setup: func(t *testing.T, tr derivedTestTree) {
				t.Helper()
				writeTarArchive(t, tr.in("a.tar"), tarFile("f", "f", 0o644))
				mkdirTest(t, tr.in("ex"))
			},
			args: []string{"tar", "-x", "-f", "a.tar", "ex"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tr := newDerivedTestTree(t)
			tc.setup(t, tr)
			validator := &derivedTestValidator{root: tr.root}
			var stdout, stderr strings.Builder
			hc := &HandlerContext{
				Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr, Dir: tr.root, LookupEnv: os.LookupEnv,
				ValidatePath: validator.follow, ValidateNoFollowPath: validator.noFollow,
			}
			ctx, cancel := context.WithCancel(WithHandlerContext(t.Context(), hc))
			cancel()
			before := derivedSnapshot(t, tr.base)
			err := BuildDefaultRegistry().Run(ctx, tc.args[0], tc.args)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if n := strings.Count(err.Error(), tc.args[0]+":"); n != 1 {
				t.Fatalf("error %q names %s %d times, want once", err, tc.args[0], n)
			}
			assertSnapshotUnchanged(t, before, derivedSnapshot(t, tr.base))
		})
	}
	t.Run("cp -r walk", func(t *testing.T) {
		t.Parallel()
		tr := newDerivedTestTree(t)
		cpTreeSetup(t, tr)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		visited := 0
		err := cpTreeDerivedPaths(ctx, tr.in("src"), tr.in("dst", "src"), false, func(derivedPath) error {
			visited++
			return nil
		})
		if !errors.Is(err, context.Canceled) || visited != 0 {
			t.Fatalf("cpTreeDerivedPaths = (%v, %d visited), want context.Canceled before any visit", err, visited)
		}
	})
}
