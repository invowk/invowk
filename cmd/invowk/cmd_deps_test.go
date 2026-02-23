// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// testDiscoveryService wraps a discovery.Discovery for use in unit tests.
// It satisfies the DiscoveryService interface by delegating to the underlying
// discovery instance without caching (tests need fresh results per invocation).
type testDiscoveryService struct {
	disc *discovery.Discovery
}

func (t *testDiscoveryService) DiscoverCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	return t.disc.DiscoverCommandSet(ctx)
}

func (t *testDiscoveryService) DiscoverAndValidateCommandSet(ctx context.Context) (discovery.CommandSetResult, error) {
	return t.disc.DiscoverAndValidateCommandSet(ctx)
}

func (t *testDiscoveryService) GetCommand(ctx context.Context, name string) (discovery.LookupResult, error) {
	return t.disc.GetCommand(ctx, name)
}

// ---------------------------------------------------------------------------
// Tool dependency tests
// ---------------------------------------------------------------------------

func TestCheckToolDependencies_NoTools(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test", invowkfiletest.WithScript("echo hello"))

	err := checkHostToolDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostToolDependencies() should return nil for command with no dependencies, got: %v", err)
	}
}

func TestCheckToolDependencies_EmptyDependsOn(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{}))

	err := checkHostToolDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostToolDependencies() should return nil for empty depends_on, got: %v", err)
	}
}

func TestCheckToolDependencies_ToolExists(t *testing.T) {
	t.Parallel()

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
			Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{invowkfile.BinaryName(existingTool)}}},
		}))

	err := checkHostToolDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostToolDependencies() should return nil for existing tool '%s', got: %v", existingTool, err)
	}
}

func TestCheckToolDependencies_ToolNotExists(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-xyz-12345"}}},
		}))

	err := checkHostToolDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostToolDependencies() should return error for non-existent tool")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Errorf("checkHostToolDependencies() should return *DependencyError, got: %T", err)
	}

	if depErr.CommandName != "test" {
		t.Errorf("DependencyError.CommandName = %q, want %q", depErr.CommandName, "test")
	}

	if len(depErr.MissingTools) != 1 {
		t.Errorf("DependencyError.MissingTools length = %d, want 1", len(depErr.MissingTools))
	}
}

func TestCheckToolDependencies_MultipleToolsNotExist(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			Tools: []invowkfile.ToolDependency{
				{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-1"}},
				{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-2"}},
				{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-3"}},
			},
		}))

	err := checkHostToolDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostToolDependencies() should return error for non-existent tools")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostToolDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingTools) != 3 {
		t.Errorf("DependencyError.MissingTools length = %d, want 3", len(depErr.MissingTools))
	}
}

func TestCheckToolDependencies_MixedToolsExistAndNotExist(t *testing.T) {
	t.Parallel()

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
				{Alternatives: []invowkfile.BinaryName{invowkfile.BinaryName(existingTool)}},
				{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-xyz"}},
			},
		}))

	err := checkHostToolDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostToolDependencies() should return error when any tool is missing")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostToolDependencies() should return *DependencyError, got: %T", err)
	}

	// Only the non-existent tool should be in the error
	if len(depErr.MissingTools) != 1 {
		t.Errorf("DependencyError.MissingTools length = %d, want 1", len(depErr.MissingTools))
	}

	if !strings.Contains(depErr.MissingTools[0].String(), "nonexistent-tool-xyz") {
		t.Errorf("MissingTools should contain 'nonexistent-tool-xyz', got: %s", depErr.MissingTools[0])
	}
}

// ---------------------------------------------------------------------------
// Command dependency tests
// ---------------------------------------------------------------------------

