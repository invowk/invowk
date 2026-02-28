// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"slices"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
)

func TestModuleMetadata_Validate(t *testing.T) {
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
				module:      invowkmod.ModuleID("io.invowk.sample"),
				version:     invowkmod.SemVer("1.0.0"),
				description: DescriptionText("A sample module"),
				requires: []ModuleRequirement{
					{GitURL: invowkmod.GitURL("https://github.com/example/utils.git"), Version: invowkmod.SemVerConstraint("^1.0.0")},
				},
			},
			true, false, 0,
		},
		{
			"valid minimal metadata (no optional fields)",
			ModuleMetadata{
				module:  invowkmod.ModuleID("io.invowk.minimal"),
				version: invowkmod.SemVer("0.1.0"),
			},
			true, false, 0,
		},
		{
			"invalid module ID (empty)",
			ModuleMetadata{
				module:  invowkmod.ModuleID(""),
				version: invowkmod.SemVer("1.0.0"),
			},
			false, true, 1,
		},
		{
			"invalid version (empty)",
			ModuleMetadata{
				module:  invowkmod.ModuleID("io.invowk.sample"),
				version: invowkmod.SemVer(""),
			},
			false, true, 1,
		},
		{
			"invalid description (whitespace-only)",
			ModuleMetadata{
				module:      invowkmod.ModuleID("io.invowk.sample"),
				version:     invowkmod.SemVer("1.0.0"),
				description: DescriptionText("   "),
			},
			false, true, 1,
		},
		{
			"invalid requirement (empty git URL)",
			ModuleMetadata{
				module:  invowkmod.ModuleID("io.invowk.sample"),
				version: invowkmod.SemVer("1.0.0"),
				requires: []ModuleRequirement{
					{GitURL: invowkmod.GitURL(""), Version: invowkmod.SemVerConstraint("^1.0.0")},
				},
			},
			false, true, 1,
		},
		{
			"multiple invalid fields",
			ModuleMetadata{
				module:      invowkmod.ModuleID(""),
				version:     invowkmod.SemVer(""),
				description: DescriptionText("   "),
			},
			false, true, 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.meta.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleMetadata.Validate() error = %v, want valid=%v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleMetadata.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidModuleMetadata) {
					t.Errorf("error should wrap ErrInvalidModuleMetadata, got: %v", err)
				}
				var metaErr *InvalidModuleMetadataError
				if !errors.As(err, &metaErr) {
					t.Fatalf("error should be *InvalidModuleMetadataError, got: %T", err)
				}
				if len(metaErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(metaErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("ModuleMetadata.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestNewModuleMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		module      invowkmod.ModuleID
		version     invowkmod.SemVer
		description DescriptionText
		requires    []ModuleRequirement
		wantErr     bool
	}{
		{
			name:        "valid complete",
			module:      "io.invowk.sample",
			version:     "1.0.0",
			description: "A sample module",
			requires: []ModuleRequirement{
				{GitURL: invowkmod.GitURL("https://github.com/example/utils.git"), Version: invowkmod.SemVerConstraint("^1.0.0")},
			},
		},
		{
			name:    "valid minimal",
			module:  "io.invowk.minimal",
			version: "0.1.0",
		},
		{
			name:    "invalid module ID",
			module:  "",
			version: "1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid version",
			module:  "io.invowk.sample",
			version: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			meta, err := NewModuleMetadata(tt.module, tt.version, tt.description, tt.requires)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if meta != nil {
					t.Error("expected nil metadata on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Module() != tt.module {
				t.Errorf("Module() = %q, want %q", meta.Module(), tt.module)
			}
			if meta.Version() != tt.version {
				t.Errorf("Version() = %q, want %q", meta.Version(), tt.version)
			}
			if meta.Description() != tt.description {
				t.Errorf("Description() = %q, want %q", meta.Description(), tt.description)
			}
		})
	}
}

func TestNewModuleMetadata_ConstructorAlwaysPassesValidate(t *testing.T) {
	t.Parallel()

	meta, err := NewModuleMetadata("io.invowk.test", "1.0.0", "Test", nil)
	if err != nil {
		t.Fatalf("NewModuleMetadata() unexpected error: %v", err)
	}
	if err := meta.Validate(); err != nil {
		t.Errorf("constructor-created ModuleMetadata should pass Validate(), got error: %v", err)
	}
}

func TestModuleMetadata_Accessors(t *testing.T) {
	t.Parallel()

	reqs := []ModuleRequirement{
		{GitURL: invowkmod.GitURL("https://github.com/example/a.git"), Version: invowkmod.SemVerConstraint("^1.0.0")},
		{GitURL: invowkmod.GitURL("https://github.com/example/b.git"), Version: invowkmod.SemVerConstraint("^2.0.0")},
	}
	meta, err := NewModuleMetadata("io.invowk.acc", "2.3.4", "Accessor test", reqs)
	if err != nil {
		t.Fatalf("NewModuleMetadata() unexpected error: %v", err)
	}

	if meta.Module() != "io.invowk.acc" {
		t.Errorf("Module() = %q, want %q", meta.Module(), "io.invowk.acc")
	}
	if meta.Version() != "2.3.4" {
		t.Errorf("Version() = %q, want %q", meta.Version(), "2.3.4")
	}
	if meta.Description() != "Accessor test" {
		t.Errorf("Description() = %q, want %q", meta.Description(), "Accessor test")
	}
	if !slices.Equal(meta.Requires(), reqs) {
		t.Errorf("Requires() = %v, want %v", meta.Requires(), reqs)
	}
}

func TestModuleMetadata_Requires_DefensiveCopy(t *testing.T) {
	t.Parallel()

	meta, err := NewModuleMetadata("io.invowk.def", "1.0.0", "", []ModuleRequirement{
		{GitURL: invowkmod.GitURL("https://github.com/example/orig.git"), Version: invowkmod.SemVerConstraint("^1.0.0")},
	})
	if err != nil {
		t.Fatalf("NewModuleMetadata() unexpected error: %v", err)
	}

	// Mutating the returned slice must not affect the original.
	got := meta.Requires()
	got[0] = ModuleRequirement{GitURL: invowkmod.GitURL("https://github.com/example/mutated.git")}

	// Re-fetch and verify the original is unchanged.
	original := meta.Requires()
	if original[0].GitURL != "https://github.com/example/orig.git" {
		t.Errorf("Requires() was mutated: GitURL = %q, want original", original[0].GitURL)
	}
}

func TestNewModuleMetadataFromInvowkmod(t *testing.T) {
	t.Parallel()

	t.Run("nil returns nil", func(t *testing.T) {
		t.Parallel()
		if got := NewModuleMetadataFromInvowkmod(nil); got != nil {
			t.Errorf("NewModuleMetadataFromInvowkmod(nil) = %v, want nil", got)
		}
	})

	t.Run("converts fields correctly", func(t *testing.T) {
		t.Parallel()
		mod := &Invowkmod{
			Module:      "io.invowk.test",
			Version:     "1.2.3",
			Description: "Test module",
			Requires: []ModuleRequirement{
				{GitURL: invowkmod.GitURL("https://github.com/example/dep.git"), Version: invowkmod.SemVerConstraint("^1.0.0")},
			},
		}
		meta := NewModuleMetadataFromInvowkmod(mod)
		if meta == nil {
			t.Fatal("expected non-nil metadata")
		}
		if meta.Module() != "io.invowk.test" {
			t.Errorf("Module() = %q, want %q", meta.Module(), "io.invowk.test")
		}
		if meta.Version() != "1.2.3" {
			t.Errorf("Version() = %q, want %q", meta.Version(), "1.2.3")
		}
		if meta.Description() != "Test module" {
			t.Errorf("Description() = %q, want %q", meta.Description(), "Test module")
		}
		if len(meta.Requires()) != 1 {
			t.Errorf("Requires() length = %d, want 1", len(meta.Requires()))
		}
	})
}
