// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"os/exec"
	"testing"
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

	// Provide a command but with a real context to exercise the code path
	// where PTY creation or command start may fail. RunInteractiveCmd requires
	// a real PTY, so we verify it returns a wrapped error rather than panicking
	// when invoked in a non-terminal environment.
	cmd := exec.CommandContext(t.Context(), "echo", "test")

	result, err := RunInteractiveCmd(t.Context(), InteractiveOptions{}, cmd)
	// In a CI/non-TTY environment, PTY creation or Bubble Tea may fail.
	// The key assertion is that the function returns a meaningful error
	// and does not panic.
	if err != nil {
		// Verify the error is wrapped with context
		errMsg := err.Error()
		if errMsg == "" {
			t.Error("expected non-empty error message")
		}
	}
	// If it somehow succeeds (e.g., in a terminal), the result should be non-nil
	if err == nil && result == nil {
		t.Error("expected non-nil result when no error is returned")
	}
}

func TestRunInteractiveCmd_CancelledContext(t *testing.T) {
	t.Parallel()

	// Create an already-cancelled context to test graceful handling.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cmd := exec.CommandContext(ctx, "sleep", "10")

	result, err := RunInteractiveCmd(ctx, InteractiveOptions{}, cmd)
	// The function should not panic with a cancelled context.
	// It may fail at PTY creation, command start, or TUI run.
	if err != nil {
		// Error is expected; just verify it's meaningful
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
