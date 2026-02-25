// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestNativeRuntime_InlineScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary invowkfile
	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	cmd := testCommandWithScript("test", "echo 'Hello from inline'", invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Multi-line script
	script := `echo "Line 1"
echo "Line 2"
echo "Line 3"`

	cmd := testCommandWithScript("multiline", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	// Create a script file
	scriptContent := `#!/bin/bash
echo "Hello from script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	cmd := testCommandWithScript("from-file", "./test.sh", invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that echoes positional parameters
	script := `echo "arg1=$1 arg2=$2 all=$@"`

	cmd := testCommandWithScript("positional", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)
	ctx.PositionalArgs = []string{"hello", "world"}

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that echoes the number of positional parameters
	script := `echo "argc=$#"`

	cmd := testCommandWithScript("no-args", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)
	// No positional args set

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that echoes the first positional parameter
	script := `echo "arg1=$1"`

	cmd := testCommandWithScript("special-chars", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)
	ctx.PositionalArgs = []string{"hello world with spaces"}

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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
	t.Parallel()

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
			t.Parallel()

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
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

	cmd := testCommandWithScript("env-isolation", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	script := `echo "Should not run"`

	// Create command with invalid working directory
	cmd := &invowkfile.Command{
		Name: "invalid-workdir",
		Implementations: []invowkfile.Implementation{
			{
				Script:    invowkfile.ScriptContent(script),
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}},
				WorkDir:   "/nonexistent/directory/that/does/not/exist",
			},
		},
	}

	rt := NewNativeRuntime()
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	tests := []struct {
		name         string
		script       string
		expectedCode ExitCode
	}{
		{"exit 0", "exit 0", 0},
		{"exit 1", "exit 1", 1},
		{"exit 42", "exit 42", 42},
		{"false command", "false", 1},
		{"true command", "true", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := testCommandWithScript("exit-test", tt.script, invowkfile.RuntimeNative)
			rt := NewNativeRuntime()
			ctx := NewExecutionContext(context.Background(), cmd, inv)
			ctx.IO.Stdout = &bytes.Buffer{}
			ctx.IO.Stderr = &bytes.Buffer{}

			result := rt.Execute(ctx)
			if result.ExitCode != tt.expectedCode {
				t.Errorf("Execute() exit code = %d, want %d", result.ExitCode, tt.expectedCode)
			}
		})
	}
}

// TestNativeRuntime_createTempScript tests temporary script file creation.
func TestNativeRuntime_createTempScript(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test that writes to filesystem in short mode")
	}

	rt := NewNativeRuntime()

	tests := []struct {
		name        string
		content     string
		interpreter string
		wantExt     string
	}{
		{
			name:        "bash script",
			content:     "#!/bin/bash\necho hello",
			interpreter: "bash",
			wantExt:     ".sh",
		},
		{
			name:        "python script",
			content:     "#!/usr/bin/env python3\nprint('hello')",
			interpreter: "python3",
			wantExt:     ".py",
		},
		{
			name:        "node script",
			content:     "console.log('hello')",
			interpreter: "node",
			wantExt:     ".js",
		},
		{
			name:        "ruby script",
			content:     "puts 'hello'",
			interpreter: "ruby",
			wantExt:     ".rb",
		},
		{
			name:        "perl script",
			content:     "print 'hello'",
			interpreter: "perl",
			wantExt:     ".pl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := rt.createTempScript(tt.content, tt.interpreter)
			if err != nil {
				t.Fatalf("createTempScript() error: %v", err)
			}
			defer func() { _ = os.Remove(path) }()

			// Check extension
			if !strings.HasSuffix(path, tt.wantExt) {
				t.Errorf("createTempScript() path = %q, want extension %q", path, tt.wantExt)
			}

			// Check content
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read temp script: %v", err)
			}
			if string(content) != tt.content {
				t.Errorf("createTempScript() content = %q, want %q", string(content), tt.content)
			}

			// Check file exists and is readable
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("failed to stat temp script: %v", err)
			}
			if info.Size() == 0 {
				t.Error("createTempScript() created empty file")
			}
		})
	}
}

// TestNativeRuntime_Validate tests the validation logic.
func TestNativeRuntime_Validate_Unit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	tests := []struct {
		name    string
		cmd     *invowkfile.Command
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid script",
			cmd:     testCommandWithScript("valid", "echo hello", invowkfile.RuntimeNative),
			wantErr: false,
		},
		{
			name: "nil implementation",
			cmd: &invowkfile.Command{
				Name: "nil-impl",
			},
			wantErr: true,
			errMsg:  "no script selected",
		},
		{
			name: "empty script",
			cmd: &invowkfile.Command{
				Name: "empty",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}},
					},
				},
			},
			wantErr: true,
			errMsg:  "no content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := NewNativeRuntime()
			ctx := NewExecutionContext(context.Background(), tt.cmd, inv)
			// For "nil implementation" test, set SelectedImpl to nil
			if tt.name == "nil implementation" {
				ctx.SelectedImpl = nil
			}

			err := rt.Validate(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("Validate() expected error, got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestExecutionContext_EffectiveWorkDir_Native tests working directory resolution via ExecutionContext.
func TestExecutionContext_EffectiveWorkDir_Native(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		cmdWorkDir    string
		ctxWorkDir    invowkfile.WorkDir
		want          string
		skipOnWindows bool
	}{
		{
			name:       "no workdir uses invowkfile directory",
			cmdWorkDir: "",
			ctxWorkDir: "",
			want:       tmpDir,
		},
		{
			name:          "context workdir takes precedence",
			cmdWorkDir:    "cmd-dir",
			ctxWorkDir:    "/override/dir",
			want:          "/override/dir",
			skipOnWindows: true, // Unix-style absolute path not meaningful on Windows
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.skipOnWindows && goruntime.GOOS == "windows" {
				t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
			}
			inv := &invowkfile.Invowkfile{
				FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
				WorkDir:  "",
			}
			cmd := &invowkfile.Command{
				Name:    "workdir-test",
				WorkDir: invowkfile.WorkDir(tt.cmdWorkDir),
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo test",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
						Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}},
					},
				},
			}

			ctx := NewExecutionContext(context.Background(), cmd, inv)
			ctx.WorkDir = tt.ctxWorkDir

			got := ctx.EffectiveWorkDir()
			if got != tt.want {
				t.Errorf("EffectiveWorkDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
