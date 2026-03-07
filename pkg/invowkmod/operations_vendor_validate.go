// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"

	"github.com/invowk/invowk/pkg/types"
)

var (
	// ErrInvalidVendoredEntry is the sentinel error wrapped by InvalidVendoredEntryError.
	ErrInvalidVendoredEntry = errors.New("invalid vendored entry")
	// ErrInvalidVendorResult is the sentinel error wrapped by InvalidVendorResultError.
	ErrInvalidVendorResult = errors.New("invalid vendor result")
)

type (
	// InvalidVendoredEntryError is returned when a VendoredEntry has invalid fields.
	// It wraps ErrInvalidVendoredEntry for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidVendoredEntryError struct {
		FieldErrors []error
	}

	// InvalidVendorResultError is returned when a VendorResult has invalid fields.
	// It wraps ErrInvalidVendorResult for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidVendorResultError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidVendoredEntryError.
func (e *InvalidVendoredEntryError) Error() string {
	return types.FormatFieldErrors("vendored entry", e.FieldErrors)
}

// Unwrap returns ErrInvalidVendoredEntry for errors.Is() compatibility.
func (e *InvalidVendoredEntryError) Unwrap() error { return ErrInvalidVendoredEntry }

// Validate returns nil if the VendoredEntry has valid fields, or an error
// collecting all field-level validation failures.
// Namespace is validated when non-empty. SourcePath and VendorPath are validated
// when non-empty (zero values are valid).
func (v VendoredEntry) Validate() error {
	var errs []error
	if v.Namespace != "" {
		if err := v.Namespace.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if v.SourcePath != "" {
		if err := v.SourcePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if v.VendorPath != "" {
		if err := v.VendorPath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidVendoredEntryError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidVendorResultError.
func (e *InvalidVendorResultError) Error() string {
	return types.FormatFieldErrors("vendor result", e.FieldErrors)
}

// Unwrap returns ErrInvalidVendorResult for errors.Is() compatibility.
func (e *InvalidVendorResultError) Unwrap() error { return ErrInvalidVendorResult }

// Validate returns nil if the VendorResult has valid fields, or an error
// collecting all field-level validation failures.
// VendorDir is validated when non-empty. Each Vendored entry is validated
// via delegation.
func (r VendorResult) Validate() error {
	var errs []error
	if r.VendorDir != "" {
		if err := r.VendorDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, entry := range r.Vendored {
		if err := entry.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidVendorResultError{FieldErrors: errs}
	}
	return nil
}
