package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/pkg/invowkfile"
)

func TestNativeRuntime_InlineScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary invowkfile
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	cmd := &invowkfile.Command{
		Name:    "test",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
		Script:  "echo 'Hello from inline'",
	}

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
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	// Multi-line script
	script := `echo "Line 1"
echo "Line 2"
echo "Line 3"`

	cmd := &invowkfile.Command{
		Name:    "multiline",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
		Script:  script,
	}

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
	defer os.RemoveAll(tmpDir)

	// Create a script file
	scriptContent := `#!/bin/bash
echo "Hello from script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	cmd := &invowkfile.Command{
		Name:    "from-file",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
		Script:  "./test.sh",
	}

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

func TestVirtualRuntime_InlineScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	cmd := &invowkfile.Command{
		Name:    "test",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
		Script:  "echo 'Hello from virtual'",
	}

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
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	script := `VAR="test value"
echo "Variable is: $VAR"
echo "Done"`

	cmd := &invowkfile.Command{
		Name:    "multiline",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
		Script:  script,
	}

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
	defer os.RemoveAll(tmpDir)

	// Create a script file (using POSIX-compatible syntax for virtual shell)
	scriptContent := `echo "Hello from virtual script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	cmd := &invowkfile.Command{
		Name:    "from-file",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
		Script:  "./test.sh",
	}

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
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	// Invalid shell syntax
	cmd := &invowkfile.Command{
		Name:    "invalid",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
		Script:  "if then fi",
	}

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(cmd, inv)

	err = rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for invalid syntax, got nil")
	}
}

func TestRuntime_ScriptNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
	}

	cmd := &invowkfile.Command{
		Name:    "missing",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeNative},
		Script:  "./nonexistent.sh",
	}

	t.Run("native runtime", func(t *testing.T) {
		rt := NewNativeRuntime()
		ctx := NewExecutionContext(cmd, inv)
		ctx.Stdout = &bytes.Buffer{}
		ctx.Stderr = &bytes.Buffer{}

		result := rt.Execute(ctx)
		if result.Error == nil {
			t.Error("Execute() expected error for missing script file, got nil")
		}
	})

	t.Run("virtual runtime", func(t *testing.T) {
		cmdVirtual := &invowkfile.Command{
			Name:    "missing",
			Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
			Script:  "./nonexistent.sh",
		}
		rt := NewVirtualRuntime(false)
		ctx := NewExecutionContext(cmdVirtual, inv)
		ctx.Context = context.Background()
		ctx.Stdout = &bytes.Buffer{}
		ctx.Stderr = &bytes.Buffer{}

		result := rt.Execute(ctx)
		if result.Error == nil {
			t.Error("Execute() expected error for missing script file, got nil")
		}
	})
}

func TestRuntime_EnvironmentVariables(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")

	inv := &invowkfile.Invowkfile{
		FilePath:       invowkfilePath,
		DefaultRuntime: invowkfile.RuntimeNative,
		Env: map[string]string{
			"GLOBAL_VAR": "global_value",
		},
	}

	cmd := &invowkfile.Command{
		Name:    "env-test",
		Runtimes: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtual},
		Script:  `echo "Global: $GLOBAL_VAR, Command: $CMD_VAR"`,
		Env: map[string]string{
			"CMD_VAR": "command_value",
		},
	}

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
	if !strings.Contains(output, "global_value") {
		t.Errorf("Execute() output missing global env var, got: %q", output)
	}
	if !strings.Contains(output, "command_value") {
		t.Errorf("Execute() output missing command env var, got: %q", output)
	}
}
