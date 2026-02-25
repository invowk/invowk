// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestVolumeMountSpec_IsValid(t *testing.T) {
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
			isValid, errs := tt.spec.IsValid()
			if isValid != tt.want {
				t.Errorf("VolumeMountSpec(%q).IsValid() = %v, want %v", tt.spec, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("VolumeMountSpec(%q).IsValid() returned no errors, want error", tt.spec)
				}
				if !errors.Is(errs[0], ErrInvalidVolumeMountSpec) {
					t.Errorf("error should wrap ErrInvalidVolumeMountSpec, got: %v", errs[0])
				}
				var vmErr *InvalidVolumeMountSpecError
				if !errors.As(errs[0], &vmErr) {
					t.Errorf("error should be *InvalidVolumeMountSpecError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("VolumeMountSpec(%q).IsValid() returned unexpected errors: %v", tt.spec, errs)
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
