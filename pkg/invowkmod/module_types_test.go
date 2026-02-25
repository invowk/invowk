// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"
)

func TestModuleAlias_IsValid(t *testing.T) {
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
		{"single char", ModuleAlias("a"), true, false},
		{"space only", ModuleAlias(" "), false, true},
		{"tab only", ModuleAlias("\t"), false, true},
		{"multiple spaces", ModuleAlias("   "), false, true},
		{"newline only", ModuleAlias("\n"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.alias.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleAlias(%q).IsValid() = %v, want %v", tt.alias, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleAlias(%q).IsValid() returned no errors, want error", tt.alias)
				}
				if !errors.Is(errs[0], ErrInvalidModuleAlias) {
					t.Errorf("error should wrap ErrInvalidModuleAlias, got: %v", errs[0])
				}
				var aliasErr *InvalidModuleAliasError
				if !errors.As(errs[0], &aliasErr) {
					t.Errorf("error should be *InvalidModuleAliasError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleAlias(%q).IsValid() returned unexpected errors: %v", tt.alias, errs)
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

func TestModuleNamespace_IsValid(t *testing.T) {
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
			isValid, errs := tt.ns.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleNamespace(%q).IsValid() = %v, want %v", tt.ns, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleNamespace(%q).IsValid() returned no errors, want error", tt.ns)
				}
				if !errors.Is(errs[0], ErrInvalidModuleNamespace) {
					t.Errorf("error should wrap ErrInvalidModuleNamespace, got: %v", errs[0])
				}
				var nsErr *InvalidModuleNamespaceError
				if !errors.As(errs[0], &nsErr) {
					t.Errorf("error should be *InvalidModuleNamespaceError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleNamespace(%q).IsValid() returned unexpected errors: %v", tt.ns, errs)
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

func TestSubdirectoryPath_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    SubdirectoryPath
		want    bool
		wantErr bool
	}{
		{"empty is valid (repo root)", SubdirectoryPath(""), true, false},
		{"simple subdir", SubdirectoryPath("modules/tools"), true, false},
		{"single segment", SubdirectoryPath("tools"), true, false},
		{"nested path", SubdirectoryPath("a/b/c/d"), true, false},
		{"dot prefix", SubdirectoryPath("./tools"), true, false},
		{"path traversal", SubdirectoryPath("../escape"), false, true},
		{"nested path traversal", SubdirectoryPath("a/../../escape"), false, true},
		{"windows-style path traversal", SubdirectoryPath(`a\..\..\escape`), false, true},
		{"absolute unix path", SubdirectoryPath("/absolute/path"), false, true},
		{"absolute windows drive path", SubdirectoryPath(`C:\absolute\path`), false, true},
		{"absolute windows root path", SubdirectoryPath(`\absolute\path`), false, true},
		{"absolute windows unc path", SubdirectoryPath(`\\server\share`), false, true},
		{"null byte", SubdirectoryPath("path\x00evil"), false, true},
		{"too long", SubdirectoryPath(strings.Repeat("a", MaxPathLength+1)), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.path.IsValid()
			if isValid != tt.want {
				t.Errorf("SubdirectoryPath(%q).IsValid() = %v, want %v", tt.path, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("SubdirectoryPath(%q).IsValid() returned no errors, want error", tt.path)
				}
				if !errors.Is(errs[0], ErrInvalidSubdirectoryPath) {
					t.Errorf("error should wrap ErrInvalidSubdirectoryPath, got: %v", errs[0])
				}
				var pathErr *InvalidSubdirectoryPathError
				if !errors.As(errs[0], &pathErr) {
					t.Errorf("error should be *InvalidSubdirectoryPathError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("SubdirectoryPath(%q).IsValid() returned unexpected errors: %v", tt.path, errs)
			}
		})
	}
}

func TestSubdirectoryPath_String(t *testing.T) {
	t.Parallel()
	p := SubdirectoryPath("modules/tools")
	if p.String() != "modules/tools" {
		t.Errorf("SubdirectoryPath.String() = %q, want %q", p.String(), "modules/tools")
	}
}
