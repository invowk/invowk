// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ToolNamePattern validates tool names before shell interpolation.
// Defense-in-depth: CUE schema constrains tool names at parse time.
var ToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9._+\-/]+$`)

// CheckToolDependenciesInContainer verifies all required tools are available inside the container.
// Called only for container runtime (caller guards non-container early return).
// Each ToolDependency has alternatives with OR semantics (any alternative found satisfies the dependency).
func CheckToolDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("%w for tool validation", ErrContainerRuntimeNotAvailable)
	}

	toolErrors := CollectToolErrors(deps.Tools, func(alt invowkfile.BinaryName) error {
		return ValidateToolInContainer(alt, rt, ctx)
	})

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  ctx.Command.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}

// ValidateToolNative validates a tool dependency against the host system PATH.
// It accepts a BinaryName and checks if it exists in the system PATH.
func ValidateToolNative(toolName invowkfile.BinaryName) error {
	_, err := exec.LookPath(string(toolName))
	if err != nil {
		return fmt.Errorf("  • %s - not found in PATH", toolName)
	}
	return nil
}

// ValidateToolInContainer validates a tool dependency within a container.
// It accepts a BinaryName and checks if it exists in the container environment.
// The runtime is passed directly (hoisted by caller) to avoid redundant registry lookups.
func ValidateToolInContainer(toolName invowkfile.BinaryName, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	toolNameStr := string(toolName)

	// Defense-in-depth: validate tool name before shell interpolation
	if !ToolNamePattern.MatchString(toolNameStr) {
		return fmt.Errorf("  • %s - invalid tool name for shell interpolation", toolName)
	}

	checkScript := fmt.Sprintf("command -v '%s' || which '%s'", ShellEscapeSingleQuote(toolNameStr), ShellEscapeSingleQuote(toolNameStr))

	validationCtx, _, stderr := NewContainerValidationContext(ctx, checkScript)

	result := rt.Execute(validationCtx)
	if result.Error != nil {
		return fmt.Errorf("  • %s - %w: %w", toolName, ErrContainerValidationFailed, result.Error)
	}
	if err := CheckTransientExitCode(result, toolNameStr); err != nil {
		return err
	}
	if result.ExitCode != 0 {
		_ = stderr // consumed by NewContainerValidationContext but not needed here
		return fmt.Errorf("  • %s - not available in container", toolName)
	}
	return nil
}

// CheckHostToolDependencies verifies all required tools are available against the HOST PATH.
// Always uses native validation regardless of selected runtime.
func CheckHostToolDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	return CheckHostToolDependenciesWithProbe(deps, ctx, newDefaultHostProbe())
}

// CheckHostToolDependenciesWithProbe verifies host tool dependencies through an injectable probe.
func CheckHostToolDependenciesWithProbe(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext, probe HostProbe) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}

	toolErrors := CollectToolErrors(deps.Tools, probe.CheckTool)

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:  ctx.Command.Name,
			MissingTools: toolErrors,
		}
	}

	return nil
}
