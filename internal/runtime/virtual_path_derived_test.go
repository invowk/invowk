// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const derivedDeniedText = "virtual path denied"

type (
	// derivedRuntimeCase is one virtual-sh script, run under restricted and
	// under full access, each on fresh work and outside trees. work is an
	// allowed root (a temporary directory); outside comes from
	// outsideVirtualRootsDir.
	derivedRuntimeCase struct {
		name  string
		setup func(t *testing.T, work, outside string)
		// script runs in work.
		script string
		// refused: the restricted run fails with "virtual path denied" and
		// neither tree changes.
		refused bool
		// names is a path, relative to outside, a refusal must name.
		names string
		// check runs on each run that is not refused.
		check func(t *testing.T, work, outside string, run virtualShTestRun, full bool)
	}

	// derivedTarEntry is one entry of a test archive.
	derivedTarEntry struct {
		name     string
		typeflag byte
		mode     int64
		body     string
	}
)

func runDerivedRuntimeCases(t *testing.T, cases []derivedRuntimeCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, access := range []invowkfile.VirtualFilesystemAccess{invowkfile.VirtualFilesystemAccessRestricted, invowkfile.VirtualFilesystemAccessFull} {
				t.Run(string(access), func(t *testing.T) {
					work := harnessPhysicalDir(t, t.TempDir())
					outside := harnessPhysicalDir(t, outsideVirtualRootsDir(t))
					tc.setup(t, work, outside)
					beforeWork, beforeOutside := derivedTreeSnapshot(t, work), derivedTreeSnapshot(t, outside)
					run := runVirtualShTest(t, work, tc.script, access)
					full := access == invowkfile.VirtualFilesystemAccessFull
					if !full && tc.refused {
						if run.exit == 0 || !strings.Contains(run.stderr, derivedDeniedText) {
							t.Fatalf("run = (exit %d, stdout %q, stderr %q), want the denial", run.exit, run.stdout, run.stderr)
						}
						if tc.names != "" && !strings.Contains(run.stderr, filepath.Join(outside, tc.names)) {
							t.Fatalf("stderr %q does not name %s", run.stderr, filepath.Join(outside, tc.names))
						}
						assertDerivedTreeUnchanged(t, beforeWork, derivedTreeSnapshot(t, work))
						assertDerivedTreeUnchanged(t, beforeOutside, derivedTreeSnapshot(t, outside))
						return
					}
					if strings.Contains(run.stderr, derivedDeniedText) {
						t.Fatalf("unexpected refusal: %q", run.stderr)
					}
					if tc.check != nil {
						tc.check(t, work, outside, run, full)
					}
				})
			}
		})
	}
}

// derivedTreeSnapshot lists every entry below dir with its type and
// permissions, and a regular file's content or a link's target.
func derivedTreeSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	type walked struct {
		path string
		info fs.FileInfo
	}
	var all []walked
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("info %s: %w", path, err)
		}
		all = append(all, walked{path: path, info: info})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]string, len(all))
	for _, entry := range all {
		value := entry.info.Mode().String()
		switch {
		case entry.info.Mode().IsRegular():
			content, err := os.ReadFile(entry.path)
			if err != nil {
				t.Fatal(err)
			}
			value += ":" + string(content)
		case entry.info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(entry.path)
			if err != nil {
				t.Fatal(err)
			}
			value += "->" + target
		}
		entries[entry.path] = value
	}
	return entries
}

func assertDerivedTreeUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	for _, path := range slices.Sorted(maps.Keys(after)) {
		if before[path] != after[path] {
			t.Errorf("changed: %s: %q -> %q", path, before[path], after[path])
		}
	}
	for _, path := range slices.Sorted(maps.Keys(before)) {
		if _, ok := after[path]; !ok {
			t.Errorf("removed: %s", path)
		}
	}
}

func derivedWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func derivedLink(t *testing.T, target, link string) {
	t.Helper()
	fstree.Require(t, fstree.CapSymlink)
	mustMkdir(t, filepath.Dir(link))
	fstree.MustSymlink(t, target, link)
}

func derivedAssertFile(t *testing.T, path, content string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("%s is %s, want a regular file", path, info.Mode())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("%s holds %q, want %q", path, got, content)
	}
}

