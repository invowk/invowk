// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/internal/uroot"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// These tests cover the operations that act on their last component (design
// D1-D3, D6 of virtual-path-no-follow-operands) through the real ShRuntime
// and LuaRuntime, under both access modes where the design lists both.

var virtualPathBothAccessModes = []invowkfile.VirtualFilesystemAccess{
	invowkfile.VirtualFilesystemAccessRestricted,
	invowkfile.VirtualFilesystemAccessFull,
}

// linkTestTree creates, in a fresh canonical directory: target.txt, live (a
// link to target.txt), stale (a link to a missing file), tdir/file.txt,
// dirlink (a link to tdir) and an empty dir.
func linkTestTree(t *testing.T) string {
	t.Helper()
	fstree.Require(t, fstree.CapSymlink)
	work := mustMkdir(t, filepath.Join(fstree.TempRoot(t), "work"))
	writeTestFile(t, filepath.Join(work, "target.txt"), "target-content\n")
	mustMkdir(t, filepath.Join(work, "tdir"))
	writeTestFile(t, filepath.Join(work, "tdir", "file.txt"), "file")
	mustMkdir(t, filepath.Join(work, "dir"))
	fstree.MustSymlink(t, "target.txt", filepath.Join(work, "live"))
	fstree.MustSymlink(t, "missing.txt", filepath.Join(work, "stale"))
	fstree.MustSymlink(t, "tdir", filepath.Join(work, "dirlink"))
	return work
}

func assertIsLink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Errorf("%s: %v", path, err)
		return
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("%s is %v, want a symbolic link", path, info.Mode())
	}
}

func TestVirtualPathNoFollowLinkOperations(t *testing.T) {
	t.Parallel()
	for _, access := range virtualPathBothAccessModes {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			t.Run("rm live keeps the target", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				if run := runVirtualShTest(t, work, "rm live", access); run.exit != 0 {
					t.Fatalf("rm live = %+v", run)
				}
				assertPathMissing(t, filepath.Join(work, "live"))
				assertPathExists(t, filepath.Join(work, "target.txt"))
			})
			t.Run("rm -f stale", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				if run := runVirtualShTest(t, work, "rm -f stale", access); run.exit != 0 {
					t.Fatalf("rm -f stale = %+v", run)
				}
				assertPathMissing(t, filepath.Join(work, "stale"))
			})
			for _, script := range []string{"rm -r dirlink", "rm -rf dirlink"} {
				t.Run(script, func(t *testing.T) {
					t.Parallel()
					work := linkTestTree(t)
					if run := runVirtualShTest(t, work, script, access); run.exit != 0 {
						t.Fatalf("%s = %+v", script, run)
					}
					assertPathMissing(t, filepath.Join(work, "dirlink"))
					assertPathExists(t, filepath.Join(work, "tdir", "file.txt"))
				})
			}
			t.Run("ln -sf replaces a dangling link", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				run := runVirtualShTest(t, work, "ln -sf target.txt stale && cat stale", access)
				if run.exit != 0 || run.stdout != "target-content\n" {
					t.Fatalf("ln -sf target.txt stale = %+v", run)
				}
				assertIsLink(t, filepath.Join(work, "stale"))
				assertPathMissing(t, filepath.Join(work, "missing.txt"))
			})
			t.Run("mv live dir moves the link", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				if run := runVirtualShTest(t, work, "mv live dir", access); run.exit != 0 {
					t.Fatalf("mv live dir = %+v", run)
				}
				assertIsLink(t, filepath.Join(work, "dir", "live"))
				assertPathExists(t, filepath.Join(work, "target.txt"))
				assertPathMissing(t, filepath.Join(work, "dir", "target.txt"))
			})
			t.Run("link file tests and cat", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				script := strings.Join([]string{
					`[ -L live ] && echo L-live`,
					`[ -h live ] && echo h-live`,
					`test -L live && echo test-L-live`,
					`[ -L target.txt ] || echo not-L-target`,
					`cat live`,
				}, "\n")
				run := runVirtualShTest(t, work, script, access)
				want := "L-live\nh-live\ntest-L-live\nnot-L-target\ntarget-content\n"
				if run.exit != 0 || run.stdout != want {
					t.Fatalf("link tests = %+v, want stdout %q", run, want)
				}
			})
		})
	}
}

