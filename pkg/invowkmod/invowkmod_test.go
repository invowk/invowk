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

	t.Run("invalid metadata version - v prefix", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "v1.0.0"
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for v-prefixed metadata version")
		}
	})

	t.Run("invalid metadata version - partial", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0"
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for partial metadata version")
		}
	})

	t.Run("invalid requirement version - v prefix", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "v1.0.0"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for v-prefixed requirement version")
		}
	})

	t.Run("valid requirement versions with comparison operators", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: ">=1.0.0"},
	{git_url: "ssh://git@example.com/utils.git", version: "<=2.0.0"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		meta, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err != nil {
			t.Fatalf("ParseInvowkmod() returned error: %v", err)
		}
		if len(meta.Requires) != 2 {
			t.Fatalf("Requires length = %d, want 2", len(meta.Requires))
		}
	})

	t.Run("invalid requirement version - trailing junk", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "1.0.0junk"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for trailing junk in requirement version")
		}
	})

	t.Run("invalid requirement alias", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "^1.0.0", alias: "1tools"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for invalid requirement alias")
		}
	})

	t.Run("invalid requirement path", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "^1.0.0", path: "../tools.invowkmod"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for invalid requirement path")
		}
	})

	t.Run("unsupported requirement URL scheme", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invowkmodPath := filepath.Join(tmpDir, "invowkmod.cue")
		content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "http://github.com/example/tools.git", version: "^1.0.0"},
]
`
		if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invowkmod.cue: %v", err)
		}

		_, err := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
		if err == nil {
			t.Error("ParseInvowkmod() should return error for unsupported URL scheme")
		}
	})

	t.Run("full metadata validation rejects invalid load path", func(t *testing.T) {
		t.Parallel()

		content := []byte(`module: "mymodule"
