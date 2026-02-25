// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

// ============================================
// Tests for invowkmod.cue parsing and validation
// ============================================

func TestParseInvowkmod_ValidModuleID(t *testing.T) {
	t.Parallel()

	// Tests valid module IDs in invowkmod.cue (module metadata file)
	tests := []struct {
		name   string
		module string
	}{
		{"simple lowercase", "mymodule"},
		{"simple uppercase", "MyModule"},
		{"with numbers", "module1"},
		{"dotted two parts", "my.module"},
		{"dotted three parts", "my.nested.module"},
		{"single letter", "a"},
		{"single letter with dotted", "a.b.c"},
		{"mixed case with dots", "My.Nested.Module1"},
		{"rdns style", "io.invowk.sample"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Module metadata is now in invowkmod.cue, not invowkfile.cue
			cueContent := `
module: "` + tt.module + `"
version: "1.0.0"
`
			tmpDir := t.TempDir()

			invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
			if writeErr := os.WriteFile(invowkmodPath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invowkmod.cue: %v", writeErr)
			}

			inv, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
			if err != nil {
				t.Fatalf("ParseInvowkmod() error = %v", err)
			}

			if string(inv.Module) != tt.module {
				t.Errorf("Module = %q, want %q", inv.Module, tt.module)
			}
		})
	}
}

func TestParseInvowkmod_InvalidModuleID(t *testing.T) {
	t.Parallel()

	// Tests invalid module IDs in invowkmod.cue are rejected
	tests := []struct {
		name   string
		module string
	}{
		{"starts with dot", ".module"},
		{"ends with dot", "module."},
		{"consecutive dots", "my..module"},
		{"starts with number", "1module"},
		{"contains hyphen", "my-module"},
		{"contains underscore", "my_module"},
		{"contains space", "my module"},
		{"empty string", ""},
		{"only dots", "..."},
		{"dot then number", "a.1b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Module metadata is now in invowkmod.cue
			cueContent := `
module: "` + tt.module + `"
version: "1.0.0"
`
			tmpDir := t.TempDir()

			invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
			if writeErr := os.WriteFile(invowkmodPath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invowkmod.cue: %v", writeErr)
			}

			_, parseErr := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
			if parseErr == nil {
				t.Errorf("ParseInvowkmod() should reject invalid module %q", tt.module)
			}
		})
	}
}

