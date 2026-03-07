// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

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
func ValidateDependencies(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext, userEnv map[string]string) error {
	// Phase 1: Host dependencies (root + cmd + impl, always validated on host)
	if err := ValidateHostDependencies(disc, cmdInfo, parentCtx, userEnv); err != nil {
		return err
	}

	// Phase 2: Runtime dependencies (selected runtime's depends_on, runtime-aware)
	return ValidateRuntimeDependencies(cmdInfo, registry, parentCtx)
}

// ValidateHostDependencies validates merged root+cmd+impl dependencies against the HOST.
// All 6 dependency types are always checked on the host, regardless of selected runtime.
// userEnv is the host environment captured eagerly at Execute() entry.
func ValidateHostDependencies(disc CommandSetProvider, cmdInfo *discovery.CommandInfo, parentCtx *runtime.ExecutionContext, userEnv map[string]string) error {
	mergedDeps := invowkfile.MergeDependsOnAll(cmdInfo.Invowkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)
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
	if err := CheckHostToolDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Filepaths: always host filesystem
	if err := CheckHostFilepathDependencies(mergedDeps, invowkfilePath, parentCtx); err != nil {
		return err
	}

	// Capabilities: host-only
	if err := CheckCapabilityDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Custom checks: always native shell on host
	if err := CheckHostCustomCheckDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Command discoverability: routed through CommandSetProvider so the
	// per-request cache avoids redundant filesystem scans.
	currentModule := ""
	if cmdInfo.Invowkfile.Metadata != nil {
		currentModule = string(cmdInfo.Invowkfile.Metadata.Module())
	}
	return CheckCommandDependenciesExist(disc, mergedDeps, currentModule, parentCtx)
}

// ValidateRuntimeDependencies validates the selected runtime config's depends_on against
// the runtime's own environment. Runtime-level depends_on is only supported for the
// container runtime -- for native/virtual, it's a no-op since CUE schema and structural
// validation prevent declaring depends_on on those runtime types.
func ValidateRuntimeDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error { //nolint:revive // cmdInfo kept for phase symmetry with ValidateHostDependencies
	selectedRuntime := parentCtx.SelectedRuntime

	// Runtime-level depends_on is only supported for container runtime
	if selectedRuntime != invowkfile.RuntimeContainer {
		return nil
	}

	// Find the selected runtime config to get its depends_on
	rc := invowkfile.FindRuntimeConfig(parentCtx.SelectedImpl.Runtimes, selectedRuntime)
	if rc == nil || rc.DependsOn == nil {
		return nil
	}

	rtDeps := rc.DependsOn

	if rtDeps.IsEmpty() {
		return nil
	}

	// Env vars: validated inside the container
	if err := CheckEnvVarDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Tools: validated inside the container
	if err := CheckToolDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Filepaths: validated inside the container
	if err := CheckFilepathDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Capabilities: validated inside the container
	if err := CheckCapabilityDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Custom checks: validated inside the container
	if err := CheckCustomCheckDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Command discoverability: validated inside the container
	return CheckCommandDependenciesInContainer(rtDeps, registry, parentCtx)
}

// CheckCommandDependenciesExist verifies that required commands are discoverable via the
// standard discovery pipeline. Uses CommandSetProvider to share the per-request cache.
func CheckCommandDependenciesExist(disc CommandSetProvider, deps *invowkfile.DependsOn, currentModule string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	available, err := discoverAvailableCommands(disc, ctx)
	if err != nil {
		return err
	}

	var commandErrors []DependencyMessage

	for _, dep := range deps.Commands {
		alternatives := normalizedCommandAlternatives(dep)
		if len(alternatives) == 0 {
			continue
		}

		if !commandAlternativeExists(available, currentModule, alternatives) {
			commandErrors = append(commandErrors, formatMissingCommandDependency(alternatives, false))
		}
	}

	if len(commandErrors) > 0 {
		return &DependencyError{
			CommandName:     ctx.Command.Name,
			MissingCommands: commandErrors,
		}
	}

	return nil
}

func discoverAvailableCommands(disc CommandSetProvider, ctx *runtime.ExecutionContext) (map[invowkfile.CommandName]struct{}, error) {
	discoverCtx := context.Background()
	if ctx != nil && ctx.Context != nil {
		discoverCtx = ctx.Context
	}

	commandSetResult, err := disc.DiscoverCommandSet(discoverCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover commands for dependency validation: %w", err)
	}

	available := make(map[invowkfile.CommandName]struct{}, len(commandSetResult.Set.Commands))
	for _, cmd := range commandSetResult.Set.Commands {
		available[cmd.Name] = struct{}{}
	}
	return available, nil
}

//goplint:ignore -- helper normalizes discovered command names for dependency checks.
func normalizedCommandAlternatives(dep invowkfile.CommandDependency) []string {
	var alternatives []string
	for _, alt := range dep.Alternatives {
		trimmed := strings.TrimSpace(string(alt))
		if trimmed != "" {
			alternatives = append(alternatives, trimmed)
		}
	}
	return alternatives
}

//goplint:ignore -- helper probes normalized command alternatives against discovery output.
func commandAlternativeExists(available map[invowkfile.CommandName]struct{}, currentModule string, alternatives []string) bool {
	for _, alt := range alternatives {
		if _, ok := available[invowkfile.CommandName(alt)]; ok { //goplint:ignore -- map key lookup only
			return true
		}

		qualified := invowkfile.CommandName(currentModule + " " + alt) //goplint:ignore -- map key lookup only
		if _, ok := available[qualified]; ok {
			return true
		}
	}
	return false
}

//goplint:ignore -- helper formats normalized command-alternative display strings.
func formatMissingCommandDependency(alternatives []string, inContainer bool) DependencyMessage {
	if len(alternatives) == 1 {
		suffix := "command not found"
		if inContainer {
			suffix = "command not found in container"
		}
		return DependencyMessage(fmt.Sprintf("  • %s - %s", alternatives[0], suffix))
	}

	message := "  • none of [%s] found"
	if inContainer {
		message = "  • none of [%s] found in container"
	}
	return DependencyMessage(fmt.Sprintf(message, strings.Join(alternatives, ", ")))
}
