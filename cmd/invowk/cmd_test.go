package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/pkg/invowkfile"
)

// testCmd creates a Command with a single script for testing
func testCmd(name string, script string) *invowkfile.Command {
	return &invowkfile.Command{
		Name: name,
		Scripts: []invowkfile.Script{
			{Script: script, Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}},
		},
	}
}

// testCmdWithDeps creates a Command with a single script and dependencies
func testCmdWithDeps(name string, script string, deps *invowkfile.DependsOn) *invowkfile.Command {
	return &invowkfile.Command{
		Name:      name,
		Scripts:   []invowkfile.Script{{Script: script, Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}}},
		DependsOn: deps,
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
		Tools: []invowkfile.ToolDependency{{Name: existingTool}},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for existing tool '%s', got: %v", existingTool, err)
	}
}

func TestCheckToolDependencies_ToolNotExists(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Name: "nonexistent-tool-xyz-12345"}},
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
			{Name: "nonexistent-tool-1"},
			{Name: "nonexistent-tool-2"},
			{Name: "nonexistent-tool-3"},
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
			{Name: existingTool},
			{Name: "nonexistent-tool-xyz"},
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

func TestCheckToolDependencies_CustomCheckScript_Success(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:         "sh",
				CheckScript:  "echo 'test output'",
				ExpectedCode: intPtr(0),
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for successful check script, got: %v", err)
	}
}

func TestCheckToolDependencies_CustomCheckScript_WrongExitCode(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:         "sh",
				CheckScript:  "exit 1",
				ExpectedCode: intPtr(0),
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for wrong exit code")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingTools[0], "exit code") {
		t.Errorf("Error message should mention exit code, got: %s", depErr.MissingTools[0])
	}
}

func TestCheckToolDependencies_CustomCheckScript_ExpectedNonZeroCode(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:         "sh",
				CheckScript:  "exit 42",
				ExpectedCode: intPtr(42),
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil when exit code matches expected, got: %v", err)
	}
}

func TestCheckToolDependencies_CustomCheckScript_OutputMatch(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:           "sh",
				CheckScript:    "echo 'version 1.2.3'",
				ExpectedOutput: "version [0-9]+\\.[0-9]+\\.[0-9]+",
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for matching output, got: %v", err)
	}
}

func TestCheckToolDependencies_CustomCheckScript_OutputNoMatch(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:           "sh",
				CheckScript:    "echo 'hello world'",
				ExpectedOutput: "^version",
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for non-matching output")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingTools[0], "does not match pattern") {
		t.Errorf("Error message should mention pattern mismatch, got: %s", depErr.MissingTools[0])
	}
}

func TestCheckToolDependencies_CustomCheckScript_BothCodeAndOutput(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:           "sh",
				CheckScript:    "echo 'go version go1.21.0'",
				ExpectedCode:   intPtr(0),
				ExpectedOutput: "go1\\.",
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil when both code and output match, got: %v", err)
	}
}

func TestCheckToolDependencies_CustomCheckScript_InvalidRegex(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:           "sh",
				CheckScript:    "echo 'test'",
				ExpectedOutput: "[invalid regex(",
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for invalid regex")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingTools[0], "invalid regex") {
		t.Errorf("Error message should mention invalid regex, got: %s", depErr.MissingTools[0])
	}
}

func TestCheckToolDependencies_CustomCheckScript_ToolNotInPath(t *testing.T) {
	cmd := testCmdWithDeps("test", "echo hello", &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{
			{
				Name:        "nonexistent-tool-xyz",
				CheckScript: "echo 'test'",
			},
		},
	})

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error when tool not in PATH")
	}

	depErr, ok := err.(*DependencyError)
	if !ok {
		t.Fatalf("checkToolDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingTools[0], "not found in PATH") {
		t.Errorf("Error message should mention not found in PATH, got: %s", depErr.MissingTools[0])
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
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}, Platforms: []invowkfile.Platform{currentOS}},
				},
			},
			expected: true,
		},
		{
			name: "current host not supported",
			cmd: &invowkfile.Command{
				Name: "test",
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}, Platforms: []invowkfile.Platform{"nonexistent"}},
				},
			},
			expected: false,
		},
		{
			name: "all hosts supported (no platforms specified)",
			cmd: &invowkfile.Command{
				Name: "test",
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}},
				},
			},
			expected: true,
		},
		{
			name: "empty scripts list",
			cmd: &invowkfile.Command{
				Name:    "test",
				Scripts: []invowkfile.Script{},
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
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}, Platforms: []invowkfile.Platform{invowkfile.HostLinux}},
				},
			},
			expected: "linux",
		},
		{
			name: "multiple platforms",
			cmd: &invowkfile.Command{
				Name: "test",
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}, Platforms: []invowkfile.Platform{invowkfile.HostLinux, invowkfile.HostMac}},
				},
			},
			expected: "linux, macos",
		},
		{
			name: "all platforms (no platforms specified)",
			cmd: &invowkfile.Command{
				Name: "test",
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}},
				},
			},
			expected: "linux, macos, windows",
		},
		{
			name: "empty scripts",
			cmd: &invowkfile.Command{
				Name:    "test",
				Scripts: []invowkfile.Script{},
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
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative, invowkfile.RuntimeContainer}},
				},
			},
			expected: invowkfile.RuntimeNative,
		},
		{
			name: "container as default",
			cmd: &invowkfile.Command{
				Name: "test",
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeContainer, invowkfile.RuntimeNative}},
				},
			},
			expected: invowkfile.RuntimeContainer,
		},
		{
			name: "empty scripts returns native",
			cmd: &invowkfile.Command{
				Name:    "test",
				Scripts: []invowkfile.Script{},
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
		Scripts: []invowkfile.Script{
			{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative, invowkfile.RuntimeVirtual}},
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
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative}},
				},
			},
			expected: "native*",
		},
		{
			name: "multiple runtimes with first marked",
			cmd: &invowkfile.Command{
				Name: "test",
				Scripts: []invowkfile.Script{
					{Script: "echo", Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative, invowkfile.RuntimeContainer}},
				},
			},
			expected: "native*, container",
		},
		{
			name: "empty scripts",
			cmd: &invowkfile.Command{
				Name:    "test",
				Scripts: []invowkfile.Script{},
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
