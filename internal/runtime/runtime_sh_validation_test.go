// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestShRuntime_Validate_ScriptSyntaxError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Invalid shell syntax
	cmd := testCommandWithScript("invalid", "if then fi", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	err := rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for invalid syntax, got nil")
	}
}

func TestShRuntime_RejectsInterpreter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Try to use interpreter with virtual runtime (should be rejected)
	script := `echo "Hello"`

	cmd := testCommandWithInterpreter("virtual-with-interp", script, "python3", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	// Test Validate method
	err := rt.Validate(ctx)
	if err == nil {
		t.Fatal("Validate() expected error for interpreter with virtual runtime")
	}
	if !errors.Is(err, invowkfile.ErrInterpreterNotAllowed) {
		t.Errorf("Validate() error = %q, want sentinel %q", err, invowkfile.ErrInterpreterNotAllowed)
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

func TestShRuntime_AcceptsShellCompatibleScriptInterpreter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	tests := []struct {
		name        string
		script      string
		interpreter string
	}{
		{
			name:        "explicit shell interpreter",
			script:      `echo "ok"`,
			interpreter: "bash",
		},
		{
			name:   "shell shebang",
			script: "#!/bin/sh\necho \"ok\"",
		},
		{
			name:        "auto with shell shebang",
			script:      "#!/usr/bin/env sh\necho \"ok\"",
			interpreter: "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := testCommandWithScript("virtual-shell-interpreter", tt.script, invowkfile.RuntimeVirtualSh)
			if tt.interpreter != "" {
				cmd.Implementations[0].Script.Interpreter = invowkfile.InterpreterSpec(tt.interpreter)
			}

			rt := NewShRuntime(false)
			ctx := NewExecutionContext(t.Context(), cmd, inv)
			result := rt.ExecuteCapture(ctx)
			if result.ExitCode != 0 || result.Error != nil {
				t.Fatalf("ExecuteCapture() = exit %d, error %v", result.ExitCode, result.Error)
			}
			if got := strings.TrimSpace(result.Output); got != "ok" {
				t.Fatalf("ExecuteCapture() output = %q, want ok", got)
			}
		})
	}
}

func TestShRuntime_RejectsNonShellShebang(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	cmd := testCommandWithScript("virtual-python-shebang", "#!/usr/bin/env python3\nprint('no')", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	err := rt.Validate(ctx)
	if err == nil {
		t.Fatal("Validate() error = nil, want non-shell shebang rejection")
	}
	if !errors.Is(err, invowkfile.ErrInterpreterNotAllowed) {
		t.Fatalf("Validate() error = %v, want %v", err, invowkfile.ErrInterpreterNotAllowed)
	}
}

func TestShRuntime_ExecuteCaptureRejectsInterpreter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	cmd := testCommandWithInterpreter("virtual-capture-with-interp", `echo "Hello"`, "python3", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	result := rt.ExecuteCapture(ctx)
	if result.ExitCode == 0 {
		t.Error("ExecuteCapture() expected non-zero exit code for interpreter with virtual runtime")
	}
	if result.Error == nil {
		t.Fatal("ExecuteCapture() expected error for interpreter with virtual runtime")
	}
	if !errors.Is(result.Error, invowkfile.ErrInterpreterNotAllowed) {
		t.Errorf("ExecuteCapture() error = %q, want sentinel %q", result.Error, invowkfile.ErrInterpreterNotAllowed)
	}
}

func TestShRuntime_Name(t *testing.T) {
	t.Parallel()

	rt := NewShRuntime(false)
	if got := rt.Name(); got != "virtual-sh" {
		t.Errorf("Name() = %q, want %q", got, "virtual-sh")
	}
}

// TestShRuntime_Available tests the Available method.
func TestShRuntime_Available(t *testing.T) {
	t.Parallel()

	rt := NewShRuntime(false)
	if !rt.Available() {
		t.Error("Available() = false, want true (virtual runtime is always available)")
	}
}

// TestShRuntime_SupportsInteractive tests the SupportsInteractive method.
func TestShRuntime_SupportsInteractive(t *testing.T) {
	t.Parallel()

	rt := NewShRuntime(false)
	if !rt.SupportsInteractive() {
		t.Error("SupportsInteractive() = false, want true")
	}
}

// TestShRuntime_Validate_EmptyScript tests validation for an empty script.
func TestShRuntime_Validate_EmptyScript(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	// Create a command with an empty script
	cmd := testCommandWithScript("empty-script", "", invowkfile.RuntimeVirtualSh)

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	err := rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for empty script, got nil")
	}
	if err != nil && !errors.Is(err, errVirtualNoScript) {
		t.Errorf("Validate() error = %v, want %v", err, errVirtualNoScript)
	}
}

// TestShRuntime_Validate_NilImpl tests validation for nil implementation.
func TestShRuntime_Validate_NilImpl(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
	}

	cmd := &invowkfile.Command{
		Name: "nil-impl",
	}

	rt := NewShRuntime(false)
	ctx := NewExecutionContext(t.Context(), cmd, inv)
	ctx.SelectedImpl = nil // Explicitly set to nil

	err := rt.Validate(ctx)
	if err == nil {
		t.Error("Validate() expected error for nil implementation, got nil")
	}
	if err != nil && !errors.Is(err, errVirtualNoImpl) {
		t.Errorf("Validate() error = %v, want %v", err, errVirtualNoImpl)
	}
}

// TestExecutionContext_EffectiveWorkDir_Virtual tests working directory resolution via ExecutionContext.
func TestShRuntime_NewShRuntime(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			rt := NewShRuntime(tt.enableUroot)
			if rt.UrootUtilsEnabled() != tt.wantUroot {
				t.Errorf("NewShRuntime(%v).UrootUtilsEnabled() = %v, want %v",
					tt.enableUroot, rt.UrootUtilsEnabled(), tt.wantUroot)
			}
		})
	}
}

// TestShRuntime_PositionalArgs_DashPrefix verifies that positional arguments
// starting with "-" or "--" are correctly passed as $1, $2, etc. and NOT interpreted
// as shell options by interp.Params(). This exercises the "--" prefix guard in virtual.go.
func TestShRuntime_MockEnvBuilder_Error(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	cmd := testCommandWithScript("env-error", "echo test", invowkfile.RuntimeVirtualSh)

	mockErr := errors.New("mock virtual env build failure")
	rt := NewShRuntime(false, WithShEnvBuilder(&MockEnvBuilder{Err: mockErr}))
	ctx := NewExecutionContext(t.Context(), cmd, inv)

	ctx.IO.Stdout = &bytes.Buffer{}
	ctx.IO.Stderr = &bytes.Buffer{}

	result := rt.Execute(ctx)
	if result.ExitCode == 0 {
		t.Error("Execute() should fail when EnvBuilder returns error")
	}
	if result.Error == nil {
		t.Fatal("Execute() should return error when EnvBuilder fails")
	}
	if !errors.Is(result.Error, mockErr) {
		t.Errorf("Execute() error = %v, want wrapped %v", result.Error, mockErr)
	}
}

// TestShRuntime_SetE_StopsOnError verifies that "set -e" (errexit) in a virtual
// script terminates execution immediately when a command fails, and the exit code
// is propagated correctly through the interp.ExitStatus error type.
