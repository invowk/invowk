// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// These tests cover the access handler (design D3, "The access handler", of
// virtual-path-no-follow-operands): `cd`'s access check and the -r, -w and -x
// tests run on the validated path, on every platform.

// recordingAccess records each path that reaches the shell's own access
// check, then runs it.
type recordingAccess struct {
	mu    sync.Mutex
	paths []string
}

func TestVirtualShAccessCdThroughLinks(t *testing.T) {
	t.Parallel()
	for _, access := range virtualPathBothAccessModes {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			work := linkTestTree(t)
			run := runVirtualShTest(t, work, "cd dirlink && [ -f file.txt ] && echo in-target", access)
			if run.exit != 0 || run.stdout != "in-target\n" {
				t.Fatalf("cd dirlink = %+v, want status 0 and the target's file", run)
			}
		})
		t.Run(string(access)+"/junction", func(t *testing.T) {
			t.Parallel()
			fstree.Require(t, fstree.CapJunction)
			work := mustMkdir(t, filepath.Join(fstree.TempRoot(t), "work"))
			target := mustMkdir(t, filepath.Join(work, "tdir"))
			writeTestFile(t, filepath.Join(target, "file.txt"), "file")
			fstree.MustJunction(t, target, filepath.Join(work, "j"))
			run := runVirtualShTest(t, work, "cd j && [ -f file.txt ] && echo in-target", access)
			if run.exit != 0 || run.stdout != "in-target\n" {
				t.Fatalf("cd j = %+v, want status 0 and the target's file", run)
			}
		})
	}
}

func TestVirtualShAccessFileTestsOnLinks(t *testing.T) {
	t.Parallel()
	for _, access := range virtualPathBothAccessModes {
		t.Run(string(access), func(t *testing.T) {
			t.Parallel()
			work := linkTestTree(t)
			script := strings.Join([]string{
				`[ -r live ] && echo r-live`,
				`[ -x dirlink ] && echo x-dirlink`,
				`[ -L live ] && echo L-live`,
				`[ -L dirlink ] && echo L-dirlink`,
			}, "\n")
			run := runVirtualShTest(t, work, script, access)
			want := "r-live\nx-dirlink\nL-live\nL-dirlink\n"
			if run.stdout != want {
				t.Fatalf("file tests = %+v, want stdout %q", run, want)
			}
		})
	}
}

func (r *recordingAccess) handler() interp.AccessHandlerFunc {
	next := interp.DefaultAccessHandler()
	return func(ctx context.Context, path string, mode interp.AccessMode) error {
		r.mu.Lock()
		r.paths = append(r.paths, path)
		r.mu.Unlock()
		return next(ctx, path, mode)
	}
}

func (r *recordingAccess) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.paths)
}

// runAccessScript runs script in a runner with the virtual-sh stat handler and
// an access handler around a recording delegate, with root as its only
// allowed root and working directory.
func runAccessScript(t *testing.T, access invowkfile.VirtualFilesystemAccess, root, script string) (string, *recordingAccess) {
	t.Helper()
	validator := pathTestValidator(access, root)
	recorder := &recordingAccess{}
	var stdout bytes.Buffer
	runner, err := interp.New(
		interp.Dir(root),
		interp.StdIO(nil, &stdout, &stdout),
		interp.StatHandler(validator.statHandler(interp.DefaultStatHandler())),
		interp.AccessHandler(validator.accessHandler(recorder.handler())),
	)
	if err != nil {
		t.Fatalf("interp.New: %v", err)
	}
	prog, err := syntax.NewParser().Parse(strings.NewReader(script), "access-test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := runner.Run(t.Context(), prog); err != nil {
		t.Fatalf("run: %v", err)
	}
	return stdout.String(), recorder
}

func TestVirtualShAccessDeniedOutsideRoots(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	root := mustMkdir(t, filepath.Join(base, "root"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	outFile := filepath.Join(outside, "f")
	writeTestFile(t, outFile, "outside")
	target := filepath.Join(root, "target.txt")
	writeTestFile(t, target, "inside")
	fstree.MustSymlink(t, "target.txt", filepath.Join(root, "live"))
	script := strings.Join([]string{
		`[ -r ` + shQuote(outFile) + ` ] && echo r-out`,
		`[ -w ` + shQuote(outFile) + ` ] && echo w-out`,
		`[ -x ` + shQuote(outside) + ` ] && echo x-out`,
		`[ -r live ] && echo r-live`,
	}, "\n")

	t.Run("restricted", func(t *testing.T) {
		t.Parallel()
		stdout, recorder := runAccessScript(t, invowkfile.VirtualFilesystemAccessRestricted, root, script)
		if stdout != "r-live\n" {
			t.Errorf("stdout = %q, want only r-live: the outside tests are false", stdout)
		}
		paths := recorder.recorded()
		for _, path := range paths {
			if strings.HasPrefix(path, outside) {
				t.Errorf("the shell's access check ran on the outside path %s", path)
			}
		}
		if !slices.Contains(paths, target) {
			t.Errorf("access check paths %q, want the link's resolved path %s", paths, target)
		}
	})
	t.Run("full", func(t *testing.T) {
		t.Parallel()
		stdout, recorder := runAccessScript(t, invowkfile.VirtualFilesystemAccessFull, root, script)
		if !strings.Contains(stdout, "r-out") || !strings.Contains(stdout, "r-live") {
			t.Errorf("stdout = %q, want the outside and link tests true", stdout)
		}
		if paths := recorder.recorded(); !slices.Contains(paths, outFile) || !slices.Contains(paths, target) {
			t.Errorf("access check paths %q, want %s and %s", paths, outFile, target)
		}
	})
}
