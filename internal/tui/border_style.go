// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
)

const (
	// BorderNone disables the border (zero value).
	BorderNone BorderStyle = ""
	// BorderNormal renders a standard single-line border.
	BorderNormal BorderStyle = "normal"
	// BorderRounded renders a single-line border with rounded corners.
	BorderRounded BorderStyle = "rounded"
	// BorderThick renders a thick/heavy border.
	BorderThick BorderStyle = "thick"
	// BorderDouble renders a double-line border.
	BorderDouble BorderStyle = "double"
	// BorderHidden renders an invisible border that still occupies space.
	BorderHidden BorderStyle = "hidden"
)

// ErrInvalidBorderStyle is the sentinel error wrapped by InvalidBorderStyleError.
var ErrInvalidBorderStyle = errors.New("invalid border style")

type (
	// BorderStyle represents a border rendering style for TUI components.
	// The zero value ("") means no border.
	BorderStyle string

	// InvalidBorderStyleError is returned when a BorderStyle value is not recognized.
	// It wraps ErrInvalidBorderStyle for errors.Is() compatibility.
	InvalidBorderStyleError struct {
		Value BorderStyle
	}
)

// String returns the string representation of the BorderStyle.
func (b BorderStyle) String() string { return string(b) }

// Validate returns nil if the BorderStyle is one of the defined styles,
// or a validation error if it is not.
func (b BorderStyle) Validate() error {
	switch b {
	case BorderNone, BorderNormal, BorderRounded, BorderThick, BorderDouble, BorderHidden:
		return nil
	default:
		return &InvalidBorderStyleError{Value: b}
	}
}

// Error implements the error interface for InvalidBorderStyleError.
func (e *InvalidBorderStyleError) Error() string {
	return fmt.Sprintf("invalid border style %q (valid: none, normal, rounded, thick, double, hidden)", e.Value)
}

// Unwrap returns ErrInvalidBorderStyle for errors.Is() compatibility.
func (e *InvalidBorderStyleError) Unwrap() error { return ErrInvalidBorderStyle }
