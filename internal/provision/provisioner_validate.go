// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidResult is the sentinel error wrapped by InvalidResultError.
var ErrInvalidResult = errors.New("invalid provision result")

// InvalidResultError is returned when a Result has invalid fields.
// It wraps ErrInvalidResult for errors.Is() compatibility and collects
// field-level validation errors from ImageTag.
type InvalidResultError struct {
	FieldErrors []error
}

// Error implements the error interface for InvalidResultError.
func (e *InvalidResultError) Error() string {
	return types.FormatFieldErrors("provision result", e.FieldErrors)
}

// Unwrap returns ErrInvalidResult for errors.Is() compatibility.
func (e *InvalidResultError) Unwrap() error { return ErrInvalidResult }

// Validate returns nil if the Result has valid fields, or a validation error if not.
// It validates ImageTag (when non-empty).
func (r Result) Validate() error {
	if r.ImageTag != "" {
		if err := r.ImageTag.Validate(); err != nil {
			return &InvalidResultError{FieldErrors: []error{err}}
		}
	}
	return nil
}
