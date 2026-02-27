// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrInvalidModuleShortName is returned when a ModuleShortName value does not match
	// the required format.
	ErrInvalidModuleShortName = errors.New("invalid module short name")

	// moduleShortNamePattern validates the ModuleShortName format: starts with a letter,
	// followed by letters, digits, dots, underscores, or hyphens.
	// This matches the naming rules for .invowkmod directory names (the folder name
	// prefix before the .invowkmod suffix). It is intentionally broader than moduleIDPattern
	// (which only allows dot-separated alphanumeric segments) because folder names may
	// contain hyphens and underscores for filesystem convenience.
	moduleShortNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)
)

type (
	// ModuleShortName represents the module folder name without the .invowkmod suffix.
	// For example, "io.invowk.sample" from "io.invowk.sample.invowkmod".
	// Using a named type prevents accidental confusion with ModuleID, SourceID,
	// or other string identifiers.
	ModuleShortName string

	// InvalidModuleShortNameError is returned when a ModuleShortName value does not match
	// the required format. It wraps ErrInvalidModuleShortName for errors.Is() compatibility.
	InvalidModuleShortNameError struct {
		Value ModuleShortName
	}
)

// String returns the string representation of the ModuleShortName.
func (n ModuleShortName) String() string { return string(n) }

//goplint:nonzero

// Validate returns nil if the ModuleShortName matches the expected format (non-empty,
// starts with a letter, contains only letters, digits, dots, underscores, or hyphens),
// or an error describing the validation failure.
func (n ModuleShortName) Validate() error {
	if n == "" || !moduleShortNamePattern.MatchString(string(n)) {
		return &InvalidModuleShortNameError{Value: n}
	}
	return nil
}

// Error implements the error interface for InvalidModuleShortNameError.
func (e *InvalidModuleShortNameError) Error() string {
	return fmt.Sprintf(
		"invalid module short name %q: must start with a letter and contain only letters, digits, dots, underscores, or hyphens",
		string(e.Value),
	)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidModuleShortNameError) Unwrap() error {
	return ErrInvalidModuleShortName
}
