// SPDX-License-Identifier: MPL-2.0

package invowkfile_test

import (
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/invowkfile"
)

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
//   - modulePath == "" (invowkfile-relative resolution)
//
// Both branches are exercised below.
func TestGetScriptFilePathWithModule_Matrix(t *testing.T) {
	t.Parallel()

	t.Run("with_module_path", func(t *testing.T) {
		t.Parallel()
		moduleDir := t.TempDir()
		invowkfileDir := t.TempDir()
		invowkfilePath := invowkfile.FilesystemPath(filepath.Join(invowkfileDir, "invowkfile.cue"))

		// Use a script-file-shaped content (extension makes IsScriptFile true).
		resolveFor := func(input string) (string, error) {
			impl := &invowkfile.Implementation{Script: invowkfile.ScriptContent(input + ".sh")}
			return string(impl.GetScriptFilePathWithModule(invowkfilePath, invowkfile.FilesystemPath(moduleDir))), nil
		}

		// On every platform: "/foo" is container-absolute and passes through.
		// "C:\foo" passes through on Windows (filepath.IsAbs true); on Linux/macOS
		// it's a relative spec and gets joined with moduleDir. UNC and
		// Windows-rooted forms behave platform-specifically — model with overrides.
		expect := pathmatrix.Expectations{
			UnixAbsolute:    pathmatrix.Pass(pathmatrix.InputUnixAbsolute + ".sh"),
			WindowsDriveAbs: pathmatrix.PassRelative(pathmatrix.InputWindowsDriveAbs + ".sh"),
			OnWindows: &pathmatrix.PlatformOverride{
				WindowsDriveAbs: func() *pathmatrix.Outcome { o := pathmatrix.Pass(pathmatrix.InputWindowsDriveAbs + ".sh"); return &o }(),
			},
			WindowsRooted:      pathmatrix.PassRelative(pathmatrix.InputWindowsRooted + ".sh"),
			UNC:                pathmatrix.PassRelative(pathmatrix.InputUNC + ".sh"),
			SlashTraversal:     pathmatrix.PassRelative(pathmatrix.InputSlashTraversal + ".sh"),
			BackslashTraversal: pathmatrix.PassRelative(pathmatrix.InputBackslashTraversal + ".sh"),
			ValidRelative:      pathmatrix.PassAny(nil),
		}
		pathmatrix.Resolver(t, moduleDir, resolveFor, expect)
	})

	t.Run("no_module_path", func(t *testing.T) {
		t.Parallel()
		invowkfileDir := t.TempDir()
		invowkfilePath := invowkfile.FilesystemPath(filepath.Join(invowkfileDir, "invowkfile.cue"))

		resolveFor := func(input string) (string, error) {
			impl := &invowkfile.Implementation{Script: invowkfile.ScriptContent(input + ".sh")}
			return string(impl.GetScriptFilePathWithModule(invowkfilePath, "")), nil
		}

		expect := pathmatrix.Expectations{
			UnixAbsolute:    pathmatrix.Pass(pathmatrix.InputUnixAbsolute + ".sh"),
			WindowsDriveAbs: pathmatrix.PassRelative(pathmatrix.InputWindowsDriveAbs + ".sh"),
			OnWindows: &pathmatrix.PlatformOverride{
				WindowsDriveAbs: func() *pathmatrix.Outcome { o := pathmatrix.Pass(pathmatrix.InputWindowsDriveAbs + ".sh"); return &o }(),
			},
			WindowsRooted:      pathmatrix.PassRelative(pathmatrix.InputWindowsRooted + ".sh"),
			UNC:                pathmatrix.PassRelative(pathmatrix.InputUNC + ".sh"),
			SlashTraversal:     pathmatrix.PassRelative(pathmatrix.InputSlashTraversal + ".sh"),
			BackslashTraversal: pathmatrix.PassRelative(pathmatrix.InputBackslashTraversal + ".sh"),
			ValidRelative:      pathmatrix.PassAny(nil),
		}
		pathmatrix.Resolver(t, invowkfileDir, resolveFor, expect)
	})
}