func TestCheckCommandDependenciesExist_SatisfiedByLocalUnqualifiedName(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

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
	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandName{"build"}}}}
	ctx := &runtime.ExecutionContext{Command: &invowkfile.Command{Name: "deploy"}}
	disc := &testDiscoveryService{disc: discovery.New(config.DefaultConfig(),
		discovery.WithBaseDir(tmpDir),
		discovery.WithCommandsDir(filepath.Join(tmpDir, ".invowk", "cmds")),
	)}

	if err := checkCommandDependenciesExist(disc, deps, "", ctx); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckCommandDependenciesExist_SatisfiedByModuleFromUserDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

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

	// Create a module in the user commands directory
	userCmdsDir := filepath.Join(tmpDir, ".invowk", "cmds")
	userModuleDir := filepath.Join(userCmdsDir, "shared.invowkmod")
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
	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandName{"shared generate-types"}}}}
	ctx := &runtime.ExecutionContext{Command: &invowkfile.Command{Name: "deploy"}}
	disc := &testDiscoveryService{disc: discovery.New(config.DefaultConfig(),
		discovery.WithBaseDir(tmpDir),
		discovery.WithCommandsDir(userCmdsDir),
	)}

	if err := checkCommandDependenciesExist(disc, deps, "", ctx); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckCommandDependenciesExist_MissingCommand(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

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

	deps := &invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandName{"build"}}}}
	ctx := &runtime.ExecutionContext{Command: &invowkfile.Command{Name: "deploy"}}
	disc := &testDiscoveryService{disc: discovery.New(config.DefaultConfig(),
		discovery.WithBaseDir(tmpDir),
		discovery.WithCommandsDir(filepath.Join(tmpDir, ".invowk", "cmds")),
	)}

	err := checkCommandDependenciesExist(disc, deps, "", ctx)
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
	if !strings.Contains(depErr.MissingCommands[0].String(), "build") {
		t.Errorf("expected missing command error to mention 'build', got %q", depErr.MissingCommands[0])
	}
}

// ---------------------------------------------------------------------------
// Custom check tests
// ---------------------------------------------------------------------------

func TestCheckCustomChecks_Success(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:         "test-check",
					CheckScript:  "echo 'test output'",
					ExpectedCode: new(types.ExitCode(0)),
				},
			},
		}))

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil for successful check script, got: %v", err)
	}
}

func TestCheckCustomChecks_WrongExitCode(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:         "test-check",
					CheckScript:  "exit 1",
					ExpectedCode: new(types.ExitCode(0)),
				},
			},
		}))

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostCustomCheckDependencies() should return error for wrong exit code")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostCustomCheckDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0].String(), "exit code") {
		t.Errorf("Error message should mention exit code, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckCustomChecks_ExpectedNonZeroCode(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:         "test-check",
					CheckScript:  "exit 42",
					ExpectedCode: new(types.ExitCode(42)),
				},
			},
		}))

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil when exit code matches expected, got: %v", err)
	}
}

func TestCheckCustomChecks_OutputMatch(t *testing.T) {
	t.Parallel()

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

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil for matching output, got: %v", err)
	}
}

func TestCheckCustomChecks_OutputNoMatch(t *testing.T) {
	t.Parallel()

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

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostCustomCheckDependencies() should return error for non-matching output")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostCustomCheckDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0].String(), "does not match pattern") {
		t.Errorf("Error message should mention pattern mismatch, got: %s", depErr.FailedCustomChecks[0])
	}
}

func TestCheckCustomChecks_BothCodeAndOutput(t *testing.T) {
	t.Parallel()

	cmd := invowkfiletest.NewTestCommand("test",
		invowkfiletest.WithScript("echo hello"),
		invowkfiletest.WithDependsOn(&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{
				{
					Name:           "test-check",
					CheckScript:    "echo 'go version go1.21.0'",
					ExpectedCode:   new(types.ExitCode(0)),
					ExpectedOutput: "go1\\.",
				},
			},
		}))

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil when both code and output match, got: %v", err)
	}
}

func TestCheckCustomChecks_InvalidRegex(t *testing.T) {
	t.Parallel()

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

	err := checkHostCustomCheckDependencies(cmd.DependsOn, &runtime.ExecutionContext{Command: cmd})
	if err == nil {
		t.Error("checkHostCustomCheckDependencies() should return error for invalid regex")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkHostCustomCheckDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.FailedCustomChecks[0].String(), "invalid regex") {
		t.Errorf("Error message should mention invalid regex, got: %s", depErr.FailedCustomChecks[0])
	}
}

// TestRenderDependencyError_FailedCustomChecks verifies that RenderDependencyError
// produces styled output containing the "Failed Custom Checks" section and check names.
func TestRenderDependencyError_FailedCustomChecks(t *testing.T) {
	t.Parallel()

	err := &DependencyError{
		CommandName: "verify",
		FailedCustomChecks: []DependencyMessage{
			"  - docker-version: exit code 127 (expected 0)",
			"  - go-version: output does not match pattern 'go1\\.'",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Dependencies not satisfied") {
		t.Error("RenderDependencyError should contain header")
	}
	if !strings.Contains(output, "'verify'") {
		t.Error("RenderDependencyError should contain command name")
	}
	if !strings.Contains(output, "Failed Custom Checks") {
		t.Error("RenderDependencyError should contain 'Failed Custom Checks' section")
	}
	if !strings.Contains(output, "docker-version") {
		t.Error("RenderDependencyError should contain first failed check name")
	}
	if !strings.Contains(output, "go-version") {
		t.Error("RenderDependencyError should contain second failed check name")
	}
}
