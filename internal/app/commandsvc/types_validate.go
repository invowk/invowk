// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
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
	// field-level validation errors from Plan.
	InvalidDryRunDataError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidRequestError.
func (e *InvalidRequestError) Error() string {
	return types.FormatFieldErrors("request", e.FieldErrors)
}

// Unwrap returns ErrInvalidRequest for errors.Is() compatibility.
func (e *InvalidRequestError) Unwrap() error { return ErrInvalidRequest }

// Error implements the error interface for InvalidResultError.
func (e *InvalidResultError) Error() string {
	return types.FormatFieldErrors("commandsvc result", e.FieldErrors)
}

// Unwrap returns ErrInvalidCommandsvcResult for errors.Is() compatibility.
func (e *InvalidResultError) Unwrap() error { return ErrInvalidCommandsvcResult }

// Error implements the error interface for InvalidDryRunDataError.
func (e *InvalidDryRunDataError) Error() string {
	return types.FormatFieldErrors("dry run data", e.FieldErrors)
}

// Unwrap returns ErrInvalidDryRunData for errors.Is() compatibility.
func (e *InvalidDryRunDataError) Unwrap() error { return ErrInvalidDryRunData }

// Validate returns nil if the Request has valid fields, or a validation error if not.
// It validates Runtime (when non-empty), FromSource (when non-empty),
// ContainerName (when non-empty), Workdir (when non-empty), EnvFiles,
// ConfigPath (when non-empty), EnvInheritMode (when non-empty),
// EnvInheritAllow, EnvInheritDeny, and ResolvedCommand (when non-nil).
func (r Request) Validate() error {
	var errs []error
	r.appendLocationValidationErrors(&errs)
	r.appendEnvValidationErrors(&errs)
	r.appendResolvedCommandValidationErrors(&errs)
	if len(errs) > 0 {
		return &InvalidRequestError{FieldErrors: errs}
	}
	return nil
}

func (r Request) appendLocationValidationErrors(errs *[]error) {
	r.appendRuntimeLocationValidationErrors(errs)
	r.appendFilesystemLocationValidationErrors(errs)
}

func (r Request) appendRuntimeLocationValidationErrors(errs *[]error) {
	if r.Runtime != "" {
		if err := r.Runtime.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if r.FromSource != "" {
		if err := r.FromSource.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if r.Platform != "" {
		if err := r.Platform.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}

func (r Request) appendFilesystemLocationValidationErrors(errs *[]error) {
	if r.ContainerName != "" {
		if err := r.ContainerName.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if r.Workdir != "" {
		if err := r.Workdir.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if r.ConfigPath != "" {
		if err := r.ConfigPath.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}

func (r Request) appendEnvValidationErrors(errs *[]error) {
	for _, f := range r.EnvFiles {
		if err := f.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if r.EnvInheritMode != "" {
		if err := r.EnvInheritMode.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	for _, name := range r.EnvInheritAllow {
		if err := name.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	for _, name := range r.EnvInheritDeny {
		if err := name.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}

func (r Request) appendResolvedCommandValidationErrors(errs *[]error) {
	if r.ResolvedCommand != nil {
		if err := r.ResolvedCommand.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}

// Validate returns nil if the Result has valid fields, or a validation error if not.
// It delegates to ExitCode.Validate().
func (r Result) Validate() error {
	if err := r.ExitCode.Validate(); err != nil {
		return &InvalidResultError{FieldErrors: []error{err}}
	}
	return nil
}

// NewDryRunPlan creates a validated dry-run plan.
func NewDryRunPlan(plan DryRunPlan) (DryRunPlan, error) {
	if err := plan.Validate(); err != nil {
		return DryRunPlan{}, err
	}
	return plan, nil
}

// Validate returns nil if the DryRunData has valid fields, or a validation error if not.
// It delegates to Plan.Validate().
func (d DryRunData) Validate() error {
	return d.Plan.Validate()
}

// Validate returns nil if the DryRunPlan has valid fields, or a validation error if not.
func (p DryRunPlan) Validate() error {
	var errs []error
	if p.Runtime == "" {
		errs = append(errs, errors.New("runtime is required"))
	}
	if p.CommandName != "" {
		if err := p.CommandName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.SourceID != "" {
		if err := p.SourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.Runtime != "" {
		if err := p.Runtime.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.Platform != "" {
		if err := p.Platform.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.WorkDir != "" {
		if err := p.WorkDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := p.Timeout.Validate(); err != nil {
		errs = append(errs, err)
	}
	if p.PersistentContainerName != "" {
		if err := p.PersistentContainerName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := p.Script.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidDryRunDataError{FieldErrors: errs}
	}
	return nil
}
