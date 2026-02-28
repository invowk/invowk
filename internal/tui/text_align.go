// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
)

const (
	// AlignLeft aligns text to the left (zero value default).
	AlignLeft TextAlign = "left"
	// AlignCenter centers text horizontally.
	AlignCenter TextAlign = "center"
	// AlignRight aligns text to the right.
	AlignRight TextAlign = "right"
)

// ErrInvalidTextAlign is the sentinel error wrapped by InvalidTextAlignError.
var ErrInvalidTextAlign = errors.New("invalid text alignment")

type (
	// TextAlign represents horizontal text alignment for TUI components.
	// The zero value ("") is valid and defaults to left alignment in rendering.
	TextAlign string

	// InvalidTextAlignError is returned when a TextAlign value is not recognized.
	// It wraps ErrInvalidTextAlign for errors.Is() compatibility.
	InvalidTextAlignError struct {
		Value TextAlign
	}
)

// String returns the string representation of the TextAlign.
func (a TextAlign) String() string { return string(a) }

// Validate returns nil if the TextAlign is one of the defined alignments,
// or a validation error if it is not.
// The zero value ("") is valid and defaults to left alignment.
func (a TextAlign) Validate() error {
	switch a {
	case "", AlignLeft, AlignCenter, AlignRight:
		return nil
	default:
		return &InvalidTextAlignError{Value: a}
	}
}

// Error implements the error interface for InvalidTextAlignError.
func (e *InvalidTextAlignError) Error() string {
	return fmt.Sprintf("invalid text alignment %q (valid: left, center, right)", e.Value)
}

// Unwrap returns ErrInvalidTextAlign for errors.Is() compatibility.
func (e *InvalidTextAlignError) Unwrap() error { return ErrInvalidTextAlign }
