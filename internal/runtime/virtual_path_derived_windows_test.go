// SPDX-License-Identifier: MPL-2.0

//go:build windows

package runtime

import (
	"archive/tar"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestVirtualPathDerivedPathsWindowsJunction runs the directory-link rows of
// the cp and tar -x derived-path checks with directory junctions: refused
// under restricted access when the junction's target is outside the allowed
// roots, with nothing changed there, and allowed when it is inside them.
func TestVirtualPathDerivedPathsWindowsJunction(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)
	restricted := invowkfile.VirtualFilesystemAccessRestricted
	tarSetup := func(t *testing.T, work string, entries ...derivedTarEntry) {
		t.Helper()
		derivedTarArchive(t, filepath.Join(work, "a.tar"), entries...)
		mustMkdir(t, filepath.Join(work, "ex"))
	}
	file := func(name string) derivedTarEntry {
		return derivedTarEntry{name: name, typeflag: tar.TypeReg, mode: 0o644, body: "entry"}
	}
	dir := func(name string) derivedTarEntry {
		return derivedTarEntry{name: name, typeflag: tar.TypeDir, mode: 0o777}
	}
	for _, tc := range []struct {
		name     string
		setup    func(t *testing.T, work string)
		junction string
		script   string
		inRoot   string
	}{
		{
			name: "cp recursive destination below a junction",
			setup: func(t *testing.T, work string) {
				t.Helper()
				derivedCpSetup(t, work, t.TempDir())
				mustMkdir(t, filepath.Join(work, "dst", "src"))
			},
			junction: "dst/src/sub", script: "cp -r src dst", inRoot: "y",
		},
		{
			name: "cp recursive destination that is a junction",
			setup: func(t *testing.T, work string) {
				t.Helper()
				derivedCpSetup(t, work, t.TempDir())
			},
			junction: "dst/src", script: "cp -r src dst", inRoot: "x",
		},
		{
			name:     "tar entry onto a junction",
			setup:    func(t *testing.T, work string) { t.Helper(); tarSetup(t, work, dir("l")) },
			junction: "ex/l", script: "tar -x -f a.tar ex",
		},
		{
			name:     "tar entry below a junction",
			setup:    func(t *testing.T, work string) { t.Helper(); tarSetup(t, work, file("l/f")) },
			junction: "ex/l", script: "tar -x -f a.tar ex", inRoot: "f",
		},
		{
			name:     "tar entries nested below a junction",
			setup:    func(t *testing.T, work string) { t.Helper(); tarSetup(t, work, dir("l/a/b"), file("l/a/b/f")) },
			junction: "ex/l", script: "tar -x -f a.tar ex", inRoot: "a/b/f",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			t.Run("target outside the roots is refused", func(t *testing.T) {
				work := harnessPhysicalDir(t, t.TempDir())
				outside := harnessPhysicalDir(t, outsideVirtualRootsDir(t))
				tc.setup(t, work)
				fstree.MustJunction(t, outside, filepath.Join(work, filepath.FromSlash(tc.junction)))
				before := derivedTreeSnapshot(t, outside)
				run := runVirtualShTest(t, work, tc.script, restricted)
				if run.exit == 0 || !strings.Contains(run.stderr, derivedDeniedText) {
					t.Fatalf("run = (exit %d, stderr %q), want the denial", run.exit, run.stderr)
				}
				assertDerivedTreeUnchanged(t, before, derivedTreeSnapshot(t, outside))
			})
			t.Run("target inside the roots is allowed", func(t *testing.T) {
				work := harnessPhysicalDir(t, t.TempDir())
				tc.setup(t, work)
				target := filepath.Join(work, "in")
				mustMkdir(t, target)
				fstree.MustJunction(t, target, filepath.Join(work, filepath.FromSlash(tc.junction)))
				run := runVirtualShTest(t, work, tc.script, restricted)
				derivedAssertSuccess(t, run)
				if tc.inRoot != "" {
					derivedAssertFile(t, filepath.Join(target, filepath.FromSlash(tc.inRoot)), map[string]string{
						"x": "x-content", "y": "y-content", "f": "entry", "a/b/f": "entry",
					}[tc.inRoot])
				}
			})
		})
	}
}

// TestVirtualPathDerivedPathsWindowsSourceJunction verifies the ModeIrregular
// branch of cpEntryDerivedPaths when a cp -r source tree contains a Windows
// directory junction without -P: refused under restricted access when its
// target lies outside allowed roots (leaving dst and outside untouched), and
// allowed when its target lies inside allowed roots.
func TestVirtualPathDerivedPathsWindowsSourceJunction(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)
	restricted := invowkfile.VirtualFilesystemAccessRestricted

	t.Run("target outside the roots is refused", func(t *testing.T) {
		t.Parallel()
		work := harnessPhysicalDir(t, t.TempDir())
		outside := harnessPhysicalDir(t, outsideVirtualRootsDir(t))
		derivedCpSetup(t, work, t.TempDir())
		derivedWrite(t, filepath.Join(outside, "secret.txt"), "secret-content")
		fstree.MustJunction(t, outside, filepath.Join(work, "src", "j"))

		before := derivedTreeSnapshot(t, outside)
		dstBefore := derivedTreeSnapshot(t, filepath.Join(work, "dst"))
		run := runVirtualShTest(t, work, "cp -r src dst", restricted)
		if run.exit == 0 || !strings.Contains(run.stderr, derivedDeniedText) {
			t.Fatalf("run = (exit %d, stderr %q), want the denial", run.exit, run.stderr)
		}
		assertDerivedTreeUnchanged(t, before, derivedTreeSnapshot(t, outside))
		assertDerivedTreeUnchanged(t, dstBefore, derivedTreeSnapshot(t, filepath.Join(work, "dst")))
	})

	t.Run("target inside the roots is allowed", func(t *testing.T) {
		t.Parallel()
		work := harnessPhysicalDir(t, t.TempDir())
		derivedCpSetup(t, work, t.TempDir())
		target := filepath.Join(work, "in")
		derivedWrite(t, filepath.Join(target, "f.txt"), "in-content")
		fstree.MustJunction(t, target, filepath.Join(work, "src", "j"))

		run := runVirtualShTest(t, work, "cp -r src dst", restricted)
		derivedAssertSuccess(t, run)
		derivedAssertFile(t, filepath.Join(work, "dst", "src", "x"), "x-content")
	})
}
