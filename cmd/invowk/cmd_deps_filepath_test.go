// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ============================================================================
// Filepath Dependency Tests
// ============================================================================

func TestCheckFilepathDependencies_NoFilepaths(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test", invowkfiletest.WithScript("echo hello"))

	err := checkHostFilepathDependencies(cmd.DependsOn, "/tmp/invowkfile.cue", &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckFilepathDependencies_EmptyDependsOn(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{}))

	err := checkHostFilepathDependencies(cmd.DependsOn, "/tmp/invowkfile.cue", &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for empty depends_on, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileExists(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for existing file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileNotExists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"nonexistent.txt"}}},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostFilepathDependencies() should return error for non-existent file")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingFilepaths) != 1 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
	}

	if !strings.Contains(depErr.MissingFilepaths[0], "does not exist") {
		t.Errorf("Error message should mention 'does not exist', got: %s", depErr.MissingFilepaths[0])
	}
}

func TestCheckFilepathDependencies_AbsolutePath(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, "/some/other/invowkfile.cue", &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should handle absolute paths, got: %v", err)
	}
}

func TestCheckFilepathDependencies_ReadableFile(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for readable file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_WritableDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"."}, Writable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for writable directory, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleFilepathDependencies(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostFilepathDependencies() should return error when any filepath dependency is not satisfied")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	// Should report both missing files (each as a separate dependency)
	if len(depErr.MissingFilepaths) != 2 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 2", len(depErr.MissingFilepaths))
	}
}

func TestCheckFilepathDependencies_AlternativesFirstExists(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil when first alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesSecondExists(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil when second alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesLastExists(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil when last alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesNoneExists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostFilepathDependencies() should return error when no alternatives exist")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostFilepathDependencies() should return *DependencyError, got: %T", err)
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
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil when alternative with proper permissions exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleAlternativesExist(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil when all alternatives exist, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleDependenciesWithAlternatives(t *testing.T) {
	t.Parallel()

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
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil when each dependency has an alternative satisfied, got: %v", err)
	}
}

func TestCheckFilepathDependencies_ExecutableFile(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Unix permission bit test not applicable on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "run.sh")
	if err := os.WriteFile(testFile, []byte("#!/bin/sh\necho hello"), 0o755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"run.sh"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for executable file (0o755), got: %v", err)
	}
}

func TestCheckFilepathDependencies_NonExecutableFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "data.txt")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"data.txt"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostFilepathDependencies() should return error for non-executable file")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingFilepaths) != 1 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
	}

	if !strings.Contains(depErr.MissingFilepaths[0], "execute") {
		t.Errorf("Error message should mention 'execute', got: %s", depErr.MissingFilepaths[0])
	}
}

func TestCheckFilepathDependencies_ExecutableDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	execDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"bin"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for executable directory, got: %v", err)
	}
}

func TestCheckFilepathDependencies_ExecutableExtensionWindows(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS != "windows" {
		t.Skip("skipping: Windows-specific executable extension test")
	}

	tmpDir := t.TempDir()

	// .exe should pass the executable check on Windows
	exeFile := filepath.Join(tmpDir, "tool.exe")
	if err := os.WriteFile(exeFile, []byte("fake exe"), 0o644); err != nil {
		t.Fatalf("Failed to create .exe file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"tool.exe"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for .exe file on Windows, got: %v", err)
	}
}

func TestCheckFilepathDependencies_NonExecutableDirectory(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Unix permission bit test not applicable on Windows")
	}

	tmpDir := t.TempDir()
	noExecDir := filepath.Join(tmpDir, "nox")
	if err := os.MkdirAll(noExecDir, 0o644); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"nox"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostFilepathDependencies() should return error for non-executable directory (0o644)")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostFilepathDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingFilepaths) != 1 {
		t.Errorf("DependencyError.MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
	}

	if !strings.Contains(depErr.MissingFilepaths[0], "execute") {
		t.Errorf("Error message should mention 'execute', got: %s", depErr.MissingFilepaths[0])
	}
}

func TestCheckFilepathDependencies_InaccessibleFile(t *testing.T) {
	t.Parallel()

	if goruntime.GOOS == "windows" {
		t.Skip("skipping: Unix chmod 0o000 test not applicable on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "denied.sh")
	if err := os.WriteFile(testFile, []byte("#!/bin/sh\necho hello"), 0o000); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"denied.sh"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostFilepathDependencies() should return error for inaccessible file (0o000)")
	}
}

func TestCheckFilepathDependencies_ExecutableAlternativesFallback(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create run.sh without execute bit and run.bat
	if err := os.WriteFile(filepath.Join(tmpDir, "run.sh"), []byte("#!/bin/sh\necho hello"), 0o644); err != nil {
		t.Fatalf("Failed to create run.sh: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "run.bat"), []byte("@echo off\necho hello"), 0o644); err != nil {
		t.Fatalf("Failed to create run.bat: %v", err)
	}

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Filepaths: []invowkfile.FilepathDependency{
				{Alternatives: []string{"run.sh", "run.bat"}, Executable: true},
			},
		}))

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkHostFilepathDependencies(cmd.DependsOn, invowkfilePath, &runtime.ExecutionContext{Command: cmd})

	if goruntime.GOOS == "windows" {
		// On Windows: run.sh fails (no .sh in PATHEXT), run.bat succeeds (.bat extension)
		if err != nil {
			t.Errorf("checkHostFilepathDependencies() should succeed on Windows via .bat alternative, got: %v", err)
		}
	} else {
		// On Unix: both fail (neither has execute bit)
		if err == nil {
			t.Fatal("checkHostFilepathDependencies() should fail on Unix when no alternative has execute permission")
		}

		depErr, ok := errors.AsType[*DependencyError](err)
		if !ok {
			t.Fatalf("checkHostFilepathDependencies() should return *DependencyError, got: %T", err)
		}

		if len(depErr.MissingFilepaths) != 1 {
			t.Errorf("DependencyError.MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
		}

		if !strings.Contains(depErr.MissingFilepaths[0], "execute") {
			t.Errorf("Error message should mention 'execute', got: %s", depErr.MissingFilepaths[0])
		}
	}
}

func TestIsExecutable_PATHEXTFallback(t *testing.T) {
	// t.Setenv modifies process state, incompatible with t.Parallel()
	if goruntime.GOOS != "windows" {
		t.Skip("skipping: PATHEXT is only consulted on Windows")
	}

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "script.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Set PATHEXT to include .PY
	t.Setenv("PATHEXT", ".EXE;.PY;.RB")

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	if !isExecutable(testFile, info) {
		t.Error("isExecutable() should return true for .py file when PATHEXT includes .PY")
	}
}

func TestIsExecutable_PATHEXTEmptyEntries(t *testing.T) {
	// t.Setenv modifies process state, incompatible with t.Parallel()
	if goruntime.GOOS != "windows" {
		t.Skip("skipping: PATHEXT is only consulted on Windows")
	}

	tmpDir := t.TempDir()
	// File with no extension should NOT match empty PATHEXT entries
	testFile := filepath.Join(tmpDir, "noext")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// PATHEXT with empty entries from trailing/double semicolons
	t.Setenv("PATHEXT", ".EXE;;.BAT;")

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	if isExecutable(testFile, info) {
		t.Error("isExecutable() should return false for extensionless file even with empty PATHEXT entries")
	}
}

// ============================================================================
// DependencyError and Render Tests
// ============================================================================

func TestDependencyError_Error(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
