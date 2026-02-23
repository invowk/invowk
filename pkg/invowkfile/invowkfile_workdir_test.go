// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for GetEffectiveWorkDir (Working Directory Hierarchy)
// ============================================================================

func TestGetEffectiveWorkDir_DefaultToInvowkfileDir(t *testing.T) {
	t.Parallel()

	// When no workdir is specified at any level, defaults to invowkfile directory
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")

	if result != tmpDir {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, tmpDir)
	}
}

func TestGetEffectiveWorkDir_RootLevel(t *testing.T) {
	t.Parallel()

	// When only root-level workdir is specified
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "build",
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "build")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_CommandLevel(t *testing.T) {
	t.Parallel()

	// Command-level workdir overrides root-level
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "cmd-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_ImplementationLevel(t *testing.T) {
	t.Parallel()

	// Implementation-level workdir overrides command and root levels
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}
	impl := &Implementation{Script: "echo test", WorkDir: "impl-workdir"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "impl-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_CLIOverride(t *testing.T) {
	t.Parallel()

	// CLI override takes highest precedence
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}
	impl := &Implementation{Script: "echo test", WorkDir: "impl-workdir"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "cli-workdir")
	expected := filepath.Join(tmpDir, "cli-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_AbsolutePath(t *testing.T) {
	t.Parallel()

	// Absolute paths should be returned as-is (not joined with invowkfile dir)
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	absPath := filepath.Join(t.TempDir(), "absolute-workdir")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  WorkDir(absPath),
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")

	if result != absPath {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, absPath)
	}
}

