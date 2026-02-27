// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidFilesystemPath is the sentinel error wrapped by InvalidFilesystemPathError.
var ErrInvalidFilesystemPath = errors.New("invalid filesystem path")

type (
	// FilesystemPath represents an absolute or relative filesystem path.
	// A valid path must be non-empty and not whitespace-only.
	// The zero value ("") is invalid — a path must always point somewhere.
	FilesystemPath string

	// InvalidFilesystemPathError is returned when a FilesystemPath value is
	// empty or whitespace-only.
	InvalidFilesystemPathError struct {
		Value FilesystemPath
	}
)

// String returns the string representation of the FilesystemPath.
func (p FilesystemPath) String() string { return string(p) }

// Validate returns an error if the FilesystemPath is invalid.
// A valid path must be non-empty and not whitespace-only.
//
//goplint:nonzero
func (p FilesystemPath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidFilesystemPathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidFilesystemPathError.
func (e *InvalidFilesystemPathError) Error() string {
	return fmt.Sprintf("invalid filesystem path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidFilesystemPath for errors.Is() compatibility.
func (e *InvalidFilesystemPathError) Unwrap() error { return ErrInvalidFilesystemPath }
