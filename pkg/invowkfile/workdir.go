// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidWorkDir is the sentinel error wrapped by InvalidWorkDirError.
var ErrInvalidWorkDir = errors.New("invalid workdir")

type (
	// WorkDir represents a working directory path for command execution.
	// The zero value ("") is valid and means "inherit from parent or use default".
	// Non-zero values must not be whitespace-only.
	WorkDir string

	// InvalidWorkDirError is returned when a WorkDir value is whitespace-only.
	// It wraps ErrInvalidWorkDir for errors.Is() compatibility.
	InvalidWorkDirError struct {
		Value WorkDir
	}
)

// Error implements the error interface for InvalidWorkDirError.
func (e *InvalidWorkDirError) Error() string {
	return fmt.Sprintf("invalid workdir %q (must not be whitespace-only)", e.Value)
}

// Unwrap returns ErrInvalidWorkDir for errors.Is() compatibility.
func (e *InvalidWorkDirError) Unwrap() error { return ErrInvalidWorkDir }

// IsValid returns whether the WorkDir is valid.
// The zero value ("") is valid — it means "inherit from parent".
// Non-zero values must not be whitespace-only.
func (w WorkDir) IsValid() (bool, []error) {
	if w == "" {
		return true, nil
	}
	if strings.TrimSpace(string(w)) == "" {
		return false, []error{&InvalidWorkDirError{Value: w}}
	}
	return true, nil
}

// String returns the string representation of the WorkDir.
func (w WorkDir) String() string { return string(w) }
