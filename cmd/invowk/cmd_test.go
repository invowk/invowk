package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invowkfile"
)

// testCmd creates a Command with a single script for testing
func testCmd(name string, script string) *invowkfile.Command {
	return &invowkfile.Command{
		Name: name,
		Implementations: []invowkfile.Implementation{
			{Script: script, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}},
		},
	}
}

// testCmdWithDeps creates a Command with a single script and dependencies
func testCmdWithDeps(name string, script string, deps *invowkfile.DependsOn) *invowkfile.Command {
	return &invowkfile.Command{
		Name:            name,
		Implementations: []invowkfile.Implementation{{Script: script, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}}},
		DependsOn:       deps,
	}
}

func TestCheckToolDependencies_NoTools(t *testing.T) {
	cmd := testCmd("test", "echo hello")

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckToolDependencies_EmptyDependsOn(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for empty depends_on, got: %v", err)
	}
}

func TestCheckToolDependencies_ToolExists(t *testing.T) {
	// Use a tool that should exist on any system
	var existingTool string
	for _, tool := range []string{"sh", "bash", "echo", "cat"} {
		if _, err := exec.LookPath(tool); err == nil {
			existingTool = tool
			break
		}
	}

	if existingTool == "" {
		t.Skip("No common tools found in PATH")
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Alternatives: []string{existingTool}}},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for existing tool '%s', got: %v", existingTool, err)
	}
}

func TestCheckToolDependencies_ToolNotExists(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Alternatives: []string{"nonexistent-tool-xyz-12345"}}},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for non-existent tool")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Errorf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	if depErr.CommandName != "test" {
		t.Errorf("DependencyError.CommandName = %q, want %q", depErr.CommandName, "test")
	}

	if len(depErr.MissingTools) != 1 {
		t.Errorf("DependencyError.MissingTools length = %d, want 1", len(depErr.MissingTools))
	}
}

func TestCheckToolDependencies_MultipleToolsNotExist(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{Alternatives: []string{"nonexistent-tool-1"}},
			{Alternatives: []string{"nonexistent-tool-2"}},
			{Alternatives: []string{"nonexistent-tool-3"}},
		},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for non-existent tools")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingTools) != 3 {
		t.Errorf("DependencyError.MissingTools length = %d, want 3", len(depErr.MissingTools))
	}
}

func TestCheckToolDependencies_MixedToolsExistAndNotExist(t *testing.T) {
	// Find an existing tool
	var existingTool string
	for _, tool := range []string{"sh", "bash", "echo", "cat"} {
		if _, err := exec.LookPath(tool); err == nil {
			existingTool = tool
			break
		}
	}

	if existingTool == "" {
		t.Skip("No common tools found in PATH")
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{Alternatives: []string{existingTool}},
			{Alternatives: []string{"nonexistent-tool-xyz"}},
		},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error when any tool is missing")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	// Only the non-existent tool should be in the error
	if len(depErr.MissingTools) != 1 {
		t.Errorf("DependencyError.MissingTools length = %d, want 1", len(depErr.MissingTools))
	}

	if !strings.Contains(depErr.MissingTools[0], "nonexistent-tool-xyz") {
		t.Errorf("MissingTools should contain 'nonexistent-tool-xyz', got: %s", depErr.MissingTools[0])
	}
}

func TestCheckCustomChecks_Success(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:         "test-check",
				CheckScript:  "echo 'test output'",
				ExpectedCode: intPtr(0),
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil for successful check script, got: %v", err)
	}
}

func TestCheckCustomChecks_WrongExitCode(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:         "test-check",
				CheckScript:  "exit 1",
				ExpectedCode: intPtr(0),
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err == nil {
		t.Error("checkCustomChecks() should return error for wrong exit code")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkCustomChecks() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0], "exit code") {
		t.Errorf("Error message should mention exit code, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckCustomChecks_ExpectedNonZeroCode(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:         "test-check",
				CheckScript:  "exit 42",
				ExpectedCode: intPtr(42),
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil when exit code matches expected, got: %v", err)
	}
}

func TestCheckCustomChecks_OutputMatch(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:           "test-check",
				CheckScript:    "echo 'version 1.2.3'",
				ExpectedOutput: "version [0-9]+\\.[0-9]+\\.[0-9]+",
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil for matching output, got: %v", err)
	}
}

