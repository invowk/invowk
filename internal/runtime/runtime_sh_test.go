// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestShRuntime_InlineScript(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	cmd := testCommandWithScript("test", "echo 'Hello from virtual'", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from virtual" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from virtual")
	}
}

func TestShRuntimeUrootBuiltinUsesPathValidator(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	script := fmt.Sprintf("cat %q", filepath.Join(homeDir, ".invowk-denied-test"))
	cmd := testCommandWithScript("cat-denied", script, invowkfile.RuntimeVirtualSh)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	result := NewShRuntime(true).ExecuteCapture(ctx)
	if result.Success() {
		t.Fatalf("ExecuteCapture() result = %#v, want path validation failure", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "virtual path denied") {
		t.Fatalf("ExecuteCapture() error = %v, want virtual path denied", result.Error)
	}
}

func TestShRuntimeFullFilesystemAccessAllowsHostPath(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	hostFile, err := os.CreateTemp(homeDir, ".invowk-full-access-*")
	if err != nil {
		t.Fatalf("CreateTemp(home) error = %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(hostFile.Name()) })
	if _, err := hostFile.WriteString("full-ok"); err != nil {
		t.Fatalf("WriteString(host file) error = %v", err)
	}
	if err := hostFile.Close(); err != nil {
		t.Fatalf("Close(host file) error = %v", err)
	}

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	script := fmt.Sprintf("cat %q", hostFile.Name())
	cmd := testCommandWithScript("cat-full", script, invowkfile.RuntimeVirtualSh)
	cmd.Implementations[0].Platforms = testPlatformsWithVirtualFilesystem(invowkfile.VirtualFilesystemAccessFull, nil)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	result := NewShRuntime(true).ExecuteCapture(ctx)
	if !result.Success() {
		t.Fatalf("ExecuteCapture() result = %#v, want success", result)
	}
	if got := result.Output; got != "full-ok" {
		t.Fatalf("output = %q, want full-ok", got)
	}
}

func TestShVirtualRuntimeEnvOverridesUserInvowkState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	script := `printf '%s\n' "$INVOWK_STATE_BIN_PATH" "$INVOWK_PATH_DB_ROOT" "$INVOWK_ANCHOR_WORK"`
	cmd := testCommandWithScript("reserved-env", script, invowkfile.RuntimeVirtualSh)
	cmd.Env = &invowkfile.EnvConfig{Vars: map[invowkfile.EnvVarName]string{
		"INVOWK_STATE_BIN_PATH": "user-bin",
		"INVOWK_PATH_DB_ROOT":   "user-path",
		"INVOWK_ANCHOR_WORK":    "user-work",
	}}
	cmd.Implementations[0].Platforms = testPlatformsWithVirtualFilesystem(
		"",
		invowkfile.VirtualFilesystemPaths{"DB_ROOT": "./db"},
	)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	result := NewShRuntime(false).ExecuteCapture(ctx)
	if !result.Success() {
		t.Fatalf("ExecuteCapture() result = %#v, want success", result)
	}

	lines := strings.Split(result.Output, "\n")
	if len(lines) < 4 {
		t.Fatalf("stdout lines = %q, want at least 3 values", result.Output)
	}
	if lines[0] != "" {
		t.Fatalf("INVOWK_STATE_BIN_PATH = %q, want runtime-owned empty value", lines[0])
	}
	resolver := mustVirtualTestResolver(t, ctx)
	dbRoot := resolver.paths["DB_ROOT"]
	if lines[1] != dbRoot {
		t.Fatalf("INVOWK_PATH_DB_ROOT = %q, want %q", lines[1], dbRoot)
	}
	workDir := resolver.anchors["@work"]
	if lines[2] != workDir {
		t.Fatalf("INVOWK_ANCHOR_WORK = %q, want %q", lines[2], workDir)
	}
}

func TestShRuntime_MultiLineScript(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	script := `VAR="test value"
echo "Variable is: $VAR"
echo "Done"`

	cmd := testCommandWithScript("multiline", script, invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := stdout.String()
	if !strings.Contains(output, "Variable is: test value") {
		t.Errorf("Execute() output missing expected content, got: %q", output)
	}
}

func TestShRuntime_ScriptFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a script file (using POSIX-compatible syntax for virtual shell)
	scriptContent := `echo "Hello from virtual script file"
`
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(invowkfilePath),
		ModulePath: invowkfile.FilesystemPath(tmpDir),
	}

	cmd := testCommandWithScriptFile("from-file", "./test.sh", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "Hello from virtual script file" {
		t.Errorf("Execute() output = %q, want %q", output, "Hello from virtual script file")
	}
}

func TestShRuntime_PositionalArgs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that echoes positional parameters
	script := `echo "arg1=$1 arg2=$2 all=$@"`

	cmd := testCommandWithScript("positional", script, invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	ctx.PositionalArgs = []string{"foo", "bar"}

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

func TestShRuntime_PositionalArgs_ArgCount(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that echoes the number of positional parameters
	script := `echo "count=$#"`

	cmd := testCommandWithScript("arg-count", script, invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	ctx.PositionalArgs = []string{"a", "b", "c", "d", "e"}

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 0 {
		t.Errorf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "count=5" {
		t.Errorf("Execute() output = %q, want %q", output, "count=5")
	}
}

func TestShRuntime_PositionalArgs_Empty(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that echoes the number of positional parameters
	script := `echo "argc=$#"`

	cmd := testCommandWithScript("no-args", script, invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

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

func TestShRuntime_EnvIsolation(t *testing.T) { //nolint:paralleltest // test mutates process environment for runtime inheritance checks.
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

	cmd := testCommandWithScript("env-isolation", script, invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

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

func TestShRuntime_ContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Script that runs forever
	script := `while true; do sleep 1; done`

	cmd := testCommandWithScript("long-running", script, invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(t.Context())

	execCtx := NewExecutionContext(ctx, cmd, inv)

	var stdout bytes.Buffer
	execCtx.IO.Stdout = &stdout
	execCtx.IO.Stderr = &bytes.Buffer{}

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

func TestShRuntime_ExitCode(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			cmd := testCommandWithScript("exit-test", tt.script, invowkfile.RuntimeVirtualSh)
			rt := NewShRuntime(false)
			ctx := NewExecutionContext(t.Context(), cmd, inv)

			ctx.IO.Stdout = &bytes.Buffer{}
			ctx.IO.Stderr = &bytes.Buffer{}

			result := rt.Execute(ctx)
			if result.ExitCode != tt.expectedCode {
				t.Errorf("Execute() exit code = %d, want %d", result.ExitCode, tt.expectedCode)
			}
		})
	}
}

// ============================================================================
// Unit Tests (Phase 5 additions)
// ============================================================================

// TestShRuntime_Name tests the Name method.
func TestExecutionContext_EffectiveWorkDir_Virtual(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	tests := []struct {
		name          string
		ctxWorkDir    invowkfile.WorkDir // WorkDir set on ExecutionContext
		cmdWorkDir    string             // WorkDir set on Command
		implWorkDir   string             // WorkDir set on Implementation
		rootWorkDir   string             // WorkDir set on Invowkfile
		wantContains  string             // Substring expected in result
		skipOnWindows bool
	}{
		{
			name:         "defaults to invowkfile directory",
			wantContains: tmpDir,
		},
		{
			name:          "context workdir takes precedence over all",
			ctxWorkDir:    "/ctx/workdir",
			cmdWorkDir:    "/cmd/workdir",
			implWorkDir:   "/impl/workdir",
			rootWorkDir:   "/root/workdir",
			wantContains:  "/ctx/workdir",
			skipOnWindows: true, // Unix-style absolute paths not meaningful on Windows
		},
		{
			name:          "impl workdir takes precedence over cmd and root",
			cmdWorkDir:    "/cmd/workdir",
			implWorkDir:   "/impl/workdir",
			rootWorkDir:   "/root/workdir",
			wantContains:  "/impl/workdir",
			skipOnWindows: true, // Unix-style absolute paths not meaningful on Windows
		},
		{
			name:          "cmd workdir takes precedence over root",
			cmdWorkDir:    "/cmd/workdir",
			rootWorkDir:   "/root/workdir",
			wantContains:  "/cmd/workdir",
			skipOnWindows: true, // Unix-style absolute paths not meaningful on Windows
		},
		{
			name:          "root workdir used when others not set",
			rootWorkDir:   "/root/workdir",
			wantContains:  "/root/workdir",
			skipOnWindows: true, // Unix-style absolute paths not meaningful on Windows
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.skipOnWindows && goruntime.GOOS == "windows" {
				t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
			}
			inv := &invowkfile.Invowkfile{
				FilePath: invowkfile.FilesystemPath(invowkfilePath),
				WorkDir:  invowkfile.WorkDir(tt.rootWorkDir),
			}

			impl := invowkfile.Implementation{
				Script:    invowkfile.ImplementationScript{Content: "echo test"},
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtualSh}},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}},
				WorkDir:   invowkfile.WorkDir(tt.implWorkDir),
			}

			cmd := &invowkfile.Command{
				Name:            "test-workdir",
				WorkDir:         invowkfile.WorkDir(tt.cmdWorkDir),
				Implementations: []invowkfile.Implementation{impl},
			}

			ctx := NewExecutionContext(t.Context(), cmd, inv)
			ctx.WorkDir = tt.ctxWorkDir
			ctx.SelectedImpl = &cmd.Implementations[0]

			got := ctx.EffectiveWorkDir()
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("EffectiveWorkDir() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

// TestShRuntime_NewShRuntime tests constructor options.
func TestShRuntime_PositionalArgs_DashPrefix(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}

	tests := []struct {
		name   string
		script string
		args   []string
		want   string
	}{
		{"single dash flag becomes positional", `echo "arg1=$1"`, []string{"-v"}, "arg1=-v"},
		{"double dash flag becomes positional", `echo "arg1=$1"`, []string{"--env=staging"}, "arg1=--env=staging"},
		{"mixed dash and normal args", `echo "count=$#"`, []string{"-v", "hello", "--debug"}, "count=3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := testCommandWithScript("dash-args", tt.script, invowkfile.RuntimeVirtualSh)
			rt := NewShRuntime(false)
			ctx := NewExecutionContext(t.Context(), cmd, inv)

			ctx.PositionalArgs = tt.args

			var stdout bytes.Buffer
			ctx.IO.Stdout = &stdout
			ctx.IO.Stderr = &bytes.Buffer{}

			result := rt.Execute(ctx)
			if result.ExitCode != 0 {
				t.Fatalf("Execute() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
			}
			if got := strings.TrimSpace(stdout.String()); got != tt.want {
				t.Errorf("Execute() output = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestShRuntime_ExecuteCapture tests that ExecuteCapture correctly captures
// stdout and stderr into separate Result fields.
func TestShRuntime_ExecuteCapture(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	script := `echo "captured stdout"
echo "captured stderr" >&2`

	cmd := testCommandWithScript("capture-test", script, invowkfile.RuntimeVirtualSh)
	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.ExecuteCapture(ctx)
	if result.ExitCode != 0 {
		t.Fatalf("ExecuteCapture() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	if !strings.Contains(result.Output, "captured stdout") {
		t.Errorf("ExecuteCapture() Output = %q, want to contain 'captured stdout'", result.Output)
	}
	if !strings.Contains(result.ErrOutput, "captured stderr") {
		t.Errorf("ExecuteCapture() ErrOutput = %q, want to contain 'captured stderr'", result.ErrOutput)
	}
}

// TestShRuntime_MockEnvBuilder_Error tests that the virtual runtime correctly
// propagates errors from the EnvBuilder during execution.
func TestShRuntime_SetE_StopsOnError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	// "set -e" should abort the script at "false" and not reach "echo after"
	script := `set -e
echo "before"
false
echo "after"`

	cmd := testCommandWithScript("set-e", script, invowkfile.RuntimeVirtualSh)
	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	var stdout bytes.Buffer
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode != 1 {
		t.Errorf("Execute() exit code = %d, want 1", result.ExitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "before") {
		t.Error("Execute() should have printed 'before' prior to failure")
	}
	if strings.Contains(output, "after") {
		t.Error("Execute() should NOT have printed 'after' since 'set -e' aborts on 'false'")
	}
}
