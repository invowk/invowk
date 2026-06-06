// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
)

const missingCommandDependencyDetailFormat = "%s - %s"

// ValidateDependencies validates dependencies for a command in two phases:
//
// Phase 1 (Host Dependencies): Root, command, and implementation-level depends_on are merged
// and ALWAYS validated against the HOST system, regardless of the selected runtime.
//
// Phase 2 (Container Dependencies): If the selected runtime is container, the runtime
// config's depends_on (if any) is validated inside the container environment.
// Runtime-level depends_on is only supported for container runtime.
//
// Note: depends_on.cmds is a discoverability check only. For host-level deps, Invowk validates
// that commands are discoverable via the standard discovery pipeline. For container runtime deps,
// it runs 'invowk internal check-cmd' inside the container. Neither phase executes the
// referenced commands.
func ValidateDependencies(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, parentCtx ExecutionContext, userEnv map[string]string) error {
	return ValidateDependenciesWithCapabilityChecker(disc, cmdInfo, nil, parentCtx, userEnv, nil)
}

// ValidateDependenciesWithCapabilityChecker validates dependencies with an injectable host capability checker.
func ValidateDependenciesWithCapabilityChecker(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, runtimeProbe RuntimeDependencyProbe, parentCtx ExecutionContext, userEnv map[string]string, hostCapabilityChecker CapabilityChecker) error {
	return ValidateDependenciesWithHostProbe(disc, cmdInfo, runtimeProbe, parentCtx, userEnv, hostCapabilityChecker, nil)
}

// ValidateDependenciesWithHostProbe validates dependencies with injectable
// host-device probes for application-service tests and adapters.
func ValidateDependenciesWithHostProbe(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, runtimeProbe RuntimeDependencyProbe, parentCtx ExecutionContext, userEnv map[string]string, hostCapabilityChecker CapabilityChecker, hostProbe HostProbe) error {
	return ValidateDependenciesWithPorts(disc, cmdInfo, runtimeProbe, parentCtx, userEnv, hostCapabilityChecker, hostProbe, nil)
}

// ValidateDependenciesWithPorts validates dependencies with injectable
// outside-device ports for application-service tests and adapters.
func ValidateDependenciesWithPorts(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, runtimeProbe RuntimeDependencyProbe, parentCtx ExecutionContext, userEnv map[string]string, hostCapabilityChecker CapabilityChecker, hostProbe HostProbe, lockProvider CommandScopeLockProvider) error {
	// Phase 1: Host dependencies (root + cmd + impl, always validated on host)
	if err := ValidateHostDependenciesWithPorts(disc, cmdInfo, parentCtx, userEnv, hostCapabilityChecker, hostProbe, lockProvider); err != nil {
		return err
	}

	// Phase 2: Runtime dependencies (selected runtime's depends_on, runtime-aware)
	return ValidateRuntimeDependencies(disc, cmdInfo, runtimeProbe, parentCtx, lockProvider)
}

// ValidateHostDependencies validates merged root+cmd+impl dependencies against the HOST.
// All 6 dependency types are always checked on the host, regardless of selected runtime.
// userEnv is the host environment captured eagerly at Execute() entry.
func ValidateHostDependencies(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, parentCtx ExecutionContext, userEnv map[string]string) error {
	return ValidateHostDependenciesWithCapabilityChecker(disc, cmdInfo, parentCtx, userEnv, nil)
}

// ValidateHostDependenciesWithCapabilityChecker validates host dependencies with an injectable capability checker.
func ValidateHostDependenciesWithCapabilityChecker(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, parentCtx ExecutionContext, userEnv map[string]string, hostCapabilityChecker CapabilityChecker) error {
	return ValidateHostDependenciesWithHostProbe(disc, cmdInfo, parentCtx, userEnv, hostCapabilityChecker, nil)
}

// ValidateHostDependenciesWithHostProbe validates host dependencies with injectable host-device probes.
func ValidateHostDependenciesWithHostProbe(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, parentCtx ExecutionContext, userEnv map[string]string, hostCapabilityChecker CapabilityChecker, hostProbe HostProbe) error {
	return ValidateHostDependenciesWithPorts(disc, cmdInfo, parentCtx, userEnv, hostCapabilityChecker, hostProbe, nil)
}

