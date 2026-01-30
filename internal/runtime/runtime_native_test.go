// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"errors"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeRuntime_InlineScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary invkfile
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	cmd := testCommandWithScript("test", "echo 'Hello from inline'", invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from inline" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from inline")
	}
}

func TestNativeRuntime_MultiLineScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Multi-line script
	script := `echo "Line 1"
echo "Line 2"
echo "Line 3"`

	cmd := testCommandWithScript("multiline", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "Line 1") || !strings.Contains(output, "Line 2") || !strings.Contains(output, "Line 3") {
		t.Errorf("Execute() output missing expected lines, got: %q", output)
	}
}

func TestNativeRuntime_ScriptFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	// Create a script file
	scriptContent := `#!/bin/bash
echo "Hello from script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	cmd := testCommandWithScript("from-file", "./test.sh", invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from script file" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from script file")
	}
}

func TestNativeRuntime_PositionalArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script that echoes positional parameters
	script := `echo "arg1=$1 arg2=$2 all=$@"`

	cmd := testCommandWithScript("positional", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)
	ctx.PositionalArgs = []string{"hello", "world"}

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, "arg1=hello") {
		t.Errorf("Execute() output missing arg1=hello, got: %q", output)
	}
	if !strings.Contains(output, "arg2=world") {
		t.Errorf("Execute() output missing arg2=world, got: %q", output)
	}
	if !strings.Contains(output, "all=hello world") {
		t.Errorf("Execute() output missing all=hello world, got: %q", output)
	}
}

func TestNativeRuntime_PositionalArgs_Empty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script that echoes the number of positional parameters
	script := `echo "argc=$#"`

	cmd := testCommandWithScript("no-args", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)
	// No positional args set

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "argc=0" {
		t.Errorf("Execute() output = %q, want %q", output, "argc=0")
	}
}

func TestNativeRuntime_PositionalArgs_SpecialChars(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script that echoes the first positional parameter
	script := `echo "arg1=$1"`

	cmd := testCommandWithScript("special-chars", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)
	ctx.PositionalArgs = []string{"hello world with spaces"}

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "arg1=hello world with spaces" {
		t.Errorf("Execute() output = %q, want %q", output, "arg1=hello world with spaces")
	}
}

func TestNativeRuntime_appendPositionalArgs(t *testing.T) {
	rt := NewNativeRuntime()

	tests := []struct {
		name           string
		shell          string
		baseArgs       []string
		positionalArgs []string
		wantArgs       []string
	}{
		{
			name:           "bash with args",
			shell:          "/bin/bash",
			baseArgs:       []string{"-c", "echo hello"},
			positionalArgs: []string{"arg1", "arg2"},
			wantArgs:       []string{"-c", "echo hello", "invowk", "arg1", "arg2"},
		},
		{
			name:           "bash with no args",
			shell:          "/bin/bash",
			baseArgs:       []string{"-c", "echo hello"},
			positionalArgs: []string{},
			wantArgs:       []string{"-c", "echo hello"},
		},
		{
			name:           "sh with args",
			shell:          "/bin/sh",
			baseArgs:       []string{"-c", "echo hello"},
			positionalArgs: []string{"foo"},
			wantArgs:       []string{"-c", "echo hello", "invowk", "foo"},
		},
		{
			name:           "zsh with args",
			shell:          "/usr/bin/zsh",
			baseArgs:       []string{"-c", "echo hello"},
			positionalArgs: []string{"a", "b", "c"},
			wantArgs:       []string{"-c", "echo hello", "invowk", "a", "b", "c"},
		},
		{
			name:           "pwsh with args",
			shell:          "/usr/bin/pwsh",
			baseArgs:       []string{"-NoProfile", "-Command", "Write-Host hello"},
			positionalArgs: []string{"arg1", "arg2"},
			wantArgs:       []string{"-NoProfile", "-Command", "Write-Host hello", "arg1", "arg2"},
		},
		{
			name:           "powershell.exe with args",
			shell:          "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
			baseArgs:       []string{"-NoProfile", "-Command", "Write-Host hello"},
			positionalArgs: []string{"arg1"},
			wantArgs:       []string{"-NoProfile", "-Command", "Write-Host hello", "arg1"},
		},
		{
			name:           "cmd.exe with args (should not add args)",
			shell:          "C:\\Windows\\System32\\cmd.exe",
			baseArgs:       []string{"/C", "echo hello"},
			positionalArgs: []string{"arg1", "arg2"},
			wantArgs:       []string{"/C", "echo hello"}, // cmd doesn't support positional args
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rt.appendPositionalArgs(tt.shell, tt.baseArgs, tt.positionalArgs)
			if len(got) != len(tt.wantArgs) {
				t.Errorf("appendPositionalArgs() length = %d, want %d", len(got), len(tt.wantArgs))
				t.Errorf("  got:  %v", got)
				t.Errorf("  want: %v", tt.wantArgs)
				return
			}
			for i := range got {
				if got[i] != tt.wantArgs[i] {
					t.Errorf("appendPositionalArgs()[%d] = %q, want %q", i, got[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestNativeRuntime_EnvIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Set environment variables that should be filtered
	restoreArg := testutil.MustSetenv(t, "INVOWK_ARG_PARENT", "parent_value")
	restoreFlag := testutil.MustSetenv(t, "INVOWK_FLAG_PARENT", "true")
	restoreArgc := testutil.MustSetenv(t, "ARGC", "5")
	restoreArg1 := testutil.MustSetenv(t, "ARG1", "first")
	defer restoreArg()
	defer restoreFlag()
	defer restoreArgc()
	defer restoreArg1()

	// Script that checks if the parent's env vars are visible
	script := `echo "INVOWK_ARG_PARENT=${INVOWK_ARG_PARENT:-unset}"
