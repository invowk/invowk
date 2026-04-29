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
// SCOPE ENFORCEMENT BOUNDARY: CanCall() is a static analysis gate enforced at
// depends_on.cmds declaration validation time (via CheckCommandDependenciesExist),
// NOT a runtime subprocess interceptor. If a module script dynamically invokes
// `invowk cmd <forbidden-command>`, the scope check is not triggered because
// the subprocess spawns a new CLI process outside the validation pipeline.
// For execution isolation, use the container runtime.
//
//goplint:mutable
//goplint:validate-all
//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
type CommandScope struct {
	// ModuleID is the module identifier that owns this scope
	ModuleID ModuleID `json:"-"`
	// ModuleSourceID is the command namespace for the module that owns this scope.
	ModuleSourceID ModuleSourceID `json:"-"`
	// GlobalModules are stable module IDs from globally installed modules.
	GlobalModules map[ModuleID]bool `json:"-"`
	// GlobalSources are command namespaces from globally installed modules.
	GlobalSources map[ModuleSourceID]bool `json:"-"`
	// DirectDeps are stable module IDs from first-level requirements.
	DirectDeps map[ModuleID]bool `json:"-"`
	// DirectSources are command namespaces from first-level requirements.
	DirectSources map[ModuleSourceID]bool `json:"-"`
}

// NewCommandScope creates a CommandScope for a parsed module.
// globalModuleIDs should contain module IDs from ~/.invowk/modules/
// directRequirements should be the requires list from the module's invowkmod.cue
func NewCommandScope(moduleID ModuleID, globalModuleIDs []ModuleID, directRequirements []ModuleRequirement) *CommandScope {
	scope := &CommandScope{
		ModuleID:       moduleID,
		ModuleSourceID: ModuleSourceID(moduleID),
		GlobalModules:  make(map[ModuleID]bool),
		GlobalSources:  make(map[ModuleSourceID]bool),
		DirectDeps:     make(map[ModuleID]bool),
		DirectSources:  make(map[ModuleSourceID]bool),
	}

	for _, id := range globalModuleIDs {
		scope.GlobalModules[id] = true
		scope.GlobalSources[ModuleSourceID(id)] = true
	}

	for _, req := range directRequirements {
		ref := ModuleRef(req)
		if req.Alias != "" {
			scope.DirectSources[ModuleSourceID(req.Alias)] = true
			continue
		}
		if sourceID := ref.DefaultSourceID(); sourceID != "" {
			scope.DirectSources[sourceID] = true
		}
	}

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
// Returns true if allowed, false with reason if not.
func (s *CommandScope) CanCall(targetCmd string) (allowed bool, reason string) {
	// Extract module prefix from command name (format: "module.name cmdname" or "module.name@version cmdname")
	targetSource := ModuleSourceID(ExtractModuleFromCommand(targetCmd)) //goplint:ignore -- used only for equality comparison

	// If no module prefix, it's a local command (always allowed)
	if targetSource == "" {
		return true, ""
	}

	// Check if target is from same module
	if targetSource == s.ModuleSourceID || ModuleID(targetSource) == s.ModuleID {
		return true, ""
	}

	// Check if target is in global modules
	if s.GlobalSources[targetSource] || s.GlobalModules[ModuleID(targetSource)] {
		return true, ""
	}

	// Check if target is in direct dependencies
	if s.DirectSources[targetSource] || s.DirectDeps[ModuleID(targetSource)] {
		return true, ""
	}

	return false, fmt.Sprintf(
		"command from module '%s' cannot call '%s': module '%s' is not accessible\n"+
			"  Commands can only call:\n"+
			"  - Commands from the same module (%s)\n"+
			"  - Commands from globally installed modules (~/.invowk/modules/)\n"+
			"  - Commands from direct dependencies declared in invowkmod.cue:requires\n"+
			"  Add '%s' to your invowkmod.cue requires list to use its commands",
		s.ModuleID, targetCmd, targetSource, s.ModuleID, targetSource)
}

// AddDirectDep adds a resolved direct dependency to the scope.
// This is called during resolution when we know the actual module ID.
func (s *CommandScope) AddDirectDep(moduleID ModuleID) {
	if s.DirectDeps == nil {
		s.DirectDeps = make(map[ModuleID]bool)
	}
	s.DirectDeps[moduleID] = true
}

// AddDirectSource adds a command namespace for a resolved direct dependency.
func (s *CommandScope) AddDirectSource(sourceID ModuleSourceID) {
	if s.DirectSources == nil {
		s.DirectSources = make(map[ModuleSourceID]bool)
	}
	s.DirectSources[sourceID] = true
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
