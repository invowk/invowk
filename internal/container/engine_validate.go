// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidRunResult is the sentinel error wrapped by InvalidRunResultError.
var ErrInvalidRunResult = errors.New("invalid run result")

// InvalidRunResultError is returned when a RunResult has invalid fields.
// It wraps ErrInvalidRunResult for errors.Is() compatibility and collects
// field-level validation errors from ContainerID and ExitCode.
type InvalidRunResultError struct {
	FieldErrors []error
}

// Error implements the error interface for InvalidRunResultError.
func (e *InvalidRunResultError) Error() string {
	return types.FormatFieldErrors("run result", e.FieldErrors)
}

// Unwrap returns ErrInvalidRunResult for errors.Is() compatibility.
func (e *InvalidRunResultError) Unwrap() error { return ErrInvalidRunResult }

// Validate returns nil if the RunResult has valid fields, or a validation error if not.
// It validates ContainerID (when non-empty) and delegates to ExitCode.Validate().
func (r RunResult) Validate() error {
	var errs []error
	if r.ContainerID != "" {
		if err := r.ContainerID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := r.ExitCode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidRunResultError{FieldErrors: errs}
	}
	return nil
}
