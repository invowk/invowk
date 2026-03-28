// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func TestRunInteractiveCmd_NilCommand(t *testing.T) {
	t.Parallel()

	// RunInteractiveCmd is called by InteractiveBuilder.Run after the nil check,
	// but we can test the builder's nil-command path which guards against this.
	builder := NewInteractive()
	result, err := builder.Run()

	if err == nil {
		t.Fatal("expected error when no command is provided")
	}
	if !errors.Is(err, errNoCommand) {
		t.Errorf("expected errNoCommand, got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when no command is provided")
	}
}

func TestRunInteractiveCmd_PTYCreationFailure(t *testing.T) {
	t.Parallel()

	// Use a short timeout: on Linux CI, xpty.NewPty() fails immediately (no terminal).
	// On Windows CI, ConPTY is always available — PTY creation succeeds but
	// tea.Program.Run() blocks waiting for terminal input that never comes.
	// The 10-second deadline prevents this from consuming the full test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "echo", "test")

	result, err := RunInteractiveCmd(ctx, InteractiveOptions{}, cmd)
	// In a CI/non-TTY environment, PTY creation, Bubble Tea, or the context
	// deadline will produce an error. The key assertion is no panic.
	if err != nil {
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("expected non-empty error message")
		}
	}
	if err == nil && result == nil {
		t.Error("expected non-nil result when no error is returned")
	}
}

func TestRunInteractiveCmd_CancelledContext(t *testing.T) {
	t.Parallel()

	// Create an already-cancelled context to test graceful handling.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cmd := exec.CommandContext(ctx, "echo", "done")

	result, err := RunInteractiveCmd(ctx, InteractiveOptions{}, cmd)
	// The function should not panic with a cancelled context.
	// It may fail at PTY creation, command start, or TUI run.
	if err != nil {
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	}
	if err == nil && result == nil {
		t.Error("expected non-nil result when no error is returned")
	}
}

func TestInteractiveBuilder_Run_WithCommand(t *testing.T) {
	t.Parallel()

	// Verify that setting a command via the builder clears the nil-command guard.
	// We cannot fully execute RunInteractiveCmd in a non-terminal test
	// environment, but we verify the builder correctly passes the command through.
	cmd := exec.CommandContext(t.Context(), "echo", "hello")

	builder := NewInteractive().Command(cmd)

	if builder.cmd == nil {
		t.Fatal("expected command to be set on builder")
	}
	if builder.cmd != cmd {
		t.Error("expected builder.cmd to be the same command that was set")
	}
}
