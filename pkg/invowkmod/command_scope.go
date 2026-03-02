// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"strings"
)

// CommandScope defines what commands a module can access.
// Commands in a module can ONLY call:
//  1. Commands from the same module
//  2. Commands from globally installed modules (~/.invowk/modules/)
//  3. Commands from first-level requirements (direct dependencies in invowkmod.cue:requires)
//
// CommandScope holds the commands visible to a module, populated post-construction
// via AddDirectDep(). Commands CANNOT call transitive dependencies.
//
//goplint:mutable
//goplint:validate-all
//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
type CommandScope struct {
	// ModuleID is the module identifier that owns this scope
	ModuleID ModuleID `json:"-"`
	// GlobalModules are commands from globally installed modules (always accessible)
	GlobalModules map[ModuleID]bool `json:"-"`
	// DirectDeps are module IDs from first-level requirements (from invowkmod.cue:requires)
	DirectDeps map[ModuleID]bool `json:"-"`
}

// NewCommandScope creates a CommandScope for a parsed module.
// globalModuleIDs should contain module IDs from ~/.invowk/modules/
// directRequirements should be the requires list from the module's invowkmod.cue
func NewCommandScope(moduleID ModuleID, globalModuleIDs []ModuleID, directRequirements []ModuleRequirement) *CommandScope {
	scope := &CommandScope{
		ModuleID:      moduleID,
		GlobalModules: make(map[ModuleID]bool),
		DirectDeps:    make(map[ModuleID]bool),
	}

	for _, id := range globalModuleIDs {
		scope.GlobalModules[id] = true
	}

	for _, req := range directRequirements {
		// The direct dependency namespace uses either alias or the resolved module ID
		if req.Alias != "" {
			scope.DirectDeps[ModuleID(req.Alias)] = true
		}
		// Note: The actual resolved module ID will be added during resolution
	}

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
// Returns true if allowed, false with reason if not.
func (s *CommandScope) CanCall(targetCmd string) (allowed bool, reason string) {
	// Extract module prefix from command name (format: "module.name cmdname" or "module.name@version cmdname")
	targetModule := ModuleID(ExtractModuleFromCommand(targetCmd)) //goplint:ignore -- used only for equality comparison

	// If no module prefix, it's a local command (always allowed)
	if targetModule == "" {
		return true, ""
	}

	// Check if target is from same module
	if targetModule == s.ModuleID {
		return true, ""
	}

	// Check if target is in global modules
	if s.GlobalModules[targetModule] {
		return true, ""
	}

	// Check if target is in direct dependencies
	if s.DirectDeps[targetModule] {
		return true, ""
	}

	return false, fmt.Sprintf(
		"command from module '%s' cannot call '%s': module '%s' is not accessible\n"+
			"  Commands can only call:\n"+
			"  - Commands from the same module (%s)\n"+
			"  - Commands from globally installed modules (~/.invowk/modules/)\n"+
			"  - Commands from direct dependencies declared in invowkmod.cue:requires\n"+
			"  Add '%s' to your invowkmod.cue requires list to use its commands",
		s.ModuleID, targetCmd, targetModule, s.ModuleID, targetModule)
}

// AddDirectDep adds a resolved direct dependency to the scope.
// This is called during resolution when we know the actual module ID.
func (s *CommandScope) AddDirectDep(moduleID ModuleID) {
	s.DirectDeps[moduleID] = true
}

// ExtractModuleFromCommand extracts the module prefix from a fully qualified command name.
// Returns empty string if no module prefix found.
// Examples:
//   - "io.invowk.sample hello" -> "io.invowk.sample"
//   - "utils@1.2.3 build" -> "utils@1.2.3"
//   - "build" -> ""
func ExtractModuleFromCommand(cmd string) string {
	// Command format: "module cmdname" where module may contain dots and @version
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) < 2 {
		// No space means it's either a local command or just a module with no command
		return ""
	}
	return parts[0]
}
