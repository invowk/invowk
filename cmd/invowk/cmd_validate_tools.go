// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"fmt"
	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
	"os/exec"
	"strings"
)

// checkToolDependenciesWithRuntime verifies all required tools are available
// The validation method depends on the runtime:
// - native: check against host system PATH
// - virtual: check against built-in utilities
// - container: check within the container environment
// Each ToolDependency has alternatives with OR semantics (any alternative found satisfies the dependency)
func checkToolDependenciesWithRuntime(deps *invkfile.DependsOn, runtimeMode invkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range deps.Tools {
		// OR semantics: try each alternative until one succeeds
		var lastErr error
		found := false
		for _, alt := range tool.Alternatives {
			var err error
			switch runtimeMode {
			case invkfile.RuntimeContainer:
				err = validateToolInContainer(alt, registry, ctx)
			case invkfile.RuntimeVirtual:
				err = validateToolInVirtual(alt, registry, ctx)
			case invkfile.RuntimeNative:
				err = validateToolNative(alt)
			}
			if err == nil {
				found = true
				break // Early return on first match
			}
			lastErr = err
		}
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

// validateToolInVirtual validates a tool dependency using the virtual runtime.
// It accepts a tool name string and checks if it exists in the virtual shell environment.
func validateToolInVirtual(toolName string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		// Fall back to native validation if virtual runtime not available
		return validateToolNative(toolName)
	}

	// Use 'command -v' to check if tool exists in virtual shell
	checkScript := fmt.Sprintf("command -v %s", toolName)

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: checkScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeVirtual}}},
		SelectedRuntime: invkfile.RuntimeVirtual,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)

	if result.ExitCode != 0 {
		return fmt.Errorf("  • %s - not available in virtual runtime", toolName)
	}
	return nil
}

// validateToolInContainer validates a tool dependency within a container.
// It accepts a tool name string and checks if it exists in the container environment.
func validateToolInContainer(toolName string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", toolName)
	}

	// Use 'command -v' or 'which' to check if tool exists in container
	checkScript := fmt.Sprintf("command -v %s || which %s", toolName, toolName)

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: checkScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}},
		SelectedRuntime: invkfile.RuntimeContainer,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)

	if result.ExitCode != 0 {
		return fmt.Errorf("  • %s - not available in container", toolName)
	}
	return nil
}

// checkToolDependencies verifies all required tools are available (legacy - uses native only).
// Each ToolDependency contains a list of alternatives; if any alternative is found, the dependency is satisfied.
func checkToolDependencies(cmd *invkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Tools) == 0 {
		return nil
	}

	var toolErrors []string

	for _, tool := range cmd.DependsOn.Tools {
		var lastErr error
		found := false
		for _, alt := range tool.Alternatives {
			if err := validateToolNative(alt); err == nil {
				found = true
				break // Early return on first match
			} else {
				lastErr = err
			}
		}
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
