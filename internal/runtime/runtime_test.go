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

// testCommandWithScript creates a Command with a single script for testing
func testCommandWithScript(name string, script string, runtime invowkfile.RuntimeMode) *invowkfile.Command {
	return &invowkfile.Command{
		Name: name,
		Implementations: []invowkfile.Implementation{
			{Script: script, Target: invowkfile.Target{Runtimes: []invowkfile.RuntimeConfig{{Name: runtime}}}},
		},
	}
}

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
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("test", "echo 'Hello from inline'", invowkfile.RuntimeNative)

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
		FilePath: invowkfilePath,
	}

	// Multi-line script
	script := `echo "Line 1"
echo "Line 2"
echo "Line 3"`

	cmd := testCommandWithScript("multiline", script, invowkfile.RuntimeNative)

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
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("from-file", "./test.sh", invowkfile.RuntimeNative)

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
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("test", "echo 'Hello from virtual'", invowkfile.RuntimeVirtual)

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
		FilePath: invowkfilePath,
	}

	script := `VAR="test value"
echo "Variable is: $VAR"
echo "Done"`

	cmd := testCommandWithScript("multiline", script, invowkfile.RuntimeVirtual)

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
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("from-file", "./test.sh", invowkfile.RuntimeVirtual)

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
		FilePath: invowkfilePath,
	}

	// Invalid shell syntax
	cmd := testCommandWithScript("invalid", "if then fi", invowkfile.RuntimeVirtual)

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
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("missing", "./nonexistent.sh", invowkfile.RuntimeNative)

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
		cmdVirtual := testCommandWithScript("missing", "./nonexistent.sh", invowkfile.RuntimeVirtual)
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
		FilePath: invowkfilePath,
	}

	currentPlatform := invowkfile.GetCurrentHostOS()
	cmd := &invowkfile.Command{
		Name: "env-test",
		Implementations: []invowkfile.Implementation{
			{
				Script: `echo "Global: $GLOBAL_VAR, Command: $CMD_VAR"`,
				Target: invowkfile.Target{
					Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
					Platforms: []invowkfile.PlatformConfig{{Name: currentPlatform, Env: map[string]string{"GLOBAL_VAR": "global_value"}}},
				},
			},
		},
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

func TestNativeRuntime_PositionalArgs(t *testing.T) {
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
		FilePath: invowkfilePath,
	}

	// Script that echoes positional parameters
	script := `echo "arg1=$1 arg2=$2 all=$@"`

	cmd := testCommandWithScript("positional", script, invowkfile.RuntimeNative)

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
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes the number of positional parameters
	script := `echo "argc=$#"`

	cmd := testCommandWithScript("no-args", script, invowkfile.RuntimeNative)

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

func TestVirtualRuntime_PositionalArgs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes positional parameters
	script := `echo "arg1=$1 arg2=$2 all=$@"`

	cmd := testCommandWithScript("positional", script, invowkfile.RuntimeVirtual)

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
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes the number of positional parameters
	script := `echo "count=$#"`

	cmd := testCommandWithScript("arg-count", script, invowkfile.RuntimeVirtual)

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
	defer os.RemoveAll(tmpDir)

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.toml")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes the number of positional parameters
	script := `echo "argc=$#"`

	cmd := testCommandWithScript("no-args", script, invowkfile.RuntimeVirtual)

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

func TestNativeRuntime_PositionalArgs_SpecialChars(t *testing.T) {
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
		FilePath: invowkfilePath,
	}

	// Script that echoes the first positional parameter
	script := `echo "arg1=$1"`

	cmd := testCommandWithScript("special-chars", script, invowkfile.RuntimeNative)

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
