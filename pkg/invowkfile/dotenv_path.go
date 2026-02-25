// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidDotenvFilePath is the sentinel error wrapped by InvalidDotenvFilePathError.
var ErrInvalidDotenvFilePath = errors.New("invalid dotenv file path")

type (
	// DotenvFilePath represents a path to a dotenv file for environment variable loading.
	// Paths are relative to the invowkfile location (or module root for modules).
	// Paths suffixed with '?' are optional and will not cause an error if missing.
	// A valid DotenvFilePath must be non-empty and not whitespace-only.
	DotenvFilePath string

	// InvalidDotenvFilePathError is returned when a DotenvFilePath value is
	// empty or whitespace-only. It wraps ErrInvalidDotenvFilePath for errors.Is().
	InvalidDotenvFilePathError struct {
		Value DotenvFilePath
	}
)

// String returns the string representation of the DotenvFilePath.
func (p DotenvFilePath) String() string { return string(p) }

// IsValid returns whether the DotenvFilePath is valid.
// A valid path must be non-empty and not whitespace-only.
func (p DotenvFilePath) IsValid() (bool, []error) {
	if strings.TrimSpace(string(p)) == "" {
		return false, []error{&InvalidDotenvFilePathError{Value: p}}
	}
	return true, nil
}

// Error implements the error interface for InvalidDotenvFilePathError.
func (e *InvalidDotenvFilePathError) Error() string {
	return fmt.Sprintf("invalid dotenv file path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidDotenvFilePath for errors.Is() compatibility.
func (e *InvalidDotenvFilePathError) Unwrap() error { return ErrInvalidDotenvFilePath }
