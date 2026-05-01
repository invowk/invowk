// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

type fakeInteractiveTerminal struct{}

func (fakeInteractiveTerminal) Read([]byte) (int, error) { return 0, errors.New("closed") }

func (fakeInteractiveTerminal) Write([]byte) (int, error) { return 0, nil }

func (fakeInteractiveTerminal) Resize(int, int) error { return nil }

func TestInteractiveBuilderRunNilCommand(t *testing.T) {
	t.Parallel()

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

func TestRunInteractiveSessionRequiresTerminal(t *testing.T) {
	t.Parallel()

	result, err := RunInteractiveSession(t.Context(), InteractiveOptions{}, nil, func(context.Context) InteractiveResult {
		return InteractiveResult{}
	})
	if err == nil {
		t.Fatal("expected error for missing terminal")
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestRunInteractiveSessionRequiresWaitFunc(t *testing.T) {
	t.Parallel()

	result, err := RunInteractiveSession(t.Context(), InteractiveOptions{}, fakeInteractiveTerminal{}, nil)
	if err == nil {
		t.Fatal("expected error for missing wait function")
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func TestInteractiveBuilder_Run_WithCommand(t *testing.T) {
	t.Parallel()

	// Verify that setting a command via the builder clears the nil-command guard.
	// Command execution is owned by the runtime adapter; this builder only stores
	// the command for backwards-compatible configuration tests.
	cmd := exec.CommandContext(t.Context(), "echo", "hello")

	builder := NewInteractive().Command(cmd)

	if builder.cmd == nil {
		t.Fatal("expected command to be set on builder")
	}
	if builder.cmd != cmd {
		t.Error("expected builder.cmd to be the same command that was set")
	}
}