func TestVirtualPathNoFollowRestricted(t *testing.T) {
	t.Parallel()
	restricted := invowkfile.VirtualFilesystemAccessRestricted
	outside := outsideVirtualRootsDir(t)
	outFile := filepath.Join(outside, "outfile")
	writeTestFile(t, outFile, "outside-content")
	writeTestFile(t, filepath.Join(outside, "name"), "name")

	t.Run("rm outlink keeps the outside file", func(t *testing.T) {
		t.Parallel()
		work := linkTestTree(t)
		fstree.MustSymlink(t, outFile, filepath.Join(work, "outlink"))
		if run := runVirtualShTest(t, work, "rm outlink", restricted); run.exit != 0 {
			t.Fatalf("rm outlink = %+v", run)
		}
		assertPathMissing(t, filepath.Join(work, "outlink"))
		assertPathExists(t, outFile)
	})
	t.Run("cat outlink is denied", func(t *testing.T) {
		t.Parallel()
		work := linkTestTree(t)
		fstree.MustSymlink(t, outFile, filepath.Join(work, "outlink"))
		run := runVirtualShTest(t, work, "cat outlink", restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, "virtual path denied") || strings.Contains(run.stdout, "outside-content") {
			t.Fatalf("cat outlink = %+v, want the denial", run)
		}
	})
	t.Run("rm below an escaping parent is denied", func(t *testing.T) {
		t.Parallel()
		work := linkTestTree(t)
		fstree.MustSymlink(t, outside, filepath.Join(work, "esc"))
		run := runVirtualShTest(t, work, "rm esc/name", restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, "virtual path denied") {
			t.Fatalf("rm esc/name = %+v, want the denial", run)
		}
		assertPathExists(t, filepath.Join(outside, "name"))
	})
	t.Run("rm cyclelink removes the link", func(t *testing.T) {
		t.Parallel()
		work := linkTestTree(t)
		fstree.MustSymlink(t, "cyclelink", filepath.Join(work, "cyclelink"))
		if run := runVirtualShTest(t, work, "rm cyclelink", restricted); run.exit != 0 {
			t.Fatalf("rm cyclelink = %+v", run)
		}
		assertPathMissing(t, filepath.Join(work, "cyclelink"))
		assertPathExists(t, filepath.Join(work, "target.txt"))
	})
}

// TestVirtualPathNoFollowDarwinTmp: on macOS /tmp is a link to the allowed
// root /private/tmp. The no-follow form of /tmp is /tmp itself, outside that
// root, so `rm -r /tmp` is denied; /tmp/<f> keeps its parent resolved, so
// `rm /tmp/<f>` is allowed. The denial is checked on the validator and the
// u-root rm is run only on the file this test created, so a regression never
// removes anything else below /private/tmp.
func TestVirtualPathNoFollowDarwinTmp(t *testing.T) {
	t.Parallel()
	if goruntime.GOOS != "darwin" {
		t.Skip("/tmp is a link to /private/tmp on macOS only")
	}
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, "/tmp")
	if got, err := validator.validateNoFollow("/", "/tmp"); !errors.Is(err, errVirtualPathDenied) {
		t.Fatalf("validateNoFollow(/tmp) = (%q, %v), want the denial", got, err)
	}
	file, err := os.CreateTemp("/tmp", "invowk-nofollow-*")
	if err != nil {
		t.Fatalf("CreateTemp(/tmp): %v", err)
	}
	_ = file.Close()
	name := "/tmp/" + filepath.Base(file.Name())
	t.Cleanup(func() { _ = os.Remove(name) })
	handler := &uroot.HandlerContext{
		Stdout:               &strings.Builder{},
		Stderr:               &strings.Builder{},
		Dir:                  "/tmp",
		ValidatePath:         validator.validate,
		ValidateNoFollowPath: validator.validateNoFollow,
	}
	if err := uroot.BuildDefaultRegistry().Run(uroot.WithHandlerContext(t.Context(), handler), "rm", []string{"rm", name}); err != nil {
		t.Fatalf("rm %s: %v", name, err)
	}
	assertPathMissing(t, "/private/tmp/"+filepath.Base(name))
}

