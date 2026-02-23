// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"github.com/invowk/invowk/pkg/invowkmod"
)

type (
	// ModuleMetadata is a lightweight module metadata snapshot attached to
	// Invowkfile during module parsing/discovery.
	ModuleMetadata struct {
		Module      invowkmod.ModuleID
		Version     invowkmod.SemVer
		Description DescriptionText
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
