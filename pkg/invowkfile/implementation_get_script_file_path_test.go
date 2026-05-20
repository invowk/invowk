// SPDX-License-Identifier: MPL-2.0

package invowkfile_test

import (
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func filesystemPathPtr(path string) *invowkfile.FilesystemPath {
	file := invowkfile.FilesystemPath(path)
	return &file
}

// TestGetScriptFilePathWithModule_Matrix exercises the seven canonical
// cross-platform path vectors against Implementation.GetScriptFilePathWithModule.
// This test is the runtime safety net for the A.2 fix from
// docs/next/cross-platform-windows-improvements.md: a strings.HasPrefix(_, "/")
// guard precedes filepath.IsAbs so that Unix-style absolute container paths
// like "/foo/bar.sh" are recognized as absolute on every platform — without
// the guard, filepath.IsAbs("/foo") returns false on Windows and the code
// silently joins the container path with the module root.
//
// GetScriptFilePathWithModule has two callable branches:
//   - modulePath != "" (module context, the scenario A.2 fixed)
//   - modulePath == "" (non-module context, where script.file is disallowed)
//
// Both branches are exercised below. Platform-divergent vectors use
// PassHostNativeAbs so the matrix delegates the pass-through-vs-join
// decision to filepath.IsAbs at test runtime, matching the resolver's
// actual contract on every platform.
func TestGetScriptFilePathWithModule_Matrix(t *testing.T) {
	t.Parallel()

	t.Run("with_module_path", func(t *testing.T) {
		t.Parallel()
		moduleDir := t.TempDir()
		invowkfileDir := t.TempDir()
		invowkfilePath := invowkfile.FilesystemPath(filepath.Join(invowkfileDir, "invowkfile.cue"))

		// Use an explicit script file reference.
		resolveFor := func(input string) (string, error) {
			impl := &invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: filesystemPathPtr(input + ".sh")}}
			return string(impl.GetScriptFilePathWithModule(invowkfilePath, invowkfile.FilesystemPath(moduleDir))), nil
		}

		expect := pathmatrix.Expectations{
			UnixAbsolute:       pathmatrix.Pass(pathmatrix.InputUnixAbsolute + ".sh"),
			WindowsDriveAbs:    pathmatrix.PassHostNativeAbs(pathmatrix.InputWindowsDriveAbs + ".sh"),
			WindowsRooted:      pathmatrix.PassHostNativeAbs(pathmatrix.InputWindowsRooted + ".sh"),
			UNC:                pathmatrix.PassHostNativeAbs(pathmatrix.InputUNC + ".sh"),
			SlashTraversal:     pathmatrix.PassRelative(pathmatrix.InputSlashTraversal + ".sh"),
			BackslashTraversal: pathmatrix.PassRelative(pathmatrix.InputBackslashTraversal + ".sh"),
			ValidRelative:      pathmatrix.PassAny(nil),
		}
		pathmatrix.Resolver(t, moduleDir, resolveFor, expect)
	})

	t.Run("no_module_path_returns_empty", func(t *testing.T) {
		t.Parallel()
		invowkfileDir := t.TempDir()
		invowkfilePath := invowkfile.FilesystemPath(filepath.Join(invowkfileDir, "invowkfile.cue"))

		resolveFor := func(input string) (string, error) {
			impl := &invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: filesystemPathPtr(input + ".sh")}}
			return string(impl.GetScriptFilePathWithModule(invowkfilePath, "")), nil
		}

		wantEmpty := pathmatrix.Custom(func(t testing.TB, got string, gotErr error) {
			t.Helper()
			if gotErr != nil {
				t.Fatalf("unexpected error: %v", gotErr)
			}
			if got != "" {
				t.Fatalf("got %q, want empty path for non-module script.file", got)
			}
		})
		expect := pathmatrix.Expectations{
			UnixAbsolute:       wantEmpty,
			WindowsDriveAbs:    wantEmpty,
			WindowsRooted:      wantEmpty,
			UNC:                wantEmpty,
			SlashTraversal:     wantEmpty,
			BackslashTraversal: wantEmpty,
			ValidRelative:      wantEmpty,
		}
		pathmatrix.Resolver(t, invowkfileDir, resolveFor, expect)
	})
}
