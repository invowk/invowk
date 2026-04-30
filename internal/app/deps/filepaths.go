// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/platform"
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
	return CheckHostFilepathDependenciesWithProbe(deps, invowkfilePath, ctx, newDefaultHostProbe())
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
	return ValidateFilepathAlternativesWithProbe(fp, invowkDir, newDefaultHostProbe())
}

// ValidateFilepathAlternativesWithProbe checks filepath alternatives through an injectable probe.
func ValidateFilepathAlternativesWithProbe(fp invowkfile.FilepathDependency, invowkDir types.FilesystemPath, probe HostProbe) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - %w", ErrNoPathAlternatives)
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

// ValidateSingleFilepath checks if a single filepath exists and has the required permissions.
// displayPath is used for error messages; resolvedPath is the absolute path for filesystem checks.
func ValidateSingleFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error {
	resolvedPathStr := string(resolvedPath)

	// Check if path exists
	info, err := os.Stat(resolvedPathStr)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", displayPath, ErrPathNotExists)
	}
	if err != nil {
		return fmt.Errorf("%s: cannot access path: %w", displayPath, err)
	}

	var permErrors []string

	// Check readable permission
	if fp.Readable {
		if !IsReadable(resolvedPathStr, info) {
			permErrors = append(permErrors, "read")
		}
	}

	// Check writable permission
	if fp.Writable {
		if !IsWritable(resolvedPathStr, info) {
			permErrors = append(permErrors, "write")
		}
	}

	// Check executable permission
	if fp.Executable {
		if !IsExecutable(resolvedPathStr, info) {
			permErrors = append(permErrors, "execute")
		}
	}

	if len(permErrors) > 0 {
		return fmt.Errorf("%s: missing permissions: %s", displayPath, strings.Join(permErrors, ", "))
	}

	return nil
}

// IsReadable checks if a path is readable (cross-platform).
func IsReadable(path string, info os.FileInfo) bool {
	if info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		_ = f.Close() // Readability probe; close error non-critical
		return true
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Readability probe; close error non-critical
	return true
}

// IsWritable checks if a path is writable (cross-platform).
// For directories, creates a temp file to verify write access.
// For files, opens in write mode.
func IsWritable(path string, info os.FileInfo) bool {
	if info.IsDir() {
		// os.CreateTemp generates a unique name, avoiding collisions with
		// user files and reducing leftover risk if cleanup fails.
		f, err := os.CreateTemp(path, ".invowk-wcheck-*")
		if err != nil {
			return false
		}
		tmpName := f.Name()
		defer func() { _ = os.Remove(tmpName) }() // Best-effort cleanup; runs even if Close fails
		_ = f.Close()                             // Probe file; close error non-critical
		return true
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Probe only; error non-critical
	return true
}

// IsExecutable checks if a path is executable (cross-platform).
//
// On Windows, executability is determined by file extension (PATHEXT convention),
// with a readability probe as a best-effort accessibility check to catch
// obviously ACL-denied files. For directories, Windows treats them as
// "executable" if they are accessible (openable), which is analogous to --
// but not identical to -- Unix directory execute (traverse) permission.
//
// On Unix-like systems, checks whether any execute bit (owner, group, or other)
// is set. This is a permissive heuristic -- it does not verify that the current
// user specifically has execute permission.
func IsExecutable(path string, info os.FileInfo) bool {
	if goruntime.GOOS == platform.Windows {
		return isExecutableOnWindows(path, info)
	}

	// On Unix-like systems, check execute permission bit
	mode := info.Mode()
	return mode&0o111 != 0
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

//goplint:ignore -- helper inspects OS-native path strings returned by os/filepath.
func isExecutableOnWindows(path string, info os.FileInfo) bool {
	if info.IsDir() {
		return canOpenPath(path)
	}
	if !windowsPathHasExecutableExtension(path) {
		return false
	}
	return canOpenReadOnly(path)
}

//goplint:ignore -- helper inspects OS-native path strings returned by os/filepath.
func windowsPathHasExecutableExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	execExts := []string{".exe", ".cmd", ".bat", ".com", ".ps1"}
	if slices.Contains(execExts, ext) {
		return true
	}

	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		return false
	}

	for pathExt := range strings.SplitSeq(strings.ToLower(pathext), ";") {
		if pathExt != "" && pathExt == ext {
			return true
		}
	}
	return false
}

//goplint:ignore -- helper probes OS-native path strings returned by os/filepath.
func canOpenPath(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close() // Probe only; close error non-critical
	return true
}

//goplint:ignore -- helper probes OS-native path strings returned by os/filepath.
func canOpenReadOnly(path string) bool {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Probe only; close error non-critical
	return true
}
