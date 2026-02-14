// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ============================================================================
// Filepath Dependency Tests
// ============================================================================

func TestCheckFilepathDependencies_NoFilepaths(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test", invowkfiletest.WithScript("echo hello"))

	err := checkFilepathDependencies(cmd, "/tmp/invowkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckFilepathDependencies_EmptyDependsOn(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{}))

	err := checkFilepathDependencies(cmd, "/tmp/invowkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for empty depends_on, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileExists(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"test.txt"}}},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for existing file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"nonexistent.txt"}}},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err == nil {
		t.Error("checkFilepathDependencies() should return error for non-existent file")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingFilepaths) != 1 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
	}

	if !strings.Contains(depErr.MissingFilepaths[0], "does not exist") {
		t.Errorf("Error message should mention 'does not exist', got: %s", depErr.MissingFilepaths[0])
	}
}

func TestCheckFilepathDependencies_AbsolutePath(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "absolute-test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{testFile}}}, // Absolute path
		}))

	// Invowkfile in different directory
	err := checkFilepathDependencies(cmd, "/some/other/invowkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should handle absolute paths, got: %v", err)
	}
}

func TestCheckFilepathDependencies_ReadableFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readable.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"readable.txt"}, Readable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for readable file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_WritableDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"."}, Writable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for writable directory, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleFilepathDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"exists.txt"}},
				{Alternatives: []string{"nonexistent1.txt"}},
				{Alternatives: []string{"nonexistent2.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err == nil {
		t.Error("checkFilepathDependencies() should return error when any filepath dependency is not satisfied")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	// Should report both missing files (each as a separate dependency)
	if len(depErr.MissingFilepaths) != 2 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 2", len(depErr.MissingFilepaths))
	}
}

func TestCheckFilepathDependencies_AlternativesFirstExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "first.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when first alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesSecondExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "second.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when second alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesLastExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "third.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when last alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesNoneExists(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err == nil {
		t.Error("checkFilepathDependencies() should return error when no alternatives exist")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingFilepaths) != 1 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
	}

	// Error should mention alternatives not satisfied
	if !strings.Contains(depErr.MissingFilepaths[0], "alternatives") {
		t.Errorf("Error message should mention 'alternatives', got: %s", depErr.MissingFilepaths[0])
	}
}

func TestCheckFilepathDependencies_AlternativesWithPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a readable file
	readableFile := filepath.Join(tmpDir, "readable.txt")
	if err := os.WriteFile(readableFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"nonexistent.txt", "readable.txt"}, Readable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when alternative with proper permissions exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleAlternativesExist(t *testing.T) {
	tmpDir := t.TempDir()
	// Create multiple files that could satisfy the requirement
	for _, name := range []string{"first.txt", "second.txt", "third.txt"} {
		testFile := filepath.Join(tmpDir, name)
		if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when all alternatives exist, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleDependenciesWithAlternatives(t *testing.T) {
	tmpDir := t.TempDir()
	// Create files that satisfy different alternative dependencies
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create go.sum: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create readme.md: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				// First doesn't exist, second does
				{Alternatives: []string{"go.mod", "go.sum"}},
				// First two don't exist, third does
				{Alternatives: []string{"README.md", "README", "readme.md"}, Readable: true},
				// Current directory should exist
				{Alternatives: []string{"."}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when each dependency has an alternative satisfied, got: %v", err)
	}
}

// ============================================================================
// DependencyError and Render Tests
// ============================================================================

func TestDependencyError_Error(t *testing.T) {
	err := &DependencyError{
		CommandName:  "my-command",
		MissingTools: []string{"tool1", "tool2"},
	}

	expected := "dependencies not satisfied for command 'my-command'"
	if err.Error() != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestRenderDependencyError_MissingTools(t *testing.T) {
	err := &DependencyError{
		CommandName: "build",
		MissingTools: []string{
			"  - git - not found in PATH",
			"  - docker (version: >=20.0) - not found in PATH",
		},
	}

	output := RenderDependencyError(err)

	// Check that output contains key elements
	if !strings.Contains(output, "Dependencies not satisfied") {
		t.Error("RenderDependencyError should contain header")
	}

	if !strings.Contains(output, "'build'") {
		t.Error("RenderDependencyError should contain command name")
	}

	if !strings.Contains(output, "Missing Tools") {
		t.Error("RenderDependencyError should contain 'Missing Tools' section")
	}

	if !strings.Contains(output, "git") {
		t.Error("RenderDependencyError should contain tool name")
	}
}

func TestRenderDependencyError_MissingCommands(t *testing.T) {
	err := &DependencyError{
		CommandName: "release",
		MissingCommands: []string{
			"  - build - command not found",
			"  - test - command not found",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Missing Commands") {
		t.Error("RenderDependencyError should contain 'Missing Commands' section")
	}

	if !strings.Contains(output, "build") {
		t.Error("RenderDependencyError should contain missing command name")
	}
}

func TestRenderDependencyError_BothToolsAndCommands(t *testing.T) {
	err := &DependencyError{
		CommandName: "deploy",
		MissingTools: []string{
			"  - kubectl - not found in PATH",
		},
		MissingCommands: []string{
			"  - build - command not found",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Missing Tools") {
		t.Error("RenderDependencyError should contain 'Missing Tools' section")
	}

	if !strings.Contains(output, "Missing Commands") {
		t.Error("RenderDependencyError should contain 'Missing Commands' section")
	}
}

func TestRenderHostNotSupportedError(t *testing.T) {
	output := RenderHostNotSupportedError("clean", "windows", "linux, mac")

	if !strings.Contains(output, "Host not supported") {
		t.Error("RenderHostNotSupportedError should contain 'Host not supported'")
	}

	if !strings.Contains(output, "'clean'") {
		t.Error("RenderHostNotSupportedError should contain command name")
	}

	if !strings.Contains(output, "windows") {
		t.Error("RenderHostNotSupportedError should contain current host")
	}

	if !strings.Contains(output, "linux, mac") {
		t.Error("RenderHostNotSupportedError should contain supported hosts")
	}
}
