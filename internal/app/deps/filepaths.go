// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// CheckFilepathDependenciesInContainer verifies all required files/directories exist inside the container.
// Called only for container runtime (caller guards non-container early return).
func CheckFilepathDependenciesInContainer(deps *invowkfile.DependsOn, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}

	var filepathErrors []DependencyMessage

	for _, fp := range deps.Filepaths {
		if err := probe.CheckFilepath(fp); err != nil {
			filepathErrors = append(filepathErrors, dependencyMessageFromDetail(err.Error()))
		}
	}

	if len(filepathErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			MissingFilepaths:   filepathErrors,
			StructuredFailures: dependencyFailures(DependencyFailureFilepath, filepathErrors),
		}
	}

	return nil
}

// CheckHostFilepathDependencies verifies all required files/directories exist on the HOST filesystem.
// Always uses native validation regardless of selected runtime.
func CheckHostFilepathDependencies(deps *invowkfile.DependsOn, invowkfilePath types.FilesystemPath, ctx ExecutionContext) error {
	return CheckHostFilepathDependenciesWithProbe(deps, invowkfilePath, ctx, nil)
}

// CheckHostFilepathDependenciesWithProbe verifies host filepath dependencies through an injectable probe.
func CheckHostFilepathDependenciesWithProbe(deps *invowkfile.DependsOn, invowkfilePath types.FilesystemPath, ctx ExecutionContext, probe HostProbe) error {
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
			CommandName:        ctx.CommandName,
			MissingFilepaths:   filepathErrors,
			StructuredFailures: dependencyFailures(DependencyFailureFilepath, filepathErrors),
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
		return fmt.Errorf("(no paths specified) - %w", ErrNoPathAlternatives)
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
		return fmt.Errorf("%s", allErrors[0])
	}
	return fmt.Errorf("none of the alternatives satisfied the requirements: %s", strings.Join(allErrors, "; "))
}
