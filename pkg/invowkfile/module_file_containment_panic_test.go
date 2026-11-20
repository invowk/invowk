// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
)

// TestRootStat_RecoversPanic: a panic inside os.Root path resolution (Go 1.27
// on Windows, a link resolving to its own directory) becomes an error wrapping
// errRootResolverPanic; a normal result passes through unchanged.
func TestRootStat_RecoversPanic(t *testing.T) {
	t.Parallel()
	fi, err := rootStat(func(string) (fs.FileInfo, error) { panic("index out of range [1] with length 1") }, "a/x")
	if !errors.Is(err, errRootResolverPanic) || fi != nil {
		t.Fatalf("panicking stat: fi=%v err=%v, want nil and errRootResolverPanic", fi, err)
	}
	if !strings.Contains(err.Error(), "index out of range") {
		t.Errorf("err = %v, want the panic value in the message", err)
	}

	want := errors.New("plain")
	var gotRel string
	_, err = rootStat(func(rel string) (fs.FileInfo, error) { gotRel = rel; return nil, want }, "a/x")
	if !errors.Is(err, want) || errors.Is(err, errRootResolverPanic) || gotRel != "a/x" {
		t.Fatalf("plain stat: err=%v rel=%q, want the stat error unchanged for a/x", err, gotRel)
	}
}

// TestJudgeRootResolverPanic: a path whose os.Root Stat panicked is passed on
// only when it is a directory (no content can be read from it); a file or a
// missing path fails closed as an escape.
func TestJudgeRootResolverPanic(t *testing.T) {
	t.Parallel()
	_, mod := containmentFixture(t)
	cause := errRootResolverPanic
	absent, err := judgeRootResolverPanic(FilesystemPath(filepath.Join(mod, "sub")), FilesystemPath(mod), cause)
	if err != nil || absent {
		t.Fatalf("directory: absent=%v err=%v, want contained", absent, err)
	}
	for _, p := range []string{"script.sh", "missing"} {
		absent, err = judgeRootResolverPanic(FilesystemPath(filepath.Join(mod, p)), FilesystemPath(mod), cause)
		if !errors.Is(err, ErrScriptPathTraversal) || !errors.Is(err, errRootResolverPanic) || absent {
			t.Errorf("%s: absent=%v err=%v, want an escape wrapping the panic", p, absent, err)
		}
	}
}

// TestCheckModuleFilePhysicalContainment_LinkToOwnDirectory: sub/self -> "."
// resolves to the directory that holds it, inside the module. It is contained
// on every platform (on Windows through the recovered os.Root panic), and the
// reader then fails on the directory, as for any directory path.
func TestCheckModuleFilePhysicalContainment_LinkToOwnDirectory(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	_, mod := containmentFixture(t)
	for _, target := range []string{".", filepath.Join("..", "sub")} {
		link := filepath.Join(mod, "sub", "self")
		_ = os.Remove(link)
		fstree.MustSymlink(t, target, link)
		absent, err := checkModuleFilePhysicalContainment(FilesystemPath(link), FilesystemPath(mod))
		if err != nil || absent {
			t.Errorf("target %q: absent=%v err=%v, want contained", target, absent, err)
		}
	}
}
