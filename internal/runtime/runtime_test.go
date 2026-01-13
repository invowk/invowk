package runtime

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/pkg/invkfile"
)

// testCommandWithScript creates a Command with a single script for testing
func testCommandWithScript(name string, script string, runtime invkfile.RuntimeMode) *invkfile.Command {
	return &invkfile.Command{
		Name: name,
		Implementations: []invkfile.Implementation{
			{Script: script, Target: invkfile.Target{Runtimes: []invkfile.RuntimeConfig{{Name: runtime}}}},
		},
	}
}

func TestNativeRuntime_InlineScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary invkfile
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

	// Create a script file
	scriptContent := `#!/bin/bash
echo "Hello from script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
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

func TestVirtualRuntime_InlineScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

	// Create a script file (using POSIX-compatible syntax for virtual shell)
	scriptContent := `echo "Hello from virtual script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
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
	defer os.RemoveAll(tmpDir)

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

func TestRuntime_ScriptNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	cmd := testCommandWithScript("missing", "./nonexistent.sh", invkfile.RuntimeNative)

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
		cmdVirtual := testCommandWithScript("missing", "./nonexistent.sh", invkfile.RuntimeVirtual)
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

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	currentPlatform := invkfile.GetCurrentHostOS()
	cmd := &invkfile.Command{
		Name: "env-test",
		Implementations: []invkfile.Implementation{
			{
				Script: `echo "Global: $GLOBAL_VAR, Command: $CMD_VAR"`,
				Target: invkfile.Target{
					Runtimes:  []invkfile.RuntimeConfig{{Name: invkfile.RuntimeVirtual}},
					Platforms: []invkfile.PlatformConfig{{Name: currentPlatform, Env: map[string]string{"GLOBAL_VAR": "global_value"}}},
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
	defer os.RemoveAll(tmpDir)

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

func TestVirtualRuntime_PositionalArgs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

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

func TestNativeRuntime_PositionalArgs_SpecialChars(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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

func TestFilterInvowkEnvVars(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		want    []string
	}{
		{
			name:    "empty environment",
			environ: []string{},
			want:    []string{},
		},
		{
			name:    "no invowk vars",
			environ: []string{"PATH=/usr/bin", "HOME=/home/user", "SHELL=/bin/bash"},
			want:    []string{"PATH=/usr/bin", "HOME=/home/user", "SHELL=/bin/bash"},
		},
		{
			name:    "filter INVOWK_ARG_ vars",
			environ: []string{"PATH=/usr/bin", "INVOWK_ARG_NAME=value", "INVOWK_ARG_FILE=test.txt"},
			want:    []string{"PATH=/usr/bin"},
		},
		{
			name:    "filter INVOWK_FLAG_ vars",
			environ: []string{"HOME=/home/user", "INVOWK_FLAG_VERBOSE=true", "INVOWK_FLAG_DRY_RUN=false"},
			want:    []string{"HOME=/home/user"},
		},
		{
			name:    "filter ARGC var",
			environ: []string{"PATH=/usr/bin", "ARGC=3", "SHELL=/bin/bash"},
			want:    []string{"PATH=/usr/bin", "SHELL=/bin/bash"},
		},
		{
			name:    "filter ARGn vars",
			environ: []string{"PATH=/usr/bin", "ARG1=first", "ARG2=second", "ARG10=tenth"},
			want:    []string{"PATH=/usr/bin"},
		},
		{
			name:    "keep ARG prefix with non-digits",
			environ: []string{"PATH=/usr/bin", "ARGS=all", "ARGNAME=test", "ARG_COUNT=5"},
			want:    []string{"PATH=/usr/bin", "ARGS=all", "ARGNAME=test", "ARG_COUNT=5"},
		},
		{
			name:    "mixed filtering",
			environ: []string{"PATH=/usr/bin", "INVOWK_ARG_X=1", "HOME=/home", "ARGC=2", "ARG1=a", "INVOWK_FLAG_Y=2", "USER=test"},
			want:    []string{"PATH=/usr/bin", "HOME=/home", "USER=test"},
		},
		{
			name:    "malformed env var kept",
			environ: []string{"PATH=/usr/bin", "MALFORMED", "HOME=/home/user"},
			want:    []string{"PATH=/usr/bin", "MALFORMED", "HOME=/home/user"},
		},
		{
			name:    "empty value preserved",
			environ: []string{"EMPTY=", "INVOWK_ARG_EMPTY="},
			want:    []string{"EMPTY="},
		},
		{
			name:    "value with equals sign",
			environ: []string{"CONFIG=key=value", "INVOWK_ARG_TEST=foo=bar"},
			want:    []string{"CONFIG=key=value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterInvowkEnvVars(tt.environ)
			if len(got) != len(tt.want) {
				t.Errorf("FilterInvowkEnvVars() length = %d, want %d", len(got), len(tt.want))
				t.Errorf("  got:  %v", got)
				t.Errorf("  want: %v", tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("FilterInvowkEnvVars()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestShouldFilterEnvVar(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		want   bool
	}{
		// INVOWK_ARG_* cases
		{"INVOWK_ARG_NAME", "INVOWK_ARG_NAME", true},
		{"INVOWK_ARG_X", "INVOWK_ARG_X", true},
		{"INVOWK_ARG_LONG_NAME", "INVOWK_ARG_LONG_NAME", true},
		{"INVOWK_ARG_ (empty suffix)", "INVOWK_ARG_", true},

		// INVOWK_FLAG_* cases
		{"INVOWK_FLAG_VERBOSE", "INVOWK_FLAG_VERBOSE", true},
		{"INVOWK_FLAG_V", "INVOWK_FLAG_V", true},
		{"INVOWK_FLAG_ (empty suffix)", "INVOWK_FLAG_", true},

		// ARGC case
		{"ARGC", "ARGC", true},

		// ARGn cases
		{"ARG1", "ARG1", true},
		{"ARG2", "ARG2", true},
		{"ARG10", "ARG10", true},
		{"ARG999", "ARG999", true},
		{"ARG0", "ARG0", true},

		// Should NOT be filtered
		{"PATH", "PATH", false},
		{"HOME", "HOME", false},
		{"INVOWK", "INVOWK", false},
		{"INVOWK_", "INVOWK_", false},
		{"INVOWK_OTHER", "INVOWK_OTHER", false},
		{"ARG", "ARG", false},
		{"ARGS", "ARGS", false},
		{"ARGNAME", "ARGNAME", false},
		{"ARG_1", "ARG_1", false},
		{"ARG1NAME", "ARG1NAME", false},
		{"MY_ARGC", "MY_ARGC", false},
		{"MY_ARG1", "MY_ARG1", false},
		{"INVOWK_ARGS", "INVOWK_ARGS", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldFilterEnvVar(tt.envVar)
			if got != tt.want {
				t.Errorf("shouldFilterEnvVar(%q) = %v, want %v", tt.envVar, got, tt.want)
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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Set environment variables that should be filtered
	os.Setenv("INVOWK_ARG_PARENT", "parent_value")
	os.Setenv("INVOWK_FLAG_PARENT", "true")
	os.Setenv("ARGC", "5")
	os.Setenv("ARG1", "first")
	defer func() {
		os.Unsetenv("INVOWK_ARG_PARENT")
		os.Unsetenv("INVOWK_FLAG_PARENT")
		os.Unsetenv("ARGC")
		os.Unsetenv("ARG1")
	}()

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

func TestVirtualRuntime_EnvIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Set environment variables that should be filtered
	os.Setenv("INVOWK_ARG_PARENT", "parent_value")
	os.Setenv("INVOWK_FLAG_PARENT", "true")
	os.Setenv("ARGC", "5")
	os.Setenv("ARG1", "first")
	defer func() {
		os.Unsetenv("INVOWK_ARG_PARENT")
		os.Unsetenv("INVOWK_FLAG_PARENT")
		os.Unsetenv("ARGC")
		os.Unsetenv("ARG1")
	}()

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

// testCommandWithInterpreter creates a Command with a script and explicit interpreter
func testCommandWithInterpreter(name, script, interpreter string, runtime invkfile.RuntimeMode) *invkfile.Command {
	return &invkfile.Command{
		Name: name,
		Implementations: []invkfile.Implementation{
			{
				Script: script,
				Target: invkfile.Target{
					Runtimes: []invkfile.RuntimeConfig{{Name: runtime, Interpreter: interpreter}},
				},
			},
		},
	}
}

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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script with Python shebang (should auto-detect)
	script := `#!/usr/bin/env python3
print("Hello from Python")`

	cmd := testCommandWithScript("python-shebang", script, invkfile.RuntimeNative)

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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script without shebang but with explicit interpreter
	script := `import sys
print(f"Python version: {sys.version_info.major}.{sys.version_info.minor}")`

	cmd := testCommandWithInterpreter("python-explicit", script, "python3", invkfile.RuntimeNative)

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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script with shebang including -u flag for unbuffered output
	script := `#!/usr/bin/env -S python3 -u
import sys
print(f"arg1={sys.argv[1] if len(sys.argv) > 1 else 'none'}")`

	cmd := testCommandWithScript("python-args", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)
	ctx.PositionalArgs = []string{"hello-world"}

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

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
	defer os.RemoveAll(tmpDir)

	// Create a Python script file
	scriptContent := `#!/usr/bin/env python3
print("Hello from Python file")
`
	scriptPath := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	cmd := testCommandWithScript("python-file", "./test.py", invkfile.RuntimeNative)

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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script with a non-existent interpreter
	script := `print("Hello")`

	cmd := testCommandWithInterpreter("nonexistent-interp", script, "nonexistent-interpreter-xyz", invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

	var stdout bytes.Buffer
	ctx.Stdout = &stdout
	ctx.Stderr = &bytes.Buffer{}

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

func TestVirtualRuntime_RejectsInterpreter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
	defer os.RemoveAll(tmpDir)

	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
	}

	// Script with Python shebang
	script := `#!/usr/bin/env python3
import sys
print("stdout output")
print("stderr output", file=sys.stderr)`

	cmd := testCommandWithScript("python-capture", script, invkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(cmd, inv)

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