// ValidateHostDependenciesWithPorts validates host dependencies with injectable
// host-device and lock-file ports.
func ValidateHostDependenciesWithPorts(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, parentCtx ExecutionContext, userEnv map[string]string, hostCapabilityChecker CapabilityChecker, hostProbe HostProbe, lockProvider CommandScopeLockProvider) error {
	mergedDeps := invowkfile.MergeDependsOnAll(cmdInfo.Invowkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.ImplementationDependsOn)
	if mergedDeps == nil {
		return nil
	}

	invowkfilePath := cmdInfo.Invowkfile.FilePath

	// Env vars: validated using the snapshot captured at Execute() entry,
	// before any downstream code could potentially modify the environment.
	if err := CheckEnvVarDependencies(mergedDeps, userEnv, parentCtx); err != nil {
		return err
	}

	// Tools: always host PATH
	if err := CheckHostToolDependenciesWithProbe(mergedDeps, parentCtx, hostProbe); err != nil {
		return err
	}

	// Filepaths: always host filesystem
	if err := CheckHostFilepathDependenciesWithProbe(mergedDeps, invowkfilePath, parentCtx, hostProbe); err != nil {
		return err
	}

	// Capabilities: host-only
	if err := CheckCapabilityDependenciesWithChecker(mergedDeps, parentCtx, hostCapabilityChecker); err != nil {
		return err
	}

	// Custom checks: host-side checks default to embedded mvdan/sh and use
	// host interpreters only when the script selects a non-shell interpreter.
	if err := CheckHostCustomCheckDependenciesWithProbe(mergedDeps, parentCtx, hostProbe); err != nil {
		return err
	}

	// Command discoverability + scope enforcement: routed through CommandSetProvider
	// so the per-request cache avoids redundant filesystem scans.
	return CheckCommandDependenciesExistWithLockProvider(disc, mergedDeps, cmdInfo, parentCtx, lockProvider)
}

// ValidateRuntimeDependencies validates the selected runtime config's depends_on against
// the runtime's own environment. Runtime-level depends_on is only supported for the
// container runtime -- for native/virtual, it's a no-op since CUE schema and structural
// validation prevent declaring depends_on on those runtime types.
func ValidateRuntimeDependencies(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, probe RuntimeDependencyProbe, parentCtx ExecutionContext, lockProvider CommandScopeLockProvider) error {
	selectedRuntime := parentCtx.SelectedRuntime

	// Runtime-level depends_on is only supported for container runtime
	if selectedRuntime != invowkfile.RuntimeContainer {
		return nil
	}

	rtDeps := parentCtx.RuntimeDependsOn
	if rtDeps == nil {
		return nil
	}
	if rtDeps.IsEmpty() {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}
	resolvedCommands, err := resolveCommandDependenciesWithLockProvider(disc, rtDeps, cmdInfo, parentCtx, lockProvider)
	if err != nil {
		return err
	}

	// Env vars: validated inside the container
	if err := CheckEnvVarDependenciesInContainer(rtDeps, probe, parentCtx); err != nil {
		return err
	}

	// Tools: validated inside the container
	if err := CheckToolDependenciesInContainer(rtDeps, probe, parentCtx); err != nil {
		return err
	}

	// Filepaths: validated inside the container
	if err := CheckFilepathDependenciesInContainer(rtDeps, probe, parentCtx); err != nil {
		return err
	}

	// Capabilities: validated inside the container
	if err := CheckCapabilityDependenciesInContainer(rtDeps, probe, parentCtx); err != nil {
		return err
	}

	// Custom checks: validated inside the container
	if err := CheckCustomCheckDependenciesInContainer(rtDeps, probe, parentCtx); err != nil {
		return err
	}

	// Command discoverability: validated inside the container
	return checkResolvedCommandDependenciesInContainer(resolvedCommands, probe, parentCtx)
}

