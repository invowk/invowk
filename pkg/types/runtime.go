// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
)

const (
	// RuntimeNative executes commands using the host system shell.
	RuntimeNative RuntimeMode = "native"
	// RuntimeVirtual executes commands using the embedded mvdan/sh interpreter.
	RuntimeVirtual RuntimeMode = "virtual"
	// RuntimeContainer executes commands inside a container.
	RuntimeContainer RuntimeMode = "container"
)

// ErrInvalidRuntimeMode is the sentinel error wrapped by InvalidRuntimeModeError.
var ErrInvalidRuntimeMode = errors.New("invalid runtime mode")

type (
	// RuntimeMode represents an Invowk execution runtime identity.
	RuntimeMode string

	// InvalidRuntimeModeError is returned when a RuntimeMode value is not recognized.
	InvalidRuntimeModeError struct {
		Value RuntimeMode
	}
)

// Error implements the error interface.
func (e *InvalidRuntimeModeError) Error() string {
	return fmt.Sprintf("invalid runtime mode %q (must be one of: native, virtual, container)", e.Value)
}

// Unwrap returns ErrInvalidRuntimeMode so callers can use errors.Is for programmatic detection.
func (e *InvalidRuntimeModeError) Unwrap() error { return ErrInvalidRuntimeMode }

// String returns the string representation of the RuntimeMode.
func (m RuntimeMode) String() string { return string(m) }

// Validate returns nil if the RuntimeMode is one of the defined runtime modes,
// or a validation error if it is not.
// Note: the zero value ("") is NOT valid; callers use it as a sentinel for "no override".
//
//goplint:nonzero
func (m RuntimeMode) Validate() error {
	switch m {
	case RuntimeNative, RuntimeVirtual, RuntimeContainer:
		return nil
	default:
		return &InvalidRuntimeModeError{Value: m}
	}
}
