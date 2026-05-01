// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"errors"
	"testing"
)

type fakeInteractiveTerminal struct{}

func (fakeInteractiveTerminal) Read([]byte) (int, error) { return 0, errors.New("closed") }

func (fakeInteractiveTerminal) Write([]byte) (int, error) { return 0, nil }

func (fakeInteractiveTerminal) Resize(int, int) error { return nil }

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
