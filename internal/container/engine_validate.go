// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidRunResult is the sentinel error wrapped by InvalidRunResultError.
	ErrInvalidRunResult = errors.New("invalid run result")

	// ErrInvalidCreateResult is the sentinel error wrapped by InvalidCreateResultError.
	ErrInvalidCreateResult = errors.New("invalid create result")

	// ErrInvalidContainerInfo is the sentinel error wrapped by InvalidContainerInfoError.
	ErrInvalidContainerInfo = errors.New("invalid container info")
)

type (
	// InvalidRunResultError is returned when a RunResult has invalid fields.
	// It wraps ErrInvalidRunResult for errors.Is() compatibility and collects
	// field-level validation errors from ContainerID and ExitCode.
	InvalidRunResultError struct {
		FieldErrors []error
	}

	// InvalidCreateResultError is returned when a CreateResult has invalid fields.
	// It wraps ErrInvalidCreateResult for errors.Is() compatibility.
	InvalidCreateResultError struct {
		FieldErrors []error
	}

	// InvalidContainerInfoError is returned when a ContainerInfo has invalid fields.
	// It wraps ErrInvalidContainerInfo for errors.Is() compatibility.
	InvalidContainerInfoError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidRunResultError.
func (e *InvalidRunResultError) Error() string {
	return types.FormatFieldErrors("run result", e.FieldErrors)
}

// Unwrap returns ErrInvalidRunResult for errors.Is() compatibility.
func (e *InvalidRunResultError) Unwrap() error { return ErrInvalidRunResult }

// Error implements the error interface for InvalidCreateResultError.
func (e *InvalidCreateResultError) Error() string {
	return types.FormatFieldErrors("create result", e.FieldErrors)
}

// Unwrap returns ErrInvalidCreateResult for errors.Is() compatibility.
func (e *InvalidCreateResultError) Unwrap() error { return ErrInvalidCreateResult }

// Error implements the error interface for InvalidContainerInfoError.
func (e *InvalidContainerInfoError) Error() string {
	return types.FormatFieldErrors("container info", e.FieldErrors)
}

// Unwrap returns ErrInvalidContainerInfo for errors.Is() compatibility.
func (e *InvalidContainerInfoError) Unwrap() error { return ErrInvalidContainerInfo }

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

// Validate returns nil if the CreateResult has valid fields.
func (r CreateResult) Validate() error {
	if err := r.ContainerID.Validate(); err != nil {
		return &InvalidCreateResultError{FieldErrors: []error{err}}
	}
	return nil
}

// Validate returns nil if the ContainerInfo has valid fields.
func (i ContainerInfo) Validate() error {
	var errs []error
	if err := i.ContainerID.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := i.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidContainerInfoError{FieldErrors: errs}
	}
	return nil
}
