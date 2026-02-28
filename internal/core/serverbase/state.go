// SPDX-License-Identifier: MPL-2.0

package serverbase

import (
	"errors"
	"fmt"
)

const (
	// StateCreated indicates the server instance was created but Start() not called.
	StateCreated State = iota
	// StateStarting indicates Start() was called and the server is initializing.
	StateStarting
	// StateRunning indicates the server is accepting connections/requests.
	StateRunning
	// StateStopping indicates Stop() was called and graceful shutdown is in progress.
	StateStopping
	// StateStopped is terminal: the server has stopped.
	StateStopped
	// StateFailed is terminal: the server failed to start or encountered a fatal error.
	StateFailed
)

// ErrInvalidState is returned when a State value is not one of the defined lifecycle states.
var ErrInvalidState = errors.New("invalid state")

type (
	// State represents the lifecycle state of a server.
	State int32

	// InvalidStateError is returned when a State value is not recognized.
	// It wraps ErrInvalidState for errors.Is() compatibility.
	InvalidStateError struct {
		Value State
	}
)

// String returns a human-readable representation of the server state.
func (s State) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Error implements the error interface for InvalidStateError.
func (e *InvalidStateError) Error() string {
	return fmt.Sprintf("invalid state %d (valid: 0=created, 1=starting, 2=running, 3=stopping, 4=stopped, 5=failed)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidStateError) Unwrap() error {
	return ErrInvalidState
}

// Validate returns nil if the State is one of the defined lifecycle states,
// or an error wrapping ErrInvalidState if it is not.
func (s State) Validate() error {
	switch s {
	case StateCreated, StateStarting, StateRunning, StateStopping, StateStopped, StateFailed:
		return nil
	default:
		return &InvalidStateError{Value: s}
	}
}

// IsTerminal returns true if the state is a terminal state (Stopped or Failed).
func (s State) IsTerminal() bool {
	return s == StateStopped || s == StateFailed
}
