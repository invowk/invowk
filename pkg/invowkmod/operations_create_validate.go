// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidCreateOptions is the sentinel error wrapped by InvalidCreateOptionsError.
var ErrInvalidCreateOptions = errors.New("invalid create options")

type (
	// InvalidCreateOptionsError is returned when a CreateOptions has invalid fields.
	// It wraps ErrInvalidCreateOptions for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidCreateOptionsError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidCreateOptionsError.
func (e *InvalidCreateOptionsError) Error() string {
	return types.FormatFieldErrors("create options", e.FieldErrors)
}

// Unwrap returns ErrInvalidCreateOptions for errors.Is() compatibility.
func (e *InvalidCreateOptionsError) Unwrap() error { return ErrInvalidCreateOptions }

// Validate returns nil if the CreateOptions has valid fields, or an error
// collecting all field-level validation failures.
// Name and Module are validated when non-empty (zero values are valid for
// deferred initialization). ParentDir is validated when non-empty.
// Description is validated when non-empty (optional field).
func (o CreateOptions) Validate() error {
	var errs []error
	if o.Name != "" {
		if err := o.Name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.ParentDir != "" {
		if err := o.ParentDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.Module != "" {
		if err := o.Module.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.Description != "" {
		if err := o.Description.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCreateOptionsError{FieldErrors: errs}
	}
	return nil
}
