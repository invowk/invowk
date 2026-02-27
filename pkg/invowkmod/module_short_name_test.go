// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestModuleShortName_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		shortName ModuleShortName
		want      bool
		wantErr   bool
	}{
		// Valid cases
		{"simple name", ModuleShortName("foo"), true, false},
		{"rdns format", ModuleShortName("io.invowk.sample"), true, false},
		{"with hyphen", ModuleShortName("my-module"), true, false},
		{"with underscore", ModuleShortName("my_module"), true, false},
		{"single letter", ModuleShortName("a"), true, false},
		{"uppercase", ModuleShortName("MyModule"), true, false},
		{"mixed case with dots", ModuleShortName("Com.Example.Utils"), true, false},
		{"alphanumeric", ModuleShortName("module123"), true, false},
		{"all allowed chars", ModuleShortName("a1.b2-c3_d4"), true, false},

		// Invalid cases
		{"empty", ModuleShortName(""), false, true},
		{"starts with digit", ModuleShortName("1module"), false, true},
		{"contains space", ModuleShortName("my module"), false, true},
		{"starts with dot", ModuleShortName(".hidden"), false, true},
		{"starts with hyphen", ModuleShortName("-invalid"), false, true},
		{"starts with underscore", ModuleShortName("_invalid"), false, true},
		{"contains @", ModuleShortName("mod@1.0"), false, true},
		{"contains /", ModuleShortName("path/to"), false, true},
		{"contains #", ModuleShortName("mod#sub"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.shortName.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleShortName(%q).Validate() error = %v, wantValid %v", tt.shortName, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleShortName(%q).Validate() returned nil, want error", tt.shortName)
				}
				if !errors.Is(err, ErrInvalidModuleShortName) {
					t.Errorf("error should wrap ErrInvalidModuleShortName, got: %v", err)
				}
				var snErr *InvalidModuleShortNameError
				if !errors.As(err, &snErr) {
					t.Errorf("error should be *InvalidModuleShortNameError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ModuleShortName(%q).Validate() returned unexpected error: %v", tt.shortName, err)
			}
		})
	}
}

func TestModuleShortName_String(t *testing.T) {
	t.Parallel()

	n := ModuleShortName("io.invowk.sample")
	if n.String() != "io.invowk.sample" {
		t.Errorf("ModuleShortName.String() = %q, want %q", n.String(), "io.invowk.sample")
	}
}
