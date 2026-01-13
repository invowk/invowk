package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
)

// testCmd creates a Command with a single script for testing
func testCmd(name string, script string) *invkfile.Command {
	return &invkfile.Command{
		Name: name,
		Implementations: []invkfile.Implementation{
			{Script: script, Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}},
		},
	}
}

// testCmdWithDeps creates a Command with a single script and dependencies
func testCmdWithDeps(name string, script string, deps *invkfile.DependsOn) *invkfile.Command {
	return &invkfile.Command{
		Name:            name,
		Implementations: []invkfile.Implementation{{Script: script, Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}}},
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{})

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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Tools: []invkfile.ToolDependency{{Alternatives: []string{existingTool}}},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for existing tool '%s', got: %v", existingTool, err)
	}
}

func TestCheckToolDependencies_ToolNotExists(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Tools: []invkfile.ToolDependency{{Alternatives: []string{"nonexistent-tool-xyz-12345"}}},
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Tools: []invkfile.ToolDependency{
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Tools: []invkfile.ToolDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		CustomChecks: []invkfile.CustomCheckDependency{
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

	err := checkFilepathDependencies(cmd, "/tmp/invkfile.cue")
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckFilepathDependencies_EmptyDependsOn(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{})

	err := checkFilepathDependencies(cmd, "/tmp/invkfile.cue")
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{{Alternatives: []string{"test.txt"}}},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for existing file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{{Alternatives: []string{"nonexistent.txt"}}},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{{Alternatives: []string{testFile}}}, // Absolute path
	})

	// Invkfile in different directory
	err := checkFilepathDependencies(cmd, "/some/other/invkfile.cue")
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"readable.txt"}, Readable: true},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil for readable file, got: %v", err)
	}
}

func TestCheckFilepathDependencies_WritableDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"."}, Writable: true},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"exists.txt"}},
			{Alternatives: []string{"nonexistent1.txt"}},
			{Alternatives: []string{"nonexistent2.txt"}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
	if err != nil {
		t.Errorf("checkFilepathDependencies() should return nil when last alternative exists, got: %v", err)
	}
}

func TestCheckFilepathDependencies_AlternativesNoneExists(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"nonexistent.txt", "readable.txt"}, Readable: true},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			{Alternatives: []string{"first.txt", "second.txt", "third.txt"}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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

	cmd := testCmdWithDeps("test", "echo hello", &invkfile.DependsOn{
		Filepaths: []invkfile.FilepathDependency{
			// First doesn't exist, second does
			{Alternatives: []string{"go.mod", "go.sum"}},
			// First two don't exist, third does
			{Alternatives: []string{"README.md", "README", "readme.md"}, Readable: true},
			// Current directory should exist
			{Alternatives: []string{"."}},
		},
	})

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	err := checkFilepathDependencies(cmd, invkfilePath)
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
	currentOS := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected bool
	}{
		{
			name: "current host supported",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: currentOS}}}},
				},
			},
			expected: true,
		},
		{
			name: "current host not supported",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: "nonexistent"}}}},
				},
			},
			expected: false,
		},
		{
			name: "all hosts supported (no platforms specified)",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}},
				},
			},
			expected: true,
		},
		{
			name: "empty scripts list",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
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
		cmd      *invkfile.Command
		expected string
	}{
		{
			name: "single platform",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}}}},
				},
			},
			expected: "linux",
		},
		{
			name: "multiple platforms",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}}}},
				},
			},
			expected: "linux, macos",
		},
		{
			name: "all platforms (no platforms specified)",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}},
				},
			},
			expected: "linux, macos, windows",
		},
		{
			name: "empty scripts",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
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
	currentOS := invkfile.GetCurrentHostOS()
	validOSes := map[invkfile.HostOS]bool{
		invkfile.HostLinux:   true,
		invkfile.HostMac:     true,
		invkfile.HostWindows: true,
	}

	if !validOSes[currentOS] {
		t.Errorf("GetCurrentHostOS() returned unexpected value: %q", currentOS)
	}
}

