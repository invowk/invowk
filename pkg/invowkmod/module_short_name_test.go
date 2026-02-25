// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"testing"
)

func TestModuleShortName_IsValid(t *testing.T) {
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

			isValid, errs := tt.shortName.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleShortName(%q).IsValid() = %v, want %v", tt.shortName, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleShortName(%q).IsValid() returned no errors, want error", tt.shortName)
				}
				if !errors.Is(errs[0], ErrInvalidModuleShortName) {
					t.Errorf("error should wrap ErrInvalidModuleShortName, got: %v", errs[0])
				}
				var snErr *InvalidModuleShortNameError
				if !errors.As(errs[0], &snErr) {
					t.Errorf("error should be *InvalidModuleShortNameError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleShortName(%q).IsValid() returned unexpected errors: %v", tt.shortName, errs)
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
