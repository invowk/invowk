// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"invowk-cli/internal/issue"
	"invowk-cli/internal/testutil"
	"invowk-cli/pkg/invkfile"
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
			ctx.IO.Stdout = &bytes.Buffer{}
			ctx.IO.Stderr = &bytes.Buffer{}

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

// --- Unit tests (no shell execution required) ---

// TestNativeRuntime_Name tests the Name() method.
func TestNativeRuntime_Name(t *testing.T) {
	rt := NewNativeRuntime()
	if name := rt.Name(); name != "native" {
		t.Errorf("Name() = %q, want %q", name, "native")
	}
}

// TestNativeRuntime_Available tests the Available() method.
func TestNativeRuntime_Available(t *testing.T) {
	rt := NewNativeRuntime()
	// On any reasonable system, a shell should be available
	if !rt.Available() {
		t.Error("Available() = false, expected true on a system with a shell")
	}
}

// TestNativeRuntime_SupportsInteractive tests the SupportsInteractive() method.
func TestNativeRuntime_SupportsInteractive(t *testing.T) {
	rt := NewNativeRuntime()
	if !rt.SupportsInteractive() {
		t.Error("SupportsInteractive() = false, want true")
	}
}

// TestNativeRuntime_getShell tests shell detection.
func TestNativeRuntime_getShell(t *testing.T) {
	t.Run("uses custom shell when set", func(t *testing.T) {
		rt := &NativeRuntime{Shell: "/custom/shell"}
		shell, err := rt.getShell()
		if err != nil {
			t.Errorf("getShell() unexpected error: %v", err)
		}
		if shell != "/custom/shell" {
			t.Errorf("getShell() = %q, want %q", shell, "/custom/shell")
		}
	})

	t.Run("uses default shell when not set", func(t *testing.T) {
		rt := NewNativeRuntime()
		shell, err := rt.getShell()
		if err != nil {
			t.Errorf("getShell() unexpected error: %v", err)
		}
		if shell == "" {
			t.Error("getShell() returned empty string")
		}
	})
}

// TestNativeRuntime_getShellArgs tests shell argument generation.
func TestNativeRuntime_getShellArgs(t *testing.T) {
	tests := []struct {
		name         string
		shell        string
		shellArgs    []string // Custom args to set on runtime
		want         []string
		skipOnNonWin bool // Skip on non-Windows (backslash paths don't work cross-platform)
	}{
		{
			name:  "bash",
			shell: "/bin/bash",
			want:  []string{"-c"},
		},
		{
			name:  "sh",
			shell: "/bin/sh",
			want:  []string{"-c"},
		},
		{
			name:  "zsh",
			shell: "/usr/bin/zsh",
			want:  []string{"-c"},
		},
		{
			name:         "cmd.exe with Windows path",
			shell:        "C:\\Windows\\System32\\cmd.exe",
			want:         []string{"/C"},
			skipOnNonWin: true, // filepath.Base doesn't handle backslashes on non-Windows
		},
		{
			name:  "cmd.exe with forward slashes",
			shell: "C:/Windows/System32/cmd.exe",
			want:  []string{"/C"},
		},
		{
			name:  "cmd without .exe",
			shell: "/usr/bin/cmd",
			want:  []string{"/C"},
		},
		{
			name:         "powershell with Windows path",
			shell:        "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe",
			want:         []string{"-NoProfile", "-Command"},
			skipOnNonWin: true, // filepath.Base doesn't handle backslashes on non-Windows
		},
		{
			name:  "powershell with forward slashes",
			shell: "C:/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
			want:  []string{"-NoProfile", "-Command"},
		},
		{
			name:  "pwsh",
			shell: "/usr/bin/pwsh",
			want:  []string{"-NoProfile", "-Command"},
		},
		{
			name:      "custom shell args override",
			shell:     "/bin/bash",
			shellArgs: []string{"--login", "-c"},
			want:      []string{"--login", "-c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipOnNonWin && goruntime.GOOS != "windows" {
				t.Skip("skipping: Windows-style backslash paths only work on Windows")
			}
			testRt := &NativeRuntime{ShellArgs: tt.shellArgs}
			got := testRt.getShellArgs(tt.shell)
			if len(got) != len(tt.want) {
				t.Errorf("getShellArgs(%q) length = %d, want %d", tt.shell, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getShellArgs(%q)[%d] = %q, want %q", tt.shell, i, got[i], tt.want[i])
				}
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
	tmpDir := t.TempDir()
	inv := &invkfile.Invkfile{
		FilePath: filepath.Join(tmpDir, "invkfile.cue"),
	}

	tests := []struct {
		name    string
		cmd     *invkfile.Command
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid script",
			cmd:     testCommandWithScript("valid", "echo hello", invkfile.RuntimeNative),
			wantErr: false,
		},
		{
			name: "nil implementation",
			cmd: &invkfile.Command{
				Name: "nil-impl",
			},
			wantErr: true,
			errMsg:  "no script selected",
		},
		{
			name: "empty script",
			cmd: &invkfile.Command{
				Name: "empty",
				Implementations: []invkfile.Implementation{
					{
						Script:   "",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
					},
				},
			},
			wantErr: true,
			errMsg:  "no content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := NewNativeRuntime()
			ctx := NewExecutionContext(tt.cmd, inv)
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

// TestNativeRuntime_getWorkDir tests working directory resolution.
func TestNativeRuntime_getWorkDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		cmdWorkDir    string
		ctxWorkDir    string
		want          string
		skipOnWindows bool
	}{
		{
			name:       "no workdir uses invkfile directory",
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
			if tt.skipOnWindows && goruntime.GOOS == "windows" {
				t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
			}
			inv := &invkfile.Invkfile{
				FilePath: filepath.Join(tmpDir, "invkfile.cue"),
				WorkDir:  "",
			}
			cmd := &invkfile.Command{
				Name:    "workdir-test",
				WorkDir: tt.cmdWorkDir,
				Implementations: []invkfile.Implementation{
					{
						Script:   "echo test",
						Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}},
					},
				},
			}

			rt := NewNativeRuntime()
			ctx := NewExecutionContext(cmd, inv)
			ctx.WorkDir = tt.ctxWorkDir

			got := rt.getWorkDir(ctx)
			if got != tt.want {
				t.Errorf("getWorkDir() = %q, want %q", got, tt.want)
			}
		})
	}
}
