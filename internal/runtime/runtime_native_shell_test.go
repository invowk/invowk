// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestNativeRuntime_getShell tests shell detection.
func TestNativeRuntime_getShell(t *testing.T) {
	t.Run("uses custom shell when set", func(t *testing.T) {
		rt := NewNativeRuntime(WithShell("/custom/shell"))
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

// TestNativeRuntime_getShell_ShellEnvFallback verifies that on non-Windows systems,
// getShell() picks up the $SHELL environment variable as first preference, falling back
// to bash and then sh if $SHELL is unset.
func TestNativeRuntime_getShell_ShellEnvFallback(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("skipping: $SHELL fallback chain only applies to non-Windows")
	}

	t.Run("prefers SHELL env var when set", func(t *testing.T) {
		restore := testutil.MustSetenv(t, "SHELL", "/usr/bin/zsh")
		defer restore()

		rt := NewNativeRuntime()
		shell, err := rt.getShell()
		if err != nil {
			t.Fatalf("getShell() unexpected error: %v", err)
		}
		if shell != "/usr/bin/zsh" {
			t.Errorf("getShell() = %q, want %q", shell, "/usr/bin/zsh")
		}
	})

	t.Run("falls back to bash when SHELL is unset", func(t *testing.T) {
		restore := testutil.MustSetenv(t, "SHELL", "")
		defer restore()

		os.Unsetenv("SHELL")

		rt := NewNativeRuntime()
		shell, err := rt.getShell()
		if err != nil {
			t.Fatalf("getShell() unexpected error: %v", err)
		}
		// On most systems bash is available; just verify it's not empty
		if shell == "" {
			t.Error("getShell() returned empty string when SHELL is unset")
		}
	})
}

// TestNativeRuntime_getShellArgs tests shell argument generation.
func TestNativeRuntime_getShellArgs(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			if tt.skipOnNonWin && goruntime.GOOS != "windows" {
				t.Skip("skipping: Windows-style backslash paths only work on Windows")
			}
			testRt := NewNativeRuntime(WithShellArgs(tt.shellArgs))
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

// TestNativeRuntime_ShellNotFoundError tests the shell not found error creation.
func TestNativeRuntime_ShellNotFoundError(t *testing.T) {
	t.Parallel()

	rt := NewNativeRuntime(WithShell("/this/shell/does/not/exist"))

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

// TestNativeRuntime_ShellNotFoundError_Format tests the verbose formatting.
func TestNativeRuntime_ShellNotFoundError_Format(t *testing.T) {
	t.Parallel()

	rt := NewNativeRuntime()

	// Get an actionable error
	errVal := rt.shellNotFoundError([]string{"bash", "sh"})

	// Check that it can be cast to *issue.ActionableError
	ae, ok := errors.AsType[*issue.ActionableError](errVal)
	if !ok {
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

// TestNativeRuntime_ExecuteCapture_Shell tests that ExecuteCapture correctly captures
// stdout and stderr separately in shell mode.
func TestNativeRuntime_ExecuteCapture_Shell(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	script := `echo "captured stdout"
echo "captured stderr" >&2`

	cmd := testCommandWithScript("capture-test", script, invowkfile.RuntimeNative)

	rt := NewNativeRuntime()
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

// TestNativeRuntime_MockEnvBuilder_Error tests that the native runtime correctly
// propagates errors from the EnvBuilder during shell execution.
func TestNativeRuntime_MockEnvBuilder_Error(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	cmd := testCommandWithScript("env-error", "echo test", invowkfile.RuntimeNative)

	mockErr := fmt.Errorf("mock env build failure")
	rt := NewNativeRuntime(WithEnvBuilder(&MockEnvBuilder{Err: mockErr}))
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
	if !strings.Contains(result.Error.Error(), "mock env build failure") {
		t.Errorf("Execute() error = %q, want to contain 'mock env build failure'", result.Error)
	}
}

// TestNativeRuntime_MockEnvBuilder_CaptureError tests that ExecuteCapture correctly
// propagates errors from the EnvBuilder.
func TestNativeRuntime_MockEnvBuilder_CaptureError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	cmd := testCommandWithScript("env-error-capture", "echo test", invowkfile.RuntimeNative)

	mockErr := fmt.Errorf("capture env build failure")
	rt := NewNativeRuntime(WithEnvBuilder(&MockEnvBuilder{Err: mockErr}))
	ctx := NewExecutionContext(context.Background(), cmd, inv)
	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.ExecuteCapture(ctx)
	if result.ExitCode == 0 {
		t.Error("ExecuteCapture() should fail when EnvBuilder returns error")
	}
	if result.Error == nil {
		t.Fatal("ExecuteCapture() should return error when EnvBuilder fails")
	}
	if !strings.Contains(result.Error.Error(), "capture env build failure") {
		t.Errorf("ExecuteCapture() error = %q, want to contain 'capture env build failure'", result.Error)
	}
}