func TestVirtualPathNoFollowTrailingSeparator(t *testing.T) {
	t.Parallel()
	for _, access := range virtualPathBothAccessModes {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			for _, script := range []string{
				"rm live/", "rm -r live/", "rm -rf live/", "rm -f live/", "rm -r live/.",
				"mv live/ x", "ln -sf t live/",
			} {
				t.Run(script, func(t *testing.T) {
					t.Parallel()
					work := linkTestTree(t)
					target, err := filepath.EvalSymlinks(filepath.Join(work, "target.txt"))
					if err != nil {
						t.Fatal(err)
					}
					run := runVirtualShTest(t, work, script, access)
					if run.exit == 0 || !strings.Contains(run.stderr, "not a directory") || !strings.Contains(run.stderr, target) {
						t.Fatalf("%s = %+v, want \"not a directory\" naming %s", script, run, target)
					}
					assertPathExists(t, filepath.Join(work, "target.txt"))
					assertIsLink(t, filepath.Join(work, "live"))
					assertPathMissing(t, filepath.Join(work, "x"))
				})
			}
			t.Run("rm -r dirlink/", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				if run := runVirtualShTest(t, work, "rm -r dirlink/", access); run.exit != 0 {
					t.Fatalf("rm -r dirlink/ = %+v", run)
				}
				assertPathMissing(t, filepath.Join(work, "tdir"))
				assertIsLink(t, filepath.Join(work, "dirlink"))
			})
			t.Run("[ -L live/ ] tests the cleaned operand", func(t *testing.T) {
				t.Parallel()
				work := linkTestTree(t)
				// The recorded mvdan limitation: the shell cleans the operand
				// before the stat handler sees it (design Risks).
				run := runVirtualShTest(t, work, "[ -L live/ ] && echo is-link", access)
				if run.stdout != "is-link\n" {
					t.Fatalf("[ -L live/ ] = %+v, want true", run)
				}
			})
		})
	}
}

func TestVirtualPathNoFollowValidatorDirectoryCheck(t *testing.T) {
	t.Parallel()
	work := linkTestTree(t)
	validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, work)
	target := filepath.Join(work, "target.txt")
	_, err := validator.validateNoFollow(work, "live"+string(filepath.Separator))
	// The text is "not a directory" on every platform: on Windows
	// syscall.ENOTDIR's own text is "The system cannot find the path
	// specified.".
	if !errors.Is(err, syscall.ENOTDIR) || err.Error() != target+": not a directory" {
		t.Errorf("validateNoFollow(live/) error = %v, want ENOTDIR as %q", err, target+": not a directory")
	}
	if got, err := validator.validateNoFollow(work, "dirlink"+string(filepath.Separator)); err != nil || got != filepath.Join(work, "tdir")+string(filepath.Separator) {
		t.Errorf("validateNoFollow(dirlink/) = (%q, %v), want %q", got, err, filepath.Join(work, "tdir")+string(filepath.Separator))
	}
	if got, err := validator.validateNoFollow(work, "live"); err != nil || got != filepath.Join(work, "live") {
		t.Errorf("validateNoFollow(live) = (%q, %v), want the link's own path", got, err)
	}
	if got, err := validator.validateNoFollow(work, "missing"+string(filepath.Separator)); err != nil || got != filepath.Join(work, "missing")+string(filepath.Separator) {
		t.Errorf("validateNoFollow(missing/) = (%q, %v), want the path with its separator", got, err)
	}
}

func TestVirtualPathNoFollowLua(t *testing.T) {
	t.Parallel()
	for _, access := range virtualPathBothAccessModes {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			work := linkTestTree(t)
			fstree.MustSymlink(t, "target.txt", filepath.Join(work, "live2"))
			script := fmt.Sprintf(`
print("rm=" .. tostring(invowk.cmd.rm("live")))
print("mv=" .. tostring(invowk.cmd.mv("live2", "dir")))
print("ln=" .. tostring(invowk.cmd.ln("-sf", "target.txt", "stale")))
local out, err, code = invowk.capture.cat("stale")
print("cat=" .. out .. tostring(code))
print(%q)
`, "done")
			run := runVirtualLuaTest(t, work, script, access)
			want := "rm=0\nmv=0\nln=0\ncat=target-content\n0\ndone\n"
			if run.stdout != want {
				t.Fatalf("lua = %+v, want stdout %q", run, want)
			}
			assertPathMissing(t, filepath.Join(work, "live"))
			assertPathExists(t, filepath.Join(work, "target.txt"))
			assertIsLink(t, filepath.Join(work, "dir", "live2"))
			assertIsLink(t, filepath.Join(work, "stale"))
			assertPathMissing(t, filepath.Join(work, "missing.txt"))
		})
	}
}
