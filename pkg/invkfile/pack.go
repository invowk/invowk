// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"invowk-cli/pkg/invkmod"
)

// ModuleRequirement represents a dependency on another module from a Git repository.
// This is the type alias for invkmod.ModuleRequirement.
type ModuleRequirement = invkmod.ModuleRequirement

// Invkmod represents a loaded module with metadata and optional commands.
// This is the type alias for invkmod.Invkmod.
// Use ParseModule() to load a module with both metadata and commands.
type Invkmod = invkmod.Invkmod

// CommandScope defines what commands a module can access.
// This is a type alias for invkmod.CommandScope.
type CommandScope = invkmod.CommandScope

// NewCommandScope creates a CommandScope for a parsed module.
// This is a wrapper for invkmod.NewCommandScope.
func NewCommandScope(moduleID string, globalModuleIDs []string, directRequirements []ModuleRequirement) *CommandScope {
	return invkmod.NewCommandScope(moduleID, globalModuleIDs, directRequirements)
}

// ExtractModuleFromCommand extracts the module prefix from a fully qualified command name.
// This is a wrapper for invkmod.ExtractModuleFromCommand.
func ExtractModuleFromCommand(cmd string) string {
	return invkmod.ExtractModuleFromCommand(cmd)
}