func TestCheckCustomChecks_OutputNoMatch(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:           "test-check",
				CheckScript:    "echo 'hello world'",
				ExpectedOutput: "^version",
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err == nil {
		t.Error("checkCustomChecks() should return error for non-matching output")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkCustomChecks() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0], "does not match pattern") {
		t.Errorf("Error message should mention pattern mismatch, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckCustomChecks_BothCodeAndOutput(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:           "test-check",
				CheckScript:    "echo 'go version go1.21.0'",
				ExpectedCode:   intPtr(0),
				ExpectedOutput: "go1\\.",
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil when both code and output match, got: %v", err)
	}
}

func TestCheckCustomChecks_InvalidRegex(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Name:           "test-check",
				CheckScript:    "echo 'test'",
				ExpectedOutput: "[invalid regex(",
			},
		},
	})

	err := checkCustomChecks(cmd)
	if err == nil {
		t.Error("checkCustomChecks() should return error for invalid regex")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkCustomChecks() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0], "invalid regex") {
		t.Errorf("Error message should mention invalid regex, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckFilepathDependencies_NoFilepaths(t *testing.T) {
	cmd := testCmd("test", "echo hello")

	err := checkFilepathDependencies(cmd, "/tmp/invowkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckFilepathDependencies_EmptyDependsOn(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{})

	err := checkFilepathDependencies(cmd, "/tmp/invowkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for empty depends_on, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileExists(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"test.txt"}}},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for existing file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"nonexistent.txt"}}},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err == nil {
		t.Error("checkFilepathDependencies() should return error for non-existent file")
	}

	depErr, ok := err.(*DependencyError)
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{testFile}}}, // Absolute path
	})

	// Invowkfile in different directory
	err := checkFilepathDependencies(cmd, "/some/other/invowkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should handle absolute paths, got: %v", err)
	}
}

func TestCheckFilepathDependencies_ReadableFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "readable.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"readable.txt"}, Readable: true},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for readable file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_WritableDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"."}, Writable: true},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for writable directory, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleFilepathDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"exists.txt"}},
			{Alternatives: []string{"nonexistent1.txt"}},
			{Alternatives: []string{"nonexistent2.txt"}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err == nil {
		t.Error("checkFilepathDependencies() should return error when any filepath dependency is not satisfied")
	}

	depErr, ok := err.(*DependencyError)
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when first alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesSecondExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "second.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when second alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesLastExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "third.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when last alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesNoneExists(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err == nil {
		t.Error("checkFilepathDependencies() should return error when no alternatives exist")
	}

	depErr, ok := err.(*DependencyError)
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
	if err := os.WriteFile(readableFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"nonexistent.txt", "readable.txt"}, Readable: true},
		},
	})

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
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when all alternatives exist, got: %v", err)
	}
}

