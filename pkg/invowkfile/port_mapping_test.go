// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestPortMappingSpec_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    PortMappingSpec
		want    bool
		wantErr bool
	}{
		{"simple mapping", PortMappingSpec("8080:80"), true, false},
		{"with protocol", PortMappingSpec("8080:80/tcp"), true, false},
		{"udp protocol", PortMappingSpec("5353:53/udp"), true, false},
		{"range", PortMappingSpec("8000-8100:80-180"), true, false},
		{"empty is invalid", PortMappingSpec(""), false, true},
		{"no colon is invalid", PortMappingSpec("8080"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.spec.IsValid()
			if isValid != tt.want {
				t.Errorf("PortMappingSpec(%q).IsValid() = %v, want %v", tt.spec, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("PortMappingSpec(%q).IsValid() returned no errors, want error", tt.spec)
				}
				if !errors.Is(errs[0], ErrInvalidPortMappingSpec) {
					t.Errorf("error should wrap ErrInvalidPortMappingSpec, got: %v", errs[0])
				}
				var pmErr *InvalidPortMappingSpecError
				if !errors.As(errs[0], &pmErr) {
					t.Errorf("error should be *InvalidPortMappingSpecError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("PortMappingSpec(%q).IsValid() returned unexpected errors: %v", tt.spec, errs)
			}
		})
	}
}

func TestPortMappingSpec_String(t *testing.T) {
	t.Parallel()
	p := PortMappingSpec("8080:80")
	if p.String() != "8080:80" {
		t.Errorf("PortMappingSpec.String() = %q, want %q", p.String(), "8080:80")
	}
}
