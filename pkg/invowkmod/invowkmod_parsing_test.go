// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"os"
	"path/filepath"
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

func TestParseModuleMetadataOnly(t *testing.T) {
	t.Parallel()

	t.Run("existing invowkmod.cue", testParseModuleMetadataOnlyExisting)
	t.Run("missing invowkmod.cue", testParseModuleMetadataOnlyMissing)
}

// ============================================
// Tests for CommandScope (command call restriction)
// ============================================

func TestParseInvowkmod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "valid invowkmod with all fields", run: testParseInvowkmodAllFields},
		{name: "minimal invowkmod (required fields only)", run: testParseInvowkmodMinimal},
		{name: "invalid invowkmod - missing version", run: rejectInvowkmod(`module: "mymodule"
`, "missing version field")},
		{name: "invalid invowkmod - missing module", run: rejectInvowkmod(`version: "1.0.0"
description: "Missing module field"
`, "missing module field")},
		{name: "invalid metadata version - v prefix", run: rejectInvowkmod(`module: "mymodule"
version: "v1.0.0"
`, "v-prefixed metadata version")},
		{name: "invalid metadata version - partial", run: rejectInvowkmod(`module: "mymodule"
version: "1.0"
`, "partial metadata version")},
		{name: "invalid requirement version - v prefix", run: rejectInvowkmod(`module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "v1.0.0"},
]
`, "v-prefixed requirement version")},
		{name: "valid requirement versions with comparison operators", run: testParseInvowkmodComparisonOperators},
		{name: "invalid requirement version - trailing junk", run: rejectInvowkmod(`module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "1.0.0junk"},
]
`, "trailing junk in requirement version")},
		{name: "invalid requirement alias", run: rejectInvowkmod(`module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "^1.0.0", alias: "1tools"},
]
`, "invalid requirement alias")},
		{name: "invalid requirement path", run: rejectInvowkmod(`module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: "^1.0.0", path: "../tools.invowkmod"},
]
`, "invalid requirement path")},
		{name: "unsupported requirement URL scheme", run: rejectInvowkmod(`module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "http://github.com/example/tools.git", version: "^1.0.0"},
]
`, "unsupported URL scheme")},
		{name: "full metadata validation rejects invalid load path", run: testParseInvowkmodInvalidLoadPath},
		{name: "invalid module name format", run: rejectInvowkmod(`module: "123invalid"
`, "invalid module name")},
	}
	//nolint:paralleltest // Each table case runner begins with t.Parallel.
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func testParseModuleMetadataOnlyExisting(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "mymodule.invowkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

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
}

func testParseModuleMetadataOnlyMissing(t *testing.T) {
	t.Parallel()

	moduleDir := filepath.Join(t.TempDir(), "mymodule.invowkmod")
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
}

func testParseInvowkmodAllFields(t *testing.T) { //nolint:thelper // Direct t.Run case body calls t.Parallel first.
	t.Parallel()

	content := `module: "io.example.mymodule"
version: "1.0.0"
description: "A test module"
requires: [
	{git_url: "https://github.com/example/utils.git", version: "^1.0.0"},
]
`
	invowkmodPath := writeInvowkmodFixture(t, content)
	meta, err := ParseInvowkmod(invowkmodPath)
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
	if meta.FilePath != invowkmodPath {
		t.Errorf("FilePath = %q, want %q", meta.FilePath, invowkmodPath)
	}
}

func testParseInvowkmodMinimal(t *testing.T) { //nolint:thelper // Direct t.Run case body calls t.Parallel first.
	t.Parallel()

	content := `module: "mymodule"
version: "1.0.0"
`
	meta, err := ParseInvowkmod(writeInvowkmodFixture(t, content))
	if err != nil {
		t.Fatalf("ParseInvowkmod() returned error: %v", err)
	}
	if meta.Module != "mymodule" {
		t.Errorf("Module = %q, want %q", meta.Module, "mymodule")
	}
}

func testParseInvowkmodComparisonOperators(t *testing.T) { //nolint:thelper // Direct t.Run case body calls t.Parallel first.
	t.Parallel()

	content := `module: "mymodule"
version: "1.0.0"
requires: [
	{git_url: "https://github.com/example/tools.git", version: ">=1.0.0"},
	{git_url: "ssh://git@example.com/utils.git", version: "<=2.0.0"},
]
`
	meta, err := ParseInvowkmod(writeInvowkmodFixture(t, content))
	if err != nil {
		t.Fatalf("ParseInvowkmod() returned error: %v", err)
	}
	if len(meta.Requires) != 2 {
		t.Fatalf("Requires length = %d, want 2", len(meta.Requires))
	}
}

func testParseInvowkmodInvalidLoadPath(t *testing.T) { //nolint:thelper // Direct t.Run case body calls t.Parallel first.
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
	for _, fieldErr := range invErr.FieldErrors {
		if errors.Is(fieldErr, types.ErrInvalidFilesystemPath) {
			return
		}
	}
	t.Fatalf("InvalidInvowkmodError.FieldErrors = %v, want ErrInvalidFilesystemPath", invErr.FieldErrors)
}

func rejectInvowkmod(content, failure string) func(*testing.T) {
	//nolint:thelper // Returned function is a direct t.Run case body and calls t.Parallel first.
	return func(t *testing.T) {
		t.Parallel()

		_, err := ParseInvowkmod(writeInvowkmodFixture(t, content))
		if err == nil {
			t.Errorf("ParseInvowkmod() should return error for %s", failure)
		}
	}
}

func writeInvowkmodFixture(t *testing.T, content string) types.FilesystemPath {
	t.Helper()

	invowkmodPath := filepath.Join(t.TempDir(), "invowkmod.cue")
	if err := os.WriteFile(invowkmodPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	return types.FilesystemPath(invowkmodPath)
}
