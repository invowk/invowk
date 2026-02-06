// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// Tests for invkmod.cue parsing and validation
// ============================================

func TestParseInvkmod_ValidModuleID(t *testing.T) {
	t.Parallel()

	// Tests valid module IDs in invkmod.cue (module metadata file)
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

			// Module metadata is now in invkmod.cue, not invkfile.cue
			cueContent := `
module: "` + tt.module + `"
version: "1.0"
`
			tmpDir := t.TempDir()

			invkmodPath := filepath.Join(tmpDir, "invkmod.cue")
			if writeErr := os.WriteFile(invkmodPath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkmod.cue: %v", writeErr)
			}

			inv, err := ParseInvkmod(invkmodPath)
			if err != nil {
				t.Fatalf("ParseInvkmod() error = %v", err)
			}

			if inv.Module != tt.module {
				t.Errorf("Module = %q, want %q", inv.Module, tt.module)
			}
		})
	}
}

func TestParseInvkmod_InvalidModuleID(t *testing.T) {
	t.Parallel()

	// Tests invalid module IDs in invkmod.cue are rejected
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

			// Module metadata is now in invkmod.cue
			cueContent := `
module: "` + tt.module + `"
version: "1.0"
`
			tmpDir := t.TempDir()

			invkmodPath := filepath.Join(tmpDir, "invkmod.cue")
			if writeErr := os.WriteFile(invkmodPath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkmod.cue: %v", writeErr)
			}

			_, parseErr := ParseInvkmod(invkmodPath)
			if parseErr == nil {
				t.Errorf("ParseInvkmod() should reject invalid module %q", tt.module)
			}
		})
	}
}

func TestParseInvkmod(t *testing.T) {
	t.Parallel()

	t.Run("valid invkmod with all fields", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invkmodPath := filepath.Join(tmpDir, "invkmod.cue")
		content := `module: "io.example.mymodule"
version: "1.0"
description: "A test module"
requires: [
	{git_url: "https://github.com/example/utils.git", version: "^1.0.0"},
]
`
		if err := os.WriteFile(invkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invkmod.cue: %v", err)
		}

		meta, err := ParseInvkmod(invkmodPath)
		if err != nil {
			t.Fatalf("ParseInvkmod() returned error: %v", err)
		}

		if meta.Module != "io.example.mymodule" {
			t.Errorf("Module = %q, want %q", meta.Module, "io.example.mymodule")
		}
		if meta.Version != "1.0" {
			t.Errorf("Version = %q, want %q", meta.Version, "1.0")
		}
		if meta.Description != "A test module" {
			t.Errorf("Description = %q, want %q", meta.Description, "A test module")
		}
		if len(meta.Requires) != 1 {
			t.Errorf("Requires length = %d, want 1", len(meta.Requires))
		}
		if meta.FilePath != invkmodPath {
			t.Errorf("FilePath = %q, want %q", meta.FilePath, invkmodPath)
		}
	})

	t.Run("minimal invkmod (module only)", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invkmodPath := filepath.Join(tmpDir, "invkmod.cue")
		content := `module: "mymodule"
`
		if err := os.WriteFile(invkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invkmod.cue: %v", err)
		}

		meta, err := ParseInvkmod(invkmodPath)
		if err != nil {
			t.Fatalf("ParseInvkmod() returned error: %v", err)
		}

		if meta.Module != "mymodule" {
			t.Errorf("Module = %q, want %q", meta.Module, "mymodule")
		}
	})

	t.Run("invalid invkmod - missing module", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invkmodPath := filepath.Join(tmpDir, "invkmod.cue")
		content := `version: "1.0"
description: "Missing module field"
`
		if err := os.WriteFile(invkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invkmod.cue: %v", err)
		}

		_, err := ParseInvkmod(invkmodPath)
		if err == nil {
			t.Error("ParseInvkmod() should return error for missing module field")
		}
	})

	t.Run("invalid module name format", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		invkmodPath := filepath.Join(tmpDir, "invkmod.cue")
		content := `module: "123invalid"
`
		if err := os.WriteFile(invkmodPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write invkmod.cue: %v", err)
		}

		_, err := ParseInvkmod(invkmodPath)
		if err == nil {
			t.Error("ParseInvkmod() should return error for invalid module name")
		}
	})
}

func TestParseModuleMetadataOnly(t *testing.T) {
	t.Parallel()

	t.Run("existing invkmod.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		moduleDir := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}

		// Create invkmod.cue
		invkmodContent := `module: "mymodule"
version: "1.0"
`
		if err := os.WriteFile(filepath.Join(moduleDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
			t.Fatalf("failed to write invkmod.cue: %v", err)
		}

		meta, err := ParseModuleMetadataOnly(moduleDir)
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

	t.Run("missing invkmod.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		moduleDir := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.MkdirAll(moduleDir, 0o755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}

		meta, err := ParseModuleMetadataOnly(moduleDir)
		if !errors.Is(err, ErrInvkmodNotFound) {
			t.Errorf("ParseModuleMetadataOnly() should return ErrInvkmodNotFound, got: %v", err)
		}

		if meta != nil {
			t.Error("ParseModuleMetadataOnly() should return nil for missing invkmod.cue")
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
		GlobalModules: map[string]bool{"global.tools": true},
		DirectDeps:    map[string]bool{"io.example.utils": true, "myalias": true},
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

	globalIDs := []string{"global.module1", "global.module2"}
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
		GlobalModules: make(map[string]bool),
		DirectDeps:    make(map[string]bool),
	}

	scope.AddDirectDep("newdep")

	if !scope.DirectDeps["newdep"] {
		t.Error("newdep should be in DirectDeps after AddDirectDep")
	}
}

func TestHasInvkfile(t *testing.T) {
	t.Parallel()

	t.Run("with invkfile.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "invkfile.cue"), []byte("cmds: []"), 0o644); err != nil {
			t.Fatalf("failed to create invkfile.cue: %v", err)
		}

		if !HasInvkfile(tmpDir) {
			t.Error("HasInvkfile() should return true when invkfile.cue exists")
		}
	})

	t.Run("without invkfile.cue", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		if HasInvkfile(tmpDir) {
			t.Error("HasInvkfile() should return false when invkfile.cue doesn't exist")
		}
	})
}

func TestPathHelpers(t *testing.T) {
	t.Parallel()

	moduleDir := "/some/path/mymodule.invkmod"

	invkfilePath := InvkfilePath(moduleDir)
	if invkfilePath != filepath.Join(moduleDir, "invkfile.cue") {
		t.Errorf("InvkfilePath() = %q, want %q", invkfilePath, filepath.Join(moduleDir, "invkfile.cue"))
	}

	invkmodPath := InvkmodPath(moduleDir)
	if invkmodPath != filepath.Join(moduleDir, "invkmod.cue") {
		t.Errorf("InvkmodPath() = %q, want %q", invkmodPath, filepath.Join(moduleDir, "invkmod.cue"))
	}
}
