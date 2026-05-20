// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestInvowkmod_Validate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		mod       Invowkmod
		want      bool
		wantErr   bool
		wantCount int
	}{
		{
			"valid complete module",
			Invowkmod{
				Module:      "io.invowk.sample",
				Version:     "1.0.0",
				Description: "A sample module",
				Requires: []ModuleRequirement{
					{GitURL: GitURL("https://github.com/example/utils.git"), Version: SemVerConstraint("^1.0.0")},
				},
				FilePath: types.FilesystemPath(filepath.Join(tmpDir, "sample.invowkmod", "invowkmod.cue")),
			},
			true, false, 0,
		},
		{
			"valid minimal module (no optional fields)",
			Invowkmod{
				Module:  "io.invowk.minimal",
				Version: "0.1.0",
			},
			true, false, 0,
		},
		{
			"invalid module ID (empty)",
			Invowkmod{
				Module:  "",
				Version: "1.0.0",
			},
			false, true, 1,
		},
		{
			"invalid version (empty)",
			Invowkmod{
				Module:  "io.invowk.sample",
				Version: "",
			},
			false, true, 1,
		},
		{
			"invalid description (whitespace-only)",
			Invowkmod{
				Module:      "io.invowk.sample",
				Version:     "1.0.0",
				Description: "   ",
			},
			false, true, 1,
		},
		{
			"invalid requirement (empty git URL)",
			Invowkmod{
				Module:  "io.invowk.sample",
				Version: "1.0.0",
				Requires: []ModuleRequirement{
					{GitURL: GitURL(""), Version: SemVerConstraint("^1.0.0")},
				},
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			Invowkmod{
				Module:      "",
				Version:     "",
				Description: "   ",
			},
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.mod.Validate()
			if (err == nil) != tt.want {
				t.Errorf("Invowkmod.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Invowkmod.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidInvowkmod) {
					t.Errorf("error should wrap ErrInvalidInvowkmod, got: %v", err)
				}
				var modErr *InvalidInvowkmodError
				if !errors.As(err, &modErr) {
					t.Fatalf("error should be *InvalidInvowkmodError, got: %T", err)
				}
				if len(modErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(modErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("Invowkmod.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