// CheckCommandDependenciesExist verifies that required commands are discoverable via the
// standard discovery pipeline AND accessible via the caller's CommandScope.
//
// Phase 1: Discoverability — each depends_on.cmds entry must exist in the flat namespace.
// Phase 2: Scope enforcement — if the caller is a module command (non-nil Metadata),
// each found command must be in the caller's CommandScope (same module, global, or
// direct dependency). Root invowkfile commands (nil Metadata) bypass scope enforcement.
func CheckCommandDependenciesExist(disc CommandSetProvider, deps *invowkfile.DependsOn, cmdInfo *discovery.CommandInfo, ctx ExecutionContext) error {
	return CheckCommandDependenciesExistWithLockProvider(disc, deps, cmdInfo, ctx, nil)
}

// CheckCommandDependenciesExistWithLockProvider verifies command dependencies
// using caller-provided lock-file state for module scope policy.
func CheckCommandDependenciesExistWithLockProvider(disc CommandSetProvider, deps *invowkfile.DependsOn, cmdInfo *discovery.CommandInfo, ctx ExecutionContext, lockProvider CommandScopeLockProvider) error {
	_, err := resolveCommandDependenciesWithLockProvider(disc, deps, cmdInfo, ctx, lockProvider)
	return err
}

// resolveCommandDependenciesWithLockProvider verifies command dependencies and
// returns the discovery-qualified command names that satisfied them.
func resolveCommandDependenciesWithLockProvider(disc CommandSetProvider, deps *invowkfile.DependsOn, cmdInfo *discovery.CommandInfo, ctx ExecutionContext, lockProvider CommandScopeLockProvider) ([]resolvedCommandDependency, error) {
	if deps == nil || len(deps.Commands) == 0 {
		return nil, nil
	}

	available, err := discoverAvailableCommands(disc, ctx)
	if err != nil {
		return nil, err
	}

	currentSource := currentCommandSourceID(cmdInfo)

	lock, err := commandScopeLock(lockProvider, cmdInfo.Invowkfile)
	if err != nil {
		return nil, err
	}

	// Build CommandScope for module commands (nil for root invowkfile).
	scope := buildCommandScope(cmdInfo, available, lock)

	var commandErrors []DependencyMessage
	var forbiddenErrors []DependencyMessage
	resolved := make([]resolvedCommandDependency, 0, len(deps.Commands))

	for _, dep := range deps.Commands {
		alternatives := normalizedCommandAlternatives(dep)
		if len(alternatives) == 0 {
			continue
		}

		matchedCmd, forbidden, found := findAccessibleCommand(available, currentSource, alternatives, scope)
		if matchedCmd != nil {
			matchedName := matchedCmd.Name
			resolved = append(resolved, resolvedCommandDependency{
				Alternatives: commandDependencyRefs(alternatives),
				Command:      &matchedName,
			})
			continue
		}
		if found {
			forbiddenErrors = append(forbiddenErrors, forbidden...)
			continue
		}
		if matchedCmd == nil {
			commandErrors = append(commandErrors, formatMissingDiscoveredCommandDependency(available, currentSource, alternatives, false))
			continue
		}
	}

	if len(commandErrors) > 0 || len(forbiddenErrors) > 0 {
		structuredFailures := dependencyFailures(DependencyFailureCommand, commandErrors)
		structuredFailures = append(structuredFailures, dependencyFailures(DependencyFailureForbiddenCommand, forbiddenErrors)...)
		return nil, &DependencyError{
			CommandName:        ctx.CommandName,
			MissingCommands:    commandErrors,
			ForbiddenCommands:  forbiddenErrors,
			StructuredFailures: structuredFailures,
		}
	}

	return resolved, nil
}

