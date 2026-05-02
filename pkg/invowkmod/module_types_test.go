// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/pathmatrix"
)

func TestModuleAlias_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		alias   ModuleAlias
		want    bool
		wantErr bool
	}{
		{"empty is valid (no alias)", ModuleAlias(""), true, false},
		{"normal alias", ModuleAlias("mytools"), true, false},
		{"alias with dots", ModuleAlias("my.tools"), true, false},
		{"alias with dashes", ModuleAlias("my-tools"), true, false},
		{"alias with underscores", ModuleAlias("my_tools"), true, false},
		{"single char", ModuleAlias("a"), true, false},
		{"max length", ModuleAlias(strings.Repeat("a", MaxModuleAliasLength)), true, false},
		{"over max length", ModuleAlias(strings.Repeat("a", MaxModuleAliasLength+1)), false, true},
		{"starts with digit", ModuleAlias("1tools"), false, true},
		{"starts with hyphen", ModuleAlias("-tools"), false, true},
		{"space only", ModuleAlias(" "), false, true},
		{"tab only", ModuleAlias("\t"), false, true},
		{"multiple spaces", ModuleAlias("   "), false, true},
		{"newline only", ModuleAlias("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.alias.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleAlias(%q).Validate() error = %v, wantValid %v", tt.alias, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleAlias(%q).Validate() returned nil, want error", tt.alias)
				}
				if !errors.Is(err, ErrInvalidModuleAlias) {
					t.Errorf("error should wrap ErrInvalidModuleAlias, got: %v", err)
				}
				var aliasErr *InvalidModuleAliasError
				if !errors.As(err, &aliasErr) {
					t.Errorf("error should be *InvalidModuleAliasError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ModuleAlias(%q).Validate() returned unexpected error: %v", tt.alias, err)
			}
		})
	}
}

func TestModuleAlias_String(t *testing.T) {
	t.Parallel()
	a := ModuleAlias("mytools")
	if a.String() != "mytools" {
		t.Errorf("ModuleAlias.String() = %q, want %q", a.String(), "mytools")
	}
}

func TestModuleNamespace_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ns      ModuleNamespace
		want    bool
		wantErr bool
	}{
		{"module@version format", ModuleNamespace("tools@1.2.3"), true, false},
		{"alias format", ModuleNamespace("mytools"), true, false},
		{"single char", ModuleNamespace("a"), true, false},
		{"with dots and at", ModuleNamespace("io.invowk.tools@2.0.0"), true, false},
		{"empty is invalid", ModuleNamespace(""), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.ns.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleNamespace(%q).Validate() error = %v, wantValid %v", tt.ns, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleNamespace(%q).Validate() returned nil, want error", tt.ns)
				}
				if !errors.Is(err, ErrInvalidModuleNamespace) {
					t.Errorf("error should wrap ErrInvalidModuleNamespace, got: %v", err)
				}
				var nsErr *InvalidModuleNamespaceError
				if !errors.As(err, &nsErr) {
					t.Errorf("error should be *InvalidModuleNamespaceError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ModuleNamespace(%q).Validate() returned unexpected error: %v", tt.ns, err)
			}
		})
	}
}

func TestModuleNamespace_String(t *testing.T) {
	t.Parallel()
	n := ModuleNamespace("tools@1.2.3")
	if n.String() != "tools@1.2.3" {
		t.Errorf("ModuleNamespace.String() = %q, want %q", n.String(), "tools@1.2.3")
	}
}

func TestSubdirectoryPath_Validate(t *testing.T) {
	t.Parallel()

	rejectInvalid := pathmatrix.RejectIs(ErrInvalidSubdirectoryPath)
	pathmatrix.Validator(t, func(s string) error {
		return SubdirectoryPath(s).Validate()
	}, pathmatrix.Expectations{
		UnixAbsolute:       rejectInvalid,
		WindowsDriveAbs:    rejectInvalid,
		WindowsRooted:      rejectInvalid,
		UNC:                rejectInvalid,
		SlashTraversal:     rejectInvalid,
		BackslashTraversal: rejectInvalid,
		ValidRelative:      pathmatrix.PassAny(nil),

		ExtraVectors: map[string]pathmatrix.VectorCase{
			"empty_is_valid_repo_root": {Input: "", Expect: pathmatrix.PassAny(nil)},
			"nested_valid_relative":    {Input: "a/b/c/d", Expect: pathmatrix.PassAny(nil)},
			"single_dot_traversal":     {Input: "../escape", Expect: rejectInvalid},
			"null_byte":                {Input: "path\x00evil", Expect: rejectInvalid},
			"too_long":                 {Input: strings.Repeat("a", MaxPathLength+1), Expect: rejectInvalid},
		},
	})

	t.Run("error_wraps_typed_struct", func(t *testing.T) {
		t.Parallel()
		err := SubdirectoryPath("../escape").Validate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var pathErr *InvalidSubdirectoryPathError
		if !errors.As(err, &pathErr) {
			t.Errorf("error should be *InvalidSubdirectoryPathError, got: %T", err)
		}
	})
}

func TestSubdirectoryPath_String(t *testing.T) {
	t.Parallel()
	p := SubdirectoryPath("modules/tools")
	if p.String() != "modules/tools" {
		t.Errorf("SubdirectoryPath.String() = %q, want %q", p.String(), "modules/tools")
	}
}
