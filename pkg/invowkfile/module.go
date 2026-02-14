// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"github.com/invowk/invowk/pkg/invowkmod"
)

type (
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