func TestGetEffectiveWorkDir_ForwardSlashConversion(t *testing.T) {
	t.Parallel()

	// Forward slashes in CUE should be converted to native path separator
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "nested/deep/path", // Forward slashes (CUE format)
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "nested", "deep", "path")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_NilCommand(t *testing.T) {
	t.Parallel()

	// Should handle nil command gracefully
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "root-workdir",
	}

	result := inv.GetEffectiveWorkDir(nil, nil, "")
	expected := filepath.Join(tmpDir, "root-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_NilImplementation(t *testing.T) {
	t.Parallel()

	// Should handle nil implementation gracefully
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
	}
	cmd := &Command{Name: "test", WorkDir: "cmd-workdir"}

	result := inv.GetEffectiveWorkDir(cmd, nil, "")
	expected := filepath.Join(tmpDir, "cmd-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_EmptyCommandWorkDir(t *testing.T) {
	t.Parallel()

	// Empty command workdir should fall through to root level
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "root-workdir",
	}
	cmd := &Command{Name: "test", WorkDir: ""} // Empty command workdir
	impl := &Implementation{Script: "echo test", WorkDir: ""}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "root-workdir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_ParentDirectory(t *testing.T) {
	t.Parallel()

	// Relative paths with .. should work correctly
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	invowkfilePath := filepath.Join(subDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  "../sibling", // Go up and into sibling directory
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(subDir, "..", "sibling")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_CurrentDirectory(t *testing.T) {
	t.Parallel()

	// "." should resolve to invowkfile directory
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath: FilesystemPath(invowkfilePath),
		WorkDir:  ".",
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, ".")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

func TestGetEffectiveWorkDir_ModulePath(t *testing.T) {
	t.Parallel()

	// When loaded from a module, paths should resolve against module directory
	tmpDir := t.TempDir()
	moduleDir := filepath.Join(tmpDir, "mymodule.invowkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("Failed to create module dir: %v", err)
	}
	invowkfilePath := filepath.Join(moduleDir, "invowkfile.cue")

	inv := &Invowkfile{
		FilePath:   FilesystemPath(invowkfilePath),
		ModulePath: FilesystemPath(moduleDir), // Loaded from module
		WorkDir:    "scripts",
	}
	cmd := &Command{Name: "test"}
	impl := &Implementation{Script: "echo test"}

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	// Should resolve against module directory (via GetScriptBasePath)
	expected := filepath.Join(moduleDir, "scripts")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

// ============================================================================
// Tests for Parsing workdir from CUE
// ============================================================================

func TestParseWorkDir_RootLevel(t *testing.T) {
	t.Parallel()

	// Test parsing workdir from CUE at root level
	cueContent := `
workdir: "build/output"

cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.WorkDir != "build/output" {
		t.Errorf("Invowkfile.WorkDir = %q, want %q", inv.WorkDir, "build/output")
	}
}

func TestParseWorkDir_CommandLevel(t *testing.T) {
	t.Parallel()

	// Test parsing workdir from CUE at command level
	cueContent := `
cmds: [
	{
		name: "test"
		workdir: "cmd-specific"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Commands[0].WorkDir != "cmd-specific" {
		t.Errorf("Command.WorkDir = %q, want %q", inv.Commands[0].WorkDir, "cmd-specific")
	}
}

func TestParseWorkDir_ImplementationLevel(t *testing.T) {
	t.Parallel()

	// Test parsing workdir from CUE at implementation level
	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				workdir: "impl-specific"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if inv.Commands[0].Implementations[0].WorkDir != "impl-specific" {
		t.Errorf("Implementation.WorkDir = %q, want %q", inv.Commands[0].Implementations[0].WorkDir, "impl-specific")
	}
}

func TestParseWorkDir_AllLevels(t *testing.T) {
	t.Parallel()

	// Test parsing workdir at all levels and verify precedence
	cueContent := `
workdir: "root-dir"

cmds: [
	{
		name: "test"
		workdir: "cmd-dir"
		implementations: [
			{
				script: "echo test"
				workdir: "impl-dir"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify all levels are parsed correctly
	if inv.WorkDir != "root-dir" {
		t.Errorf("Invowkfile.WorkDir = %q, want %q", inv.WorkDir, "root-dir")
	}
	if inv.Commands[0].WorkDir != "cmd-dir" {
		t.Errorf("Command.WorkDir = %q, want %q", inv.Commands[0].WorkDir, "cmd-dir")
	}
	if inv.Commands[0].Implementations[0].WorkDir != "impl-dir" {
		t.Errorf("Implementation.WorkDir = %q, want %q", inv.Commands[0].Implementations[0].WorkDir, "impl-dir")
	}

	// Verify precedence using GetEffectiveWorkDir
	cmd := &inv.Commands[0]
	impl := &inv.Commands[0].Implementations[0]

	result := inv.GetEffectiveWorkDir(cmd, impl, "")
	expected := filepath.Join(tmpDir, "impl-dir")

	if result != expected {
		t.Errorf("GetEffectiveWorkDir() = %q, want %q", result, expected)
	}
}

// ============================================================================
// Tests for GenerateCUE with workdir
// ============================================================================

func TestGenerateCUE_WithWorkDir(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		WorkDir: "build",
		Commands: []Command{
			{
				Name:    "deploy",
				WorkDir: "deploy-dir",
				Implementations: []Implementation{
					{
						Script:  "echo deploying",
						WorkDir: "impl-dir",

						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check root-level workdir
	if !strings.Contains(output, `workdir: "build"`) {
		t.Error("GenerateCUE should contain root-level 'workdir: \"build\"'")
	}

	// Check command-level workdir
	if !strings.Contains(output, `workdir: "deploy-dir"`) {
		t.Error("GenerateCUE should contain command-level 'workdir: \"deploy-dir\"'")
	}

	// Check implementation-level workdir
	if !strings.Contains(output, `workdir: "impl-dir"`) {
		t.Error("GenerateCUE should contain implementation-level 'workdir: \"impl-dir\"'")
	}
}

func TestGenerateCUE_WithWorkDir_RoundTrip(t *testing.T) {
	t.Parallel()

	// Create an invowkfile with workdir at all levels, generate CUE, parse it back, and verify
	original := &Invowkfile{
		WorkDir: "root-workdir",
		Commands: []Command{
			{
				Name:    "build",
				WorkDir: "cmd-workdir",
				Implementations: []Implementation{
					{
						Script:  "echo building",
						WorkDir: "impl-workdir",

						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	// Generate CUE
	cueContent := GenerateCUE(original)

	// Write to temp file and parse back
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	// Verify parsed workdir values match original
	if parsed.WorkDir != "root-workdir" {
		t.Errorf("Invowkfile.WorkDir = %q, want %q", parsed.WorkDir, "root-workdir")
	}
	if parsed.Commands[0].WorkDir != "cmd-workdir" {
		t.Errorf("Command.WorkDir = %q, want %q", parsed.Commands[0].WorkDir, "cmd-workdir")
	}
	if parsed.Commands[0].Implementations[0].WorkDir != "impl-workdir" {
		t.Errorf("Implementation.WorkDir = %q, want %q", parsed.Commands[0].Implementations[0].WorkDir, "impl-workdir")
	}
}