func TestCommand_GetDefaultRuntimeForPlatform(t *testing.T) {
	currentPlatform := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected invkfile.RuntimeMode
	}{
		{
			name: "first runtime is default",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeContainer}}}},
				},
			},
			expected: invkfile.RuntimeNative,
		},
		{
			name: "container as default",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}, {Name: invkfile.RuntimeNative}}}},
				},
			},
			expected: invkfile.RuntimeContainer,
		},
		{
			name: "empty scripts returns native",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
			},
			expected: invkfile.RuntimeNative,
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
	currentPlatform := invkfile.GetCurrentHostOS()

	cmd := &invkfile.Command{
		Name: "test",
		Implementations: []invkfile.Implementation{
			{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeVirtual}}}},
		},
	}

	tests := []struct {
		runtime  invkfile.RuntimeMode
		expected bool
	}{
		{invkfile.RuntimeNative, true},
		{invkfile.RuntimeVirtual, true},
		{invkfile.RuntimeContainer, false},
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
	currentPlatform := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected string
	}{
		{
			name: "single runtime with asterisk",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}},
				},
			},
			expected: "native*",
		},
		{
			name: "multiple runtimes with first marked",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeContainer}}}},
				},
			},
			expected: "native*, container",
		},
		{
			name: "empty scripts",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
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
	deps := &invkfile.DependsOn{
		Capabilities: []invkfile.CapabilityDependency{},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkCapabilityDependencies() with empty capabilities returned error: %v", err)
	}
}

func TestCheckCapabilityDependencies_NilDeps(t *testing.T) {
	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(nil, ctx)
	if err != nil {
		t.Errorf("checkCapabilityDependencies() with nil deps returned error: %v", err)
	}
}

func TestCheckCapabilityDependencies_DuplicateSkipped(t *testing.T) {
	// This test verifies that duplicate capabilities are silently skipped
	// The actual success/failure depends on network connectivity
	deps := &invkfile.DependsOn{
		Capabilities: []invkfile.CapabilityDependency{
			{Alternatives: []invkfile.CapabilityName{invkfile.CapabilityLocalAreaNetwork}},
			{Alternatives: []invkfile.CapabilityName{invkfile.CapabilityLocalAreaNetwork}}, // duplicate
			{Alternatives: []invkfile.CapabilityName{invkfile.CapabilityLocalAreaNetwork}}, // another duplicate
		},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test"},
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

// Tests for env_vars dependency validation

func TestCheckEnvVarDependencies_ExistingEnvVar(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{
			{
				Alternatives: []invkfile.EnvVarCheck{
					{Name: "TEST_ENV_VAR"},
				},
			},
		},
	}

	userEnv := map[string]string{
		"TEST_ENV_VAR": "some_value",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when env var exists, got: %v", err)
	}
}

func TestCheckEnvVarDependencies_MissingEnvVar(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{
			{
				Alternatives: []invkfile.EnvVarCheck{
					{Name: "NONEXISTENT_ENV_VAR"},
				},
			},
		},
	}

	userEnv := map[string]string{} // Empty environment

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail when env var is missing")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingEnvVars) != 1 {
		t.Errorf("Expected 1 missing env var error, got %d", len(depErr.MissingEnvVars))
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "NONEXISTENT_ENV_VAR") {
		t.Errorf("Error message should contain env var name, got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_ValidationRegexPass(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{
			{
				Alternatives: []invkfile.EnvVarCheck{
					{Name: "GO_VERSION", Validation: `^[0-9]+\.[0-9]+\.[0-9]+$`},
				},
			},
		},
	}

	userEnv := map[string]string{
		"GO_VERSION": "1.25.0",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when regex matches, got: %v", err)
	}
}

