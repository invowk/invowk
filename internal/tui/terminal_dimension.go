// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrInvalidTerminalDimension is the sentinel error wrapped by InvalidTerminalDimensionError.
var ErrInvalidTerminalDimension = errors.New("invalid terminal dimension")

type (
	// TerminalDimension represents a size in terminal units (columns or lines).
	// The zero value (0) is valid and means "auto" (use terminal default).
	// Negative values are invalid.
	TerminalDimension int

	// InvalidTerminalDimensionError is returned when a TerminalDimension is negative.
	// It wraps ErrInvalidTerminalDimension for errors.Is() compatibility.
	InvalidTerminalDimensionError struct {
		Value TerminalDimension
	}
)

// String returns the decimal string representation of the TerminalDimension.
func (d TerminalDimension) String() string { return strconv.Itoa(int(d)) }

// Validate returns nil if the TerminalDimension is valid, or a validation error
// if it is negative. The zero value (0) means "auto" and is valid.
func (d TerminalDimension) Validate() error {
	if d < 0 {
		return &InvalidTerminalDimensionError{Value: d}
	}
	return nil
}

// Error implements the error interface for InvalidTerminalDimensionError.
func (e *InvalidTerminalDimensionError) Error() string {
	return fmt.Sprintf("invalid terminal dimension %d: must be >= 0 (0 means auto)", e.Value)
}

// Unwrap returns ErrInvalidTerminalDimension for errors.Is() compatibility.
func (e *InvalidTerminalDimensionError) Unwrap() error { return ErrInvalidTerminalDimension }
