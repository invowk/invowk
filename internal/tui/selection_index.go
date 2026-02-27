// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
)

// ErrInvalidSelectionIndex is the sentinel error wrapped by InvalidSelectionIndexError.
var ErrInvalidSelectionIndex = errors.New("invalid selection index")

type (
	// SelectionIndex represents a 0-based position in an option list.
	SelectionIndex int

	// InvalidSelectionIndexError is returned when a SelectionIndex is negative.
	// It wraps ErrInvalidSelectionIndex for errors.Is() compatibility.
	InvalidSelectionIndexError struct {
		Value SelectionIndex
	}
)

// String returns the decimal string representation of the SelectionIndex.
func (i SelectionIndex) String() string { return strconv.Itoa(int(i)) }

// Validate returns nil if the SelectionIndex is valid, or a validation error
// if it is negative.
func (i SelectionIndex) Validate() error {
	if i < 0 {
		return &InvalidSelectionIndexError{Value: i}
	}
	return nil
}

// Error implements the error interface for InvalidSelectionIndexError.
func (e *InvalidSelectionIndexError) Error() string {
	return fmt.Sprintf("invalid selection index %d: must be >= 0", e.Value)
}

// Unwrap returns ErrInvalidSelectionIndex for errors.Is() compatibility.
func (e *InvalidSelectionIndexError) Unwrap() error { return ErrInvalidSelectionIndex }
