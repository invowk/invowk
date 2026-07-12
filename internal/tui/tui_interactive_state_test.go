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

func TestExecutionState_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		state   executionState
		wantErr bool
	}{
		{name: "executing", state: stateExecuting},
		{name: "completed", state: stateCompleted},
		{name: "tui", state: stateTUI},
		{name: "invalid", state: executionState(99), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.state.validate()
			if !tt.wantErr {
				if err != nil {
					t.Errorf("validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("validate() error = nil, want invalid state error")
			}
			var stateErr *invalidExecutionStateError
			if !errors.As(err, &stateErr) {
				t.Fatalf("error type = %T, want *invalidExecutionStateError", err)
			}
			if stateErr.value != tt.state {
				t.Errorf("error value = %d, want %d", stateErr.value, tt.state)
			}
		})
	}
}

func TestInvalidExecutionStateError_Error(t *testing.T) {
	t.Parallel()
	err := &invalidExecutionStateError{value: executionState(42)}
	want := "invalid execution state: 42"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