func TestCheckEnvVarDependencies_ValidationRegexFail(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{
			{
				Alternatives: []invkfile.EnvVarCheck{
					{Name: "GO_VERSION", Validation: `^[0-9]+\.[0-9]+\.[0-9]+$`},
				},
			},
		},
	}

	userEnv := map[string]string{
		"GO_VERSION": "invalid-version",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail when regex doesn't match")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingEnvVars) != 1 {
		t.Errorf("Expected 1 env var error, got %d", len(depErr.MissingEnvVars))
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "does not match required pattern") {
		t.Errorf("Error message should mention pattern mismatch, got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_AlternativesORSemantics(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{
			{
				Alternatives: []invkfile.EnvVarCheck{
					{Name: "AWS_ACCESS_KEY_ID"},
					{Name: "AWS_PROFILE"},
				},
			},
		},
	}

	// Test 1: First alternative exists
	userEnv := map[string]string{
		"AWS_ACCESS_KEY_ID": "AKIAIOSFODNN7EXAMPLE",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when first alternative exists, got: %v", err)
	}

	// Test 2: Second alternative exists
	userEnv = map[string]string{
		"AWS_PROFILE": "dev",
	}

	err = checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when second alternative exists, got: %v", err)
	}

	// Test 3: Neither alternative exists
	userEnv = map[string]string{}

	err = checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail when no alternatives exist")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "none of") {
		t.Errorf("Error message should mention 'none of' for multiple alternatives, got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_EmptyName(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{
			{
				Alternatives: []invkfile.EnvVarCheck{
					{Name: "   "}, // Whitespace-only name
				},
			},
		},
	}

	userEnv := map[string]string{}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail with empty name")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "empty") {
		t.Errorf("Error message should mention 'empty', got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_NilDeps(t *testing.T) {
	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(nil, map[string]string{}, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should handle nil deps gracefully, got: %v", err)
	}
}

func TestCheckEnvVarDependencies_EmptyEnvVars(t *testing.T) {
	deps := &invkfile.DependsOn{
		EnvVars: []invkfile.EnvVarDependency{}, // Empty list
	}

	ctx := &runtime.ExecutionContext{
		Command: &invkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, map[string]string{}, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should handle empty env_vars list gracefully, got: %v", err)
	}
}

