// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"github.com/invowk/invowk/pkg/invowkmod"
)

type (
	// ModuleMetadata is a lightweight module metadata snapshot attached to
	// Invowkfile during module parsing/discovery.
	ModuleMetadata struct {
		Module      string
		Version     string
		Description string
		Requires    []ModuleRequirement
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
// This is a wrapper for invowkmod.NewCommandScope.
func NewCommandScope(moduleID string, globalModuleIDs []string, directRequirements []ModuleRequirement) *CommandScope {
	return invowkmod.NewCommandScope(moduleID, globalModuleIDs, directRequirements)
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
