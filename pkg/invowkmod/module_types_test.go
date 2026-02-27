// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"strings"
	"testing"
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
		{"single char", ModuleAlias("a"), true, false},
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
			err := tt.path.Validate()
			if (err == nil) != tt.want {
				t.Errorf("SubdirectoryPath(%q).Validate() error = %v, wantValid %v", tt.path, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SubdirectoryPath(%q).Validate() returned nil, want error", tt.path)
				}
				if !errors.Is(err, ErrInvalidSubdirectoryPath) {
					t.Errorf("error should wrap ErrInvalidSubdirectoryPath, got: %v", err)
				}
				var pathErr *InvalidSubdirectoryPathError
				if !errors.As(err, &pathErr) {
					t.Errorf("error should be *InvalidSubdirectoryPathError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("SubdirectoryPath(%q).Validate() returned unexpected error: %v", tt.path, err)
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
