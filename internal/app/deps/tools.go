// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"regexp"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// ToolNamePattern validates tool names before shell interpolation.
// Defense-in-depth: CUE schema constrains tool names at parse time.
var ToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9._+\-/]+$`)

// CheckToolDependenciesInContainer verifies all required tools are available inside the container.
// Called only for container runtime (caller guards non-container early return).
// Each ToolDependency has alternatives with OR semantics (any alternative found satisfies the dependency).
func CheckToolDependenciesInContainer(deps *invowkfile.DependsOn, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}

	toolErrors := CollectToolErrors(deps.Tools, func(alt invowkfile.BinaryName) error {
		return probe.CheckTool(alt)
	})

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			MissingTools:       toolErrors,
			StructuredFailures: dependencyFailures(DependencyFailureTool, toolErrors),
		}
	}

	return nil
}

// CheckHostToolDependencies verifies all required tools are available against the HOST PATH.
// Always uses native validation regardless of selected runtime.
func CheckHostToolDependencies(deps *invowkfile.DependsOn, ctx ExecutionContext) error {
	return CheckHostToolDependenciesWithProbe(deps, ctx, nil)
}

// CheckHostToolDependenciesWithProbe verifies host tool dependencies through an injectable probe.
func CheckHostToolDependenciesWithProbe(deps *invowkfile.DependsOn, ctx ExecutionContext, probe HostProbe) error {
	if deps == nil || len(deps.Tools) == 0 {
		return nil
	}
	if probe == nil {
		return ErrHostProbeRequired
	}

	toolErrors := CollectToolErrors(deps.Tools, probe.CheckTool)

	if len(toolErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			MissingTools:       toolErrors,
			StructuredFailures: dependencyFailures(DependencyFailureTool, toolErrors),
		}
	}

	return nil
}
