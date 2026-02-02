// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
)

// validateDependencies validates merged dependencies for a command.
// Dependencies are merged from root-level, command-level, and implementation-level, and
// validated according to the selected runtime:
// - native: validated against the native standard shell from the host
// - virtual: validated against invowk's built-in sh interpreter with core utils
// - container: validated against the container's default shell from within the container
//
// Note: `depends_on.cmds` is an existence check only. Invowk validates that referenced
// commands are discoverable (in this invkfile, modules, or configured search paths),
// but it does not execute them automatically.
func validateDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	// Merge root-level, command-level, and implementation-level dependencies
	mergedDeps := invkfile.MergeDependsOnAll(cmdInfo.Invkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)

	if mergedDeps == nil {
		return nil
	}

	// Get the selected runtime for context-aware validation
	selectedRuntime := parentCtx.SelectedRuntime

	// FIRST: Check env var dependencies (host-only, validated BEFORE invowk sets any env vars)
	// We capture the user's environment here to ensure we validate against their actual env,
	// not any variables that invowk might set from the 'env' construct
	if err := checkEnvVarDependencies(mergedDeps, captureUserEnv(), parentCtx); err != nil {
		return err
	}

	// Then check tool dependencies (runtime-aware)
	if err := checkToolDependenciesWithRuntime(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check filepath dependencies (runtime-aware)
	if err := checkFilepathDependenciesWithRuntime(mergedDeps, cmdInfo.Invkfile.FilePath, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check capability dependencies (host-only, not runtime-aware)
	if err := checkCapabilityDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Then check custom check dependencies (runtime-aware)
	if err := checkCustomCheckDependencies(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check command dependencies (existence-only; these are not executed automatically)
	// Get module ID from metadata (nil for non-module invkfiles)
	currentModule := ""
	if cmdInfo.Invkfile.Metadata != nil {
		currentModule = cmdInfo.Invkfile.Metadata.Module
	}
	if err := checkCommandDependenciesExist(mergedDeps, currentModule, parentCtx); err != nil {
		return err
	}

	return nil
}

func checkCommandDependenciesExist(deps *invkfile.DependsOn, currentModule string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	availableCommands, err := disc.DiscoverCommands()
	if err != nil {
		return fmt.Errorf("failed to discover commands for dependency validation: %w", err)
	}

	available := make(map[string]struct{}, len(availableCommands))
	for _, cmd := range availableCommands {
		available[cmd.Name] = struct{}{}
	}

	var commandErrors []string

	for _, dep := range deps.Commands {
		var alternatives []string
		for _, alt := range dep.Alternatives {
			alt = strings.TrimSpace(alt)
			if alt == "" {
				continue
			}
			alternatives = append(alternatives, alt)
		}
		if len(alternatives) == 0 {
			continue
		}

		// OR semantics: any alternative being discoverable satisfies this dependency.
		found := false
		for _, alt := range alternatives {
			if _, ok := available[alt]; ok {
				found = true
				break
			}

			// Also allow referencing commands from the current invkfile without a module prefix.
			qualified := currentModule + " " + alt
			if _, ok := available[qualified]; ok {
				found = true
				break
			}
		}

		if !found {
			if len(alternatives) == 1 {
				commandErrors = append(commandErrors, fmt.Sprintf("  • %s - command not found", alternatives[0]))
			} else {
				commandErrors = append(commandErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(alternatives, ", ")))
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
