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

	// ErrInvalidModuleDirectoryName is returned when a ModuleDirectoryName value
	// does not match the required format.
	ErrInvalidModuleDirectoryName = errors.New("invalid module directory name")

	// moduleShortNamePattern validates the ModuleShortName format: starts with a letter,
	// followed by letters, digits, dots, underscores, or hyphens.
	// This supports source namespace derivation from repository and subdirectory basenames.
	moduleShortNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]*$`)

	// moduleDirectoryNamePattern validates the .invowkmod folder prefix format.
	moduleDirectoryNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)
)

type (
	// ModuleShortName represents source namespace material derived from module
	// paths, repository basenames, or monorepo subdirectories.
	ModuleShortName string

	// ModuleDirectoryName represents a real .invowkmod directory name without the suffix.
	ModuleDirectoryName string

	// InvalidModuleShortNameError is returned when a ModuleShortName value does not match
	// the required format. It wraps ErrInvalidModuleShortName for errors.Is() compatibility.
	InvalidModuleShortNameError struct {
		Value ModuleShortName
	}

	// InvalidModuleDirectoryNameError is returned when a ModuleDirectoryName value
	// does not match the required format. It wraps ErrInvalidModuleDirectoryName.
	InvalidModuleDirectoryNameError struct {
		Value ModuleDirectoryName
	}
)

// String returns the string representation of the ModuleShortName.
func (n ModuleShortName) String() string { return string(n) }

// String returns the string representation of the ModuleDirectoryName.
func (n ModuleDirectoryName) String() string { return string(n) }

//goplint:nonzero

// Validate returns nil if the ModuleShortName matches the expected format (non-empty,
// starts with a letter, contains only letters, digits, dots, underscores, or hyphens),
// or an error describing the validation failure.
func (n ModuleShortName) Validate() error {
	if !moduleShortNamePattern.MatchString(string(n)) {
		return &InvalidModuleShortNameError{Value: n}
	}
	return nil
}

//goplint:nonzero

// Validate returns nil if the ModuleDirectoryName matches the .invowkmod folder prefix format.
func (n ModuleDirectoryName) Validate() error {
	if !moduleDirectoryNamePattern.MatchString(string(n)) {
		return &InvalidModuleDirectoryNameError{Value: n}
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

// Error implements the error interface for InvalidModuleDirectoryNameError.
func (e *InvalidModuleDirectoryNameError) Error() string {
	return fmt.Sprintf(
		"invalid module directory name %q: must start with a letter and contain only alphanumeric characters, with optional dot-separated segments",
		string(e.Value),
	)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidModuleDirectoryNameError) Unwrap() error {
	return ErrInvalidModuleDirectoryName
}
