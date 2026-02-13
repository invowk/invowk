// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/internal/runtime"
	"invowk-cli/internal/testutil"
	"invowk-cli/internal/testutil/invowkfiletest"
	"invowk-cli/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// Tool dependency tests
// ---------------------------------------------------------------------------

func TestCheckToolDependencies_NoTools(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test", invowkfiletest.WithScript("echo hello"))

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckToolDependencies_EmptyDependsOn(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{}))

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

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{{Alternatives: []string{existingTool}}},
		}))

	err := checkToolDependencies(cmd)
	if err != nil {
		t.Errorf("checkToolDependencies() should return nil for existing tool '%s', got: %v", existingTool, err)
	}
}

func TestCheckToolDependencies_ToolNotExists(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{{Alternatives: []string{"nonexistent-tool-xyz-12345"}}},
		}))

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for non-existent tool")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
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
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{
				{Alternatives: []string{"nonexistent-tool-1"}},
				{Alternatives: []string{"nonexistent-tool-2"}},
				{Alternatives: []string{"nonexistent-tool-3"}},
			},
		}))

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error for non-existent tools")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
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

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{
				{Alternatives: []string{existingTool}},
				{Alternatives: []string{"nonexistent-tool-xyz"}},
			},
		}))

	err := checkToolDependencies(cmd)
	if err == nil {
		t.Error("checkToolDependencies() should return error when any tool is missing")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
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

// ---------------------------------------------------------------------------
// Command dependency tests
// ---------------------------------------------------------------------------

func TestCheckCommandDependenciesExist_SatisfiedByLocalUnqualifiedName(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	homeCleanup := testutil.SetHomeDir(t, tmpDir)
	t.Cleanup(homeCleanup)

	// invowkfile.cue now only contains commands - module metadata is in invowkmod.cue
	invowkfileContent := `cmds: [
	{
		name: "build"
		implementations: [{
			script: "echo build"
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}]
		}]
	},
	{
		name: "deploy"
		implementations: [{
			script: "echo deploy"
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}]
		}]
	},
]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Standalone invowkfile has no module identifier, so pass empty string
	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []string{"build"}}}}
	ctx := &runtime.ExecutionContext{Command: &invowkfile.Command{Name: "deploy"}}

	if err := checkCommandDependenciesExist(config.DefaultConfig(), deps, "", ctx); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckCommandDependenciesExist_SatisfiedByModuleFromUserDir(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	homeCleanup := testutil.SetHomeDir(t, tmpDir)
	t.Cleanup(homeCleanup)

	// Root invowkfile with a command that depends on a user-dir module command
	invowkfileContent := `cmds: [{
	name: "deploy"
	implementations: [{
		script: "echo deploy"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	// Create a module in ~/.invowk/cmds/ (user-dir discovers modules only)
	userModuleDir := filepath.Join(tmpDir, ".invowk", "cmds", "shared.invowkmod")
	if err := os.MkdirAll(userModuleDir, 0o755); err != nil {
		t.Fatalf("failed to create user module dir: %v", err)
	}
	invowkmodContent := `module: "shared"
version: "1.0.0"
`
	if err := os.WriteFile(filepath.Join(userModuleDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	userInvowkfileContent := `cmds: [{
	name: "generate-types"
	implementations: [{
		script: "echo generate"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`
	if err := os.WriteFile(filepath.Join(userModuleDir, "invowkfile.cue"), []byte(userInvowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write user module invowkfile: %v", err)
	}

	// Module command is prefixed: "shared generate-types"
	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []string{"shared generate-types"}}}}
	ctx := &runtime.ExecutionContext{Command: &invowkfile.Command{Name: "deploy"}}

	if err := checkCommandDependenciesExist(config.DefaultConfig(), deps, "", ctx); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckCommandDependenciesExist_MissingCommand(t *testing.T) {
	tmpDir := t.TempDir()

	originalWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	homeCleanup := testutil.SetHomeDir(t, tmpDir)
	t.Cleanup(homeCleanup)

	// invowkfile.cue now only contains commands - module metadata is in invowkmod.cue
	invowkfileContent := `cmds: [{
	name: "deploy"
	implementations: [{
		script: "echo deploy"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(invowkfileContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []string{"build"}}}}
	ctx := &runtime.ExecutionContext{Command: &invowkfile.Command{Name: "deploy"}}

	err := checkCommandDependenciesExist(config.DefaultConfig(), deps, "", ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("expected *DependencyError, got %T", err)
	}

	if len(depErr.MissingCommands) != 1 {
		t.Fatalf("expected 1 missing command, got %d", len(depErr.MissingCommands))
	}
	if !strings.Contains(depErr.MissingCommands[0], "build") {
		t.Errorf("expected missing command error to mention 'build', got %q", depErr.MissingCommands[0])
	}
}

// ---------------------------------------------------------------------------
// Custom check tests
// ---------------------------------------------------------------------------

func TestCheckCustomChecks_Success(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:         "test-check",
					CheckScript:  "echo 'test output'",
					ExpectedCode: new(0),
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil for successful check script, got: %v", err)
	}
}

func TestCheckCustomChecks_WrongExitCode(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:         "test-check",
					CheckScript:  "exit 1",
					ExpectedCode: new(0),
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err == nil {
		t.Error("checkCustomChecks() should return error for wrong exit code")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkCustomChecks() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0], "exit code") {
		t.Errorf("Error message should mention exit code, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckCustomChecks_ExpectedNonZeroCode(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:         "test-check",
					CheckScript:  "exit 42",
					ExpectedCode: new(42),
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil when exit code matches expected, got: %v", err)
	}
}

func TestCheckCustomChecks_OutputMatch(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:           "test-check",
					CheckScript:    "echo 'version 1.2.3'",
					ExpectedOutput: "version [0-9]+\\.[0-9]+\\.[0-9]+",
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil for matching output, got: %v", err)
	}
}

func TestCheckCustomChecks_OutputNoMatch(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:           "test-check",
					CheckScript:    "echo 'hello world'",
					ExpectedOutput: "^version",
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err == nil {
		t.Error("checkCustomChecks() should return error for non-matching output")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkCustomChecks() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0], "does not match pattern") {
		t.Errorf("Error message should mention pattern mismatch, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckCustomChecks_BothCodeAndOutput(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:           "test-check",
					CheckScript:    "echo 'go version go1.21.0'",
					ExpectedCode:   new(0),
					ExpectedOutput: "go1\\.",
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err != nil {
		t.Errorf("checkCustomChecks() should return nil when both code and output match, got: %v", err)
	}
}

func TestCheckCustomChecks_InvalidRegex(t *testing.T) {
	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:           "test-check",
					CheckScript:    "echo 'test'",
					ExpectedOutput: "[invalid regex(",
				},
			},
		}))

	err := checkCustomChecks(cmd)
	if err == nil {
		t.Error("checkCustomChecks() should return error for invalid regex")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkCustomChecks() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0], "invalid regex") {
		t.Errorf("Error message should mention invalid regex, got: %s", depErr.FailedCustomChecks[0])
	}
}
