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

// IsValid returns whether the SelectionIndex is valid.
func (i SelectionIndex) IsValid() (bool, []error) {
	if i < 0 {
		return false, []error{&InvalidSelectionIndexError{Value: i}}
	}
	return true, nil
}

// Error implements the error interface for InvalidSelectionIndexError.
func (e *InvalidSelectionIndexError) Error() string {
	return fmt.Sprintf("invalid selection index %d: must be >= 0", e.Value)
}

// Unwrap returns ErrInvalidSelectionIndex for errors.Is() compatibility.
func (e *InvalidSelectionIndexError) Unwrap() error { return ErrInvalidSelectionIndex }
