// SPDX-License-Identifier: MPL-2.0

package invowkfile_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestContainerfilePath_Validate runs the canonical seven-vector matrix
// against ContainerfilePath.Validate. ContainerfilePath has a strict
// "relative-only" contract: every absolute form is rejected, parent-directory
// traversal is rejected before normalization, and valid relative names pass
// through. Behavior surfaced by the matrix:
//   - All four absolute dialects rejected on every platform.
//   - Slash and backslash traversal rejected on every platform.
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
		SlashTraversal:     rejectInvalid,
		BackslashTraversal: rejectInvalid,
		ValidRelative:      pathmatrix.PassAny(nil),

		ExtraVectors: map[string]pathmatrix.VectorCase{
			"empty_zero_value_valid":   {Input: "", Expect: pathmatrix.PassAny(nil)},
			"whitespace_only_invalid":  {Input: "   ", Expect: rejectInvalid},
			"tab_only_invalid":         {Input: "\t", Expect: rejectInvalid},
			"windows_drive_with_slash": {Input: pathmatrix.InputWindowsDriveSlash, Expect: rejectInvalid},
			"simple_filename":          {Input: "Containerfile", Expect: pathmatrix.PassAny(nil)},
			"relative_dotted":          {Input: "./docker/Dockerfile", Expect: pathmatrix.PassAny(nil)},
			"consecutive_dots":         {Input: "docker/v1..2/Containerfile", Expect: pathmatrix.PassAny(nil)},
			"dotted_filename":          {Input: "Containerfile..backup", Expect: pathmatrix.PassAny(nil)},
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

func TestContainerfilePathValidationErrorPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       invowkfile.ContainerfilePath
		wantReason string
	}{
		{
			name:       "whitespace",
			path:       " \t ",
			wantReason: "non-empty value must not be whitespace-only",
		},
		{
			name:       "too long",
			path:       invowkfile.ContainerfilePath(strings.Repeat("a", invowkfile.MaxPathLength+1)),
			wantReason: fmt.Sprintf("path too long (%d chars, max %d)", invowkfile.MaxPathLength+1, invowkfile.MaxPathLength),
		},
		{
			name:       "absolute",
			path:       "/absolute",
			wantReason: "path must be relative, not absolute",
		},
		{
			name:       "null byte",
			path:       "Container\x00file",
			wantReason: "path contains null byte",
		},
		{
			name:       "parent segment",
			path:       `docker\..\Containerfile`,
			wantReason: "path contains parent-directory segment '..'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			requireContainerfilePathValidationErrorPayload(t, tt.path, tt.wantReason)
		})
	}
}

func requireContainerfilePathValidationErrorPayload(
	t *testing.T,
	path invowkfile.ContainerfilePath,
	wantReason string,
) {
	t.Helper()

	err := path.Validate()
	if !errors.Is(err, invowkfile.ErrInvalidContainerfilePath) {
		t.Fatalf("ContainerfilePath(%q).Validate() error = %v, want ErrInvalidContainerfilePath", path, err)
	}
	var pathErr *invowkfile.InvalidContainerfilePathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("ContainerfilePath(%q).Validate() error type = %T, want *InvalidContainerfilePathError", path, err)
	}
	if pathErr.Value != path {
		t.Fatalf("InvalidContainerfilePathError.Value = %q, want %q", pathErr.Value, path)
	}
	if pathErr.Reason != wantReason {
		t.Fatalf("InvalidContainerfilePathError.Reason = %q, want %q", pathErr.Reason, wantReason)
	}
	wantError := fmt.Sprintf("invalid containerfile path %q: %s", path, wantReason)
	if err.Error() != wantError {
		t.Fatalf("InvalidContainerfilePathError.Error() = %q, want %q", err.Error(), wantError)
	}
}

func TestContainerfilePathLengthBoundary(t *testing.T) {
	t.Parallel()

	path := invowkfile.ContainerfilePath(strings.Repeat("a", invowkfile.MaxPathLength))
	if err := path.Validate(); err != nil {
		t.Fatalf("ContainerfilePath(max length).Validate() = %v, want nil", err)
	}
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
