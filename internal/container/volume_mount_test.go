// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

func TestVolumeMount_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mount    VolumeMount
		want     bool
		wantErr  bool
		wantErrs int // expected number of field errors (0 means don't check count)
	}{
		{
			"all valid fields",
			VolumeMount{
				HostPath:      "/home/user/data",
				ContainerPath: "/app/data",
				ReadOnly:      false,
				SELinux:       SELinuxLabelShared,
			},
			true, false, 0,
		},
		{
			"all valid with readonly and private SELinux",
			VolumeMount{
				HostPath:      "/var/lib/db",
				ContainerPath: "/data",
				ReadOnly:      true,
				SELinux:       SELinuxLabelPrivate,
			},
			true, false, 0,
		},
		{
			"valid with no SELinux label (zero value)",
			VolumeMount{
				HostPath:      "/tmp/test",
				ContainerPath: "/workspace",
				SELinux:       SELinuxLabelNone,
			},
			true, false, 0,
		},
		{
			"invalid host path (empty)",
			VolumeMount{
				HostPath:      "",
				ContainerPath: "/app",
				SELinux:       SELinuxLabelNone,
			},
			false, true, 1,
		},
		{
			"invalid container path (whitespace)",
			VolumeMount{
				HostPath:      "/data",
				ContainerPath: "   ",
				SELinux:       SELinuxLabelNone,
			},
			false, true, 1,
		},
		{
			"invalid SELinux label",
			VolumeMount{
				HostPath:      "/data",
				ContainerPath: "/app",
				SELinux:       SELinuxLabel("bogus"),
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			VolumeMount{
				HostPath:      "",
				ContainerPath: "",
				SELinux:       SELinuxLabel("invalid"),
			},
			false, true, 3,
		},
		{
			"zero value (all fields empty)",
			VolumeMount{},
			false, true, 2, // HostPath and ContainerPath invalid; SELinux "" is valid (SELinuxLabelNone)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.mount.IsValid()
			if isValid != tt.want {
				t.Errorf("VolumeMount.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatal("VolumeMount.IsValid() returned no errors, want error")
				}
				if tt.wantErrs > 0 && len(errs) != tt.wantErrs {
					t.Errorf("VolumeMount.IsValid() returned %d errors, want %d: %v",
						len(errs), tt.wantErrs, errs)
				}
			} else if len(errs) > 0 {
				t.Errorf("VolumeMount.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestVolumeMount_IsValid_FieldErrorTypes(t *testing.T) {
	t.Parallel()

	// Verify that each field error wraps the correct sentinel and has the correct type.
	mount := VolumeMount{
		HostPath:      "   ",
		ContainerPath: "\t",
		SELinux:       SELinuxLabel("bad"),
	}
	isValid, errs := mount.IsValid()
	if isValid {
		t.Fatal("expected invalid, got valid")
	}
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors (one per invalid field), got %d: %v", len(errs), errs)
	}

	// First error: HostPath
	if !errors.Is(errs[0], ErrInvalidHostFilesystemPath) {
		t.Errorf("first error should wrap ErrInvalidHostFilesystemPath, got: %v", errs[0])
	}
	var hfpErr *InvalidHostFilesystemPathError
	if !errors.As(errs[0], &hfpErr) {
		t.Errorf("first error should be *InvalidHostFilesystemPathError, got: %T", errs[0])
	}

	// Second error: ContainerPath
	if !errors.Is(errs[1], ErrInvalidMountTargetPath) {
		t.Errorf("second error should wrap ErrInvalidMountTargetPath, got: %v", errs[1])
	}
	var mtpErr *InvalidMountTargetPathError
	if !errors.As(errs[1], &mtpErr) {
		t.Errorf("second error should be *InvalidMountTargetPathError, got: %T", errs[1])
	}

	// Third error: SELinux
	if !errors.Is(errs[2], ErrInvalidSELinuxLabel) {
		t.Errorf("third error should wrap ErrInvalidSELinuxLabel, got: %v", errs[2])
	}
	var slErr *InvalidSELinuxLabelError
	if !errors.As(errs[2], &slErr) {
		t.Errorf("third error should be *InvalidSELinuxLabelError, got: %T", errs[2])
	}
}

func TestInvalidVolumeMountError(t *testing.T) {
	t.Parallel()

	fieldErrs := []error{
		&InvalidHostFilesystemPathError{Value: ""},
		&InvalidMountTargetPathError{Value: ""},
	}
	err := &InvalidVolumeMountError{
		Value:     VolumeMount{HostPath: "", ContainerPath: ""},
		FieldErrs: fieldErrs,
	}

	if err.Error() == "" {
		t.Error("InvalidVolumeMountError.Error() returned empty string")
	}
	if !errors.Is(err, ErrInvalidVolumeMount) {
		t.Error("InvalidVolumeMountError should wrap ErrInvalidVolumeMount")
	}
}
