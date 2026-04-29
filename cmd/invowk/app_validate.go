// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidExecuteRequest is the sentinel error wrapped by InvalidExecuteRequestError.
	ErrInvalidExecuteRequest = commandsvc.ErrInvalidRequest

	// ErrInvalidExecuteResult is the sentinel error wrapped by InvalidExecuteResultError.
	ErrInvalidExecuteResult = errors.New("invalid execute result")

	// ErrInvalidSourceFilter is the sentinel error wrapped by InvalidSourceFilterError.
	ErrInvalidSourceFilter = errors.New("invalid source filter")

	// errNoCommandSpecified is returned when a command verb is required but none was provided.
	errNoCommandSpecified = errors.New("no command specified")
)

type (
	// InvalidExecuteRequestError is the CLI alias for commandsvc request validation errors.
	InvalidExecuteRequestError = commandsvc.InvalidRequestError

	// InvalidExecuteResultError is returned when an ExecuteResult has invalid fields.
	// It wraps ErrInvalidExecuteResult for errors.Is() compatibility and collects
	// field-level validation errors from ExitCode.
	InvalidExecuteResultError struct {
		FieldErrors []error
	}

	// InvalidSourceFilterError is returned when a SourceFilter has invalid fields.
	// It wraps ErrInvalidSourceFilter for errors.Is() compatibility and collects
	// field-level validation errors from SourceID.
	InvalidSourceFilterError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidExecuteResultError.
func (e *InvalidExecuteResultError) Error() string {
	return types.FormatFieldErrors("execute result", e.FieldErrors)
}

// Unwrap returns ErrInvalidExecuteResult for errors.Is() compatibility.
func (e *InvalidExecuteResultError) Unwrap() error { return ErrInvalidExecuteResult }

// Error implements the error interface for InvalidSourceFilterError.
func (e *InvalidSourceFilterError) Error() string {
	return types.FormatFieldErrors("source filter", e.FieldErrors)
}

// Unwrap returns ErrInvalidSourceFilter for errors.Is() compatibility.
func (e *InvalidSourceFilterError) Unwrap() error { return ErrInvalidSourceFilter }

// Validate returns nil if the ExecuteResult has valid fields, or a validation error if not.
// It delegates to ExitCode.Validate().
// Note: validation of the CLI-layer result is done for completeness; the service layer
// validates commandsvc.Result via commandsvc.Request.Validate() in Service.Execute().
func (r ExecuteResult) Validate() error {
	if err := r.ExitCode.Validate(); err != nil {
		return &InvalidExecuteResultError{FieldErrors: []error{err}}
	}
	return nil
}

// Validate returns nil if the SourceFilter has valid fields, or a validation error if not.
// It delegates to SourceID.Validate(). SourceID is always validated (it is the filter's
// primary purpose — an empty or invalid SourceID makes the filter meaningless).
func (f SourceFilter) Validate() error {
	if err := f.SourceID.Validate(); err != nil {
		return &InvalidSourceFilterError{FieldErrors: []error{err}}
	}
	return nil
}
