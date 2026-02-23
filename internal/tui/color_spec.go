// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidColorSpec is the sentinel error wrapped by InvalidColorSpecError.
var ErrInvalidColorSpec = errors.New("invalid color spec")

type (
	// ColorSpec represents a color specification for TUI styling.
	// Accepts CSS hex codes, ANSI color numbers, or named colors.
	// The zero value ("") is valid and means "no color" (use terminal default).
	ColorSpec string

	// InvalidColorSpecError is returned when a ColorSpec value is whitespace-only.
	// It wraps ErrInvalidColorSpec for errors.Is() compatibility.
	InvalidColorSpecError struct {
		Value ColorSpec
	}
)

// String returns the string representation of the ColorSpec.
func (c ColorSpec) String() string { return string(c) }

// IsValid returns whether the ColorSpec is valid.
// The zero value ("") is valid (means "no color").
// Non-empty values must not be whitespace-only.
func (c ColorSpec) IsValid() (bool, []error) {
	if c != "" && strings.TrimSpace(string(c)) == "" {
		return false, []error{&InvalidColorSpecError{Value: c}}
	}
	return true, nil
}

// Error implements the error interface for InvalidColorSpecError.
func (e *InvalidColorSpecError) Error() string {
	return fmt.Sprintf("invalid color spec %q: must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidColorSpec for errors.Is() compatibility.
func (e *InvalidColorSpecError) Unwrap() error { return ErrInvalidColorSpec }
