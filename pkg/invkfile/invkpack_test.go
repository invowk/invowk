// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================
// Tests for invkpack.cue parsing and validation
// ============================================

func TestParseInvkpack_ValidPackID(t *testing.T) {
	// Tests valid pack IDs in invkpack.cue (pack metadata file)
	tests := []struct {
		name string
		pack string
	}{
		{"simple lowercase", "mypack"},
		{"simple uppercase", "MyPack"},
		{"with numbers", "pack1"},
		{"dotted two parts", "my.pack"},
		{"dotted three parts", "my.nested.pack"},
		{"single letter", "a"},
		{"single letter with dotted", "a.b.c"},
		{"mixed case with dots", "My.Nested.Pack1"},
		{"rdns style", "io.invowk.sample"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pack metadata is now in invkpack.cue, not invkfile.cue
			cueContent := `
pack: "` + tt.pack + `"
version: "1.0"
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			invkpackPath := filepath.Join(tmpDir, "invkpack.cue")
			if err := os.WriteFile(invkpackPath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkpack.cue: %v", err)
			}

			inv, err := ParseInvkpack(invkpackPath)
			if err != nil {
				t.Fatalf("ParseInvkpack() error = %v", err)
			}

			if inv.Pack != tt.pack {
				t.Errorf("Pack = %q, want %q", inv.Pack, tt.pack)
			}
		})
	}
}

func TestParseInvkpack_InvalidPackID(t *testing.T) {
	// Tests invalid pack IDs in invkpack.cue are rejected
	tests := []struct {
		name string
		pack string
	}{
		{"starts with dot", ".pack"},
		{"ends with dot", "pack."},
		{"consecutive dots", "my..pack"},
		{"starts with number", "1pack"},
		{"contains hyphen", "my-pack"},
		{"contains underscore", "my_pack"},
		{"contains space", "my pack"},
		{"empty string", ""},
		{"only dots", "..."},
		{"dot then number", "a.1b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pack metadata is now in invkpack.cue
			cueContent := `
pack: "` + tt.pack + `"
version: "1.0"
`
			tmpDir, err := os.MkdirTemp("", "invowk-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			invkpackPath := filepath.Join(tmpDir, "invkpack.cue")
			if err := os.WriteFile(invkpackPath, []byte(cueContent), 0644); err != nil {
				t.Fatalf("Failed to write invkpack.cue: %v", err)
			}

			_, err = ParseInvkpack(invkpackPath)
			if err == nil {
				t.Errorf("ParseInvkpack() should reject invalid pack %q", tt.pack)
			}
		})
	}
}

func TestParseInvkpack(t *testing.T) {
	t.Run("valid invkpack with all fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		invkpackPath := filepath.Join(tmpDir, "invkpack.cue")
		content := `pack: "io.example.mypack"
version: "1.0"
description: "A test pack"
requires: [
	{git_url: "https://github.com/example/utils.git", version: "^1.0.0"},
]
`
		if err := os.WriteFile(invkpackPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write invkpack.cue: %v", err)
		}

		meta, err := ParseInvkpack(invkpackPath)
		if err != nil {
			t.Fatalf("ParseInvkpack() returned error: %v", err)
		}

		if meta.Pack != "io.example.mypack" {
			t.Errorf("Pack = %q, want %q", meta.Pack, "io.example.mypack")
		}
		if meta.Version != "1.0" {
			t.Errorf("Version = %q, want %q", meta.Version, "1.0")
		}
		if meta.Description != "A test pack" {
			t.Errorf("Description = %q, want %q", meta.Description, "A test pack")
		}
		if len(meta.Requires) != 1 {
			t.Errorf("Requires length = %d, want 1", len(meta.Requires))
		}
		if meta.FilePath != invkpackPath {
			t.Errorf("FilePath = %q, want %q", meta.FilePath, invkpackPath)
		}
	})

	t.Run("minimal invkpack (pack only)", func(t *testing.T) {
		tmpDir := t.TempDir()
		invkpackPath := filepath.Join(tmpDir, "invkpack.cue")
		content := `pack: "mypack"
`
		if err := os.WriteFile(invkpackPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write invkpack.cue: %v", err)
		}

		meta, err := ParseInvkpack(invkpackPath)
		if err != nil {
			t.Fatalf("ParseInvkpack() returned error: %v", err)
		}

		if meta.Pack != "mypack" {
			t.Errorf("Pack = %q, want %q", meta.Pack, "mypack")
		}
	})

	t.Run("invalid invkpack - missing pack", func(t *testing.T) {
		tmpDir := t.TempDir()
		invkpackPath := filepath.Join(tmpDir, "invkpack.cue")
		content := `version: "1.0"
description: "Missing pack field"
`
		if err := os.WriteFile(invkpackPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write invkpack.cue: %v", err)
		}

		_, err := ParseInvkpack(invkpackPath)
		if err == nil {
			t.Error("ParseInvkpack() should return error for missing pack field")
		}
	})

	t.Run("invalid pack name format", func(t *testing.T) {
		tmpDir := t.TempDir()
		invkpackPath := filepath.Join(tmpDir, "invkpack.cue")
		content := `pack: "123invalid"
`
		if err := os.WriteFile(invkpackPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write invkpack.cue: %v", err)
		}

		_, err := ParseInvkpack(invkpackPath)
		if err == nil {
			t.Error("ParseInvkpack() should return error for invalid pack name")
		}
	})
}

func TestParsePackFull(t *testing.T) {
	t.Run("valid pack with both files", func(t *testing.T) {
		tmpDir := t.TempDir()
		packDir := filepath.Join(tmpDir, "mypack.invkpack")
		if err := os.MkdirAll(packDir, 0755); err != nil {
			t.Fatalf("failed to create pack dir: %v", err)
		}

		// Create invkpack.cue
		invkpackContent := `pack: "mypack"
version: "1.0"
description: "Test pack"
`
		if err := os.WriteFile(filepath.Join(packDir, "invkpack.cue"), []byte(invkpackContent), 0644); err != nil {
			t.Fatalf("failed to write invkpack.cue: %v", err)
		}

		// Create invkfile.cue
		invkfileContent := `cmds: [{name: "test", implementations: [{script: "echo test", runtimes: [{name: "native"}]}]}]`
		if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(invkfileContent), 0644); err != nil {
			t.Fatalf("failed to write invkfile.cue: %v", err)
		}

		pack, err := ParsePackFull(packDir)
		if err != nil {
			t.Fatalf("ParsePackFull() returned error: %v", err)
		}

		if pack.Metadata == nil {
			t.Fatal("Metadata should not be nil")
		}
		if pack.Metadata.Pack != "mypack" {
			t.Errorf("Metadata.Pack = %q, want %q", pack.Metadata.Pack, "mypack")
		}
		if pack.Commands == nil {
			t.Fatal("Commands should not be nil")
		}
		if len(pack.Commands.Commands) != 1 {
			t.Errorf("Commands.Commands length = %d, want 1", len(pack.Commands.Commands))
		}
		if pack.IsLibraryOnly {
			t.Error("IsLibraryOnly should be false")
		}
		if pack.PackPath != packDir {
			t.Errorf("PackPath = %q, want %q", pack.PackPath, packDir)
		}
	})

	t.Run("library-only pack (no invkfile.cue)", func(t *testing.T) {
		tmpDir := t.TempDir()
		packDir := filepath.Join(tmpDir, "mylib.invkpack")
		if err := os.MkdirAll(packDir, 0755); err != nil {
			t.Fatalf("failed to create pack dir: %v", err)
		}

		// Create only invkpack.cue
		invkpackContent := `pack: "mylib"
version: "1.0"
`
		if err := os.WriteFile(filepath.Join(packDir, "invkpack.cue"), []byte(invkpackContent), 0644); err != nil {
			t.Fatalf("failed to write invkpack.cue: %v", err)
		}

		pack, err := ParsePackFull(packDir)
		if err != nil {
			t.Fatalf("ParsePackFull() returned error: %v", err)
		}

		if pack.Metadata == nil {
			t.Fatal("Metadata should not be nil")
		}
		if pack.Commands != nil {
			t.Error("Commands should be nil for library-only pack")
		}
		if !pack.IsLibraryOnly {
			t.Error("IsLibraryOnly should be true")
		}
	})

	t.Run("missing invkpack.cue fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		packDir := filepath.Join(tmpDir, "mypack.invkpack")
		if err := os.MkdirAll(packDir, 0755); err != nil {
			t.Fatalf("failed to create pack dir: %v", err)
		}

		// Only create invkfile.cue (no invkpack.cue)
		invkfileContent := `cmds: []`
		if err := os.WriteFile(filepath.Join(packDir, "invkfile.cue"), []byte(invkfileContent), 0644); err != nil {
			t.Fatalf("failed to write invkfile.cue: %v", err)
		}

		_, err := ParsePackFull(packDir)
		if err == nil {
			t.Error("ParsePackFull() should return error when invkpack.cue is missing")
		}
	})
}

// ============================================
// Tests for CommandScope (command call restriction)
// ============================================

func TestCommandScope_CanCall(t *testing.T) {
	scope := &CommandScope{
		PackID:      "io.example.mypack",
		GlobalPacks: map[string]bool{"global.tools": true},
		DirectDeps:  map[string]bool{"io.example.utils": true, "myalias": true},
	}

	tests := []struct {
		name       string
		targetCmd  string
		expectOK   bool
		expectDesc string
	}{
		{
			name:      "local command (no pack prefix)",
			targetCmd: "build",
			expectOK:  true,
		},
		{
			name:      "command from same pack",
			targetCmd: "io.example.mypack test",
			expectOK:  true,
		},
		{
			name:      "command from global pack",
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
			name:       "command from unknown pack",
			targetCmd:  "unknown.pack cmd",
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

func TestExtractPackFromCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected string
	}{
		{"io.invowk.sample hello", "io.invowk.sample"},
		{"utils@1.2.3 build", "utils@1.2.3"},
		{"build", ""},
		{"", ""},
		{"singleword", ""},
		{"pack.name command with args", "pack.name"},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := ExtractPackFromCommand(tt.cmd)
			if result != tt.expected {
				t.Errorf("ExtractPackFromCommand(%q) = %q, want %q", tt.cmd, result, tt.expected)
			}
		})
	}
}

func TestNewCommandScope(t *testing.T) {
	globalIDs := []string{"global.pack1", "global.pack2"}
	requirements := []PackRequirement{
		{GitURL: "https://github.com/example/dep1.git", Version: "^1.0.0"},
		{GitURL: "https://github.com/example/dep2.git", Version: "^2.0.0", Alias: "dep2alias"},
	}

	scope := NewCommandScope("mypack", globalIDs, requirements)

	if scope.PackID != "mypack" {
		t.Errorf("PackID = %q, want %q", scope.PackID, "mypack")
	}

	// Check global packs are set
	if !scope.GlobalPacks["global.pack1"] {
		t.Error("global.pack1 should be in GlobalPacks")
	}
	if !scope.GlobalPacks["global.pack2"] {
		t.Error("global.pack2 should be in GlobalPacks")
	}

	// Check aliased dependency is set
	if !scope.DirectDeps["dep2alias"] {
		t.Error("dep2alias should be in DirectDeps")
	}
}

func TestCommandScope_AddDirectDep(t *testing.T) {
	scope := &CommandScope{
		PackID:      "mypack",
		GlobalPacks: make(map[string]bool),
		DirectDeps:  make(map[string]bool),
	}

	scope.AddDirectDep("newdep")

	if !scope.DirectDeps["newdep"] {
		t.Error("newdep should be in DirectDeps after AddDirectDep")
	}
}
