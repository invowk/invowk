// SPDX-License-Identifier: MPL-2.0

package types_test

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/types"
)

// TestFilesystemPath_Validate runs the canonical seven-vector matrix.
// FilesystemPath.Validate is intentionally permissive — it only rejects
// empty and whitespace-only values; traversal/UNC/Windows-rooted forms are
// all valid because containment checks belong at higher layers.
func TestFilesystemPath_Validate(t *testing.T) {
	t.Parallel()

	pass := pathmatrix.PassAny(nil)
	pathmatrix.Validator(t, func(s string) error {
		return types.FilesystemPath(s).Validate()
	}, pathmatrix.Expectations{
		UnixAbsolute:       pass,
		WindowsDriveAbs:    pass,
		WindowsRooted:      pass,
		UNC:                pass,
		SlashTraversal:     pass,
		BackslashTraversal: pass,
		ValidRelative:      pass,

		ExtraVectors: map[string]pathmatrix.VectorCase{
			"empty_is_invalid":         {Input: "", Expect: pathmatrix.RejectIs(types.ErrInvalidFilesystemPath)},
			"whitespace_only_invalid":  {Input: "   ", Expect: pathmatrix.RejectIs(types.ErrInvalidFilesystemPath)},
			"tab_only_invalid":         {Input: "\t", Expect: pathmatrix.RejectIs(types.ErrInvalidFilesystemPath)},
			"path_with_spaces":         {Input: "/path/to/my file.txt", Expect: pass},
			"dot_path":                 {Input: ".", Expect: pass},
			"extended_length_unc_path": {Input: `\\?\C:\path`, Expect: pass},
		},
	})

	t.Run("error_wraps_typed_struct", func(t *testing.T) {
		t.Parallel()
		err := types.FilesystemPath("").Validate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var fpErr *types.InvalidFilesystemPathError
		if !errors.As(err, &fpErr) {
			t.Errorf("error should be *InvalidFilesystemPathError, got: %T", err)
		}
	})
}

// TestFilesystemPath_String confirms String() returns the underlying value
// unchanged.
func TestFilesystemPath_String(t *testing.T) {
	t.Parallel()
	p := types.FilesystemPath("/usr/bin/bash")
	if p.String() != "/usr/bin/bash" {
		t.Errorf("FilesystemPath.String() = %q, want %q", p.String(), "/usr/bin/bash")
	}
}
