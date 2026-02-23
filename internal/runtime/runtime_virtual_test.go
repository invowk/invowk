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

func TestVirtualRuntime_InlineScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("test", "echo 'Hello from virtual'", invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

func TestVirtualRuntime_MultiLineScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	script := `VAR="test value"
echo "Variable is: $VAR"
echo "Done"`

	cmd := testCommandWithScript("multiline", script, invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	cmd := testCommandWithScript("from-file", "./test.sh", invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

func TestVirtualRuntime_Validate_ScriptSyntaxError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Invalid shell syntax
	cmd := testCommandWithScript("invalid", "if then fi", invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes positional parameters
	script := `echo "arg1=$1 arg2=$2 all=$@"`

	cmd := testCommandWithScript("positional", script, invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

func TestVirtualRuntime_PositionalArgs_ArgCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes the number of positional parameters
	script := `echo "count=$#"`

	cmd := testCommandWithScript("arg-count", script, invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

func TestVirtualRuntime_PositionalArgs_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that echoes the number of positional parameters
	script := `echo "argc=$#"`

	cmd := testCommandWithScript("no-args", script, invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
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

func TestVirtualRuntime_EnvIsolation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
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

	cmd := testCommandWithScript("env-isolation", script, invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
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

func TestVirtualRuntime_RejectsInterpreter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Try to use interpreter with virtual runtime (should be rejected)
	script := `echo "Hello"`

	cmd := testCommandWithInterpreter("virtual-with-interp", script, "python3", invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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
	ctx.IO.Stdout = &stdout
	ctx.IO.Stderr = &bytes.Buffer{}

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

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Script that runs forever
	script := `while true; do sleep 1; done`

	cmd := testCommandWithScript("long-running", script, invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

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

func TestVirtualRuntime_ExitCode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { testutil.MustRemoveAll(t, tmpDir) }()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
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
			cmd := testCommandWithScript("exit-test", tt.script, invowkfile.RuntimeVirtual)
			rt := NewVirtualRuntime(false)
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

// ============================================================================
// Unit Tests (Phase 5 additions)
// ============================================================================

// TestVirtualRuntime_Name tests the Name method.
func TestVirtualRuntime_Name(t *testing.T) {
	rt := NewVirtualRuntime(false)
	if got := rt.Name(); got != "virtual" {
		t.Errorf("Name() = %q, want %q", got, "virtual")
	}
}

// TestVirtualRuntime_Available tests the Available method.
func TestVirtualRuntime_Available(t *testing.T) {
	rt := NewVirtualRuntime(false)
	if !rt.Available() {
		t.Error("Available() = false, want true (virtual runtime is always available)")
	}
}

// TestVirtualRuntime_SupportsInteractive tests the SupportsInteractive method.
func TestVirtualRuntime_SupportsInteractive(t *testing.T) {
	rt := NewVirtualRuntime(false)
	if !rt.SupportsInteractive() {
		t.Error("SupportsInteractive() = false, want true")
	}
}

// TestVirtualRuntime_Validate_EmptyScript tests validation for an empty script.
func TestVirtualRuntime_Validate_EmptyScript(t *testing.T) {
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	// Create a command with an empty script
	cmd := testCommandWithScript("empty-script", "", invowkfile.RuntimeVirtual)

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	err := rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for empty script, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no content to execute") {
		t.Errorf("Validate() error = %q, want error containing 'no content to execute'", err)
	}
}

// TestVirtualRuntime_Validate_NilImpl tests validation for nil implementation.
func TestVirtualRuntime_Validate_NilImpl(t *testing.T) {
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfilePath,
	}

	cmd := &invowkfile.Command{
		Name: "nil-impl",
	}

	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)
	ctx.SelectedImpl = nil // Explicitly set to nil

	err := rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for nil implementation, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "no script selected") {
		t.Errorf("Validate() error = %q, want error containing 'no script selected'", err)
	}
}

// TestExecutionContext_EffectiveWorkDir_Virtual tests working directory resolution via ExecutionContext.
func TestExecutionContext_EffectiveWorkDir_Virtual(t *testing.T) {
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
			if tt.skipOnWindows && goruntime.GOOS == "windows" {
				t.Skip("skipping: Unix-style absolute paths are not meaningful on Windows")
			}
			inv := &invowkfile.Invowkfile{
				FilePath: invowkfilePath,
				WorkDir:  invowkfile.WorkDir(tt.rootWorkDir),
			}

			impl := invowkfile.Implementation{
				Script:    "echo test",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}},
				WorkDir:   invowkfile.WorkDir(tt.implWorkDir),
			}

			cmd := &invowkfile.Command{
				Name:            "test-workdir",
				WorkDir:         invowkfile.WorkDir(tt.cmdWorkDir),
				Implementations: []invowkfile.Implementation{impl},
			}

			ctx := NewExecutionContext(context.Background(), cmd, inv)
			ctx.WorkDir = tt.ctxWorkDir
			ctx.SelectedImpl = &cmd.Implementations[0]

			got := ctx.EffectiveWorkDir()
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("EffectiveWorkDir() = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}

// TestVirtualRuntime_NewVirtualRuntime tests constructor options.
func TestVirtualRuntime_NewVirtualRuntime(t *testing.T) {
	tests := []struct {
		name        string
		enableUroot bool
		wantUroot   bool
	}{
		{
			name:        "uroot disabled",
			enableUroot: false,
			wantUroot:   false,
		},
		{
			name:        "uroot enabled",
			enableUroot: true,
			wantUroot:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := NewVirtualRuntime(tt.enableUroot)
			if rt.EnableUrootUtils != tt.wantUroot {
				t.Errorf("NewVirtualRuntime(%v).EnableUrootUtils = %v, want %v",
					tt.enableUroot, rt.EnableUrootUtils, tt.wantUroot)
			}
		})
	}
}

// TestVirtualRuntime_PositionalArgs_DashPrefix verifies that positional arguments
// starting with "-" or "--" are correctly passed as $1, $2, etc. and NOT interpreted
// as shell options by interp.Params(). This exercises the "--" prefix guard in virtual.go.
func TestVirtualRuntime_PositionalArgs_DashPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{FilePath: filepath.Join(tmpDir, "invowkfile.cue")}

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
			cmd := testCommandWithScript("dash-args", tt.script, invowkfile.RuntimeVirtual)
			rt := NewVirtualRuntime(false)
			ctx := NewExecutionContext(context.Background(), cmd, inv)

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

// TestVirtualRuntime_ExecuteCapture tests that ExecuteCapture correctly captures
// stdout and stderr into separate Result fields.
func TestVirtualRuntime_ExecuteCapture(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	script := `echo "captured stdout"
echo "captured stderr" >&2`

	cmd := testCommandWithScript("capture-test", script, invowkfile.RuntimeVirtual)
	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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

// TestVirtualRuntime_MockEnvBuilder_Error tests that the virtual runtime correctly
// propagates errors from the EnvBuilder during execution.
func TestVirtualRuntime_MockEnvBuilder_Error(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	cmd := testCommandWithScript("env-error", "echo test", invowkfile.RuntimeVirtual)

	mockErr := fmt.Errorf("mock virtual env build failure")
	rt := NewVirtualRuntime(false, WithVirtualEnvBuilder(&MockEnvBuilder{Err: mockErr}))
	ctx := NewExecutionContext(context.Background(), cmd, inv)

	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode == 0 {
		t.Error("Execute() should fail when EnvBuilder returns error")
	}
	if result.Error == nil {
		t.Fatal("Execute() should return error when EnvBuilder fails")
	}
	if !strings.Contains(result.Error.Error(), "mock virtual env build failure") {
		t.Errorf("Execute() error = %q, want to contain 'mock virtual env build failure'", result.Error)
	}
}

// TestVirtualRuntime_SetE_StopsOnError verifies that "set -e" (errexit) in a virtual
// script terminates execution immediately when a command fails, and the exit code
// is propagated correctly through the interp.ExitStatus error type.
func TestVirtualRuntime_SetE_StopsOnError(t *testing.T) {
	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: filepath.Join(tmpDir, "invowkfile.cue"),
	}

	// "set -e" should abort the script at "false" and not reach "echo after"
	script := `set -e
echo "before"
false
echo "after"`

	cmd := testCommandWithScript("set-e", script, invowkfile.RuntimeVirtual)
	rt := NewVirtualRuntime(false)
	ctx := NewExecutionContext(context.Background(), cmd, inv)

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
