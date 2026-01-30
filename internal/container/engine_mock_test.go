// SPDX-License-Identifier: MPL-2.0

package container

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

type (
	// MockCommandRecorder captures arguments passed to exec.Command for verification.
	// It uses the TestHelperProcess pattern to simulate command execution.
	MockCommandRecorder struct {
		// Invocations records each call to the mock exec.Command
		Invocations []MockInvocation
		// ExitCode is the exit code to return (0 = success)
		ExitCode int
		// Stdout is the output to write to stdout
		Stdout string
		// Stderr is the output to write to stderr
		Stderr string
		// FailOnCommand can be set to a command that should fail
		FailOnCommand string
	}

	// MockInvocation represents a single invocation of exec.Command.
	MockInvocation struct {
		// Name is the command name (e.g., "docker", "podman")
		Name string
		// Args are the arguments passed to the command
		Args []string
	}
)

// NewMockCommandRecorder creates a new recorder with default settings (success, no output).
func NewMockCommandRecorder() *MockCommandRecorder {
	return &MockCommandRecorder{
		Invocations: make([]MockInvocation, 0),
		ExitCode:    0,
	}
}

// CommandFunc returns a function that can replace execCommand for testing.
// The function records invocations and returns a command that runs TestHelperProcess.
func (m *MockCommandRecorder) CommandFunc(t *testing.T) func(name string, args ...string) *exec.Cmd {
	t.Helper()
	return func(name string, args ...string) *exec.Cmd {
		// Record the invocation
		m.Invocations = append(m.Invocations, MockInvocation{
			Name: name,
			Args: args,
		})

		// Build a helper process command that will return our configured output
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		//nolint:gosec // TestHelperProcess is a test-only pattern
		cmd := exec.Command(os.Args[0], cs...) //nolint:noctx // exec.Command used intentionally for test helper
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			fmt.Sprintf("GO_HELPER_EXIT_CODE=%d", m.ExitCode),
			fmt.Sprintf("GO_HELPER_STDOUT=%s", m.Stdout),
			fmt.Sprintf("GO_HELPER_STDERR=%s", m.Stderr),
		}

		// Check if this command should fail
		if m.FailOnCommand != "" && len(args) > 0 && args[0] == m.FailOnCommand {
			cmd.Env = append(cmd.Env, "GO_HELPER_EXIT_CODE=1")
		}

		return cmd
	}
}

// ContextCommandFunc returns a function that can replace execCommandContext for testing.
func (m *MockCommandRecorder) ContextCommandFunc(t *testing.T) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmdFunc := m.CommandFunc(t)
	return func(_ context.Context, name string, args ...string) *exec.Cmd {
		return cmdFunc(name, args...)
	}
}

// LastInvocation returns the most recent invocation, or nil if none.
func (m *MockCommandRecorder) LastInvocation() *MockInvocation {
	if len(m.Invocations) == 0 {
		return nil
	}
	return &m.Invocations[len(m.Invocations)-1]
}

// LastArgs returns the arguments from the most recent invocation.
func (m *MockCommandRecorder) LastArgs() []string {
	if inv := m.LastInvocation(); inv != nil {
		return inv.Args
	}
	return nil
}

// AssertCommandName verifies the last command name matches.
func (m *MockCommandRecorder) AssertCommandName(t *testing.T, expected string) {
	t.Helper()
	if inv := m.LastInvocation(); inv == nil {
		t.Errorf("expected command %q but no commands were invoked", expected)
	} else if inv.Name != expected {
		t.Errorf("expected command %q, got %q", expected, inv.Name)
	}
}

// AssertArgsContain verifies that the last invocation args contain the expected string.
func (m *MockCommandRecorder) AssertArgsContain(t *testing.T, expected string) {
	t.Helper()
	args := m.LastArgs()
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, expected) {
		t.Errorf("expected args to contain %q, got: %v", expected, args)
	}
}

// AssertArgsContainAll verifies that the last invocation args contain all expected strings.
func (m *MockCommandRecorder) AssertArgsContainAll(t *testing.T, expected []string) {
	t.Helper()
	args := m.LastArgs()
	argsStr := strings.Join(args, " ")
	for _, exp := range expected {
		if !strings.Contains(argsStr, exp) {
			t.Errorf("expected args to contain %q, got: %v", exp, args)
		}
	}
}

// AssertArgsNotContain verifies that the last invocation args do NOT contain the expected string.
func (m *MockCommandRecorder) AssertArgsNotContain(t *testing.T, unexpected string) {
	t.Helper()
	args := m.LastArgs()
	argsStr := strings.Join(args, " ")
	if strings.Contains(argsStr, unexpected) {
		t.Errorf("expected args to NOT contain %q, got: %v", unexpected, args)
	}
}

