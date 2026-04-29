// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidShellPath is the sentinel error wrapped by InvalidShellPathError.
var ErrInvalidShellPath = errors.New("invalid shell path")

type (
	// ShellPath represents a filesystem path to a shell executable.
	// The zero value ("") is valid and means "use system default shell".
	// Non-zero values must not be whitespace-only.
	ShellPath string

	// InvalidShellPathError is returned when a ShellPath value is whitespace-only.
	// It wraps ErrInvalidShellPath for errors.Is() compatibility.
	InvalidShellPathError struct {
		Value ShellPath
	}
)

// Error implements the error interface for InvalidShellPathError.
func (e *InvalidShellPathError) Error() string {
	return fmt.Sprintf("invalid shell path %q (must not be whitespace-only)", e.Value)
}

// Unwrap returns ErrInvalidShellPath for errors.Is() compatibility.
func (e *InvalidShellPathError) Unwrap() error { return ErrInvalidShellPath }

// Validate returns nil if the ShellPath is valid, or a validation error if not.
func (s ShellPath) Validate() error {
	if s == "" {
		return nil
	}
	if strings.TrimSpace(string(s)) == "" {
		return &InvalidShellPathError{Value: s}
	}
	return nil
}

// String returns the string representation of the ShellPath.
func (s ShellPath) String() string { return string(s) }