echo "INVOWK_FLAG_PARENT=${INVOWK_FLAG_PARENT:-unset}"
echo "ARGC=${ARGC:-unset}"
echo "ARG1=${ARG1:-unset}"`

	cmd := testCommandWithScript("env-isolation", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	// All parent env vars should be filtered (show as "unset")
	if !strings.Contains(output, "INVOWK_ARG_PARENT=unset") {
		t.Errorf("Execute() INVOWK_ARG_PARENT should be unset, got: %q", output)
	}
	if !strings.Contains(output, "INVOWK_FLAG_PARENT=unset") {
		t.Errorf("Execute() INVOWK_FLAG_PARENT should be unset, got: %q", output)
	}
	if !strings.Contains(output, "ARGC=unset") {
		t.Errorf("Execute() ARGC should be unset, got: %q", output)
	}
	if !strings.Contains(output, "ARG1=unset") {
		t.Errorf("Execute() ARG1 should be unset, got: %q", output)
	}
}

func TestNativeRuntime_InvalidWorkingDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	script := `echo "Should not run"`

	// Create command with invalid working directory
	cmd := &invkfile.Command{
		Name: "invalid-workdir",
		Implementations: []invkfile.Implementation{
			{
				Script:   script,
				Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
				WorkDir:  "/nonexistent/directory/that/does/not/exist",
			},
		},
	}

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)

	// Should fail due to invalid working directory
	if result.ExitCode == 0 {
		t.Error("Execute() should fail with invalid working directory")
	}
	if result.Error == nil {
		t.Error("Execute() should return error for invalid working directory")
	}
	if result.Error != nil && !strings.Contains(result.Error.Error(), "directory") {
		t.Errorf("Execute() error = %q, want error mentioning directory", result.Error)
	}
}

func TestNativeRuntime_ExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	tests := []struct {
		name         string
		script       string
		expectedCode int
	}{
		{"exit 0", "exit 0", 0},
		{"exit 1", "exit 1", 1},
		{"exit 42", "exit 42", 42},
		{"false command", "false", 1},
		{"true command", "true", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := testCommandWithScript("exit-test", tt.script, invkfile.RuntimeNative)
			rt := NewNativeRuntime()
			ctx := NewExecutionContext(cmd, inv)
			ctx.Stdout = &bytes.Buffer{}
			ctx.Stderr = &bytes.Buffer{}

			result := rt.Execute(ctx)
			if result.ExitCode != tt.expectedCode {
				t.Errorf("Execute() exit code = %d, want %d", result.ExitCode, tt.expectedCode)
			}
		})
	}
}

// T102: Test for shell not found error format
func TestNativeRuntime_ShellNotFoundError(t *testing.T) {
	rt := &NativeRuntime{
		// Set a shell that doesn't exist to force an error
		Shell: "/this/shell/does/not/exist",
	}

	// getShell should still succeed because it uses Shell directly
	shell, err := rt.getShell()
	if err != nil {
		t.Errorf("getShell() with explicit Shell should succeed, got error: %v", err)
	}
	if shell != "/this/shell/does/not/exist" {
		t.Errorf("getShell() = %q, want /this/shell/does/not/exist", shell)
	}

	// Test the shellNotFoundError helper directly
	errActionable := rt.shellNotFoundError([]string{"$SHELL", "bash", "sh"})
	if errActionable == nil {
		t.Fatal("shellNotFoundError() should return error")
	}

	errStr := errActionable.Error()

	// Verify error contains operation context
	if !strings.Contains(errStr, "find shell") {
		t.Errorf("error should contain operation 'find shell', got: %s", errStr)
	}

	// Verify error contains resource (attempted shells)
	if !strings.Contains(errStr, "shells attempted") {
		t.Errorf("error should contain resource 'shells attempted', got: %s", errStr)
	}

	// Verify error contains "no shell found" cause
	if !strings.Contains(errStr, "no shell found") {
		t.Errorf("error should contain cause 'no shell found', got: %s", errStr)
	}
}

// TestNativeRuntime_ShellNotFoundError_Format tests the verbose formatting
func TestNativeRuntime_ShellNotFoundError_Format(t *testing.T) {
	rt := &NativeRuntime{}

	// Get an actionable error
	errVal := rt.shellNotFoundError([]string{"bash", "sh"})

	// Check that it can be cast to *issue.ActionableError
	var ae *issue.ActionableError
	if !errors.As(errVal, &ae) {
		t.Fatal("shellNotFoundError should return *issue.ActionableError")
	}

	// Test verbose format includes suggestions
	formatted := ae.Format(false)
	if !strings.Contains(formatted, "find shell") {
		t.Errorf("formatted error should contain operation, got: %s", formatted)
	}

	// Test that suggestions are included
	if !strings.Contains(formatted, "•") {
		t.Errorf("formatted error should contain bullet points for suggestions, got: %s", formatted)
	}
}