func TestCheckFilepathDependencies_MultipleDependenciesWithAlternatives(t *testing.T) {
	tmpDir := t.TempDir()
	// Create files that satisfy different alternative dependencies
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create go.sum: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create readme.md: %v", err)
	}

	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{
			// First doesn't exist, second does
			{Alternatives: []string{"go.mod", "go.sum"}},
			// First two don't exist, third does
			{Alternatives: []string{"README.md", "README", "readme.md"}, Readable: true},
			// Current directory should exist
			{Alternatives: []string{"."}},
		},
	})

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	err := checkFilepathDependencies(cmd, invowkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when each dependency has an alternative satisfied, got: %v", err)
	}
}

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
			"  • git - not found in PATH",
			"  • docker (version: >=20.0) - not found in PATH",
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
			"  • build - command not found",
			"  • test - command not found",
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
			"  • kubectl - not found in PATH",
		},
		MissingCommands: []string{
			"  • build - command not found",
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

// intPtr is a helper to create a pointer to an int
func intPtr(i int) *int {
	return &i
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

func TestCommand_CanRunOnCurrentHost(t *testing.T) {
	currentOS := invowkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected bool
	}{
		{
			name: "current host supported",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: currentOS}}}},
				},
			},
			expected: true,
		},
		{
			name: "current host not supported",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: "nonexistent"}}}},
				},
			},
			expected: false,
		},
		{
			name: "all hosts supported (no platforms specified)",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}},
				},
			},
			expected: true,
		},
		{
			name: "empty scripts list",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.CanRunOnCurrentHost()
			if result != tt.expected {
				t.Errorf("CanRunOnCurrentHost() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommand_GetPlatformsString(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected string
	}{
		{
			name: "single platform",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}}}},
				},
			},
			expected: "linux",
		},
		{
			name: "multiple platforms",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}}}},
				},
			},
			expected: "linux, macos",
		},
		{
			name: "all platforms (no platforms specified)",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}},
				},
			},
			expected: "linux, macos, windows",
		},
		{
			name: "empty scripts",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetPlatformsString()
			if result != tt.expected {
				t.Errorf("GetPlatformsString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetCurrentHostOS(t *testing.T) {
	// Just verify it returns one of the expected values
	currentOS := invowkfile.GetCurrentHostOS()
	validOSes := map[invowkfile.HostOS]bool{
		invowkfile.HostLinux:   true,
		invowkfile.HostMac:     true,
		invowkfile.HostWindows: true,
	}

	if !validOSes[currentOS] {
		t.Errorf("GetCurrentHostOS() returned unexpected value: %q", currentOS)
	}
}

func TestCommand_GetDefaultRuntimeForPlatform(t *testing.T) {
	currentPlatform := invowkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected invowkfile.RuntimeMode
	}{
		{
			name: "first runtime is default",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeContainer}}}},
				},
			},
			expected: invowkfile.RuntimeNative,
		},
		{
			name: "container as default",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}, {Name: invowkfile.RuntimeNative}}}},
				},
			},
			expected: invowkfile.RuntimeContainer,
		},
		{
			name: "empty scripts returns native",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
			},
			expected: invowkfile.RuntimeNative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetDefaultRuntimeForPlatform(currentPlatform)
			if result != tt.expected {
				t.Errorf("GetDefaultRuntimeForPlatform() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommand_IsRuntimeAllowedForPlatform(t *testing.T) {
	currentPlatform := invowkfile.GetCurrentHostOS()

	cmd := &invowkfile.Command{
		Name: "test",
		Implementations: []invowkfile.Implementation{
			{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeVirtual}}}},
		},
	}

	tests := []struct {
		runtime  invowkfile.RuntimeMode
		expected bool
	}{
		{invowkfile.RuntimeNative, true},
		{invowkfile.RuntimeVirtual, true},
		{invowkfile.RuntimeContainer, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			result := cmd.IsRuntimeAllowedForPlatform(currentPlatform, tt.runtime)
			if result != tt.expected {
				t.Errorf("IsRuntimeAllowedForPlatform(%v) = %v, want %v", tt.runtime, result, tt.expected)
			}
		})
	}
}

func TestCommand_GetRuntimesStringForPlatform(t *testing.T) {
	currentPlatform := invowkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected string
	}{
		{
			name: "single runtime with asterisk",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}},
				},
			},
			expected: "native*",
		},
		{
			name: "multiple runtimes with first marked",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeContainer}}}},
				},
			},
			expected: "native*, container",
		},
		{
			name: "empty scripts",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetRuntimesStringForPlatform(currentPlatform)
			if result != tt.expected {
				t.Errorf("GetRuntimesStringForPlatform() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRenderRuntimeNotAllowedError(t *testing.T) {
	output := RenderRuntimeNotAllowedError("build", "container", "native, virtual")

	if !strings.Contains(output, "Runtime not allowed") {
		t.Error("RenderRuntimeNotAllowedError should contain 'Runtime not allowed'")
	}

	if !strings.Contains(output, "'build'") {
		t.Error("RenderRuntimeNotAllowedError should contain command name")
	}

	if !strings.Contains(output, "container") {
		t.Error("RenderRuntimeNotAllowedError should contain selected runtime")
	}

	if !strings.Contains(output, "native, virtual") {
		t.Error("RenderRuntimeNotAllowedError should contain allowed runtimes")
	}
}

func TestCheckCapabilityDependencies_NoCapabilities(t *testing.T) {
	deps := &invowkfile.DependsOn{
		Capabilities: []invowkfile.CapabilityDependency{},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkCapabilityDependencies() with empty capabilities returned error: %v", err)
	}
}

func TestCheckCapabilityDependencies_NilDeps(t *testing.T) {
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(nil, ctx)
	if err != nil {
		t.Errorf("checkCapabilityDependencies() with nil deps returned error: %v", err)
	}
}

func TestCheckCapabilityDependencies_DuplicateSkipped(t *testing.T) {
	// This test verifies that duplicate capabilities are silently skipped
	// The actual success/failure depends on network connectivity
	deps := &invowkfile.DependsOn{
		Capabilities: []invowkfile.CapabilityDependency{
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityLocalAreaNetwork}},
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityLocalAreaNetwork}}, // duplicate
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityLocalAreaNetwork}}, // another duplicate
		},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(deps, ctx)

	// If there's an error, it should only report the capability once
	if err != nil {
		depErr, ok := err.(*DependencyError)
		if !ok {
			t.Fatalf("checkCapabilityDependencies() should return *DependencyError, got: %T", err)
		}
		// Even with 3 duplicate entries, we should only have 1 error
		if len(depErr.MissingCapabilities) > 1 {
			t.Errorf("Expected at most 1 capability error (duplicates should be skipped), got %d", len(depErr.MissingCapabilities))
		}
	}
	// If no error, that's fine too - machine has network
}

