// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	// MaxDescriptionTextLength is the maximum number of runes allowed in a shared description.
	MaxDescriptionTextLength = 10 * 1024
)

// ErrInvalidDescriptionText is the sentinel error wrapped by InvalidDescriptionTextError.
var ErrInvalidDescriptionText = errors.New("invalid description text")

type (
	// DescriptionText represents a human-readable description for commands, flags,
	// arguments, or modules. The zero value ("") is valid (means no description
	// provided). Non-zero values must not be whitespace-only.
	DescriptionText string

	// InvalidDescriptionTextError is returned when a DescriptionText value is
	// non-empty but whitespace-only.
	InvalidDescriptionTextError struct {
		Value DescriptionText
	}
)

// String returns the string representation of the DescriptionText.
func (d DescriptionText) String() string { return string(d) }

// Validate returns an error if the DescriptionText is invalid.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (d DescriptionText) Validate() error {
	if d == "" {
		return nil
	}
	if strings.TrimSpace(string(d)) == "" {
		return &InvalidDescriptionTextError{Value: d}
	}
	if utf8.RuneCountInString(string(d)) > MaxDescriptionTextLength {
		return &InvalidDescriptionTextError{Value: d}
	}
	return nil
}

// Error implements the error interface for InvalidDescriptionTextError.
func (e *InvalidDescriptionTextError) Error() string {
	return fmt.Sprintf("invalid description text: non-empty value must not be whitespace-only and must be at most %d runes (got %q)", MaxDescriptionTextLength, e.Value)
}

// Unwrap returns ErrInvalidDescriptionText for errors.Is() compatibility.
func (e *InvalidDescriptionTextError) Unwrap() error { return ErrInvalidDescriptionText }
