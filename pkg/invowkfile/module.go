// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"slices"

	"github.com/invowk/invowk/pkg/invowkmod"
)

// ErrInvalidModuleMetadata is the sentinel error wrapped by InvalidModuleMetadataError.
var ErrInvalidModuleMetadata = errors.New("invalid module metadata")

type (
	//goplint:validate-all
	//
	// ModuleMetadata is a lightweight module metadata snapshot attached to
	// Invowkfile during module parsing/discovery.
	// Fields are unexported for immutability; use Module(), Version(),
	// Description(), and Requires() accessors.
	ModuleMetadata struct {
		module      invowkmod.ModuleID
		version     invowkmod.SemVer
		description DescriptionText
		requires    []ModuleRequirement
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

// NewModuleMetadata creates a validated ModuleMetadata snapshot.
// Module and Version are required; Description and Requires are optional
// (zero values are valid). The requires slice is defensively copied.
func NewModuleMetadata(module invowkmod.ModuleID, version invowkmod.SemVer, description DescriptionText, requires []ModuleRequirement) (*ModuleMetadata, error) {
	m := &ModuleMetadata{
		module:      module,
		version:     version,
		description: description,
	}
	if len(requires) > 0 {
		m.requires = make([]ModuleRequirement, len(requires))
		copy(m.requires, requires)
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// NewModuleMetadataFromInvowkmod converts invowkmod metadata to the lightweight
// invowkfile-local metadata shape. This is a non-validating factory used during
// CUE parsing where the metadata may be in an intermediate state.
func NewModuleMetadataFromInvowkmod(meta *Invowkmod) *ModuleMetadata {
	if meta == nil {
		return nil
	}

	requires := make([]ModuleRequirement, len(meta.Requires))
	copy(requires, meta.Requires)

	return &ModuleMetadata{
		module:      meta.Module,
		version:     meta.Version,
		description: meta.Description,
		requires:    requires,
	}
}

// Module returns the module identifier.
func (m ModuleMetadata) Module() invowkmod.ModuleID { return m.module }

// Version returns the module version.
func (m ModuleMetadata) Version() invowkmod.SemVer { return m.version }

// Description returns the module description text.
func (m ModuleMetadata) Description() DescriptionText { return m.description }

// Requires returns a copy of the module requirements slice.
// The returned slice is a defensive copy — callers cannot mutate the original.
func (m ModuleMetadata) Requires() []ModuleRequirement {
	return slices.Clone(m.requires)
}

// Validate returns nil if the ModuleMetadata has valid fields, or a validation error if not.
// It delegates to Module.Validate(), Version.Validate(), and each
// Requires entry's Validate(). Description is validated only when
// non-empty (the zero value is valid).
func (m ModuleMetadata) Validate() error {
	var errs []error
	if err := m.module.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.version.Validate(); err != nil {
		errs = append(errs, err)
	}
	if m.description != "" {
		if err := m.description.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, req := range m.requires {
		if err := req.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidModuleMetadataError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidModuleMetadataError.
func (e *InvalidModuleMetadataError) Error() string {
	return fmt.Sprintf("invalid module metadata: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidModuleMetadata for errors.Is() compatibility.
func (e *InvalidModuleMetadataError) Unwrap() error { return ErrInvalidModuleMetadata }
