// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestPortMappingSpec_Validate(t *testing.T) {
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
			err := tt.spec.Validate()
			if (err == nil) != tt.want {
				t.Errorf("PortMappingSpec(%q).Validate() error = %v, want valid=%v", tt.spec, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("PortMappingSpec(%q).Validate() returned nil, want error", tt.spec)
				}
				if !errors.Is(err, ErrInvalidPortMappingSpec) {
					t.Errorf("error should wrap ErrInvalidPortMappingSpec, got: %v", err)
				}
				var pmErr *InvalidPortMappingSpecError
				if !errors.As(err, &pmErr) {
					t.Errorf("error should be *InvalidPortMappingSpecError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("PortMappingSpec(%q).Validate() returned unexpected error: %v", tt.spec, err)
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