func TestParseInvowkmod(t *testing.T) {
	t.Parallel()

	t.Run("valid invowkmod with all fields", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "io.example.mymodule"
version: "1.0.0"
description: "A test module"
requires: [
	{git_url: "https://github.com/example/utils.git", version: "^1.0.0"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		meta, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err != nil {
			t.Fatalf("ParseInvowkmod() returned error: %v", err)
		}

		if meta.Module != "io.example.mymodule" {
			t.Errorf("Module = %q, want %q", meta.Module, "io.example.mymodule")
		}
		if meta.Version != "1.0.0" {
			t.Errorf("Version = %q, want %q", meta.Version, "1.0.0")
		}
		if meta.Description != "A test module" {
			t.Errorf("Description = %q, want %q", meta.Description, "A test module")
		}
		if len(meta.Requires) != 1 {
			t.Errorf("Requires length = %d, want 1", len(meta.Requires))
		}
		if string(meta.FilePath) != invowkmodPath {
			t.Errorf("FilePath = %q, want %q", meta.FilePath, invowkmodPath)
		}
	})

	t.Run("minimal invowkmod (required fields only)", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		meta, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err != nil {
			t.Fatalf("ParseInvowkmod() returned error: %v", err)
		}

		if meta.Module != "mymodule" {
			t.Errorf("Module = %q, want %q", meta.Module, "mymodule")
		}
	})

	t.Run("invalid invowkmod - missing version", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for missing version field")
		}
	})

	t.Run("invalid invowkmod - missing module", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `version: "1.0.0"
description: "Missing module field"
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for missing module field")
		}
	})

	t.Run("invalid module name format", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "123invalid"
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for invalid module name")
		}
	})
}

func TestParseModuleMetadataOnly(t *testing.T) {
	t.Parallel()

	t.Run("existing invowkmod.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		moduleDir := filepath.Join(tmpDir, "mymodule.invowkmod")
		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}

		// Create invowkmod.cue
		invowkmodContent := `module: "mymodule"
version: "1.0.0"
`
		if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		meta, err := ParseModuleMetadataOnly(types.FilesystemPath(moduleDir))
		if err != nil {
			t.Fatalf("ParseModuleMetadataOnly() returned error: %v", err)
		}

		if meta == nil {
			t.Fatal("ParseModuleMetadataOnly() should not return nil")
		}
		if meta.Module != "mymodule" {
			t.Errorf("Module = %q, want %q", meta.Module, "mymodule")
		}
	})

	t.Run("missing invowkmod.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		moduleDir := filepath.Join(tmpDir, "mymodule.invowkmod")
		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}

		meta, err := ParseModuleMetadataOnly(types.FilesystemPath(moduleDir))
		if !errors.Is(err, ErrInvowkmodNotFound) {
			t.Errorf("ParseModuleMetadataOnly() should return ErrInvowkmodNotFound, got: %v", err)
		}

		if meta != nil {
			t.Error("ParseModuleMetadataOnly() should return nil for missing invowkmod.cue")
		}
	})
}

// ============================================
// Tests for CommandScope (command call restriction)
// ============================================

func TestCommandScope_CanCall(t *testing.T) {
	t.Parallel()

	scope := &CommandScope{
		ModuleID:      "io.example.mymodule",
		GlobalModules: map[ModuleID]bool{"global.tools": true},
		DirectDeps:    map[ModuleID]bool{"io.example.utils": true, "myalias": true},
	}

	tests := []struct {
		name       string
		targetCmd  string
		expectOK   bool
		expectDesc string
	}{
		{
			name:      "local command (no module prefix)",
			targetCmd: "build",
			expectOK:  true,
		},
		{
			name:      "command from same module",
			targetCmd: "io.example.mymodule test",
			expectOK:  true,
		},
		{
			name:      "command from global module",
			targetCmd: "global.tools lint",
			expectOK:  true,
		},
		{
			name:      "command from direct dependency",
			targetCmd: "io.example.utils helper",
			expectOK:  true,
		},
		{
			name:      "command from aliased dependency",
			targetCmd: "myalias run",
			expectOK:  true,
		},
		{
			name:       "command from unknown module",
			targetCmd:  "unknown.module cmd",
			expectOK:   false,
			expectDesc: "not accessible",
		},
		{
			name:       "transitive dependency (not allowed)",
			targetCmd:  "transitive.dep cmd",
			expectOK:   false,
			expectDesc: "not accessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			allowed, reason := scope.CanCall(tt.targetCmd)
			if allowed != tt.expectOK {
				t.Errorf("CanCall(%q) = %v, want %v", tt.targetCmd, allowed, tt.expectOK)
			}
			if !tt.expectOK && tt.expectDesc != "" && !strings.Contains(reason, tt.expectDesc) {
				t.Errorf("reason should contain %q, got %q", tt.expectDesc, reason)
			}
		})
	}
}

func TestExtractModuleFromCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cmd      string
		expected string
	}{
		{"io.invowk.sample hello", "io.invowk.sample"},
		{"utils@1.2.3 build", "utils@1.2.3"},
		{"build", ""},
		{"", ""},
		{"singleword", ""},
		{"module.name command with args", "module.name"},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			t.Parallel()

			result := ExtractModuleFromCommand(tt.cmd)
			if result != tt.expected {
				t.Errorf("ExtractModuleFromCommand(%q) = %q, want %q", tt.cmd, result, tt.expected)
			}
		})
	}
}

func TestNewCommandScope(t *testing.T) {
	t.Parallel()

	globalIDs := []ModuleID{"global.module1", "global.module2"}
	requirements := []ModuleRequirement{
		{GitURL: "https://github.com/example/dep1.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/example/dep2.git", Version: "^2.0.0", Alias: "dep2alias"},
	}

	scope := NewCommandScope("mymodule", globalIDs, requirements)

	if scope.ModuleID != "mymodule" {
		t.Errorf("ModuleID = %q, want %q", scope.ModuleID, "mymodule")
	}

	// Check global modules are set
	if !scope.GlobalModules["global.module1"] {
		t.Error("global.module1 should be in GlobalModules")
	}
	if !scope.GlobalModules["global.module2"] {
		t.Error("global.module2 should be in GlobalModules")
	}

	// Check aliased dependency is set
	if !scope.DirectDeps["dep2alias"] {
		t.Error("dep2alias should be in DirectDeps")
	}
}

func TestCommandScope_AddDirectDep(t *testing.T) {
	t.Parallel()

	scope := &CommandScope{
		ModuleID:      "mymodule",
		GlobalModules: make(map[ModuleID]bool),
		DirectDeps:    make(map[ModuleID]bool),
	}

	scope.AddDirectDep("newdep")

	if !scope.DirectDeps["newdep"] {
		t.Error("newdep should be in DirectDeps after AddDirectDep")
	}
}

func TestHasInvowkfile(t *testing.T) {
	t.Parallel()

	t.Run("with invowkfile.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte("cmds: []"), 0o644); err != nil {
			t.Fatalf("failed to create invowkfile.cue: %v", err)
		}

		if !HasInvowkfile(types.FilesystemPath(tmpDir)) {
			t.Error("HasInvowkfile() should return true when invowkfile.cue exists")
		}
	})

	t.Run("without invowkfile.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		if HasInvowkfile(types.FilesystemPath(tmpDir)) {
			t.Error("HasInvowkfile() should return false when invowkfile.cue doesn't exist")
		}
	})
}

// ============================================
// Tests for IsValid() methods on type definitions
// ============================================

func TestValidationIssueType_IsValid(t *testing.T) {
	t.Parallel()

	validTypes := []ValidationIssueType{
		IssueTypeStructure, IssueTypeNaming, IssueTypeInvowkmod,
		IssueTypeSecurity, IssueTypeCompatibility, IssueTypeInvowkfile,
		IssueTypeCommandTree,
	}

	for _, vt := range validTypes {
		t.Run(string(vt), func(t *testing.T) {
			t.Parallel()

			isValid, errs := vt.IsValid()
			if !isValid {
				t.Errorf("ValidationIssueType(%q).IsValid() = false, want true", vt)
			}
			if len(errs) > 0 {
				t.Errorf("ValidationIssueType(%q).IsValid() returned unexpected errors: %v", vt, errs)
			}
		})
	}

	invalidTypes := []ValidationIssueType{"", "invalid", "STRUCTURE"}
	for _, vt := range invalidTypes {
		name := string(vt)
		if name == "" {
			name = "empty"
		}

		t.Run("invalid_"+name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := vt.IsValid()
			if isValid {
				t.Errorf("ValidationIssueType(%q).IsValid() = true, want false", vt)
			}
			if len(errs) == 0 {
				t.Fatalf("ValidationIssueType(%q).IsValid() returned no errors, want error", vt)
			}
			if !errors.Is(errs[0], ErrInvalidValidationIssueType) {
				t.Errorf("error should wrap ErrInvalidValidationIssueType, got: %v", errs[0])
			}
		})
	}
}

func TestModuleRequirement_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		req      ModuleRequirement
		want     bool
		wantErrs int
		wantErr  bool
	}{
		{
			"all valid",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			true, 0, false,
		},
		{
			"all valid with optional fields",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Alias:   "myalias",
				Path:    "subdir",
			},
			true, 0, false,
		},
		{
			"optional fields empty are valid",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "~2.0.0",
				Alias:   "",
				Path:    "",
			},
			true, 0, false,
		},
		{
			"invalid git url",
			ModuleRequirement{
				GitURL:  "not-a-url",
				Version: "^1.0.0",
			},
			false, 1, true,
		},
		{
			"invalid version",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "not-semver",
			},
			false, 1, true,
		},
		{
			"multiple invalid fields",
			ModuleRequirement{
				GitURL:  "not-a-url",
				Version: "not-semver",
				Alias:   "   ",
				Path:    "../escape",
			},
			false, 4, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.req.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleRequirement.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleRequirement.IsValid() returned no errors, want error")
				}
				if tt.wantErrs > 0 && len(errs) != tt.wantErrs {
					t.Errorf("ModuleRequirement.IsValid() returned %d errors, want %d: %v",
						len(errs), tt.wantErrs, errs)
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleRequirement.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}

func TestValidationIssueType_String(t *testing.T) {
	t.Parallel()

	if got := IssueTypeStructure.String(); got != "structure" {
		t.Errorf("IssueTypeStructure.String() = %q, want %q", got, "structure")
	}
	if got := ValidationIssueType("").String(); got != "" {
		t.Errorf("ValidationIssueType(\"\").String() = %q, want %q", got, "")
	}
}

func TestModuleID_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      ModuleID
		want    bool
		wantErr bool
	}{
		{"simple", ModuleID("mymodule"), true, false},
		{"rdns two segments", ModuleID("io.invowk"), true, false},
		{"rdns three segments", ModuleID("io.invowk.sample"), true, false},
		{"single letter", ModuleID("a"), true, false},
		{"alphanumeric", ModuleID("foo123.bar456"), true, false},
		{"mixed case", ModuleID("My.Module1"), true, false},
		{"max length valid", ModuleID(strings.Repeat("a", MaxModuleIDLength)), true, false},
		{"empty", ModuleID(""), false, true},
		{"starts with digit", ModuleID("1module"), false, true},
		{"starts with dot", ModuleID(".module"), false, true},
		{"ends with dot", ModuleID("module."), false, true},
		{"consecutive dots", ModuleID("io..invowk"), false, true},
		{"contains hyphen", ModuleID("io.inv-owk"), false, true},
		{"contains underscore", ModuleID("io.inv_owk"), false, true},
		{"contains space", ModuleID("io invowk"), false, true},
		{"segment starts with digit", ModuleID("io.1invowk"), false, true},
		{"too long", ModuleID(strings.Repeat("a", MaxModuleIDLength+1)), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.id.IsValid()
			if isValid != tt.want {
				t.Errorf("ModuleID(%q).IsValid() = %v, want %v", tt.id, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ModuleID(%q).IsValid() returned no errors, want error", tt.id)
				}
				if !errors.Is(errs[0], ErrInvalidModuleID) {
					t.Errorf("error should wrap ErrInvalidModuleID, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ModuleID(%q).IsValid() returned unexpected errors: %v", tt.id, errs)
			}
		})
	}
}

func TestModuleID_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id   ModuleID
		want string
	}{
		{ModuleID("io.invowk.sample"), "io.invowk.sample"},
		{ModuleID("mymodule"), "mymodule"},
		{ModuleID(""), ""},
	}

	for _, tt := range tests {
		if got := tt.id.String(); got != tt.want {
			t.Errorf("ModuleID(%q).String() = %q, want %q", string(tt.id), got, tt.want)
		}
	}
}

func TestPathHelpers(t *testing.T) {
	t.Parallel()

	moduleDir := types.FilesystemPath("/some/path/mymodule.invowkmod")

	invowkfilePath := InvowkfilePath(moduleDir)
	if string(invowkfilePath) != filepath.Join(string(moduleDir), "invowkfile.cue") {
		t.Errorf("InvowkfilePath() = %q, want %q", invowkfilePath, filepath.Join(string(moduleDir), "invowkfile.cue"))
	}

	invowkmodPath := InvowkmodPath(moduleDir)
	if string(invowkmodPath) != filepath.Join(string(moduleDir), "invowkmod.cue") {
		t.Errorf("InvowkmodPath() = %q, want %q", invowkmodPath, filepath.Join(string(moduleDir), "invowkmod.cue"))
	}
}

func TestInvowkmod_IsValid(t *testing.T) {
	t.Parallel()

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
				FilePath: types.FilesystemPath("/home/user/modules/sample.invowkmod/invowkmod.cue"),
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
			isValid, errs := tt.mod.IsValid()
			if isValid != tt.want {
				t.Errorf("Invowkmod.IsValid() = %v, want %v", isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Invowkmod.IsValid() returned no errors, want error")
				}
				if !errors.Is(errs[0], ErrInvalidInvowkmod) {
					t.Errorf("error should wrap ErrInvalidInvowkmod, got: %v", errs[0])
				}
				var modErr *InvalidInvowkmodError
				if !errors.As(errs[0], &modErr) {
					t.Fatalf("error should be *InvalidInvowkmodError, got: %T", errs[0])
				}
				if len(modErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(modErr.FieldErrors), tt.wantCount)
				}
			} else if len(errs) > 0 {
				t.Errorf("Invowkmod.IsValid() returned unexpected errors: %v", errs)
			}
		})
	}
}
