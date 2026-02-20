// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// toolNamePattern validates tool names before shell interpolation.
// Defense-in-depth: CUE schema constrains tool names at parse time.
var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9._+\-/]+$`)

// checkToolDependenciesInContainer verifies all required tools are available inside the container.
// Called only for container runtime (caller guards non-container early return).
// Each ToolDependency has alternatives with OR semantics (any alternative found satisfies the dependency).
func checkToolDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("container runtime not available for tool validation")
	}

	var toolErrors []string

	for _, tool := range deps.Tools {
		found, lastErr := evaluateAlternatives(tool.Alternatives, func(alt string) error {
			return validateToolInContainer(alt, rt, ctx)
		})
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, lastErr.Error())
			} else {
				toolErrors = append(toolErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(tool.Alternatives, ", ")))
			}
		}
	}

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  ctx.Command.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}

// validateToolNative validates a tool dependency against the host system PATH.
// It accepts a tool name string and checks if it exists in the system PATH.
func validateToolNative(toolName string) error {
	_, err := exec.LookPath(toolName)
	if err != nil {
		return fmt.Errorf("  • %s - not found in PATH", toolName)
	}
	return nil
}

// validateToolInContainer validates a tool dependency within a container.
// It accepts a tool name string and checks if it exists in the container environment.
// The runtime is passed directly (hoisted by caller) to avoid redundant registry lookups.
func validateToolInContainer(toolName string, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	// Defense-in-depth: validate tool name before shell interpolation
	if !toolNamePattern.MatchString(toolName) {
		return fmt.Errorf("  • %s - invalid tool name for shell interpolation", toolName)
	}

	checkScript := fmt.Sprintf("command -v '%s' || which '%s'", shellEscapeSingleQuote(toolName), shellEscapeSingleQuote(toolName))

	validationCtx, _, stderr := newContainerValidationContext(ctx, checkScript)

	result := rt.Execute(validationCtx)
	if result.Error != nil {
		return fmt.Errorf("  • %s - container validation failed: %w", toolName, result.Error)
	}
	if err := checkTransientExitCode(result, toolName); err != nil {
		return err
	}
	if result.ExitCode != 0 {
		_ = stderr // consumed by newContainerValidationContext but not needed here
		return fmt.Errorf("  • %s - not available in container", toolName)
	}
	return nil
}

// checkHostToolDependencies verifies all required tools are available against the HOST PATH.
// Always uses native validation regardless of selected runtime.
func checkHostToolDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range deps.Tools {
		found, lastErr := evaluateAlternatives(tool.Alternatives, validateToolNative)
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, lastErr.Error())
			} else {
				toolErrors = append(toolErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(tool.Alternatives, ", ")))
			}
		}
	}

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  ctx.Command.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}

// checkToolDependencies verifies all required tools are available (native-only fallback).
// Each ToolDependency contains a list of alternatives; if any alternative is found, the dependency is satisfied.
func checkToolDependencies(cmd *invowkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range cmd.DependsOn.Tools {
		found, lastErr := evaluateAlternatives(tool.Alternatives, validateToolNative)
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, lastErr.Error())
			} else {
				toolErrors = append(toolErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(tool.Alternatives, ", ")))
			}
		}
	}

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  cmd.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}
