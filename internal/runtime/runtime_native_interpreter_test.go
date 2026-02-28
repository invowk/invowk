// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestNativeRuntime_InterpreterShebangDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script with Python shebang (should auto-detect)
	script := `#!/usr/bin/env python3
print("Hello from Python")`

	cmd := testCommandWithScript("python-shebang", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from Python" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from Python")
	}
}

func TestNativeRuntime_ExplicitInterpreter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script without shebang but with explicit interpreter
	script := `import sys
print(f"Python version: {sys.version_info.major}.{sys.version_info.minor}")`

	cmd := testCommandWithInterpreter("python-explicit", script, "python3", invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(output, "Python version: 3.") {
		t.Errorf("Execute() output = %q, want something starting with 'Python version: 3.'", output)
	}
}

func TestNativeRuntime_InterpreterWithArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script with shebang including -u flag for unbuffered output
	script := `#!/usr/bin/env -S python3 -u
import sys
print(f"arg1={sys.argv[1] if len(sys.argv) > 1 else 'none'}")`

	cmd := testCommandWithScript("python-args", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.PositionalArgs = []string{"hello-world"}

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "arg1=hello-world" {
		t.Errorf("Execute() output = %q, want %q", output, "arg1=hello-world")
	}
}

func TestNativeRuntime_InterpreterScriptFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	// Create a Python script file
	scriptContent := `#!/usr/bin/env python3
print("Hello from Python file")
`
	scriptPath := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	cmd := testCommandWithScript("python-file", "./test.py", invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from Python file" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from Python file")
	}
}

func TestNativeRuntime_InterpreterNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script with a non-existent interpreter
	script := `print("Hello")`

	cmd := testCommandWithInterpreter("nonexistent-interp", script, "nonexistent-interpreter-xyz", invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode == 0 {
		t.Error("Execute() expected non-zero exit code for nonexistent interpreter")
	}
	if result.Error == nil {
		t.Error("Execute() expected error for nonexistent interpreter")
	}
	if result.Error != nil && !strings.Contains(result.Error.Error(), "not found") {
		t.Errorf("Execute() error = %q, want error containing 'not found'", result.Error)
	}
}

func TestNativeRuntime_InterpreterCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script with Python shebang
	script := `#!/usr/bin/env python3
import sys
print("stdout output")
print("stderr output", file=sys.stderr)`

	cmd := testCommandWithScript("python-capture", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	result := rt.ExecuteCapture(ctx)
	if result.ExitCode != 0 {
		t.Errorf("ExecuteCapture() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	if !strings.Contains(result.Output, "stdout output") {
		t.Errorf("ExecuteCapture() stdout = %q, want to contain 'stdout output'", result.Output)
	}
	if !strings.Contains(result.ErrOutput, "stderr output") {
		t.Errorf("ExecuteCapture() stderr = %q, want to contain 'stderr output'", result.ErrOutput)
	}
}

func TestNativeRuntime_PrepareCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Simple echo command
	script := `echo "PrepareCommand test"`

	cmd := testCommandWithScript("prepare-test", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	// Prepare the command without executing
	prepared, err := rt.PrepareCommand(ctx)
	if err != nil {
		t.Fatalf("PrepareCommand() error = %v", err)
	}

	// Verify the command was created
	if prepared.Cmd == nil {
		t.Fatal("PrepareCommand() returned nil Cmd")
	}

	// Verify the command path is set (should be a shell)
	if prepared.Cmd.Path == "" {
		t.Error("PrepareCommand() Cmd.Path is empty")
	}

	// Verify environment is set
	if len(prepared.Cmd.Env) == 0 {
		t.Error("PrepareCommand() Cmd.Env is empty")
	}

	// Execute the prepared command and verify output
	var stdout bytes.Buffer
	prepared.Cmd.Stdout = &stdout
	prepared.Cmd.Stderr = &bytes.Buffer{}

	err = prepared.Cmd.Run()
	if err != nil {
		t.Errorf("prepared.Cmd.Run() error = %v", err)
	}

	// Cleanup if needed
	if prepared.Cleanup != nil {
		prepared.Cleanup()
	}

	output := strings.TrimSpace(stdout.String())
	if output != "PrepareCommand test" {
		t.Errorf("prepared command output = %q, want %q", output, "PrepareCommand test")
	}
}

func TestNativeRuntime_PrepareCommandWithInterpreter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if python3 is available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available, skipping test")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Python script with explicit interpreter
	script := `print("PrepareCommand Python test")`

	cmd := testCommandWithInterpreter("prepare-python", script, "python3", invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	// Prepare the command without executing
	prepared, err := rt.PrepareCommand(ctx)
	if err != nil {
		t.Fatalf("PrepareCommand() error = %v", err)
	}

	// Verify the command was created
	if prepared.Cmd == nil {
		t.Fatal("PrepareCommand() returned nil Cmd")
	}

	// For interpreter-based command, cleanup should be set (temp file created)
	if prepared.Cleanup == nil {
		t.Error("PrepareCommand() with inline interpreter script should have Cleanup function")
	}

	// Execute the prepared command and verify output
	var stdout bytes.Buffer
	prepared.Cmd.Stdout = &stdout
	prepared.Cmd.Stderr = &bytes.Buffer{}

	err = prepared.Cmd.Run()
	if err != nil {
		t.Errorf("prepared.Cmd.Run() error = %v", err)
	}

	// Cleanup
	if prepared.Cleanup != nil {
		prepared.Cleanup()
	}

	output := strings.TrimSpace(stdout.String())
	if output != "PrepareCommand Python test" {
		t.Errorf("prepared command output = %q, want %q", output, "PrepareCommand Python test")
	}
}