// AssertFirstArg verifies the first argument (subcommand) matches.
func (m *MockCommandRecorder) AssertFirstArg(t *testing.T, expected string) {
	t.Helper()
	args := m.LastArgs()
	if len(args) == 0 {
		t.Errorf("expected first arg %q but args are empty", expected)
		return
	}
	if args[0] != expected {
		t.Errorf("expected first arg %q, got %q", expected, args[0])
	}
}

// AssertInvocationCount verifies the number of command invocations.
func (m *MockCommandRecorder) AssertInvocationCount(t *testing.T, expected int) {
	t.Helper()
	if len(m.Invocations) != expected {
		t.Errorf("expected %d invocations, got %d", expected, len(m.Invocations))
	}
}

// HasArg checks if the last invocation contains a specific argument.
func (m *MockCommandRecorder) HasArg(arg string) bool {
	return slices.Contains(m.LastArgs(), arg)
}

// HasArgPair checks if the last invocation contains a flag-value pair (e.g., "-t", "myimage").
func (m *MockCommandRecorder) HasArgPair(flag, value string) bool {
	args := m.LastArgs()
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// Reset clears all recorded invocations.
func (m *MockCommandRecorder) Reset() {
	m.Invocations = m.Invocations[:0]
}

// TestHelperProcess is used by the mock to simulate command execution.
// It reads configuration from environment variables and outputs accordingly.
// This function should not be called directly - it is invoked by the mock.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Write configured stdout
	if stdout := os.Getenv("GO_HELPER_STDOUT"); stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
	}

	// Write configured stderr
	if stderr := os.Getenv("GO_HELPER_STDERR"); stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
	}

	// Exit with configured code
	exitCode := 0
	if code := os.Getenv("GO_HELPER_EXIT_CODE"); code != "" {
		fmt.Sscanf(code, "%d", &exitCode)
	}

	os.Exit(exitCode)
}

// withMockExecCommand sets up exec command mocking for a test and returns a cleanup function.
// Usage:
//
//	recorder, cleanup := withMockExecCommand(t)
//	defer cleanup()
//	// ... test code ...
//	recorder.AssertArgsContain(t, "build")
func withMockExecCommand(t *testing.T) (recorder *MockCommandRecorder, cleanup func()) {
	t.Helper()

	recorder = NewMockCommandRecorder()
	oldExecCommand := execCommand
	execCommand = recorder.ContextCommandFunc(t)

	cleanup = func() {
		execCommand = oldExecCommand
	}

	return recorder, cleanup
}

// withMockExecCommandOutput sets up exec command mocking with configured output.
func withMockExecCommandOutput(t *testing.T, stdout, stderr string, exitCode int) (recorder *MockCommandRecorder, cleanup func()) {
	t.Helper()

	recorder = NewMockCommandRecorder()
	recorder.Stdout = stdout
	recorder.Stderr = stderr
	recorder.ExitCode = exitCode

	oldExecCommand := execCommand
	execCommand = recorder.ContextCommandFunc(t)

	cleanup = func() {
		execCommand = oldExecCommand
	}

	return recorder, cleanup
}

// TestMockCommandRecorder_Basic verifies the mock recorder works correctly.
func TestMockCommandRecorder_Basic(t *testing.T) {
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	// Create and run a mock command
	cmd := execCommand(context.Background(), "docker", "build", "-t", "test:latest", ".")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify recording
	recorder.AssertInvocationCount(t, 1)
	recorder.AssertCommandName(t, "docker")
	recorder.AssertFirstArg(t, "build")
	recorder.AssertArgsContain(t, "-t")
	recorder.AssertArgsContain(t, "test:latest")
}

// TestMockCommandRecorder_Output verifies the mock can produce output.
func TestMockCommandRecorder_Output(t *testing.T) {
	recorder, cleanup := withMockExecCommandOutput(t, "version 1.0.0", "", 0)
	defer cleanup()

	cmd := execCommand(context.Background(), "docker", "version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.String() != "version 1.0.0" {
		t.Errorf("expected stdout 'version 1.0.0', got %q", stdout.String())
	}

	recorder.AssertInvocationCount(t, 1)
}

// TestMockCommandRecorder_ExitCode verifies the mock can return exit codes.
func TestMockCommandRecorder_ExitCode(t *testing.T) {
	_, cleanup := withMockExecCommandOutput(t, "", "build failed", 1)
	defer cleanup()

	cmd := execCommand(context.Background(), "docker", "build")

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}
