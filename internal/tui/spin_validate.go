// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidSpinOptions is the sentinel error wrapped by InvalidSpinOptionsError.
	ErrInvalidSpinOptions = errors.New("invalid spin options")

	// ErrInvalidSpinCommandOptions is the sentinel error wrapped by InvalidSpinCommandOptionsError.
	ErrInvalidSpinCommandOptions = errors.New("invalid spin command options")
)

type (
	// InvalidSpinOptionsError is returned when SpinOptions has invalid fields.
	// It wraps ErrInvalidSpinOptions for errors.Is() compatibility and collects
	// field-level validation errors from Type.
	InvalidSpinOptionsError struct {
		FieldErrors []error
	}

	// InvalidSpinCommandOptionsError is returned when SpinCommandOptions has invalid fields.
	// It wraps ErrInvalidSpinCommandOptions for errors.Is() compatibility and collects
	// field-level validation errors from Type.
	InvalidSpinCommandOptionsError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidSpinOptionsError.
func (e *InvalidSpinOptionsError) Error() string {
	return types.FormatFieldErrors("spin options", e.FieldErrors)
}

// Unwrap returns ErrInvalidSpinOptions for errors.Is() compatibility.
func (e *InvalidSpinOptionsError) Unwrap() error { return ErrInvalidSpinOptions }

// Error implements the error interface for InvalidSpinCommandOptionsError.
func (e *InvalidSpinCommandOptionsError) Error() string {
	return types.FormatFieldErrors("spin command options", e.FieldErrors)
}

// Unwrap returns ErrInvalidSpinCommandOptions for errors.Is() compatibility.
func (e *InvalidSpinCommandOptionsError) Unwrap() error { return ErrInvalidSpinCommandOptions }

// Validate returns nil if the SpinOptions has valid fields, or a validation error if not.
// It delegates to Type.Validate().
func (o SpinOptions) Validate() error {
	if err := o.Type.Validate(); err != nil {
		return &InvalidSpinOptionsError{FieldErrors: []error{err}}
	}
	return nil
}

// Validate returns nil if the SpinCommandOptions has valid fields, or a validation error if not.
// It delegates to Type.Validate().
func (o SpinCommandOptions) Validate() error {
	if err := o.Type.Validate(); err != nil {
		return &InvalidSpinCommandOptionsError{FieldErrors: []error{err}}
	}
	return nil
}
