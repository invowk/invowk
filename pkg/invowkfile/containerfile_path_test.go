// SPDX-License-Identifier: MPL-2.0

package invowkfile_test

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestContainerfilePath_Validate runs the canonical seven-vector matrix
// against ContainerfilePath.Validate. ContainerfilePath has a strict
// "relative-only" contract: every absolute form is rejected, traversal is
// allowed (it's just a string forwarded to the container engine), and
// valid relative names pass through. Behavior surfaced by the matrix:
//   - All four absolute dialects rejected on every platform.
//   - Slash and backslash traversal accepted (delegated to the engine).
//   - Valid relative names accepted everywhere.
func TestContainerfilePath_Validate(t *testing.T) {
	t.Parallel()

	rejectInvalid := pathmatrix.RejectIs(invowkfile.ErrInvalidContainerfilePath)
	pathmatrix.Validator(t, func(s string) error {
		return invowkfile.ContainerfilePath(s).Validate()
	}, pathmatrix.Expectations{
		UnixAbsolute:       rejectInvalid,
		WindowsDriveAbs:    rejectInvalid,
		WindowsRooted:      rejectInvalid,
		UNC:                rejectInvalid,
		SlashTraversal:     pathmatrix.PassAny(nil),
		BackslashTraversal: pathmatrix.PassAny(nil),
		ValidRelative:      pathmatrix.PassAny(nil),

		ExtraVectors: map[string]pathmatrix.VectorCase{
			"empty_zero_value_valid":   {Input: "", Expect: pathmatrix.PassAny(nil)},
			"whitespace_only_invalid":  {Input: "   ", Expect: rejectInvalid},
			"tab_only_invalid":         {Input: "\t", Expect: rejectInvalid},
			"windows_drive_with_slash": {Input: pathmatrix.InputWindowsDriveSlash, Expect: rejectInvalid},
			"simple_filename":          {Input: "Containerfile", Expect: pathmatrix.PassAny(nil)},
			"relative_dotted":          {Input: "./docker/Dockerfile", Expect: pathmatrix.PassAny(nil)},
		},
	})

	t.Run("error_wraps_typed_struct", func(t *testing.T) {
		t.Parallel()
		err := invowkfile.ContainerfilePath("/absolute").Validate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var cpErr *invowkfile.InvalidContainerfilePathError
		if !errors.As(err, &cpErr) {
			t.Errorf("error should be *InvalidContainerfilePathError, got: %T", err)
		}
	})
}

// TestContainerfilePath_String confirms String() returns the underlying
// value unchanged.
func TestContainerfilePath_String(t *testing.T) {
	t.Parallel()
	p := invowkfile.ContainerfilePath("Containerfile")
	if p.String() != "Containerfile" {
		t.Errorf("ContainerfilePath.String() = %q, want %q", p.String(), "Containerfile")
	}
}
