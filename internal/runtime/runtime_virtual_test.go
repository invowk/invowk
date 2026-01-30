// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVirtualRuntime_InlineScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	cmd := testCommandWithScript("test", "echo 'Hello from virtual'", invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from virtual" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from virtual")
	}
}

func TestVirtualRuntime_MultiLineScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	script := `VAR="test value"
echo "Variable is: $VAR"
echo "Done"`

	cmd := testCommandWithScript("multiline", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "Variable is: test value") {
		t.Errorf("Execute() output missing expected content, got: %q", output)
	}
}

func TestVirtualRuntime_ScriptFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	// Create a script file (using POSIX-compatible syntax for virtual shell)
	scriptContent := `echo "Hello from virtual script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	cmd := testCommandWithScript("from-file", "./test.sh", invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from virtual script file" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from virtual script file")
	}
}

func TestVirtualRuntime_Validate_ScriptSyntaxError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Invalid shell syntax
	cmd := testCommandWithScript("invalid", "if then fi", invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)

	err = rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for invalid syntax, got nil")
	}
}

func TestVirtualRuntime_PositionalArgs(t *testing.T) {
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

	cmd := testCommandWithScript("positional", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
	ctx.PositionalArgs = []string{"foo", "bar"}

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.Contains(output, "arg1=foo") {
		t.Errorf("Execute() output missing arg1=foo, got: %q", output)
	}
	if !strings.Contains(output, "arg2=bar") {
		t.Errorf("Execute() output missing arg2=bar, got: %q", output)
	}
	if !strings.Contains(output, "all=foo bar") {
		t.Errorf("Execute() output missing all=foo bar, got: %q", output)
	}
}

func TestVirtualRuntime_PositionalArgs_ArgCount(t *testing.T) {
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
	script := `echo "count=$#"`

	cmd := testCommandWithScript("arg-count", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
	ctx.PositionalArgs = []string{"a", "b", "c", "d", "e"}

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "count=5" {
		t.Errorf("Execute() output = %q, want %q", output, "count=5")
	}
}

func TestVirtualRuntime_PositionalArgs_Empty(t *testing.T) {
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

	cmd := testCommandWithScript("no-args", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
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

func TestVirtualRuntime_EnvIsolation(t *testing.T) {
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

	cmd := testCommandWithScript("env-isolation", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

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

func TestVirtualRuntime_RejectsInterpreter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Try to use interpreter with virtual runtime (should be rejected)
	script := `echo "Hello"`

	cmd := testCommandWithInterpreter("virtual-with-interp", script, "python3", invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	// Test Validate method
	err = rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for interpreter with virtual runtime")
	}
	if err != nil && !strings.Contains(err.Error(), "interpreter field is not allowed for virtual runtime") {
		t.Errorf("Validate() error = %q, want error containing 'interpreter field is not allowed for virtual runtime'", err)
	}

	// Test Execute method (as a safety net)
	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode == 0 {
		t.Error("Execute() expected non-zero exit code for interpreter with virtual runtime")
	}
	if result.Error == nil {
		t.Error("Execute() expected error for interpreter with virtual runtime")
	}
}

func TestVirtualRuntime_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script that runs forever
	script := `while true; do sleep 1; done`

	cmd := testCommandWithScript("long-running", script, invkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	execCtx := NewExecutionContext(cmd, inv)
	execCtx.Context = ctx

	var stdout bytes.Buffer
	execCtx.Stdout = &stdout
	execCtx.Stderr = &bytes.Buffer{}

	// Cancel the context after a short delay
	go func() {
		cancel()
	}()

	result := rt.Execute(execCtx)

	// Should exit with non-zero (or interrupted) after context cancellation
	if result.ExitCode == 0 && result.Error == nil {
		t.Error("Execute() should fail when context is cancelled")
	}
}

func TestVirtualRuntime_ExitCode(t *testing.T) {
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
			cmd := testCommandWithScript("exit-test", tt.script, invkfile.RuntimeVirtual)
			rt := NewVirtualRuntime(false)
			ctx := NewExecutionContext(cmd, inv)
			ctx.Context = context.Background()
			ctx.Stdout = &bytes.Buffer{}
			ctx.Stderr = &bytes.Buffer{}

			result := rt.Execute(ctx)
			if result.ExitCode != tt.expectedCode {
				t.Errorf("Execute() exit code = %d, want %d", result.ExitCode, tt.expectedCode)
			}
		})
	}
}