// buildCommandScope constructs a CommandScope for scope enforcement.
// Returns nil for root invowkfile commands (no scope restrictions).
func buildCommandScope(cmdInfo *discovery.CommandInfo, available map[invowkfile.CommandName]*discovery.CommandInfo, lock *invowkmod.LockFile) *invowkmod.CommandScope {
	if cmdInfo.Invowkfile.Metadata == nil {
		return nil // Root invowkfile — no scope restrictions.
	}

	moduleID := cmdInfo.Invowkfile.Metadata.Module()
	if cmdInfo.ModuleID != nil {
		moduleID = *cmdInfo.ModuleID
	}

	requirements := cmdInfo.Invowkfile.Metadata.Requires()

	// Wire direct dependencies from declarations resolved through lock-file
	// identity. Raw aliases are command namespaces, not authorization proof.
	scope := invowkmod.NewCommandScope(moduleID)
	scope.ModuleSourceID = invowkmod.ModuleSourceID(cmdInfo.SourceID) //goplint:ignore -- SourceID validated by discovery
	for _, cmd := range available {
		if cmd.IsGlobalModule {
			scope.AddGlobalSource(invowkmod.ModuleSourceID(cmd.SourceID)) //goplint:ignore -- SourceID validated by discovery
		}
	}

	// Wire resolved RDNS module IDs and command namespaces for direct deps.
	// A dependency is authorized only when the declaration and lock-file entry
	// agree with the discovered module identity and command source.
	for _, cmd := range available {
		if cmd.ModuleID == nil {
			continue
		}
		if commandMatchesDirectRequirement(requirements, lock, cmd) {
			scope.AddDirectDependency(*cmd.ModuleID, invowkmod.ModuleSourceID(cmd.SourceID)) //goplint:ignore -- SourceID validated by discovery
		}
	}

	return scope
}

func commandScopeLock(provider CommandScopeLockProvider, inv *invowkfile.Invowkfile) (*invowkmod.LockFile, error) {
	if provider == nil || inv == nil || inv.ModulePath == "" {
		return &invowkmod.LockFile{}, nil
	}
	return provider.LoadCommandScopeLock(inv)
}

func commandScopeDenialDetail(scope *invowkmod.CommandScope, decision invowkmod.CommandScopeDecision) DependencyMessage {
	return dependencyMessageFromDetail(fmt.Sprintf(
		"%s - command from module '%s' cannot call '%s': module '%s' is not accessible\n"+
			"Commands can only call commands from the same module (%s), commands from globally installed user command modules (~/.invowk/cmds/), or commands from direct dependencies declared in invowkmod.cue:requires and resolved in invowkmod.lock.cue. "+
			"Declare the dependency module in invowkmod.cue:requires if it is missing, then run 'invowk module sync' to refresh lock metadata",
		decision.TargetCommand, scope.ModuleID, decision.TargetCommand, decision.TargetSource, scope.ModuleID))
}

func commandMatchesDirectRequirement(requirements []invowkmod.ModuleRequirement, lock *invowkmod.LockFile, cmd *discovery.CommandInfo) bool {
	if cmd == nil || cmd.ModuleID == nil {
		return false
	}
	if lock == nil {
		return false
	}
	sourceID := invowkmod.ModuleSourceID(cmd.SourceID) //goplint:ignore -- SourceID validated by discovery
	for _, req := range requirements {
		ref := invowkmod.ModuleRef(req)
		locked, ok := lock.Modules[ref.Key()]
		if !ok {
			continue
		}
		if locked.IdentityModuleID() == *cmd.ModuleID && locked.EffectiveCommandSourceID() == sourceID {
			return true
		}
	}
	return false
}

func findAccessibleCommand(available map[invowkfile.CommandName]*discovery.CommandInfo, currentSource invowkmod.ModuleSourceID, alternatives []commandDependencyAlternative, scope *invowkmod.CommandScope) (*discovery.CommandInfo, []DependencyMessage, bool) {
	var forbidden []DependencyMessage
	for _, alt := range alternatives {
		for _, candidate := range matchingCommandCandidates(available, currentSource, alt) {
			decision := commandScopeDecision(scope, candidate)
			if decision.Allowed {
				return candidate, nil, true
			}
			forbidden = append(forbidden, commandScopeDenialDetail(scope, decision))
		}
	}
	return nil, forbidden, len(forbidden) > 0
}

func commandScopeDecision(scope *invowkmod.CommandScope, cmd *discovery.CommandInfo) invowkmod.CommandScopeDecision {
	if scope == nil {
		return invowkmod.CommandScopeDecision{Allowed: true, TargetCommand: invowkmod.CommandReference(cmd.Name)}
	}
	moduleID := invowkmod.ModuleID("")
	if cmd.ModuleID != nil {
		moduleID = *cmd.ModuleID
	}
	return scope.CanCallTarget(invowkmod.CommandTarget{
		Reference: invowkmod.CommandReference(cmd.Name),
		SourceID:  invowkmod.ModuleSourceID(cmd.SourceID), //goplint:ignore -- SourceID validated by discovery
		ModuleID:  moduleID,
	})
}

