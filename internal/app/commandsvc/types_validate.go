// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidRequest is the sentinel error wrapped by InvalidRequestError.
	ErrInvalidRequest = errors.New("invalid request")

	// ErrInvalidCommandsvcResult is the sentinel error wrapped by InvalidResultError.
	ErrInvalidCommandsvcResult = errors.New("invalid commandsvc result")

	// ErrInvalidDryRunData is the sentinel error wrapped by InvalidDryRunDataError.
	ErrInvalidDryRunData = errors.New("invalid dry run data")
)

type (
	// InvalidRequestError is returned when a Request has invalid fields.
	// It wraps ErrInvalidRequest for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidRequestError struct {
		FieldErrors []error
	}

	// InvalidResultError is returned when a Result has invalid fields.
	// It wraps ErrInvalidCommandsvcResult for errors.Is() compatibility and collects
	// field-level validation errors from ExitCode.
	InvalidResultError struct {
		FieldErrors []error
	}

	// InvalidDryRunDataError is returned when a DryRunData has invalid fields.
	// It wraps ErrInvalidDryRunData for errors.Is() compatibility and collects
	// field-level validation errors from SourceID and Selection.
	InvalidDryRunDataError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidRequestError.
func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid request: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidRequest for errors.Is() compatibility.
func (e *InvalidRequestError) Unwrap() error { return ErrInvalidRequest }

// Error implements the error interface for InvalidResultError.
func (e *InvalidResultError) Error() string {
	return fmt.Sprintf("invalid commandsvc result: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidCommandsvcResult for errors.Is() compatibility.
func (e *InvalidResultError) Unwrap() error { return ErrInvalidCommandsvcResult }

// Error implements the error interface for InvalidDryRunDataError.
func (e *InvalidDryRunDataError) Error() string {
	return fmt.Sprintf("invalid dry run data: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidDryRunData for errors.Is() compatibility.
func (e *InvalidDryRunDataError) Unwrap() error { return ErrInvalidDryRunData }

// Validate returns nil if the Request has valid fields, or a validation error if not.
// It validates Runtime (when non-empty), FromSource (when non-empty),
// Workdir (when non-empty), EnvFiles, ConfigPath (when non-empty),
// EnvInheritMode (when non-empty), EnvInheritAllow, and EnvInheritDeny.
func (r Request) Validate() error {
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
	if len(errs) > 0 {
		return &InvalidRequestError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the Result has valid fields, or a validation error if not.
// It delegates to ExitCode.Validate().
func (r Result) Validate() error {
	if err := r.ExitCode.Validate(); err != nil {
		return &InvalidResultError{FieldErrors: []error{err}}
	}
	return nil
}

// Validate returns nil if the DryRunData has valid fields, or a validation error if not.
// It validates SourceID (when non-empty) and delegates to Selection.Validate().
func (d DryRunData) Validate() error {
	var errs []error
	if d.SourceID != "" {
		if err := d.SourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := d.Selection.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidDryRunDataError{FieldErrors: errs}
	}
	return nil
}
