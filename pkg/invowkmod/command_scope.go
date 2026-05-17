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
	//  3. Commands from first-level requirements resolved in invowkmod.lock.cue
	//
	// CommandScope holds the commands visible to a module, populated post-construction
	// from discovered command sources. Global command sources are added via
	// AddGlobalSource(), and direct dependency sources are added via
	// AddDirectDependency() after requires and lock metadata agree. Commands
	// CANNOT call transitive dependencies.
	//
	// SCOPE ENFORCEMENT BOUNDARY: CanCallTarget() is a static analysis gate enforced at
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
		// GlobalSources are command namespaces from globally installed modules.
		GlobalSources map[ModuleSourceID]bool `json:"-"`
		// DirectDependencySources maps stable module IDs to the command namespaces
		// they are allowed to publish under after lock-file identity checks.
		DirectDependencySources map[ModuleID]map[ModuleSourceID]bool `json:"-"`
	}

	// CommandReference identifies a command reference used in depends_on.cmds scope checks.
	CommandReference string

	// CommandTarget identifies a discovered target command using its stable
	// discovery identity, not only its rendered command reference.
	CommandTarget struct {
		Reference CommandReference
		SourceID  ModuleSourceID
		ModuleID  ModuleID
	}

	// CommandScopeDenyReason identifies why a command reference is outside scope.
	CommandScopeDenyReason string

	// CommandScopeDecision reports whether a target command is visible from a module scope.
	CommandScopeDecision struct {
		// Allowed reports whether the target command can be called.
		Allowed bool
		// TargetCommand is the original target command reference.
		TargetCommand CommandReference
		// TargetSource is the command namespace reported by discovery.
		TargetSource ModuleSourceID
		// TargetModuleID is the stable module ID reported by discovery.
		TargetModuleID ModuleID
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

// Validate returns nil if the target has a valid command reference and any
// supplied discovery identity fields are valid.
func (t CommandTarget) Validate() error {
	var errs []error
	if err := t.Reference.Validate(); err != nil {
		errs = append(errs, err)
	}
	if t.SourceID != "" {
		if err := t.SourceID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if t.ModuleID != "" {
		if err := t.ModuleID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
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
	if d.TargetModuleID != "" {
		if err := d.TargetModuleID.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := d.Reason.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// NewCommandScope creates a CommandScope for a parsed module.
// Direct dependency sources and global command sources are added only after
// discovery metadata identifies the actual command namespace.
func NewCommandScope(moduleID ModuleID) *CommandScope {
	return &CommandScope{
		ModuleID:                moduleID,
		ModuleSourceID:          ModuleSourceID(moduleID),
		GlobalSources:           make(map[ModuleSourceID]bool),
		DirectDependencySources: make(map[ModuleID]map[ModuleSourceID]bool),
	}
}

// CanCallTarget checks if a discovered command target is visible from this scope.
func (s *CommandScope) CanCallTarget(target CommandTarget) CommandScopeDecision {
	targetSource := targetDecisionSource(target)
	decision := CommandScopeDecision{
		TargetCommand:  target.Reference,
		TargetSource:   targetSource,
		TargetModuleID: target.ModuleID,
	}
	if err := target.Validate(); err != nil {
		decision.Reason = CommandScopeDenyInaccessible
		return decision
	}

	// If discovery did not attach module identity, the target is a local/root command.
	if target.SourceID == "" && target.ModuleID == "" {
		decision.Allowed = true
		return decision
	}

	// Check if target is from same module. Discovered targets must prove same-module
	// identity via the stable module ID, not only via a command-source alias.
	if target.ModuleID == s.ModuleID {
		decision.Allowed = true
		return decision
	}

	// Check if target is in global modules.
	if s.targetIsGlobal(target) {
		decision.Allowed = true
		return decision
	}

	// Check if target is in direct dependencies.
	if s.targetIsDirectDependency(target) {
		decision.Allowed = true
		return decision
	}

	decision.Reason = CommandScopeDenyInaccessible
	return decision
}

// AddGlobalSource adds a discovered global command source to the scope.
func (s *CommandScope) AddGlobalSource(sourceID ModuleSourceID) {
	if s.GlobalSources == nil {
		s.GlobalSources = make(map[ModuleSourceID]bool)
	}
	s.GlobalSources[sourceID] = true
}

// AddDirectDependency adds a resolved direct dependency identity/source pair
// to the scope.
func (s *CommandScope) AddDirectDependency(moduleID ModuleID, sourceID ModuleSourceID) {
	if s.DirectDependencySources == nil {
		s.DirectDependencySources = make(map[ModuleID]map[ModuleSourceID]bool)
	}
	if s.DirectDependencySources[moduleID] == nil {
		s.DirectDependencySources[moduleID] = make(map[ModuleSourceID]bool)
	}
	s.DirectDependencySources[moduleID][sourceID] = true
}

func targetDecisionSource(target CommandTarget) ModuleSourceID {
	if target.SourceID != "" {
		return target.SourceID
	}
	if target.ModuleID != "" {
		return ModuleSourceID(target.ModuleID) //goplint:ignore -- fallback only for diagnostics when source ID is unavailable.
	}
	return ""
}

func (s *CommandScope) targetIsGlobal(target CommandTarget) bool {
	return target.SourceID != "" && s.GlobalSources[target.SourceID]
}

func (s *CommandScope) targetIsDirectDependency(target CommandTarget) bool {
	if target.ModuleID != "" && target.SourceID != "" {
		return s.DirectDependencySources[target.ModuleID][target.SourceID]
	}
	return false
}
