// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestVolumeMountSpec_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    VolumeMountSpec
		want    bool
		wantErr bool
	}{
		{"host:container", VolumeMountSpec("/host:/container"), true, false},
		{"with ro option", VolumeMountSpec("/host:/container:ro"), true, false},
		{"relative paths", VolumeMountSpec("./data:/data"), true, false},
		{"empty is invalid", VolumeMountSpec(""), false, true},
		{"no colon is invalid", VolumeMountSpec("/just-a-path"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.spec.Validate()
			if (err == nil) != tt.want {
				t.Errorf("VolumeMountSpec(%q).Validate() error = %v, want valid=%v", tt.spec, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("VolumeMountSpec(%q).Validate() returned nil, want error", tt.spec)
				}
				if !errors.Is(err, ErrInvalidVolumeMountSpec) {
					t.Errorf("error should wrap ErrInvalidVolumeMountSpec, got: %v", err)
				}
				var vmErr *InvalidVolumeMountSpecError
				if !errors.As(err, &vmErr) {
					t.Errorf("error should be *InvalidVolumeMountSpecError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("VolumeMountSpec(%q).Validate() returned unexpected error: %v", tt.spec, err)
			}
		})
	}
}

func TestVolumeMountSpec_String(t *testing.T) {
	t.Parallel()
	v := VolumeMountSpec("/host:/container:ro")
	if v.String() != "/host:/container:ro" {
		t.Errorf("VolumeMountSpec.String() = %q, want %q", v.String(), "/host:/container:ro")
	}
}
