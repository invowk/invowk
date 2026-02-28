// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// validateDependencies validates dependencies for a command in two phases:
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
func validateDependencies(disc DiscoveryService, cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext, userEnv map[string]string) error {
	// Phase 1: Host dependencies (root + cmd + impl, always validated on host)
	if err := validateHostDependencies(disc, cmdInfo, parentCtx, userEnv); err != nil {
		return err
	}

	// Phase 2: Runtime dependencies (selected runtime's depends_on, runtime-aware)
	return validateRuntimeDependencies(cmdInfo, registry, parentCtx)
}

// validateHostDependencies validates merged root+cmd+impl dependencies against the HOST.
// All 6 dependency types are always checked on the host, regardless of selected runtime.
// userEnv is the host environment captured eagerly at Execute() entry.
func validateHostDependencies(disc DiscoveryService, cmdInfo *discovery.CommandInfo, parentCtx *runtime.ExecutionContext, userEnv map[string]string) error {
	mergedDeps := invowkfile.MergeDependsOnAll(cmdInfo.Invowkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)
	if mergedDeps == nil {
		return nil
	}

	invowkfilePath := cmdInfo.Invowkfile.FilePath

	// Env vars: validated using the snapshot captured at Execute() entry,
	// before any downstream code could potentially modify the environment.
	if err := checkEnvVarDependencies(mergedDeps, userEnv, parentCtx); err != nil {
		return err
	}

	// Tools: always host PATH
	if err := checkHostToolDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Filepaths: always host filesystem
	if err := checkHostFilepathDependencies(mergedDeps, invowkfilePath, parentCtx); err != nil {
		return err
	}

	// Capabilities: host-only
	if err := checkCapabilityDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Custom checks: always native shell on host
	if err := checkHostCustomCheckDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Command discoverability: routed through DiscoveryService so the
	// per-request cache avoids redundant filesystem scans.
	currentModule := ""
	if cmdInfo.Invowkfile.Metadata != nil {
		currentModule = string(cmdInfo.Invowkfile.Metadata.Module())
	}
	return checkCommandDependenciesExist(disc, mergedDeps, currentModule, parentCtx)
}

// validateRuntimeDependencies validates the selected runtime config's depends_on against
// the runtime's own environment. Runtime-level depends_on is only supported for the
// container runtime — for native/virtual, it's a no-op since CUE schema and structural
// validation prevent declaring depends_on on those runtime types.
func validateRuntimeDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
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
	if err := checkEnvVarDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Tools: validated inside the container
	if err := checkToolDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Filepaths: validated inside the container
	if err := checkFilepathDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Capabilities: validated inside the container
	if err := checkCapabilityDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Custom checks: validated inside the container
	if err := checkCustomCheckDependenciesInContainer(rtDeps, registry, parentCtx); err != nil {
		return err
	}

	// Command discoverability: validated inside the container
	return checkCommandDependenciesInContainer(rtDeps, registry, parentCtx)
}

func checkCommandDependenciesExist(disc DiscoveryService, deps *invowkfile.DependsOn, currentModule string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	discoverCtx := context.Background()
	if ctx != nil && ctx.Context != nil {
		discoverCtx = ctx.Context
	}

	commandSetResult, err := disc.DiscoverCommandSet(discoverCtx)
	if err != nil {
		return fmt.Errorf("failed to discover commands for dependency validation: %w", err)
	}

	availableCommands := commandSetResult.Set.Commands

	available := make(map[invowkfile.CommandName]struct{}, len(availableCommands))
	for _, cmd := range availableCommands {
		available[cmd.Name] = struct{}{}
	}

	var commandErrors []DependencyMessage

	for _, dep := range deps.Commands {
		var alternatives []string
		for _, alt := range dep.Alternatives {
			trimmed := strings.TrimSpace(string(alt))
			if trimmed == "" {
				continue
			}
			alternatives = append(alternatives, trimmed)
		}
		if len(alternatives) == 0 {
			continue
		}

		// OR semantics: any alternative being discoverable satisfies this dependency.
		found := false
		for _, alt := range alternatives {
			if _, ok := available[invowkfile.CommandName(alt)]; ok { //goplint:ignore -- map key lookup only
				found = true
				break
			}

			// Also allow referencing commands from the current invowkfile without a module prefix.
			qualified := invowkfile.CommandName(currentModule + " " + alt) //goplint:ignore -- map key lookup only
			if _, ok := available[qualified]; ok {
				found = true
				break
			}
		}

		if !found {
			if len(alternatives) == 1 {
				commandErrors = append(commandErrors, DependencyMessage(fmt.Sprintf("  • %s - command not found", alternatives[0])))
			} else {
				commandErrors = append(commandErrors, DependencyMessage(fmt.Sprintf("  • none of [%s] found", strings.Join(alternatives, ", "))))
			}
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
