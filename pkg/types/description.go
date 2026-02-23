// SPDX-License-Identifier: MPL-2.0

// Package types defines cross-cutting DDD Value Types used by multiple domain
// packages (invowkfile, invowkmod, etc.). These are foundation types that carry
// semantic meaning and validation but have no domain-specific dependencies.
//
// This package is a leaf dependency: it imports only the standard library.
// Domain packages import it; it never imports domain packages.
package types

import (
	"errors"
	"fmt"
	"strings"
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

// IsValid returns whether the DescriptionText is valid.
// The zero value ("") is valid. Non-zero values must not be whitespace-only.
func (d DescriptionText) IsValid() (bool, []error) {
	if d == "" {
		return true, nil
	}
	if strings.TrimSpace(string(d)) == "" {
		return false, []error{&InvalidDescriptionTextError{Value: d}}
	}
	return true, nil
}

// Error implements the error interface for InvalidDescriptionTextError.
func (e *InvalidDescriptionTextError) Error() string {
	return fmt.Sprintf("invalid description text: non-empty value must not be whitespace-only (got %q)", e.Value)
}

// Unwrap returns ErrInvalidDescriptionText for errors.Is() compatibility.
func (e *InvalidDescriptionTextError) Unwrap() error { return ErrInvalidDescriptionText }