version: "1.0.0"
`)
		_, err := ParseInvowkmodBytes(content, " \t ")
		if err == nil {
			t.Error("ParseInvowkmodBytes() should return error for invalid metadata FilePath")
		}
		if !errors.Is(err, ErrInvalidInvowkmod) {
			t.Fatalf("ParseInvowkmodBytes() error = %v, want ErrInvalidInvowkmod", err)
		}
		var invErr *InvalidInvowkmodError
		if !errors.As(err, &invErr) {
			t.Fatalf("ParseInvowkmodBytes() error = %T, want InvalidInvowkmodError", err)
		}
		hasPathErr := false
		for _, fieldErr := range invErr.FieldErrors {
			if errors.Is(fieldErr, types.ErrInvalidFilesystemPath) {
				hasPathErr = true
			}
		}
		if !hasPathErr {
			t.Fatalf("InvalidInvowkmodError.FieldErrors = %v, want ErrInvalidFilesystemPath", invErr.FieldErrors)
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

func TestCommandScope_CanCallTargetUsesDiscoveryIdentity(t *testing.T) {
	t.Parallel()

	scope := NewCommandScope("io.example.caller")
	scope.AddDirectDependency("io.example.tools", "allowed-tools")
	scope.AddGlobalSource("global-tools")

	t.Run("allows resolved direct dependency source pair", func(t *testing.T) {
		t.Parallel()

		decision := scope.CanCallTarget(CommandTarget{
			Reference: "allowed-tools test",
			SourceID:  "allowed-tools",
			ModuleID:  "io.example.tools",
		})
		if !decision.Allowed {
			t.Fatalf("CanCallTarget() denied resolved source pair: %+v", decision)
		}
	})

	t.Run("denies presentation alias with unrelated discovery source", func(t *testing.T) {
		t.Parallel()

		decision := scope.CanCallTarget(CommandTarget{
			Reference: "allowed-tools test",
			SourceID:  "other-tools",
			ModuleID:  "io.example.tools",
		})
		if decision.Allowed {
			t.Fatalf("CanCallTarget() allowed mismatched source pair: %+v", decision)
		}
		if decision.TargetSource != "other-tools" {
			t.Fatalf("TargetSource = %q, want discovery source", decision.TargetSource)
		}
		if decision.Reason != CommandScopeDenyInaccessible {
			t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
		}
	})

	t.Run("denies discovered target without split identity pair", func(t *testing.T) {
		t.Parallel()

		decision := scope.CanCallTarget(CommandTarget{
			Reference: "allowed-tools test",
			SourceID:  "allowed-tools",
		})
		if decision.Allowed {
			t.Fatalf("CanCallTarget() allowed source-only direct dependency: %+v", decision)
		}
	})

	t.Run("allows discovered global command source", func(t *testing.T) {
		t.Parallel()

		decision := scope.CanCallTarget(CommandTarget{
			Reference: "global-tools lint",
			SourceID:  "global-tools",
			ModuleID:  "io.example.global",
		})
		if !decision.Allowed {
			t.Fatalf("CanCallTarget() denied global source: %+v", decision)
		}
	})

	t.Run("denies unrelated module whose source matches caller module id", func(t *testing.T) {
		t.Parallel()

		aliasedScope := NewCommandScope("io.example.caller")
		aliasedScope.ModuleSourceID = "caller-alias"

		decision := aliasedScope.CanCallTarget(CommandTarget{
			Reference: "io.example.caller test",
			SourceID:  "io.example.caller",
			ModuleID:  "io.example.other",
		})
		if decision.Allowed {
			t.Fatalf("CanCallTarget() allowed source-only same-module fallback: %+v", decision)
		}
		if decision.Reason != CommandScopeDenyInaccessible {
			t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
		}
	})

	t.Run("denies non-global source sharing a global module id", func(t *testing.T) {
		t.Parallel()

		globalScope := NewCommandScope("io.example.caller")

		decision := globalScope.CanCallTarget(CommandTarget{
			Reference: "local-global lint",
			SourceID:  "local-global",
			ModuleID:  "io.example.global",
		})
		if decision.Allowed {
			t.Fatalf("CanCallTarget() allowed module-id-only global fallback: %+v", decision)
		}
		if decision.Reason != CommandScopeDenyInaccessible {
			t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
		}
	})

	t.Run("denies source matching global module id without discovered global source", func(t *testing.T) {
		t.Parallel()

		globalScope := NewCommandScope("io.example.caller")

		decision := globalScope.CanCallTarget(CommandTarget{
			Reference: "io.example.global lint",
			SourceID:  "io.example.global",
			ModuleID:  "io.example.other",
		})
		if decision.Allowed {
			t.Fatalf("CanCallTarget() allowed global module id as source fallback: %+v", decision)
		}
		if decision.Reason != CommandScopeDenyInaccessible {
			t.Fatalf("Reason = %q, want %q", decision.Reason, CommandScopeDenyInaccessible)
		}
	})
}

func TestNewCommandScope(t *testing.T) {
	t.Parallel()

	scope := NewCommandScope("mymodule")

	if scope.ModuleID != "mymodule" {
		t.Errorf("ModuleID = %q, want %q", scope.ModuleID, "mymodule")
	}
	if scope.ModuleSourceID != "mymodule" {
		t.Errorf("ModuleSourceID = %q, want %q", scope.ModuleSourceID, "mymodule")
	}
	if scope.GlobalSources == nil {
		t.Fatal("GlobalSources should be initialized")
	}
	if len(scope.GlobalSources) != 0 {
		t.Errorf("GlobalSources length = %d, want 0", len(scope.GlobalSources))
	}
	if scope.DirectDependencySources == nil {
		t.Fatal("DirectDependencySources should be initialized")
	}
	if len(scope.DirectDependencySources) != 0 {
		t.Errorf("DirectDependencySources length = %d, want 0", len(scope.DirectDependencySources))
	}
}

func TestCommandScope_AddGlobalSource(t *testing.T) {
	t.Parallel()

	scope := &CommandScope{ModuleID: "mymodule"}

	scope.AddGlobalSource("global-tools")

	if !scope.GlobalSources["global-tools"] {
		t.Error("global-tools should be in GlobalSources after AddGlobalSource")
	}
}

func TestCommandScope_AddDirectDependency(t *testing.T) {
	t.Parallel()

	scope := &CommandScope{
		ModuleID: "mymodule",
	}

	scope.AddDirectDependency("io.example.newdep", "newdep")

	if !scope.DirectDependencySources["io.example.newdep"]["newdep"] {
		t.Error("io.example.newdep/newdep pair should be in DirectDependencySources after AddDirectDependency")
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
// Tests for Validate() methods on type definitions
// ============================================

func TestValidationIssueType_Validate(t *testing.T) {
	t.Parallel()

	validTypes := []ValidationIssueType{
		IssueTypeStructure, IssueTypeNaming, IssueTypeInvowkmod,
		IssueTypeSecurity, IssueTypeCompatibility, IssueTypeInvowkfile,
		IssueTypeCommandTree,
	}

	for _, vt := range validTypes {
		t.Run(string(vt), func(t *testing.T) {
			t.Parallel()

			err := vt.Validate()
			if err != nil {
				t.Errorf("ValidationIssueType(%q).Validate() returned unexpected error: %v", vt, err)
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

			err := vt.Validate()
			if err == nil {
				t.Fatalf("ValidationIssueType(%q).Validate() returned nil, want error", vt)
			}
			if !errors.Is(err, ErrInvalidValidationIssueType) {
				t.Errorf("error should wrap ErrInvalidValidationIssueType, got: %v", err)
			}
		})
	}
}

func TestModuleRequirement_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     ModuleRequirement
		want    bool
		wantErr bool
	}{
		{
			"all valid",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
			},
			true, false,
		},
		{
			"all valid with optional fields",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "^1.0.0",
				Alias:   "myalias",
				Path:    "subdir",
			},
			true, false,
		},
		{
			"optional fields empty are valid",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "~2.0.0",
				Alias:   "",
				Path:    "",
			},
			true, false,
		},
		{
			"invalid git url",
			ModuleRequirement{
				GitURL:  "not-a-url",
				Version: "^1.0.0",
			},
			false, true,
		},
		{
			"invalid version",
			ModuleRequirement{
				GitURL:  "https://github.com/user/repo.git",
				Version: "not-semver",
			},
			false, true,
		},
		{
			"multiple invalid fields",
			ModuleRequirement{
				GitURL:  "not-a-url",
				Version: "not-semver",
				Alias:   "   ",
				Path:    "../escape",
			},
			false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleRequirement.Validate() error = %v, wantValid %v", err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleRequirement.Validate() returned nil, want error")
				}
			} else if err != nil {
				t.Errorf("ModuleRequirement.Validate() returned unexpected error: %v", err)
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

func TestModuleID_Validate(t *testing.T) {
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

			err := tt.id.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ModuleID(%q).Validate() error = %v, wantValid %v", tt.id, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ModuleID(%q).Validate() returned nil, want error", tt.id)
				}
				if !errors.Is(err, ErrInvalidModuleID) {
					t.Errorf("error should wrap ErrInvalidModuleID, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("ModuleID(%q).Validate() returned unexpected error: %v", tt.id, err)
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

	moduleDir := types.FilesystemPath(filepath.Join(t.TempDir(), "mymodule.invowkmod"))

	invowkfilePath := InvowkfilePath(moduleDir)
	if string(invowkfilePath) != filepath.Join(string(moduleDir), "invowkfile.cue") {
		t.Errorf("InvowkfilePath() = %q, want %q", invowkfilePath, filepath.Join(string(moduleDir), "invowkfile.cue"))
	}

	invowkmodPath := InvowkmodPath(moduleDir)
	if string(invowkmodPath) != filepath.Join(string(moduleDir), "invowkmod.cue") {
		t.Errorf("InvowkmodPath() = %q, want %q", invowkmodPath, filepath.Join(string(moduleDir), "invowkmod.cue"))
	}
}
