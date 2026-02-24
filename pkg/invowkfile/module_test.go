// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestModuleMetadata_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		meta      ModuleMetadata
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete metadata",
			ModuleMetadata{
				Module:      invowkmod.ModuleID("io.invowk.sample"),
				Version:     invowkmod.SemVer("1.0.0"),
				Description: DescriptionText("A sample module"),
				Requires: []ModuleRequirement{
					{GitURL: invowkmod.GitURL("https://github.com/example/utils.git"), Version: invowkmod.SemVerConstraint("^1.0.0")},
				},
			},
			true, false, 0,
		},
		{
			"valid minimal metadata (no optional fields)",
			ModuleMetadata{
				Module:  invowkmod.ModuleID("io.invowk.minimal"),
				Version: invowkmod.SemVer("0.1.0"),
			},
			true, false, 0,
		},
		{
			"invalid module ID (empty)",
			ModuleMetadata{
				Module:  invowkmod.ModuleID(""),
				Version: invowkmod.SemVer("1.0.0"),
			},
			false, true, 1,
		},
		{
			"invalid version (empty)",
			ModuleMetadata{
				Module:  invowkmod.ModuleID("io.invowk.sample"),
				Version: invowkmod.SemVer(""),
			},
			false, true, 1,
		},
		{
			"invalid description (whitespace-only)",
			ModuleMetadata{
				Module:      invowkmod.ModuleID("io.invowk.sample"),
				Version:     invowkmod.SemVer("1.0.0"),
				Description: DescriptionText("   "),
			},
			false, true, 1,
		},
		{
			"invalid requirement (empty git URL)",
			ModuleMetadata{
				Module:  invowkmod.ModuleID("io.invowk.sample"),
				Version: invowkmod.SemVer("1.0.0"),
				Requires: []ModuleRequirement{
					{GitURL: invowkmod.GitURL(""), Version: invowkmod.SemVerConstraint("^1.0.0")},
				},
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			ModuleMetadata{
				Module:      invowkmod.ModuleID(""),
				Version:     invowkmod.SemVer(""),
				Description: DescriptionText("   "),
			},
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.meta.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleMetadata.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleMetadata.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidModuleMetadata) {
					t.Errorf("error should wrap ErrInvalidModuleMetadata, got: %v", errs[0])
				}
				var metaErr *InvalidModuleMetadataError
				if !errors.As(errs[0], &metaErr) {
					t.Fatalf("error should be *InvalidModuleMetadataError, got: %T", errs[0])
				}
				if len(metaErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(metaErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleMetadata.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}
