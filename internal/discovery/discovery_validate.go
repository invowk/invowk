// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidCommandInfo is the sentinel error wrapped by InvalidCommandInfoError.
	ErrInvalidCommandInfo = errors.New("invalid command info")

	// ErrInvalidDiscoveredFile is the sentinel error wrapped by InvalidDiscoveredFileError.
	ErrInvalidDiscoveredFile = errors.New("invalid discovered file")

	// ErrInvalidLookupResult is the sentinel error wrapped by InvalidLookupResultError.
	ErrInvalidLookupResult = errors.New("invalid lookup result")
)

type (
	// InvalidCommandInfoError is returned when a CommandInfo has invalid fields.
	// It wraps ErrInvalidCommandInfo for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidCommandInfoError struct {
		FieldErrors []error
	}

	// InvalidDiscoveredFileError is returned when a DiscoveredFile has invalid fields.
	// It wraps ErrInvalidDiscoveredFile for errors.Is() compatibility and collects
	// field-level validation errors from Path and Source.
	InvalidDiscoveredFileError struct {
		FieldErrors []error
	}

	// InvalidLookupResultError is returned when a LookupResult has invalid fields.
	// It wraps ErrInvalidLookupResult for errors.Is() compatibility and collects
	// field-level validation errors from Command.
	InvalidLookupResultError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidCommandInfoError.
func (e *InvalidCommandInfoError) Error() string {
	return fmt.Sprintf("invalid command info: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidCommandInfo for errors.Is() compatibility.
func (e *InvalidCommandInfoError) Unwrap() error { return ErrInvalidCommandInfo }

// Error implements the error interface for InvalidDiscoveredFileError.
func (e *InvalidDiscoveredFileError) Error() string {
	return fmt.Sprintf("invalid discovered file: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidDiscoveredFile for errors.Is() compatibility.
func (e *InvalidDiscoveredFileError) Unwrap() error { return ErrInvalidDiscoveredFile }

// Error implements the error interface for InvalidLookupResultError.
func (e *InvalidLookupResultError) Error() string {
	return fmt.Sprintf("invalid lookup result: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidLookupResult for errors.Is() compatibility.
func (e *InvalidLookupResultError) Unwrap() error { return ErrInvalidLookupResult }

// Validate returns nil if the CommandInfo has valid fields, or a validation error if not.
// It validates Name (when non-empty), Description (when non-empty), Source,
// FilePath (when non-empty), SimpleName (when non-empty), SourceID (when non-empty),
// and ModuleID (when non-nil).
// Many fields are zero-value-valid since CommandInfo is populated incrementally.
func (c CommandInfo) Validate() error {
	var errs []error
	if c.Name != "" {
		if err := c.Name.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.Description != "" {
		if err := c.Description.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.Source.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.FilePath != "" {
		if err := c.FilePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.SimpleName != "" {
		if err := c.SimpleName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.SourceID != "" {
		if err := c.SourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.ModuleID != nil {
		if err := c.ModuleID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCommandInfoError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the DiscoveredFile has valid fields, or a validation error if not.
// It validates Path (when non-empty) and Source.
func (d DiscoveredFile) Validate() error {
	var errs []error
	if d.Path != "" {
		if err := d.Path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := d.Source.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidDiscoveredFileError{FieldErrors: errs}
	}
	return nil
}

// Validate returns nil if the LookupResult has valid fields, or a validation error if not.
// It delegates to Command.Validate() when Command is non-nil.
func (r LookupResult) Validate() error {
	var errs []error
	if r.Command != nil {
		if err := r.Command.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidLookupResultError{FieldErrors: errs}
	}
	return nil
}
