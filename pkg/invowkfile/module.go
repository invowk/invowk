// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"

	"github.com/invowk/invowk/pkg/invowkmod"
)

// ErrInvalidModuleMetadata is the sentinel error wrapped by InvalidModuleMetadataError.
var ErrInvalidModuleMetadata = errors.New("invalid module metadata")

type (
	// ModuleMetadata is a lightweight module metadata snapshot attached to
	// Invowkfile during module parsing/discovery.
	ModuleMetadata struct {
		Module      invowkmod.ModuleID
		Version     invowkmod.SemVer
		Description DescriptionText
		Requires    []ModuleRequirement
	}

	// InvalidModuleMetadataError is returned when a ModuleMetadata has invalid fields.
	// It wraps ErrInvalidModuleMetadata for errors.Is() compatibility and collects
	// field-level validation errors from Module, Version, Description, and Requires.
	InvalidModuleMetadataError struct {
		FieldErrors []error
	}

	// ModuleRequirement represents a dependency on another module from a Git repository.
	// This is the type alias for invowkmod.ModuleRequirement.
	ModuleRequirement = invowkmod.ModuleRequirement

	// Invowkmod represents a loaded module with metadata and optional commands.
	// This is the type alias for invowkmod.Invowkmod.
	// Use ParseModule() to load a module with both metadata and commands.
	Invowkmod = invowkmod.Invowkmod

	// CommandScope defines what commands a module can access.
	// This is a type alias for invowkmod.CommandScope.
	CommandScope = invowkmod.CommandScope
)

// NewCommandScope creates a CommandScope for a parsed module.
// This is a wrapper for invowkmod.NewCommandScope that accepts plain strings
// and converts to ModuleID at the boundary.
func NewCommandScope(moduleID string, globalModuleIDs []string, directRequirements []ModuleRequirement) *CommandScope {
	modID := invowkmod.ModuleID(moduleID)
	globalIDs := make([]invowkmod.ModuleID, len(globalModuleIDs))
	for i, id := range globalModuleIDs {
		globalIDs[i] = invowkmod.ModuleID(id)
	}
	return invowkmod.NewCommandScope(modID, globalIDs, directRequirements)
}

// ExtractModuleFromCommand extracts the module prefix from a fully qualified command name.
// This is a wrapper for invowkmod.ExtractModuleFromCommand.
func ExtractModuleFromCommand(cmd string) string {
	return invowkmod.ExtractModuleFromCommand(cmd)
}

// NewModuleMetadataFromInvowkmod converts invowkmod metadata to the lightweight
// invowkfile-local metadata shape.
func NewModuleMetadataFromInvowkmod(meta *Invowkmod) *ModuleMetadata {
	if meta == nil {
		return nil
	}

	requires := make([]ModuleRequirement, len(meta.Requires))
	copy(requires, meta.Requires)

	return &ModuleMetadata{
		Module:      meta.Module,
		Version:     meta.Version,
		Description: meta.Description,
		Requires:    requires,
	}
}

// IsValid returns whether the ModuleMetadata has valid fields.
// It delegates to Module.IsValid(), Version.IsValid(), and each
// Requires entry's IsValid(). Description is validated only when
// non-empty (the zero value is valid).
func (m ModuleMetadata) IsValid() (bool, []error) {
	var errs []error
	if valid, fieldErrs := m.Module.IsValid(); !valid {
		errs = append(errs, fieldErrs...)
	}
	if valid, fieldErrs := m.Version.IsValid(); !valid {
		errs = append(errs, fieldErrs...)
	}
	if m.Description != "" {
		if valid, fieldErrs := m.Description.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	for _, req := range m.Requires {
		if valid, fieldErrs := req.IsValid(); !valid {
			errs = append(errs, fieldErrs...)
		}
	}
	if len(errs) > 0 {
		return false, []error{&InvalidModuleMetadataError{FieldErrors: errs}}
	}
	return true, nil
}

// Error implements the error interface for InvalidModuleMetadataError.
func (e *InvalidModuleMetadataError) Error() string {
	return fmt.Sprintf("invalid module metadata: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidModuleMetadata for errors.Is() compatibility.
func (e *InvalidModuleMetadataError) Unwrap() error { return ErrInvalidModuleMetadata }
