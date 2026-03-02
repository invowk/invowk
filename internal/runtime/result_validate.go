// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidResult is the sentinel error wrapped by InvalidResultError.
	ErrInvalidResult = errors.New("invalid result")

	// ErrInvalidInitDiagnostic is the sentinel error wrapped by InvalidInitDiagnosticError.
	ErrInvalidInitDiagnostic = errors.New("invalid init diagnostic")

	// ErrInvalidExecutionContext is the sentinel error wrapped by InvalidExecutionContextError.
	ErrInvalidExecutionContext = errors.New("invalid execution context")
)

type (
	// InvalidResultError is returned when a Result has invalid fields.
	// It wraps ErrInvalidResult for errors.Is() compatibility and collects
	// field-level validation errors from ExitCode.
	InvalidResultError struct {
		FieldErrors []error
	}

	// InvalidInitDiagnosticError is returned when an InitDiagnostic has invalid fields.
	// It wraps ErrInvalidInitDiagnostic for errors.Is() compatibility and collects
	// field-level validation errors from Code.
	InvalidInitDiagnosticError struct {
		FieldErrors []error
	}

	// InvalidExecutionContextError is returned when an ExecutionContext has invalid fields.
	// It wraps ErrInvalidExecutionContext for errors.Is() compatibility and collects
	// field-level validation errors from SelectedRuntime, WorkDir, ExecutionID, Env, and TUI.
	InvalidExecutionContextError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidResultError.
func (e *InvalidResultError) Error() string {
	return fmt.Sprintf("invalid result: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidResult for errors.Is() compatibility.
func (e *InvalidResultError) Unwrap() error { return ErrInvalidResult }

// Error implements the error interface for InvalidInitDiagnosticError.
func (e *InvalidInitDiagnosticError) Error() string {
	return fmt.Sprintf("invalid init diagnostic: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidInitDiagnostic for errors.Is() compatibility.
func (e *InvalidInitDiagnosticError) Unwrap() error { return ErrInvalidInitDiagnostic }

// Error implements the error interface for InvalidExecutionContextError.
func (e *InvalidExecutionContextError) Error() string {
	return fmt.Sprintf("invalid execution context: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidExecutionContext for errors.Is() compatibility.
func (e *InvalidExecutionContextError) Unwrap() error { return ErrInvalidExecutionContext }

// Validate returns nil if the Result has valid fields, or a validation error if not.
// It delegates to ExitCode.Validate().
func (r Result) Validate() error {
	if err := r.ExitCode.Validate(); err != nil {
		return &InvalidResultError{FieldErrors: []error{err}}
	}
	return nil
}

// Validate returns nil if the InitDiagnostic has valid fields, or a validation error if not.
// It delegates to Code.Validate().
func (d InitDiagnostic) Validate() error {
	if err := d.Code.Validate(); err != nil {
		return &InvalidInitDiagnosticError{FieldErrors: []error{err}}
	}
	return nil
}

// Validate returns nil if the ExecutionContext has valid fields, or a validation error if not.
// It validates SelectedRuntime, WorkDir (when non-empty), ExecutionID (when non-empty),
// and delegates to Env.Validate() and TUI.Validate().
//
// This method exists for completeness and API symmetry. In practice, ExecutionContext
// is constructed by NewExecutionContext() from pre-validated data (runtime selection and
// workdir are validated upstream in BuildExecutionContext via BuildExecutionContextOptions.Validate()),
// so construction already guarantees validity.
func (ctx ExecutionContext) Validate() error {
	var errs []error
	if err := ctx.SelectedRuntime.Validate(); err != nil {
		errs = append(errs, err)
	}
	if ctx.WorkDir != "" {
		if err := ctx.WorkDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if ctx.ExecutionID != "" {
		if err := ctx.ExecutionID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := ctx.Env.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := ctx.TUI.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidExecutionContextError{FieldErrors: errs}
	}
	return nil
}
