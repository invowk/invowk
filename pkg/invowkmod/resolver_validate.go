// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidModuleRef is the sentinel error wrapped by InvalidModuleRefError.
	ErrInvalidModuleRef = errors.New("invalid module ref")
	// ErrInvalidResolvedModule is the sentinel error wrapped by InvalidResolvedModuleError.
	ErrInvalidResolvedModule = errors.New("invalid resolved module")
	// ErrInvalidAmbiguousMatch is the sentinel error wrapped by InvalidAmbiguousMatchError.
	ErrInvalidAmbiguousMatch = errors.New("invalid ambiguous match")
	// ErrInvalidRemoveResult is the sentinel error wrapped by InvalidRemoveResultError.
	ErrInvalidRemoveResult = errors.New("invalid remove result")
)

type (
	// InvalidModuleRefError is returned when a ModuleRef has invalid fields.
	// It wraps ErrInvalidModuleRef for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidModuleRefError struct {
		FieldErrors []error
	}

	// InvalidResolvedModuleError is returned when a ResolvedModule has invalid fields.
	// It wraps ErrInvalidResolvedModule for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidResolvedModuleError struct {
		FieldErrors []error
	}

	// InvalidAmbiguousMatchError is returned when an AmbiguousMatch has invalid fields.
	// It wraps ErrInvalidAmbiguousMatch for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidAmbiguousMatchError struct {
		FieldErrors []error
	}

	// InvalidRemoveResultError is returned when a RemoveResult has invalid fields.
	// It wraps ErrInvalidRemoveResult for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidRemoveResultError struct {
		FieldErrors []error
	}
)

// Error implements the error interface for InvalidModuleRefError.
func (e *InvalidModuleRefError) Error() string {
	return fmt.Sprintf("invalid module ref: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidModuleRef for errors.Is() compatibility.
func (e *InvalidModuleRefError) Unwrap() error { return ErrInvalidModuleRef }

// Validate returns nil if the ModuleRef has valid fields, or an error
// collecting all field-level validation failures.
// All fields are zero-value-valid (validated only when non-empty).
func (r ModuleRef) Validate() error {
	var errs []error
	if r.GitURL != "" {
		if err := r.GitURL.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.Version != "" {
		if err := r.Version.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.Alias != "" {
		if err := r.Alias.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.Path != "" {
		if err := r.Path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidModuleRefError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidResolvedModuleError.
func (e *InvalidResolvedModuleError) Error() string {
	return fmt.Sprintf("invalid resolved module: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidResolvedModule for errors.Is() compatibility.
func (e *InvalidResolvedModuleError) Unwrap() error { return ErrInvalidResolvedModule }

// Validate returns nil if the ResolvedModule has valid fields, or an error
// collecting all field-level validation failures.
// ModuleRef is a composite — always validated via delegation.
// Other fields are validated only when non-empty (zero values are valid).
func (r ResolvedModule) Validate() error {
	var errs []error
	if err := r.ModuleRef.Validate(); err != nil {
		errs = append(errs, err)
	}
	if r.ResolvedVersion != "" {
		if err := r.ResolvedVersion.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.GitCommit != "" {
		if err := r.GitCommit.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.CachePath != "" {
		if err := r.CachePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.Namespace != "" {
		if err := r.Namespace.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.ModuleName != "" {
		if err := r.ModuleName.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.ModuleID != "" {
		if err := r.ModuleID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidResolvedModuleError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidAmbiguousMatchError.
func (e *InvalidAmbiguousMatchError) Error() string {
	return fmt.Sprintf("invalid ambiguous match: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidAmbiguousMatch for errors.Is() compatibility.
func (e *InvalidAmbiguousMatchError) Unwrap() error { return ErrInvalidAmbiguousMatch }

// Validate returns nil if the AmbiguousMatch has valid fields, or an error
// collecting all field-level validation failures.
// LockKey is validated when non-empty. Namespace and GitURL are validated
// when non-empty (zero values are valid for optional contexts).
func (m AmbiguousMatch) Validate() error {
	var errs []error
	if m.LockKey != "" {
		if err := m.LockKey.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.Namespace != "" {
		if err := m.Namespace.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.GitURL != "" {
		if err := m.GitURL.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidAmbiguousMatchError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidRemoveResultError.
func (e *InvalidRemoveResultError) Error() string {
	return fmt.Sprintf("invalid remove result: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidRemoveResult for errors.Is() compatibility.
func (e *InvalidRemoveResultError) Unwrap() error { return ErrInvalidRemoveResult }

// Validate returns nil if the RemoveResult has valid fields, or an error
// collecting all field-level validation failures.
// Both LockKey and RemovedEntry are validated via delegation.
func (r RemoveResult) Validate() error {
	var errs []error
	if err := r.LockKey.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.RemovedEntry.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &InvalidRemoveResultError{FieldErrors: errs}
	}
	return nil
}