func discoverAvailableCommands(disc CommandSetProvider, ctx ExecutionContext) (map[invowkfile.CommandName]*discovery.CommandInfo, error) {
	commandSetResult, err := disc.DiscoverCommandSet(ctx.GoContext())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDependencyDiscoveryFailed, err)
	}

	available := make(map[invowkfile.CommandName]*discovery.CommandInfo, len(commandSetResult.Set.Commands))
	for _, cmd := range commandSetResult.Set.Commands {
		available[cmd.Name] = cmd
	}
	return available, nil
}

func normalizedCommandAlternatives(dep invowkfile.CommandDependency) []commandDependencyAlternative {
	var alternatives []commandDependencyAlternative
	for _, ref := range dep.Alternatives {
		parts, err := ref.Parse()
		if err != nil {
			continue
		}
		alt := commandDependencyAlternative{
			Ref:   ref,
			Parts: parts,
		}
		if err := alt.Validate(); err != nil {
			continue
		}
		alternatives = append(alternatives, alt)
	}
	return alternatives
}

func commandDependencyRefs(alternatives []commandDependencyAlternative) []invowkfile.CommandDependencyRef {
	refs := make([]invowkfile.CommandDependencyRef, 0, len(alternatives))
	for _, alt := range alternatives {
		refs = append(refs, alt.Ref)
	}
	return refs
}

// findMatchingCommand returns the first CommandInfo matching any alternative,
// or nil if none found. Bare alternatives resolve only against the caller's
// own command source; @source alternatives resolve by source ID and command name.
func findMatchingCommand(available map[invowkfile.CommandName]*discovery.CommandInfo, currentSource invowkmod.ModuleSourceID, alternatives []commandDependencyAlternative) *discovery.CommandInfo {
	for _, alt := range alternatives {
		candidates := matchingCommandCandidates(available, currentSource, alt)
		if len(candidates) > 0 {
			return candidates[0]
		}
	}
	return nil
}

func matchingCommandCandidates(available map[invowkfile.CommandName]*discovery.CommandInfo, currentSource invowkmod.ModuleSourceID, alt commandDependencyAlternative) []*discovery.CommandInfo {
	if alt.Parts.Qualified {
		return sourceCommandCandidates(available, invowkmod.ModuleSourceID(alt.Parts.SourceID), alt.Parts.Command)
	}
	return sourceCommandCandidates(available, currentSource, alt.Parts.Command)
}

func sourceCommandCandidates(available map[invowkfile.CommandName]*discovery.CommandInfo, source invowkmod.ModuleSourceID, command invowkfile.CommandName) []*discovery.CommandInfo {
	ordered := append(prioritizedCommandLookups(available, source, command), commandMapValues(available)...)
	return uniqueMatchingCommandCandidates(ordered, source, command)
}

func prioritizedCommandLookups(available map[invowkfile.CommandName]*discovery.CommandInfo, source invowkmod.ModuleSourceID, command invowkfile.CommandName) []*discovery.CommandInfo {
	var candidates []*discovery.CommandInfo
	if source != "" {
		qualified := invowkfile.CommandName(string(source) + " " + string(command)) //goplint:ignore -- map key lookup only
		if cmd, ok := available[qualified]; ok {
			candidates = append(candidates, cmd)
		}
	}
	if cmd, ok := available[command]; ok {
		candidates = append(candidates, cmd)
	}
	return candidates
}

func commandMatchesSourceAndName(cmd *discovery.CommandInfo, source invowkmod.ModuleSourceID, command invowkfile.CommandName) bool {
	if cmd == nil {
		return false
	}
	cmdSource := commandInfoSourceID(cmd)
	if cmdSource != source {
		return false
	}
	return commandInfoSimpleName(cmd, cmdSource) == command
}

func commandInfoSourceID(cmd *discovery.CommandInfo) invowkmod.ModuleSourceID {
	if cmd == nil {
		return ""
	}
	return invowkmod.ModuleSourceID(cmd.SourceID) //goplint:ignore -- SourceID validated by discovery
}

