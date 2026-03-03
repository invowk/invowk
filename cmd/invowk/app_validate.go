// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidExecuteRequest is the sentinel error wrapped by InvalidExecuteRequestError.
	ErrInvalidExecuteRequest = errors.New("invalid execute request")

	// ErrInvalidExecuteResult is the sentinel error wrapped by InvalidExecuteResultError.
	ErrInvalidExecuteResult = errors.New("invalid execute result")

	// ErrInvalidSourceFilter is the sentinel error wrapped by InvalidSourceFilterError.
	ErrInvalidSourceFilter = errors.New("invalid source filter")
)

type (
	// InvalidExecuteRequestError is returned when an ExecuteRequest has invalid fields.
	// It wraps ErrInvalidExecuteRequest for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidExecuteRequestError struct {
		FieldErrors []error
	}

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

// Error implements the error interface for InvalidExecuteRequestError.
func (e *InvalidExecuteRequestError) Error() string {
	return fmt.Sprintf("invalid execute request: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidExecuteRequest for errors.Is() compatibility.
func (e *InvalidExecuteRequestError) Unwrap() error { return ErrInvalidExecuteRequest }

// Error implements the error interface for InvalidExecuteResultError.
func (e *InvalidExecuteResultError) Error() string {
	return fmt.Sprintf("invalid execute result: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidExecuteResult for errors.Is() compatibility.
func (e *InvalidExecuteResultError) Unwrap() error { return ErrInvalidExecuteResult }

// Error implements the error interface for InvalidSourceFilterError.
func (e *InvalidSourceFilterError) Error() string {
	return fmt.Sprintf("invalid source filter: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidSourceFilter for errors.Is() compatibility.
func (e *InvalidSourceFilterError) Unwrap() error { return ErrInvalidSourceFilter }

// Validate returns nil if the ExecuteRequest has valid fields, or a validation error if not.
// It validates Runtime (when non-empty), FromSource (when non-empty), Workdir (when non-empty),
// EnvFiles, ConfigPath (when non-empty), EnvInheritMode (when non-empty), EnvInheritAllow,
// EnvInheritDeny, and ResolvedCommand (when non-nil).
func (r ExecuteRequest) Validate() error {
	var errs []error
	if r.Runtime != "" {
		if err := r.Runtime.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.FromSource != "" {
		if err := r.FromSource.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.Workdir != "" {
		if err := r.Workdir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, f := range r.EnvFiles {
		if err := f.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.ConfigPath != "" {
		if err := r.ConfigPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.EnvInheritMode != "" {
		if err := r.EnvInheritMode.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, name := range r.EnvInheritAllow {
		if err := name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, name := range r.EnvInheritDeny {
		if err := name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.ResolvedCommand != nil {
		if err := r.ResolvedCommand.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidExecuteRequestError{FieldErrors: errs}
	}
	return nil
}

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
