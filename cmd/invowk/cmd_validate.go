// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// validateDependencies validates merged dependencies for a command.
// Dependencies are merged from root-level, command-level, and implementation-level, and
// validated according to the selected runtime:
// - native: validated against the native standard shell from the host
// - virtual: validated against invowk's built-in sh interpreter with core utils
// - container: validated against the container's default shell from within the container
//
// Note: `depends_on.cmds` is an existence check only. Invowk validates that referenced
// commands are discoverable (in this invowkfile, modules, or configured search paths),
// but it does not execute them automatically.
func validateDependencies(cfg *config.Config, cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	// Merge root-level, command-level, and implementation-level dependencies
	mergedDeps := invowkfile.MergeDependsOnAll(cmdInfo.Invowkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)

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
	if err := checkFilepathDependenciesWithRuntime(mergedDeps, cmdInfo.Invowkfile.FilePath, selectedRuntime, registry, parentCtx); err != nil {
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
	// Get module ID from metadata (nil for non-module invowkfiles)
	currentModule := ""
	if cmdInfo.Invowkfile.Metadata != nil {
		currentModule = cmdInfo.Invowkfile.Metadata.Module
	}
	if err := checkCommandDependenciesExist(cfg, mergedDeps, currentModule, parentCtx); err != nil {
		return err
	}

	return nil
}

func checkCommandDependenciesExist(cfg *config.Config, deps *invowkfile.DependsOn, currentModule string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	disc := discovery.New(cfg)

	discoverCtx := context.Background()
	if ctx != nil && ctx.Context != nil {
		discoverCtx = ctx.Context
	}

	commandSetResult, err := disc.DiscoverCommandSet(discoverCtx)
	if err != nil {
		return fmt.Errorf("failed to discover commands for dependency validation: %w", err)
	}

	availableCommands := commandSetResult.Set.Commands

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

			// Also allow referencing commands from the current invowkfile without a module prefix.
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