func TestDependencyError_WithCapabilities(t *testing.T) {
	err := &DependencyError{
		CommandName: "test",
		MissingCapabilities: []string{
			"  • capability \"internet\" not available: no connection",
		},
	}

	expected := "dependencies not satisfied for command 'test'"
	if err.Error() != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestRenderDependencyError_MissingCapabilities(t *testing.T) {
	err := &DependencyError{
		CommandName: "deploy",
		MissingCapabilities: []string{
			"  • capability \"internet\" not available: no connection",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Dependencies not satisfied") {
		t.Error("RenderDependencyError should contain header")
	}

	if !strings.Contains(output, "'deploy'") {
		t.Error("RenderDependencyError should contain command name")
	}

	if !strings.Contains(output, "Missing Capabilities") {
		t.Error("RenderDependencyError should contain 'Missing Capabilities' section")
	}

	if !strings.Contains(output, "internet") {
		t.Error("RenderDependencyError should contain capability name")
	}
}

func TestRenderDependencyError_AllDependencyTypes(t *testing.T) {
	err := &DependencyError{
		CommandName: "complex-deploy",
		MissingTools: []string{
			"  • kubectl - not found in PATH",
		},
		MissingCommands: []string{
			"  • build - command not found",
		},
		MissingFilepaths: []string{
			"  • config.yaml - file not found",
		},
		MissingCapabilities: []string{
			"  • capability \"internet\" not available: no connection",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Missing Tools") {
		t.Error("RenderDependencyError should contain 'Missing Tools' section")
	}

	if !strings.Contains(output, "Missing Commands") {
		t.Error("RenderDependencyError should contain 'Missing Commands' section")
	}

	if !strings.Contains(output, "Missing or Inaccessible Files") {
		t.Error("RenderDependencyError should contain 'Missing or Inaccessible Files' section")
	}

	if !strings.Contains(output, "Missing Capabilities") {
		t.Error("RenderDependencyError should contain 'Missing Capabilities' section")
	}
}

// Tests for flag functionality

func TestFlagNameToEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "verbose",
			expected: "INVOWK_FLAG_VERBOSE",
		},
		{
			name:     "name with hyphen",
			input:    "output-file",
			expected: "INVOWK_FLAG_OUTPUT_FILE",
		},
		{
			name:     "name with multiple hyphens",
			input:    "dry-run-mode",
			expected: "INVOWK_FLAG_DRY_RUN_MODE",
		},
		{
			name:     "name with underscore",
			input:    "output_file",
			expected: "INVOWK_FLAG_OUTPUT_FILE",
		},
		{
			name:     "mixed case preserved as uppercase",
			input:    "outputFile",
			expected: "INVOWK_FLAG_OUTPUTFILE",
		},
		{
			name:     "already uppercase",
			input:    "VERBOSE",
			expected: "INVOWK_FLAG_VERBOSE",
		},
		{
			name:     "single character",
			input:    "v",
			expected: "INVOWK_FLAG_V",
		},
		{
			name:     "numeric suffix",
			input:    "level2",
			expected: "INVOWK_FLAG_LEVEL2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlagNameToEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("FlagNameToEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRunCommandWithFlags_FlagsInjectedAsEnvVars(t *testing.T) {
	// This test verifies that flag values are correctly converted to environment variables
	// We'll test the FlagNameToEnvVar conversion more extensively since
	// the actual runCommandWithFlags requires a full invowkfile setup

	// Test that the conversion is consistent
	flagName := "my-custom-flag"
	envVar := FlagNameToEnvVar(flagName)

	if envVar != "INVOWK_FLAG_MY_CUSTOM_FLAG" {
		t.Errorf("FlagNameToEnvVar(%q) = %q, expected INVOWK_FLAG_MY_CUSTOM_FLAG", flagName, envVar)
	}

	// Verify the pattern: INVOWK_FLAG_ prefix, uppercase, hyphens replaced with underscores
	if !strings.HasPrefix(envVar, "INVOWK_FLAG_") {
		t.Error("FlagNameToEnvVar result should have INVOWK_FLAG_ prefix")
	}

	if strings.Contains(envVar, "-") {
		t.Error("FlagNameToEnvVar result should not contain hyphens")
	}

	if envVar != strings.ToUpper(envVar) {
		t.Error("FlagNameToEnvVar result should be all uppercase")
	}
}

// testCmdWithFlags creates a Command with flags for testing
func testCmdWithFlags(name string, script string, flags []invowkfile.Flag) *invowkfile.Command {
	return &invowkfile.Command{
		Name:  name,
		Flags: flags,
		Implementations: []invowkfile.Implementation{
			{Script: script, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}},
		},
	}
}

func TestCommand_WithFlags(t *testing.T) {
	flags := []invowkfile.Flag{
		{Name: "env", Description: "Target environment"},
		{Name: "dry-run", Description: "Perform dry run", DefaultValue: "false"},
	}

	cmd := testCmdWithFlags("deploy", "echo deploying", flags)

	if len(cmd.Flags) != 2 {
		t.Errorf("Command should have 2 flags, got %d", len(cmd.Flags))
	}

	// Verify flag properties
	if cmd.Flags[0].Name != "env" {
		t.Errorf("First flag name should be 'env', got %q", cmd.Flags[0].Name)
	}

	if cmd.Flags[1].DefaultValue != "false" {
		t.Errorf("Second flag default value should be 'false', got %q", cmd.Flags[1].DefaultValue)
	}
}

func TestFlagNameToEnvVar_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "INVOWK_FLAG_",
		},
		{
			name:     "only hyphens",
			input:    "---",
			expected: "INVOWK_FLAG____",
		},
		{
			name:     "starts with hyphen",
			input:    "-flag",
			expected: "INVOWK_FLAG__FLAG",
		},
		{
			name:     "ends with hyphen",
			input:    "flag-",
			expected: "INVOWK_FLAG_FLAG_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlagNameToEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("FlagNameToEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Tests for positional arguments functionality

func TestArgNameToEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "env",
			expected: "INVOWK_ARG_ENV",
		},
		{
			name:     "name with hyphen",
			input:    "output-file",
			expected: "INVOWK_ARG_OUTPUT_FILE",
		},
		{
			name:     "name with multiple hyphens",
			input:    "my-config-path",
			expected: "INVOWK_ARG_MY_CONFIG_PATH",
		},
		{
			name:     "mixed case",
			input:    "myArg",
			expected: "INVOWK_ARG_MYARG",
		},
		{
			name:     "already uppercase",
			input:    "VERBOSE",
			expected: "INVOWK_ARG_VERBOSE",
		},
		{
			name:     "single character",
			input:    "v",
			expected: "INVOWK_ARG_V",
		},
		{
			name:     "numeric suffix",
			input:    "arg1",
			expected: "INVOWK_ARG_ARG1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "INVOWK_ARG_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ArgNameToEnvVar(tt.input)
			if result != tt.expected {
				t.Errorf("ArgNameToEnvVar(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildCommandUsageString(t *testing.T) {
	tests := []struct {
		name     string
		cmdPart  string
		args     []invowkfile.Argument
		expected string
	}{
		{
			name:     "no arguments",
			cmdPart:  "deploy",
			args:     []invowkfile.Argument{},
			expected: "deploy",
		},
		{
			name:    "single required argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: true},
			},
			expected: "deploy <env>",
		},
		{
			name:    "single optional argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: false},
			},
			expected: "deploy [env]",
		},
		{
			name:    "required and optional arguments",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: true},
				{Name: "replicas", Required: false},
			},
			expected: "deploy <env> [replicas]",
		},
		{
			name:    "required variadic argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "services", Required: true, Variadic: true},
			},
			expected: "deploy <services>...",
		},
		{
			name:    "optional variadic argument",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "services", Required: false, Variadic: true},
			},
			expected: "deploy [services]...",
		},
		{
			name:    "multiple args with variadic",
			cmdPart: "deploy",
			args: []invowkfile.Argument{
				{Name: "env", Required: true},
				{Name: "replicas", Required: false},
				{Name: "services", Required: false, Variadic: true},
			},
			expected: "deploy <env> [replicas] [services]...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCommandUsageString(tt.cmdPart, tt.args)
			if result != tt.expected {
				t.Errorf("buildCommandUsageString(%q, ...) = %q, want %q", tt.cmdPart, result, tt.expected)
			}
		})
	}
}