func derivedAssertSuccess(t *testing.T, run virtualShTestRun) {
	t.Helper()
	if run.exit != 0 {
		t.Fatalf("run = (exit %d, stdout %q, stderr %q), want success", run.exit, run.stdout, run.stderr)
	}
}

func derivedTarArchive(t *testing.T, path string, entries ...derivedTarEntry) {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range entries {
		hdr := &tar.Header{Name: entry.name, Typeflag: entry.typeflag, Mode: entry.mode, Size: int64(len(entry.body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entry.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	derivedWrite(t, path, buf.String())
}

// derivedCpSetup writes work/f, work/g, work/src/x, work/src/sub/y and the
// directories work/d and work/dst, and outside/t and outside/dir.
func derivedCpSetup(t *testing.T, work, outside string) {
	t.Helper()
	derivedWrite(t, filepath.Join(work, "f"), "f-content")
	derivedWrite(t, filepath.Join(work, "g"), "g-content")
	derivedWrite(t, filepath.Join(work, "src", "x"), "x-content")
	derivedWrite(t, filepath.Join(work, "src", "sub", "y"), "y-content")
	mustMkdir(t, filepath.Join(work, "d"))
	mustMkdir(t, filepath.Join(work, "dst"))
	derivedWrite(t, filepath.Join(outside, "t"), "outside-content")
	mustMkdir(t, filepath.Join(outside, "dir"))
}

func TestVirtualPathDerivedPathsCp(t *testing.T) {
	t.Parallel()
	linkIn := func(rel, outsideRel string) func(t *testing.T, work, outside string) {
		return func(t *testing.T, work, outside string) {
			t.Helper()
			derivedCpSetup(t, work, outside)
			derivedLink(t, filepath.Join(outside, outsideRel), filepath.Join(work, filepath.FromSlash(rel)))
		}
	}
	outsideFile := func(rel, content string) func(t *testing.T, work, outside string, run virtualShTestRun, full bool) {
		return func(t *testing.T, _, outside string, run virtualShTestRun, _ bool) {
			t.Helper()
			derivedAssertSuccess(t, run)
			derivedAssertFile(t, filepath.Join(outside, filepath.FromSlash(rel)), content)
		}
	}
	runDerivedRuntimeCases(t, []derivedRuntimeCase{
		{
			name: "destination entry that is a link", setup: linkIn("d/f", "t"), script: "cp f d",
			refused: true, names: "t", check: outsideFile("t", "f-content"),
		},
		{
			name: "destination entry that is a dangling link", setup: linkIn("d/f", "new"), script: "cp f d",
			refused: true, check: outsideFile("new", "f-content"),
		},
		{
			name: "two sources with a destination entry that is a link", setup: linkIn("d/g", "t"), script: "cp f g d",
			refused: true, check: outsideFile("t", "g-content"),
		},
		{
			name: "recursive destination file that is a link", setup: linkIn("dst/src/x", "t"),
			script: "cp -r src dst", refused: true, check: outsideFile("t", "x-content"),
		},
		{
			name: "no-dereference recursive destination file that is a link", setup: linkIn("dst/src/x", "t"),
			script: "cp -P -r src dst", refused: true, check: outsideFile("t", "x-content"),
		},
		{
			name: "recursive destination directory that is a link", setup: linkIn("dst/src/sub", "dir"),
			script: "cp -r src dst", refused: true, check: outsideFile("dir/y", "y-content"),
		},
		{
			name: "recursive top destination that is a link", setup: linkIn("dst/src", "dir"),
			script: "cp -r src dst", refused: true, check: outsideFile("dir/x", "x-content"),
		},
		{
			name: "source link to a file outside", setup: linkIn("src/l", "t"), script: "cp -r src dst", refused: true,
			check: func(t *testing.T, work, _ string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				derivedAssertFile(t, filepath.Join(work, "dst", "src", "l"), "outside-content")
			},
		},
		{
			name: "source link to a directory outside", setup: linkIn("src/l", "dir"), script: "cp -r src dst", refused: true,
			check: func(t *testing.T, _, _ string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
			},
		},
		{
			name: "no-dereference source link copied as a link under full access", setup: linkIn("src/l", "t"),
			script: "cp -P -r src dst", refused: true,
			check: func(t *testing.T, work, outside string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				target, err := os.Readlink(filepath.Join(work, "dst", "src", "l"))
				if err != nil || target != filepath.Join(outside, "t") {
					t.Fatalf("dst/src/l -> %q, %v; want a link to %s", target, err, filepath.Join(outside, "t"))
				}
			},
		},
		{
			name: "links inside the roots keep working",
			setup: func(t *testing.T, work, outside string) {
				t.Helper()
				derivedCpSetup(t, work, outside)
				derivedWrite(t, filepath.Join(work, "t2"), "old")
				derivedLink(t, filepath.Join(work, "t2"), filepath.Join(work, "d", "f"))
				mustMkdir(t, filepath.Join(work, "in"))
				mustMkdir(t, filepath.Join(work, "dst", "src"))
				derivedLink(t, filepath.Join(work, "in"), filepath.Join(work, "dst", "src", "sub"))
				derivedWrite(t, filepath.Join(work, "t3"), "t3-content")
				derivedLink(t, filepath.Join(work, "t3"), filepath.Join(work, "src", "l"))
			},
			script: "cp f d && cp -r src dst",
			check: func(t *testing.T, work, _ string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				derivedAssertFile(t, filepath.Join(work, "t2"), "f-content")
				derivedAssertFile(t, filepath.Join(work, "in", "y"), "y-content")
				derivedAssertFile(t, filepath.Join(work, "dst", "src", "l"), "t3-content")
			},
		},
	})
}

func TestVirtualPathDerivedPathsCpPrediction(t *testing.T) {
	t.Parallel()
	tree := func(files map[string]string, dirs ...string) func(t *testing.T, work, outside string) {
		return func(t *testing.T, work, _ string) {
			t.Helper()
			for name, content := range files {
				derivedWrite(t, filepath.Join(work, filepath.FromSlash(name)), content)
			}
			for _, dir := range dirs {
				mustMkdir(t, filepath.Join(work, filepath.FromSlash(dir)))
			}
		}
	}
	succeeds := func(rel, content string) func(t *testing.T, work, outside string, run virtualShTestRun, full bool) {
		return func(t *testing.T, work, _ string, run virtualShTestRun, _ bool) {
			t.Helper()
			derivedAssertSuccess(t, run)
			derivedAssertFile(t, filepath.Join(work, filepath.FromSlash(rel)), content)
		}
	}
	runDerivedRuntimeCases(t, []derivedRuntimeCase{
		{
			name: "later source that does not exist", setup: tree(map[string]string{"a/s/x/f": "af"}, "d"),
			script: "cp -r a/s d/s/x d", refused: true, check: succeeds("d/x/f", "af"),
		},
		{
			name: "missing first source keeps cp's behaviour", setup: tree(map[string]string{"f": "f"}, "d"),
			script: "cp missing f d", check: succeeds("d/f", "f"),
		},
		{
			name: "directory copied into itself", setup: tree(map[string]string{"d/x": "x"}),
			script: "cp -r d d", refused: true,
		},
		{
			name: "earlier source copied into a later source", setup: tree(map[string]string{"a/s/x": "ax", "d/s/x": "dx"}),
			script: "cp -r a/s d/s d", refused: true,
		},
		{
			name: "copy onto the same entries", setup: tree(map[string]string{"d/x": "x"}),
			script: "cp -r d .", check: succeeds("d/x", "x"),
		},
		{
			name: "same-named trees merged", setup: tree(map[string]string{"a/s/x": "ax", "b/s/y": "by"}, "d"),
			script: "cp -r a/s b/s d && cat d/s/x", check: succeeds("d/s/y", "by"),
		},
		{
			name: "no-dereference copy creating a link next to other destinations",
			setup: func(t *testing.T, work, outside string) {
				t.Helper()
				tree(map[string]string{"a/s/x": "ax", "b/s/f": "bf"}, "d")(t, work, outside)
				derivedLink(t, "x", filepath.Join(work, "a", "s", "l"))
			},
			script: "cp -P -r a/s b/s d", refused: true, check: succeeds("d/s/f", "bf"),
		},
		{
			name: "no-dereference copy of one tree that holds a link",
			setup: func(t *testing.T, work, outside string) {
				t.Helper()
				tree(map[string]string{"src/f": "sf"}, "dst")(t, work, outside)
				derivedLink(t, "f", filepath.Join(work, "src", "l"))
			},
			script: "cp -P -r src dst", refused: true, check: succeeds("dst/src/f", "sf"),
		},
		{
			name: "no-dereference link operand copies its target",
			setup: func(t *testing.T, work, outside string) {
				t.Helper()
				tree(map[string]string{"real": "real-content"}, "d")(t, work, outside)
				derivedLink(t, filepath.Join(work, "real"), filepath.Join(work, "lnk"))
			},
			script: "cp -P lnk d", check: succeeds("d/real", "real-content"),
		},
	})
}

func TestVirtualPathDerivedPathsTar(t *testing.T) {
	t.Parallel()
	archive := func(entries ...derivedTarEntry) func(t *testing.T, work, outside string) {
		return func(t *testing.T, work, outside string) {
			t.Helper()
			derivedTarArchive(t, filepath.Join(work, "a.tar"), entries...)
			mustMkdir(t, filepath.Join(work, "ex"))
			derivedWrite(t, filepath.Join(outside, "t"), "outside-content")
			mustMkdir(t, filepath.Join(outside, "dir"))
		}
	}
	withLink := func(setup func(t *testing.T, work, outside string), rel, outsideRel string) func(t *testing.T, work, outside string) {
		return func(t *testing.T, work, outside string) {
			t.Helper()
			setup(t, work, outside)
			derivedLink(t, filepath.Join(outside, outsideRel), filepath.Join(work, filepath.FromSlash(rel)))
		}
	}
	file := func(name, body string, mode int64) derivedTarEntry {
		return derivedTarEntry{name: name, typeflag: tar.TypeReg, mode: mode, body: body}
	}
	dir := func(name string, mode int64) derivedTarEntry {
		return derivedTarEntry{name: name, typeflag: tar.TypeDir, mode: mode}
	}
	script := "tar -x -f a.tar ex"
	runDerivedRuntimeCases(t, []derivedRuntimeCase{
		{
			name: "file entry onto a link", setup: withLink(archive(file("t", "entry", 0o777)), "ex/t", "t"),
			script: script, refused: true, names: "t",
			check: func(t *testing.T, _, outside string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				derivedAssertFile(t, filepath.Join(outside, "t"), "entry")
			},
		},
		{
			name: "file entry onto a dangling link", setup: withLink(archive(file("t", "entry", 0o644)), "ex/t", "new"),
			script: script, refused: true,
		},
		{
			name: "directory entry onto a directory link", setup: withLink(archive(dir("l", 0o777)), "ex/l", "dir"),
			script: script, refused: true,
		},
		{
			name: "file entry below a directory link", setup: withLink(archive(file("l/f", "f", 0o644)), "ex/l", "dir"),
			script: script, refused: true,
			check: func(t *testing.T, _, outside string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				derivedAssertFile(t, filepath.Join(outside, "dir", "f"), "f")
			},
		},
		{
			name:   "nested entries below a directory link",
			setup:  withLink(archive(dir("l/a/b", 0o755), file("l/a/b/f", "f", 0o644)), "ex/l", "dir"),
			script: script, refused: true,
		},
		{
			name: "entries u-root skips stay skipped", setup: archive(file("../up", "up", 0o644), file("ok", "ok", 0o644)),
			script: script,
			check: func(t *testing.T, work, _ string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				derivedAssertFile(t, filepath.Join(work, "ex", "ok"), "ok")
				assertPathMissing(t, filepath.Join(work, "up"))
			},
		},
		{
			name: "entries below a directory link inside the roots",
			setup: func(t *testing.T, work, outside string) {
				t.Helper()
				archive(file("l/f", "f", 0o644))(t, work, outside)
				mustMkdir(t, filepath.Join(work, "in"))
				derivedLink(t, filepath.Join(work, "in"), filepath.Join(work, "ex", "l"))
			},
			script: script,
			check: func(t *testing.T, work, _ string, run virtualShTestRun, _ bool) {
				t.Helper()
				derivedAssertSuccess(t, run)
				derivedAssertFile(t, filepath.Join(work, "in", "f"), "f")
			},
		},
	})
}

// TestVirtualPathDerivedPathsLua checks that virtual-lua reaches cp and
// tar -x through the same checks: invowk.capture reports the refusal with a
// non-zero code (it returns the command's own output, not the Go error), and
// invowk.cmd returns the refusal's text.
func TestVirtualPathDerivedPathsLua(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	for name, args := range map[string]string{
		"cp":  `"cp", "f", "d"`,
		"tar": `"tar", "-x", "-f", "a.tar", "ex"`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, access := range []invowkfile.VirtualFilesystemAccess{invowkfile.VirtualFilesystemAccessRestricted, invowkfile.VirtualFilesystemAccessFull} {
				for _, form := range []string{"capture", "cmd"} {
					work := harnessPhysicalDir(t, t.TempDir())
					outside := harnessPhysicalDir(t, outsideVirtualRootsDir(t))
					derivedWrite(t, filepath.Join(work, "f"), "f-content")
					derivedTarArchive(t, filepath.Join(work, "a.tar"), derivedTarEntry{name: "f", typeflag: tar.TypeReg, mode: 0o644, body: "entry"})
					derivedWrite(t, filepath.Join(outside, "t"), "outside-content")
					derivedLink(t, filepath.Join(outside, "t"), filepath.Join(work, "d", "f"))
					derivedLink(t, filepath.Join(outside, "t"), filepath.Join(work, "ex", "f"))
					script := "local out, err, code = invowk.capture(" + args + ")\n"
					if form == "cmd" {
						script = "local code, err = invowk.cmd(" + args + ")\n"
					}
					script += "print(\"err=\" .. tostring(err))\nprint(\"code=\" .. tostring(code))\n"
					run := runVirtualLuaTest(t, work, script, access)
					content, err := os.ReadFile(filepath.Join(outside, "t"))
					if err != nil {
						t.Fatal(err)
					}
					if access == invowkfile.VirtualFilesystemAccessFull {
						if strings.Contains(run.stdout+run.stderr, derivedDeniedText) || !strings.Contains(run.stdout, "code=0") || string(content) == "outside-content" {
							t.Fatalf("full %s: stdout %q, stderr %q, outside %q; want u-root's write", form, run.stdout, run.stderr, content)
						}
						continue
					}
					if !strings.Contains(run.stdout, "code=1") || (form == "cmd" && !strings.Contains(run.stdout, derivedDeniedText)) {
						t.Fatalf("restricted %s: stdout %q, stderr %q; want the denial", form, run.stdout, run.stderr)
					}
					if string(content) != "outside-content" {
						t.Fatalf("restricted %s: outside file changed to %q", form, content)
					}
				}
			}
		})
	}
}

func TestVirtualPathDerivedPathsTarCreate(t *testing.T) {
	t.Parallel()
	for _, access := range virtualPathBothAccessModes {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			work := harnessPhysicalDir(t, t.TempDir())
			derivedWrite(t, filepath.Join(work, "src", "file.txt"), "hello-tar")
			mustMkdir(t, filepath.Join(work, "ex"))
			run := runVirtualShTest(t, work, "tar -c -f a.tar src && tar -t -f a.tar && tar -x -f a.tar ex", access)
			derivedAssertSuccess(t, run)
			if !strings.Contains(run.stdout, "src/file.txt") {
				t.Errorf("tar -t stdout = %q, want src/file.txt", run.stdout)
			}
			derivedAssertFile(t, filepath.Join(work, "ex", "src", "file.txt"), "hello-tar")
		})
	}

	t.Run("outside archive refused under restricted access", func(t *testing.T) {
		t.Parallel()
		work := harnessPhysicalDir(t, t.TempDir())
		outside := harnessPhysicalDir(t, outsideVirtualRootsDir(t))
		derivedWrite(t, filepath.Join(work, "src", "file.txt"), "hello-tar")
		outTar := filepath.Join(outside, "out.tar")
		run := runVirtualShTest(t, work, "tar -c -f "+shQuote(outTar)+" src", invowkfile.VirtualFilesystemAccessRestricted)
		if run.exit == 0 || !strings.Contains(run.stderr, derivedDeniedText) {
			t.Fatalf("restricted tar -c outside = (exit %d, stderr %q), want denial", run.exit, run.stderr)
		}
		if _, err := os.Stat(outTar); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("outside archive created unexpectedly: stat err = %v", err)
		}
	})
}
