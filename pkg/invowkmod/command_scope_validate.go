// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
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
	return fmt.Sprintf("invalid command scope: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidCommandScope for errors.Is() compatibility.
func (e *InvalidCommandScopeError) Unwrap() error { return ErrInvalidCommandScope }

// Validate returns nil if the CommandScope has valid fields, or an error
// collecting all field-level validation failures.
// ModuleID is the only validatable field — map keys are ModuleID but are
// populated post-construction and not validated here.
func (s CommandScope) Validate() error {
	if err := s.ModuleID.Validate(); err != nil {
		return &InvalidCommandScopeError{FieldErrors: []error{err}}
	}
	return nil
}
