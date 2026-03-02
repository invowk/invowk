// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"errors"
	"fmt"
)

// ErrInvalidBuildExecutionContextOptions is the sentinel error wrapped by
// InvalidBuildExecutionContextOptionsError.
var ErrInvalidBuildExecutionContextOptions = errors.New("invalid build execution context options")

// InvalidBuildExecutionContextOptionsError is returned when BuildExecutionContextOptions
// has invalid fields. It wraps ErrInvalidBuildExecutionContextOptions for errors.Is()
// compatibility and collects field-level validation errors.
type InvalidBuildExecutionContextOptionsError struct {
	FieldErrors []error
}

// Error implements the error interface for InvalidBuildExecutionContextOptionsError.
func (e *InvalidBuildExecutionContextOptionsError) Error() string {
	return fmt.Sprintf("invalid build execution context options: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidBuildExecutionContextOptions for errors.Is() compatibility.
func (e *InvalidBuildExecutionContextOptionsError) Unwrap() error {
	return ErrInvalidBuildExecutionContextOptions
}

// Validate returns nil if the BuildExecutionContextOptions has valid fields,
// or a validation error if not.
// It validates Selection, Workdir (when non-empty), EnvFiles, EnvInheritMode
// (when non-empty), EnvInheritAllow, EnvInheritDeny, SourceID (when non-empty),
// and Platform (when non-empty).
func (o BuildExecutionContextOptions) Validate() error {
	var errs []error
	if err := o.Selection.Validate(); err != nil {
		errs = append(errs, err)
	}
	if o.Workdir != "" {
		if err := o.Workdir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, f := range o.EnvFiles {
		if err := f.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.EnvInheritMode != "" {
		if err := o.EnvInheritMode.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, name := range o.EnvInheritAllow {
		if err := name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, name := range o.EnvInheritDeny {
		if err := name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.SourceID != "" {
		if err := o.SourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.Platform != "" {
		if err := o.Platform.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidBuildExecutionContextOptionsError{FieldErrors: errs}
	}
	return nil
}
