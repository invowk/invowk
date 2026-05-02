// SPDX-License-Identifier: MPL-2.0

package invowkfile_test

import (
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestGetEffectiveWorkDir_Matrix exercises the seven canonical
// cross-platform path vectors against (*Invowkfile).GetEffectiveWorkDir.
// This complements the precedence-focused tests in
// invowkfile_workdir_test.go: precedence asks "which workdir wins?", the
// matrix asks "for one chosen workdir, what does the resolver do with each
// of the seven canonical input shapes?"
//
// GetEffectiveWorkDir's contract for non-empty inputs:
//   - Unix-absolute "/foo" passes through unchanged on every platform —
//     the v0.10.0 strings.HasPrefix("/") guard is what makes this work.
//   - Other absolute forms pass through if filepath.IsAbs accepts them
//     on the running platform (Windows-drive on Windows; otherwise the
//     resolver joins them as relative segments).
//   - Relative inputs (including traversal) are joined to the invowkfile
//     directory; containment isn't enforced here.
func TestGetEffectiveWorkDir_Matrix(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	invowkfilePath := invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))

	resolveFor := func(input string) (string, error) {
		inv := &invowkfile.Invowkfile{
			FilePath: invowkfilePath,
			WorkDir:  invowkfile.WorkDir(input),
		}
		// Pass nil cmd/impl; root-level WorkDir is exercised by the
		// precedence rules, which are validated separately.
		return string(inv.GetEffectiveWorkDir(nil, nil, "")), nil
	}

	// Platform-divergent vectors use PassHostNativeAbs so the matrix
	// agrees with whatever filepath.IsAbs reports on the running
	// platform — pass-through when host considers absolute, joined
	// when relative. No per-platform override needed.
	expect := pathmatrix.Expectations{
		UnixAbsolute:       pathmatrix.Pass(pathmatrix.InputUnixAbsolute),
		WindowsDriveAbs:    pathmatrix.PassHostNativeAbs(pathmatrix.InputWindowsDriveAbs),
		WindowsRooted:      pathmatrix.PassHostNativeAbs(pathmatrix.InputWindowsRooted),
		UNC:                pathmatrix.PassHostNativeAbs(pathmatrix.InputUNC),
		SlashTraversal:     pathmatrix.PassRelative(pathmatrix.InputSlashTraversal),
		BackslashTraversal: pathmatrix.PassRelative(pathmatrix.InputBackslashTraversal),
		ValidRelative:      pathmatrix.PassAny(nil),
	}
	pathmatrix.Resolver(t, tmpDir, resolveFor, expect)
}