func TestBuildArgsDocumentation(t *testing.T) {
	tests := []struct {
		name          string
		args          []invowkfile.Argument
		shouldHave    []string
		shouldNotHave []string
	}{
		{
			name: "required argument",
			args: []invowkfile.Argument{
				{Name: "env", Description: "Target environment", Required: true},
			},
			shouldHave: []string{"env", "(required)", "Target environment"},
		},
		{
			name: "optional with default",
			args: []invowkfile.Argument{
				{Name: "replicas", Description: "Number of replicas", DefaultValue: "1"},
			},
			shouldHave: []string{"replicas", `(default: "1")`, "Number of replicas"},
		},
		{
			name: "optional without default",
			args: []invowkfile.Argument{
				{Name: "tag", Description: "Image tag"},
			},
			shouldHave: []string{"tag", "(optional)", "Image tag"},
		},
		{
			name: "typed argument",
			args: []invowkfile.Argument{
				{Name: "count", Description: "Count value", Type: invowkfile.ArgumentTypeInt},
			},
			shouldHave: []string{"count", "[int]", "Count value"},
		},
		{
			name: "variadic argument",
			args: []invowkfile.Argument{
				{Name: "services", Description: "Services to deploy", Variadic: true},
			},
			shouldHave: []string{"services", "(variadic)", "Services to deploy"},
		},
		{
			name: "string type not shown",
			args: []invowkfile.Argument{
				{Name: "name", Description: "Name", Type: invowkfile.ArgumentTypeString},
			},
			shouldHave:    []string{"name", "Name"},
			shouldNotHave: []string{"[string]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildArgsDocumentation(tt.args)

			for _, s := range tt.shouldHave {
				if !strings.Contains(result, s) {
					t.Errorf("buildArgsDocumentation() should contain %q, got: %q", s, result)
				}
			}

			for _, s := range tt.shouldNotHave {
				if strings.Contains(result, s) {
					t.Errorf("buildArgsDocumentation() should NOT contain %q, got: %q", s, result)
				}
			}
		})
	}
}

func TestArgumentValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ArgumentValidationError
		expected string
	}{
		{
			name: "missing required",
			err: &ArgumentValidationError{
				Type:         ArgErrMissingRequired,
				CommandName:  "deploy",
				MinArgs:      2,
				ProvidedArgs: []string{"prod"},
			},
			expected: "missing required arguments for command 'deploy': expected at least 2, got 1",
		},
		{
			name: "too many",
			err: &ArgumentValidationError{
				Type:         ArgErrTooMany,
				CommandName:  "deploy",
				MaxArgs:      2,
				ProvidedArgs: []string{"prod", "3", "extra"},
			},
			expected: "too many arguments for command 'deploy': expected at most 2, got 3",
		},
		{
			name: "invalid value",
			err: &ArgumentValidationError{
				Type:         ArgErrInvalidValue,
				CommandName:  "deploy",
				InvalidArg:   "replicas",
				InvalidValue: "abc",
				ValueError:   fmt.Errorf("not a valid integer"),
			},
			expected: "invalid value for argument 'replicas': not a valid integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRenderArgumentValidationError_MissingRequired(t *testing.T) {
	err := &ArgumentValidationError{
		Type:        ArgErrMissingRequired,
		CommandName: "deploy",
		ArgDefs: []invowkfile.Argument{
			{Name: "env", Description: "Target environment", Required: true},
			{Name: "replicas", Description: "Number of replicas", DefaultValue: "1"},
		},
		ProvidedArgs: []string{},
		MinArgs:      1,
	}

	output := RenderArgumentValidationError(err)

	if !strings.Contains(output, "Missing required arguments") {
		t.Error("Should contain 'Missing required arguments'")
	}
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}
	if !strings.Contains(output, "env") {
		t.Error("Should contain argument name 'env'")
	}
	if !strings.Contains(output, "(required)") {
		t.Error("Should indicate required arguments")
	}
	if !strings.Contains(output, "--help") {
		t.Error("Should contain help hint")
	}
}

