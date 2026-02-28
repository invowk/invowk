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

// Validate returns nil if the ColorSpec is valid, or a validation error
// if it is non-empty but whitespace-only.
// The zero value ("") is valid (means "no color").
func (c ColorSpec) Validate() error {
	if c != "" && strings.TrimSpace(string(c)) == "" {
		return &InvalidColorSpecError{Value: c}
	}
	return nil
}

// Error implements the error interface for InvalidColorSpecError.
func (e *InvalidColorSpecError) Error() string {
	return fmt.Sprintf("invalid color spec %q: must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidColorSpec for errors.Is() compatibility.
func (e *InvalidColorSpecError) Unwrap() error { return ErrInvalidColorSpec }
