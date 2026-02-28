// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidInterpreterSpec is the sentinel error wrapped by InvalidInterpreterSpecError.
var ErrInvalidInterpreterSpec = errors.New("invalid interpreter spec")

type (
	// InterpreterSpec represents a command interpreter specification in runtime config.
	// The zero value ("") is valid and means "auto" (detect from shebang).
	// Non-zero values must not be whitespace-only.
	InterpreterSpec string

	// InvalidInterpreterSpecError is returned when an InterpreterSpec value is
	// non-empty but whitespace-only. It wraps ErrInvalidInterpreterSpec for errors.Is().
	InvalidInterpreterSpecError struct {
		Value InterpreterSpec
	}
)

// String returns the string representation of the InterpreterSpec.
func (s InterpreterSpec) String() string { return string(s) }

// Validate returns nil if the InterpreterSpec is valid, or a validation error if not.
// The zero value ("") is valid (means "auto"). Non-zero values must not be whitespace-only.
func (s InterpreterSpec) Validate() error {
	if s == "" {
		return nil
	}
	if strings.TrimSpace(string(s)) == "" {
		return &InvalidInterpreterSpecError{Value: s}
	}
	return nil
}

// Error implements the error interface for InvalidInterpreterSpecError.
func (e *InvalidInterpreterSpecError) Error() string {
	return fmt.Sprintf("invalid interpreter spec %q: non-empty value must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidInterpreterSpec for errors.Is() compatibility.
func (e *InvalidInterpreterSpecError) Unwrap() error { return ErrInvalidInterpreterSpec }