func commandInfoSimpleName(cmd *discovery.CommandInfo, source invowkmod.ModuleSourceID) invowkfile.CommandName {
	if cmd == nil {
		return ""
	}
	if cmd.SimpleName != "" {
		return cmd.SimpleName
	}
	if source != "" {
		prefix := string(source) + " "
		return invowkfile.CommandName(strings.TrimPrefix(string(cmd.Name), prefix)) //goplint:ignore -- derived from discovered command name
	}
	return cmd.Name
}

func commandMapValues(available map[invowkfile.CommandName]*discovery.CommandInfo) []*discovery.CommandInfo {
	commands := make([]*discovery.CommandInfo, 0, len(available))
	for _, cmd := range available {
		commands = append(commands, cmd)
	}
	return commands
}

func uniqueMatchingCommandCandidates(commands []*discovery.CommandInfo, source invowkmod.ModuleSourceID, command invowkfile.CommandName) []*discovery.CommandInfo {
	var candidates []*discovery.CommandInfo
	for _, cmd := range commands {
		if !commandMatchesSourceAndName(cmd, source, command) || slices.Contains(candidates, cmd) {
			continue
		}
		candidates = append(candidates, cmd)
	}
	return candidates
}

func commandSourceExists(available map[invowkfile.CommandName]*discovery.CommandInfo, source invowkmod.ModuleSourceID) bool {
	for _, cmd := range available {
		if commandInfoSourceID(cmd) == source {
			return true
		}
	}
	return false
}

func currentCommandSourceID(cmdInfo *discovery.CommandInfo) invowkmod.ModuleSourceID {
	if cmdInfo == nil {
		return ""
	}
	if cmdInfo.SourceID != "" {
		return invowkmod.ModuleSourceID(cmdInfo.SourceID) //goplint:ignore -- SourceID validated by discovery
	}
	if cmdInfo.Invowkfile != nil && cmdInfo.Invowkfile.Metadata != nil {
		return invowkmod.ModuleSourceID(cmdInfo.Invowkfile.Metadata.Module())
	}
	return ""
}

func formatMissingDiscoveredCommandDependency(
	available map[invowkfile.CommandName]*discovery.CommandInfo,
	currentSource invowkmod.ModuleSourceID,
	alternatives []commandDependencyAlternative,
	inContainer bool,
) DependencyMessage {
	if len(alternatives) == 1 && alternatives[0].Parts.Qualified {
		source := invowkmod.ModuleSourceID(alternatives[0].Parts.SourceID)
		suffix := fmt.Sprintf("command %q not found in source %q", alternatives[0].Parts.Command, source)
		if !commandSourceExists(available, source) {
			suffix = fmt.Sprintf("source %q not found", source)
		}
		if inContainer {
			suffix += " in container"
		}
		return dependencyMessageFromDetail(fmt.Sprintf(missingCommandDependencyDetailFormat, alternatives[0].Ref, suffix))
	}
	if len(alternatives) == 1 && currentSource != "" {
		suffix := fmt.Sprintf("command not found in source %q", currentSource)
		if inContainer {
			suffix += " in container"
		}
		return dependencyMessageFromDetail(fmt.Sprintf(missingCommandDependencyDetailFormat, alternatives[0].Ref, suffix))
	}
	return formatMissingCommandDependency(alternatives, inContainer)
}

func formatMissingCommandDependency(alternatives []commandDependencyAlternative, inContainer bool) DependencyMessage {
	if len(alternatives) == 1 {
		suffix := "command not found"
		if inContainer {
			suffix = "command not found in container"
		}
		return dependencyMessageFromDetail(fmt.Sprintf(missingCommandDependencyDetailFormat, alternatives[0].Ref, suffix))
	}

	message := "none of [%s] found"
	if inContainer {
		message = "none of [%s] found in container"
	}
	return dependencyMessageFromDetail(fmt.Sprintf(message, commandNamesDisplay(alternatives)))
}

//goplint:ignore -- dependency error rendering needs a comma-separated display string for typed command names.
func commandNamesDisplay(names []commandDependencyAlternative) string {
	display := make([]string, 0, len(names))
	for _, name := range names {
		display = append(display, name.Ref.String())
	}
	return strings.Join(display, ", ")
}
