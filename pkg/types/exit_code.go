// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrInvalidExitCode is the sentinel error wrapped by InvalidExitCodeError.
var ErrInvalidExitCode = errors.New("invalid exit code")

type (
	// ExitCode represents a process exit status code.
	// Exit codes are in the range 0-255 on POSIX systems.
	// The zero value (0) means success.
	ExitCode int

	// InvalidExitCodeError is returned when an ExitCode is outside the
	// valid range (0-255).
	InvalidExitCodeError struct {
		Value ExitCode
	}
)

// Error implements the error interface.
func (e *InvalidExitCodeError) Error() string {
	return fmt.Sprintf("invalid exit code %d (must be in range 0-255)", e.Value)
}

// Unwrap returns ErrInvalidExitCode so callers can use errors.Is for programmatic detection.
func (e *InvalidExitCodeError) Unwrap() error { return ErrInvalidExitCode }

// Validate returns an error if the ExitCode is outside the valid range (0-255).
func (c ExitCode) Validate() error {
	if c < 0 || c > 255 {
		return &InvalidExitCodeError{Value: c}
	}
	return nil
}

// IsSuccess returns true if the exit code indicates successful execution.
func (c ExitCode) IsSuccess() bool { return c == 0 }

// IsTransient returns true if the exit code indicates a transient container
// engine error that may succeed on retry (codes 125 and 126).
func (c ExitCode) IsTransient() bool { return c == 125 || c == 126 }

// String returns the decimal string representation of the ExitCode.
func (c ExitCode) String() string { return strconv.Itoa(int(c)) }
