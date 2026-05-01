// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// CheckFilepathDependenciesInContainer verifies all required files/directories exist inside the container.
// Called only for container runtime (caller guards non-container early return).
func CheckFilepathDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("%w for filepath validation", ErrContainerRuntimeNotAvailable)
	}

	var filepathErrors []DependencyMessage

	for _, fp := range deps.Filepaths {
		if err := ValidateFilepathInContainer(fp, rt, ctx); err != nil {
			filepathErrors = append(filepathErrors, dependencyMessageFromDetail(err.Error()))
		}
	}

	if len(filepathErrors) > 0 {
		return &DependencyError{
			CommandName:      ctx.Command.Name,
			MissingFilepaths: filepathErrors,
		}
	}

	return nil
}

// ValidateFilepathInContainer validates a filepath dependency within a container.
// The runtime is passed directly (hoisted by caller) to avoid redundant registry lookups.
func ValidateFilepathInContainer(fp invowkfile.FilepathDependency, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - %w", ErrNoPathAlternatives)
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		detail, err := validateContainerFilepathAlternative(string(altPath), fp, rt, ctx)
		if err != nil {
			return err
		}
		if detail == "" {
			return nil
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, detail))
	}

	return formatContainerFilepathError(fp.Alternatives, allErrors)
}

// CheckHostFilepathDependencies verifies all required files/directories exist on the HOST filesystem.
// Always uses native validation regardless of selected runtime.
func CheckHostFilepathDependencies(deps *invowkfile.DependsOn, invowkfilePath types.FilesystemPath, ctx *runtime.ExecutionContext) error {
	return CheckHostFilepathDependenciesWithProbe(deps, invowkfilePath, ctx, nil)
}

// CheckHostFilepathDependenciesWithProbe verifies host filepath dependencies through an injectable probe.
func CheckHostFilepathDependenciesWithProbe(deps *invowkfile.DependsOn, invowkfilePath types.FilesystemPath, ctx *runtime.ExecutionContext, probe HostProbe) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []DependencyMessage
	invowkDir := fspath.Dir(invowkfilePath)

	for _, fp := range deps.Filepaths {
		if err := ValidateFilepathAlternativesWithProbe(fp, invowkDir, probe); err != nil {
			filepathErrors = append(filepathErrors, dependencyMessageFromDetail(err.Error()))
		}
	}

	if len(filepathErrors) > 0 {
		return &DependencyError{
			CommandName:      ctx.Command.Name,
			MissingFilepaths: filepathErrors,
		}
	}

	return nil
}

// ValidateFilepathAlternatives checks if any of the alternative paths exists and has the required permissions.
// Returns nil (success) if any alternative satisfies all requirements.
func ValidateFilepathAlternatives(fp invowkfile.FilepathDependency, invowkDir types.FilesystemPath) error {
	return ValidateFilepathAlternativesWithProbe(fp, invowkDir, nil)
}

// ValidateFilepathAlternativesWithProbe checks filepath alternatives through an injectable probe.
func ValidateFilepathAlternativesWithProbe(fp invowkfile.FilepathDependency, invowkDir types.FilesystemPath, probe HostProbe) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - %w", ErrNoPathAlternatives)
	}
	if probe == nil {
		return ErrHostProbeRequired
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		altPathStr := string(altPath)
		resolvedPath := resolveHostFilepathAlternative(invowkDir, altPathStr)
		if err := probe.CheckFilepath(types.FilesystemPath(altPathStr), types.FilesystemPath(resolvedPath), fp); err == nil { //goplint:ignore -- path from CUE alternatives list
			// Success! This alternative satisfies the dependency
			return nil
		} else {
			allErrors = append(allErrors, err.Error())
		}
	}

	return formatHostFilepathError(fp.Alternatives, allErrors)
}

//goplint:ignore -- helper evaluates transient container path strings from dependency alternatives.
func validateContainerFilepathAlternative(altPath string, fp invowkfile.FilepathDependency, rt runtime.Runtime, ctx *runtime.ExecutionContext) (string, error) {
	checkScript := buildContainerFilepathCheckScript(fp, altPath)
	validationCtx, _, stderr := NewContainerValidationContext(ctx, checkScript)

	result := rt.Execute(validationCtx)
	if result.Error != nil {
		return "", fmt.Errorf("  • %w for path %s: %w", ErrContainerValidationFailed, altPath, result.Error)
	}
	if err := CheckTransientExitCode(result, altPath); err != nil {
		return "", err
	}
	if result.ExitCode == 0 {
		return "", nil
	}

	detail := "not found or permission denied in container"
	if stderrStr := strings.TrimSpace(stderr.String()); stderrStr != "" {
		detail = stderrStr
	}
	return detail, nil
}

//goplint:ignore -- helper builds a shell probe from transient dependency path strings.
func buildContainerFilepathCheckScript(fp invowkfile.FilepathDependency, altPath string) string {
	escapedPath := ShellEscapeSingleQuote(altPath)
	checks := []string{fmt.Sprintf("test -e '%s'", escapedPath)}
	if fp.Readable {
		checks = append(checks, fmt.Sprintf("test -r '%s'", escapedPath))
	}
	if fp.Writable {
		checks = append(checks, fmt.Sprintf("test -w '%s'", escapedPath))
	}
	if fp.Executable {
		checks = append(checks, fmt.Sprintf("test -x '%s'", escapedPath))
	}
	return strings.Join(checks, " && ")
}

//goplint:ignore -- helper formats transient aggregated filepath probe output.
func formatContainerFilepathError(alternatives []invowkfile.FilesystemPath, allErrors []string) error {
	if len(alternatives) == 1 {
		return fmt.Errorf("  • %s", allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements in container:\n      - %s", strings.Join(allErrors, "\n      - "))
}

//goplint:ignore -- helper resolves transient dependency path strings against a validated base dir.
func resolveHostFilepathAlternative(invowkDir types.FilesystemPath, altPath string) string {
	if filepath.IsAbs(altPath) {
		return altPath
	}
	return filepath.Join(string(invowkDir), altPath)
}

//goplint:ignore -- helper formats transient aggregated filepath probe output.
func formatHostFilepathError(alternatives []invowkfile.FilesystemPath, allErrors []string) error {
	if len(alternatives) == 1 {
		return fmt.Errorf("  • %s", allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements:\n      - %s", strings.Join(allErrors, "\n      - "))
}
