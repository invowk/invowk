// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidResult is the sentinel error wrapped by InvalidResultError.
var (
	ErrInvalidResult         = errors.New("invalid provision result")
	ErrInvalidWarningMessage = errors.New("invalid provision warning message")
)

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

// String returns the underlying warning message.
func (m WarningMessage) String() string { return string(m) }

// Validate returns nil when the warning message is non-empty after trimming.
func (m WarningMessage) Validate() error {
	if strings.TrimSpace(string(m)) == "" {
		return fmt.Errorf("%w: message must not be empty", ErrInvalidWarningMessage)
	}
	return nil
}

// Validate returns nil when the warning is well-formed.
func (w Warning) Validate() error {
	return w.Message.Validate()
}

// Validate returns nil if the Result has valid fields, or a validation error if not.
// It validates ImageTag (when non-empty).
func (r Result) Validate() error {
	var errs []error
	if r.ImageTag != "" {
		if err := r.ImageTag.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, warning := range r.Warnings {
		if err := warning.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidResultError{FieldErrors: errs}
	}
	return nil
}
