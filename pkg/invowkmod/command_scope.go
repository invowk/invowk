// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// CommandScopeDenyInaccessible means the target module is not local,
	// global, or a direct dependency.
	CommandScopeDenyInaccessible CommandScopeDenyReason = "inaccessible"
)

var (
	// ErrInvalidCommandReference is returned when a command reference is empty.
	ErrInvalidCommandReference = errors.New("invalid command reference")
	// ErrInvalidCommandScopeDenyReason is returned when a denial reason is not recognized.
	ErrInvalidCommandScopeDenyReason = errors.New("invalid command scope deny reason")
	// ErrInvalidCommandScopeDecision is returned when a scope decision is internally inconsistent.
	ErrInvalidCommandScopeDecision = errors.New("invalid command scope decision")
)

type (
	// CommandScope defines what commands a module can access.
	// Commands in a module can ONLY call:
	//  1. Commands from the same module
	//  2. Commands from globally installed user command modules (~/.invowk/cmds/)
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
	CommandScope struct {
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

	// CommandReference identifies a command reference used in depends_on.cmds scope checks.
	CommandReference string

	// CommandScopeDenyReason identifies why a command reference is outside scope.
	CommandScopeDenyReason string

	// CommandScopeDecision reports whether a target command is visible from a module scope.
	CommandScopeDecision struct {
		// Allowed reports whether the target command can be called.
		Allowed bool
		// TargetCommand is the original target command reference.
		TargetCommand CommandReference
		// TargetSource is the command namespace extracted from TargetCommand.
		TargetSource ModuleSourceID
		// Reason classifies the denial when Allowed is false.
		Reason CommandScopeDenyReason
	}
)

// Validate returns nil if the command reference is non-empty and not whitespace-only.
func (r CommandReference) Validate() error {
	if strings.TrimSpace(string(r)) == "" {
		return ErrInvalidCommandReference
	}
	return nil
}

// String returns the string representation of the command reference.
func (r CommandReference) String() string {
	return string(r)
}

// Validate returns nil if the denial reason is recognized.
func (r CommandScopeDenyReason) Validate() error {
	switch r {
	case CommandScopeDenyInaccessible:
		return nil
	default:
		return ErrInvalidCommandScopeDenyReason
	}
}

// String returns the string representation of the denial reason.
func (r CommandScopeDenyReason) String() string {
	return string(r)
}

// Validate returns nil if the decision is internally consistent.
func (d CommandScopeDecision) Validate() error {
	var errs []error
	if err := d.TargetCommand.Validate(); err != nil {
		errs = append(errs, err)
	}
	if d.Allowed {
		if d.Reason != "" {
			errs = append(errs, fmt.Errorf("%w: allowed decision has denial reason %q", ErrInvalidCommandScopeDecision, d.Reason))
		}
		return errors.Join(errs...)
	}
	if d.TargetSource == "" {
		errs = append(errs, fmt.Errorf("%w: denied decision has empty target source", ErrInvalidCommandScopeDecision))
	} else if err := d.TargetSource.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.Reason.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// NewCommandScope creates a CommandScope for a parsed module.
// globalModuleIDs should contain module IDs from ~/.invowk/modules/
// directRequirements is accepted for API compatibility. Direct dependency
// sources are added only after discovery and lock-file identity checks.
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

	_ = directRequirements

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
func (s *CommandScope) CanCall(targetCmd CommandReference) CommandScopeDecision {
	// Extract module prefix from command name (format: "module.name cmdname" or "module.name@version cmdname")
	targetSource := ModuleSourceID(ExtractModuleFromCommand(string(targetCmd))) //goplint:ignore -- used only for equality comparison

	// If no module prefix, it's a local command (always allowed)
	if targetSource == "" {
		return CommandScopeDecision{Allowed: true, TargetCommand: targetCmd}
	}

	// Check if target is from same module
	if targetSource == s.ModuleSourceID || ModuleID(targetSource) == s.ModuleID {
		return CommandScopeDecision{Allowed: true, TargetCommand: targetCmd, TargetSource: targetSource}
	}

	// Check if target is in global modules
	if s.GlobalSources[targetSource] || s.GlobalModules[ModuleID(targetSource)] {
		return CommandScopeDecision{Allowed: true, TargetCommand: targetCmd, TargetSource: targetSource}
	}

	// Check if target is in direct dependencies
	if s.DirectSources[targetSource] || s.DirectDeps[ModuleID(targetSource)] {
		return CommandScopeDecision{Allowed: true, TargetCommand: targetCmd, TargetSource: targetSource}
	}

	return CommandScopeDecision{
		Allowed:       false,
		TargetCommand: targetCmd,
		TargetSource:  targetSource,
		Reason:        CommandScopeDenyInaccessible,
	}
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
