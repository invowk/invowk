// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"
)

func TestNewErrorResult(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test error")
	result := NewErrorResult(1, testErr)

	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode 1, got %d", result.ExitCode)
	}
	if !errors.Is(result.Error, testErr) {
		t.Errorf("expected error %v, got %v", testErr, result.Error)
	}
	if result.Output != "" {
		t.Errorf("expected empty Output, got %q", result.Output)
	}
	if result.ErrOutput != "" {
		t.Errorf("expected empty ErrOutput, got %q", result.ErrOutput)
	}
}

func TestNewErrorResult_ZeroCodeNilError(t *testing.T) {
	t.Parallel()

	result := NewErrorResult(0, nil)

	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if result.Error != nil {
		t.Errorf("expected nil error, got %v", result.Error)
	}
	if result.Output != "" {
		t.Errorf("expected empty Output, got %q", result.Output)
	}
	if result.ErrOutput != "" {
		t.Errorf("expected empty ErrOutput, got %q", result.ErrOutput)
	}
}

func TestNewSuccessResult(t *testing.T) {
	t.Parallel()

	result := NewSuccessResult()

	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if result.Error != nil {
		t.Errorf("expected nil error, got %v", result.Error)
	}
	if result.Output != "" {
		t.Errorf("expected empty Output, got %q", result.Output)
	}
	if result.ErrOutput != "" {
		t.Errorf("expected empty ErrOutput, got %q", result.ErrOutput)
	}
}

func TestNewExitCodeResult(t *testing.T) {
	t.Parallel()

	result := NewExitCodeResult(42)

	if result.ExitCode != 42 {
		t.Errorf("expected ExitCode 42, got %d", result.ExitCode)
	}
	if result.Error != nil {
		t.Errorf("expected nil error, got %v", result.Error)
	}
	if result.Output != "" {
		t.Errorf("expected empty Output, got %q", result.Output)
	}
	if result.ErrOutput != "" {
		t.Errorf("expected empty ErrOutput, got %q", result.ErrOutput)
	}
}
