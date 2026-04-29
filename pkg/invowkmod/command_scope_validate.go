// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidCommandScope is the sentinel error wrapped by InvalidCommandScopeError.
var ErrInvalidCommandScope = errors.New("invalid command scope")

type (
	// InvalidCommandScopeError is returned when a CommandScope has invalid fields.
	// It wraps ErrInvalidCommandScope for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidCommandScopeError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidCommandScopeError.
func (e *InvalidCommandScopeError) Error() string {
	return types.FormatFieldErrors("command scope", e.FieldErrors)
}

// Unwrap returns ErrInvalidCommandScope for errors.Is() compatibility.
func (e *InvalidCommandScopeError) Unwrap() error { return ErrInvalidCommandScope }

// Validate returns nil if the CommandScope has valid fields, or an error
// collecting all field-level validation failures.
// ModuleID and ModuleSourceID identify the owning module. Map keys are populated
// post-construction and not validated here.
func (s CommandScope) Validate() error {
	var errs []error
	if err := s.ModuleID.Validate(); err != nil {
		errs = append(errs, err)
	}
	if s.ModuleSourceID != "" {
		if err := s.ModuleSourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	} else if len(errs) == 0 {
		if err := ModuleSourceID(s.ModuleID).Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCommandScopeError{FieldErrors: errs}
	}
	return nil
}
