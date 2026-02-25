// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestExecutionState_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state executionState
		want  string
	}{
		{stateExecuting, "executing"},
		{stateCompleted, "completed"},
		{stateTUI, "tui"},
		{executionState(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			if got := tt.state.String(); got != tt.want {
				t.Errorf("executionState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestExecutionState_IsValid(t *testing.T) {
	t.Parallel()

	t.Run("stateExecuting is valid", func(t *testing.T) {
		t.Parallel()
		valid, errs := stateExecuting.isValid()
		if !valid {
			t.Errorf("stateExecuting should be valid, got errors: %v", errs)
		}
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %d", len(errs))
		}
	})

	t.Run("stateCompleted is valid", func(t *testing.T) {
		t.Parallel()
		valid, errs := stateCompleted.isValid()
		if !valid {
			t.Errorf("stateCompleted should be valid, got errors: %v", errs)
		}
	})

	t.Run("stateTUI is valid", func(t *testing.T) {
		t.Parallel()
		valid, errs := stateTUI.isValid()
		if !valid {
			t.Errorf("stateTUI should be valid, got errors: %v", errs)
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		t.Parallel()
		invalid := executionState(99)
		valid, errs := invalid.isValid()
		if valid {
			t.Fatal("executionState(99) should be invalid")
		}
		if len(errs) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
		}

		var stateErr *invalidExecutionStateError
		if !errors.As(errs[0], &stateErr) {
			t.Fatalf("error should be *invalidExecutionStateError, got: %T", errs[0])
		}
		if stateErr.value != 99 {
			t.Errorf("error value = %d, want 99", stateErr.value)
		}
	})
}

func TestInvalidExecutionStateError_Error(t *testing.T) {
	t.Parallel()
	err := &invalidExecutionStateError{value: executionState(42)}
	want := "invalid execution state: 42"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