func TestDependencyError_WithEnvVars(t *testing.T) {
	err := &DependencyError{
		CommandName: "test",
		MissingEnvVars: []string{
			"  • AWS_ACCESS_KEY_ID - not set in environment",
		},
	}

	expected := "dependencies not satisfied for command 'test'"
	if err.Error() != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestRenderDependencyError_MissingEnvVars(t *testing.T) {
	err := &DependencyError{
		CommandName: "deploy",
		MissingEnvVars: []string{
			"  • AWS_ACCESS_KEY_ID - not set in environment",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Dependencies not satisfied") {
		t.Error("RenderDependencyError should contain header")
	}

	if !strings.Contains(output, "'deploy'") {
		t.Error("RenderDependencyError should contain command name")
	}

	if !strings.Contains(output, "Missing or Invalid Environment Variables") {
		t.Error("RenderDependencyError should contain 'Missing or Invalid Environment Variables' section")
	}

	if !strings.Contains(output, "AWS_ACCESS_KEY_ID") {
		t.Error("RenderDependencyError should contain env var name")
	}
}

func TestRenderDependencyError_AllDependencyTypesIncludingEnvVars(t *testing.T) {
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
		MissingEnvVars: []string{
			"  • AWS_ACCESS_KEY_ID - not set in environment",
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

	if !strings.Contains(output, "Missing or Invalid Environment Variables") {
		t.Error("RenderDependencyError should contain 'Missing or Invalid Environment Variables' section")
	}
}

func TestCaptureUserEnv(t *testing.T) {
	// Set a test environment variable
	testKey := "INVOWK_TEST_CAPTURE_ENV_VAR"
	testValue := "test_value_12345"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	env := captureUserEnv()

	if env[testKey] != testValue {
		t.Errorf("captureUserEnv() should capture env var, got %q, want %q", env[testKey], testValue)
	}

	// Verify PATH is captured (should exist on all systems)
	if _, exists := env["PATH"]; !exists {
		t.Error("captureUserEnv() should capture PATH environment variable")
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
	// the actual runCommandWithFlags requires a full invkfile setup

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
func testCmdWithFlags(name string, script string, flags []invkfile.Flag) *invkfile.Command {
	return &invkfile.Command{
		Name:  name,
		Flags: flags,
		Implementations: []invkfile.Implementation{
			{Script: script, Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}},
		},
	}
}

func TestCommand_WithFlags(t *testing.T) {
	flags := []invkfile.Flag{
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
		args     []invkfile.Argument
		expected string
	}{
		{
			name:     "no arguments",
			cmdPart:  "deploy",
			args:     []invkfile.Argument{},
			expected: "deploy",
		},
		{
			name:    "single required argument",
			cmdPart: "deploy",
			args: []invkfile.Argument{
				{Name: "env", Required: true},
			},
			expected: "deploy <env>",
		},
		{
			name:    "single optional argument",
			cmdPart: "deploy",
			args: []invkfile.Argument{
				{Name: "env", Required: false},
			},
			expected: "deploy [env]",
		},
		{
			name:    "required and optional arguments",
			cmdPart: "deploy",
			args: []invkfile.Argument{
				{Name: "env", Required: true},
				{Name: "replicas", Required: false},
			},
			expected: "deploy <env> [replicas]",
		},
		{
			name:    "required variadic argument",
			cmdPart: "deploy",
			args: []invkfile.Argument{
				{Name: "services", Required: true, Variadic: true},
			},
			expected: "deploy <services>...",
		},
		{
			name:    "optional variadic argument",
			cmdPart: "deploy",
			args: []invkfile.Argument{
				{Name: "services", Required: false, Variadic: true},
			},
			expected: "deploy [services]...",
		},
		{
			name:    "multiple args with variadic",
			cmdPart: "deploy",
			args: []invkfile.Argument{
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
		args          []invkfile.Argument
		shouldHave    []string
		shouldNotHave []string
	}{
		{
			name: "required argument",
			args: []invkfile.Argument{
				{Name: "env", Description: "Target environment", Required: true},
			},
			shouldHave: []string{"env", "(required)", "Target environment"},
		},
		{
			name: "optional with default",
			args: []invkfile.Argument{
				{Name: "replicas", Description: "Number of replicas", DefaultValue: "1"},
			},
			shouldHave: []string{"replicas", `(default: "1")`, "Number of replicas"},
		},
		{
			name: "optional without default",
			args: []invkfile.Argument{
				{Name: "tag", Description: "Image tag"},
			},
			shouldHave: []string{"tag", "(optional)", "Image tag"},
		},
		{
			name: "typed argument",
			args: []invkfile.Argument{
				{Name: "count", Description: "Count value", Type: invkfile.ArgumentTypeInt},
			},
			shouldHave: []string{"count", "[int]", "Count value"},
		},
		{
			name: "variadic argument",
			args: []invkfile.Argument{
				{Name: "services", Description: "Services to deploy", Variadic: true},
			},
			shouldHave: []string{"services", "(variadic)", "Services to deploy"},
		},
		{
			name: "string type not shown",
			args: []invkfile.Argument{
				{Name: "name", Description: "Name", Type: invkfile.ArgumentTypeString},
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
		ArgDefs: []invkfile.Argument{
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
		ArgDefs: []invkfile.Argument{
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
	args := []invkfile.Argument{
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
func testCmdWithArgs(name string, script string, args []invkfile.Argument) *invkfile.Command {
	return &invkfile.Command{
		Name: name,
		Args: args,
		Implementations: []invkfile.Implementation{
			{Script: script, Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}}},
		},
	}
}

func TestCommand_WithArgs(t *testing.T) {
	args := []invkfile.Argument{
		{Name: "env", Description: "Target environment", Required: true},
		{Name: "replicas", Description: "Number of replicas", Type: invkfile.ArgumentTypeInt, DefaultValue: "1"},
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
	if cmd.Args[1].Type != invkfile.ArgumentTypeInt {
		t.Errorf("Second arg type should be 'int', got %q", cmd.Args[1].Type)
	}
	if cmd.Args[1].DefaultValue != "1" {
		t.Errorf("Second arg default value should be '1', got %q", cmd.Args[1].DefaultValue)
	}
	if !cmd.Args[2].Variadic {
		t.Error("Third arg should be variadic")
	}
}