func TestRenderArgumentValidationError_TooMany(t *testing.T) {
	err := &ArgumentValidationError{
		Type:        ArgErrTooMany,
		CommandName: "deploy",
		ArgDefs: []invowkfile.Argument{
			{Name: "env", Description: "Target environment"},
		},
		ProvidedArgs: []string{"prod", "extra1", "extra2"},
		MaxArgs:      1,
	}

	output := RenderArgumentValidationError(err)

	if !strings.Contains(output, "Too many arguments") {
		t.Error("Should contain 'Too many arguments'")
	}
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}
	if !strings.Contains(output, "Provided:") {
		t.Error("Should show provided arguments")
	}
}

func TestRenderArgumentValidationError_InvalidValue(t *testing.T) {
	err := &ArgumentValidationError{
		Type:         ArgErrInvalidValue,
		CommandName:  "deploy",
		InvalidArg:   "replicas",
		InvalidValue: "abc",
		ValueError:   fmt.Errorf("must be a valid integer"),
	}

	output := RenderArgumentValidationError(err)

	if !strings.Contains(output, "Invalid argument value") {
		t.Error("Should contain 'Invalid argument value'")
	}
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}
	if !strings.Contains(output, "'replicas'") {
		t.Error("Should contain argument name")
	}
	if !strings.Contains(output, "abc") {
		t.Error("Should contain invalid value")
	}
	if !strings.Contains(output, "must be a valid integer") {
		t.Error("Should contain error message")
	}
}

func TestRenderArgsSubcommandConflictError(t *testing.T) {
	args := []invowkfile.Argument{
		{Name: "env", Description: "Target environment"},
		{Name: "replicas", Description: "Number of replicas"},
	}
	subcommands := []string{"deploy status", "deploy logs"}

	output := RenderArgsSubcommandConflictError("deploy", args, subcommands)

	// Check header
	if !strings.Contains(output, "Conflict") {
		t.Error("Should contain 'Conflict' header")
	}

	// Check command name
	if !strings.Contains(output, "'deploy'") {
		t.Error("Should contain command name")
	}

	// Check args are listed
	if !strings.Contains(output, "env") {
		t.Error("Should list argument 'env'")
	}
	if !strings.Contains(output, "replicas") {
		t.Error("Should list argument 'replicas'")
	}

	// Check subcommands are listed
	if !strings.Contains(output, "deploy status") {
		t.Error("Should list subcommand 'deploy status'")
	}
	if !strings.Contains(output, "deploy logs") {
		t.Error("Should list subcommand 'deploy logs'")
	}

	// Check hint
	if !strings.Contains(output, "Remove either the 'args' field or the subcommands") {
		t.Error("Should contain resolution hint")
	}
}

// testCmdWithArgs creates a Command with args for testing
func testCmdWithArgs(name string, script string, args []invowkfile.Argument) *invowkfile.Command {
	return &invowkfile.Command{
		Name: name,
		Args: args,
		Implementations: []invowkfile.Implementation{
			{Script: script, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}}},
		},
	}
}

func TestCommand_WithArgs(t *testing.T) {
	args := []invowkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
		{Name: "replicas", Description: "Number of replicas", Type: invowkfile.ArgumentTypeInt, DefaultValue: "1"},
		{Name: "services", Description: "Services to deploy", Variadic: true},
	}

	cmd := testCmdWithArgs("deploy", "echo deploying", args)

	if len(cmd.Args) != 3 {
		t.Errorf("Command should have 3 args, got %d", len(cmd.Args))
	}

	// Verify arg properties
	if cmd.Args[0].Name != "env" {
		t.Errorf("First arg name should be 'env', got %q", cmd.Args[0].Name)
	}
	if !cmd.Args[0].Required {
		t.Error("First arg should be required")
	}
	if cmd.Args[1].Type != invowkfile.ArgumentTypeInt {
		t.Errorf("Second arg type should be 'int', got %q", cmd.Args[1].Type)
	}
	if cmd.Args[1].DefaultValue != "1" {
		t.Errorf("Second arg default value should be '1', got %q", cmd.Args[1].DefaultValue)
	}
	if !cmd.Args[2].Variadic {
		t.Error("Third arg should be variadic")
	}
}
