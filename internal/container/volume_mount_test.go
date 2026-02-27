// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"testing"
)

func TestVolumeMount_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mount   VolumeMount
		want    bool
		wantErr bool
	}{
		{
			"all valid fields",
			VolumeMount{
				HostPath:      "/home/user/data",
				ContainerPath: "/app/data",
				ReadOnly:      false,
				SELinux:       SELinuxLabelShared,
			},
			true, false,
		},
		{
			"all valid with readonly and private SELinux",
			VolumeMount{
				HostPath:      "/var/lib/db",
				ContainerPath: "/data",
				ReadOnly:      true,
				SELinux:       SELinuxLabelPrivate,
			},
			true, false,
		},
		{
			"valid with no SELinux label (zero value)",
			VolumeMount{
				HostPath:      "/tmp/test",
				ContainerPath: "/workspace",
				SELinux:       SELinuxLabelNone,
			},
			true, false,
		},
		{
			"invalid host path (empty)",
			VolumeMount{
				HostPath:      "",
				ContainerPath: "/app",
				SELinux:       SELinuxLabelNone,
			},
			false, true,
		},
		{
			"invalid container path (whitespace)",
			VolumeMount{
				HostPath:      "/data",
				ContainerPath: "   ",
				SELinux:       SELinuxLabelNone,
			},
			false, true,
		},
		{
			"invalid SELinux label",
			VolumeMount{
				HostPath:      "/data",
				ContainerPath: "/app",
				SELinux:       SELinuxLabel("bogus"),
			},
			false, true,
		},
		{
			"multiple invalid fields",
			VolumeMount{
				HostPath:      "",
				ContainerPath: "",
				SELinux:       SELinuxLabel("invalid"),
			},
			false, true,
		},
		{
			"zero value (all fields empty)",
			VolumeMount{},
			false, true, // HostPath and ContainerPath invalid; SELinux "" is valid (SELinuxLabelNone)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.mount.Validate()
			if (err == nil) != tt.want {
				t.Errorf("VolumeMount.Validate() error = %v, want valid=%v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("VolumeMount.Validate() returned nil, want error")
				}
			} else if err != nil {
				t.Errorf("VolumeMount.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestVolumeMount_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mount VolumeMount
		want  string
	}{
		{
			"basic_rw",
			VolumeMount{HostPath: "/host", ContainerPath: "/container"},
			"/host:/container",
		},
		{
			"read_only",
			VolumeMount{HostPath: "/data", ContainerPath: "/mnt", ReadOnly: true},
			"/data:/mnt:ro",
		},
		{
			"selinux_shared",
			VolumeMount{HostPath: "/src", ContainerPath: "/app", SELinux: SELinuxLabelShared},
			"/src:/app:z",
		},
		{
			"selinux_private_readonly",
			VolumeMount{HostPath: "/etc", ContainerPath: "/config", SELinux: SELinuxLabelPrivate, ReadOnly: true},
			"/etc:/config:Z:ro",
		},
		{
			"empty_paths",
			VolumeMount{},
			":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.mount.String()
			if got != tt.want {
				t.Errorf("VolumeMount.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVolumeMount_Validate_FieldErrorTypes(t *testing.T) {
	t.Parallel()

	// Verify that the joined error wraps the correct sentinels.
	mount := VolumeMount{
		HostPath:      "   ",
		ContainerPath: "\t",
		SELinux:       SELinuxLabel("bad"),
	}
	err := mount.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The joined error should wrap all three sentinels
	if !errors.Is(err, ErrInvalidHostFilesystemPath) {
		t.Errorf("error should wrap ErrInvalidHostFilesystemPath, got: %v", err)
	}
	if !errors.Is(err, ErrInvalidMountTargetPath) {
		t.Errorf("error should wrap ErrInvalidMountTargetPath, got: %v", err)
	}
	if !errors.Is(err, ErrInvalidSELinuxLabel) {
		t.Errorf("error should wrap ErrInvalidSELinuxLabel, got: %v", err)
	}

	// Verify individual error types via errors.As
	var hfpErr *InvalidHostFilesystemPathError
	if !errors.As(err, &hfpErr) {
		t.Errorf("error should contain *InvalidHostFilesystemPathError, got: %T", err)
	}

	var mtpErr *InvalidMountTargetPathError
	if !errors.As(err, &mtpErr) {
		t.Errorf("error should contain *InvalidMountTargetPathError, got: %T", err)
	}

	var slErr *InvalidSELinuxLabelError
	if !errors.As(err, &slErr) {
		t.Errorf("error should contain *InvalidSELinuxLabelError, got: %T", err)
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
