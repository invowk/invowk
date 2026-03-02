// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/runtime"
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
		return errors.New("container runtime not available for filepath validation")
	}

	var filepathErrors []DependencyMessage

	for _, fp := range deps.Filepaths {
		if err := ValidateFilepathInContainer(fp, rt, ctx); err != nil {
			filepathErrors = append(filepathErrors, DependencyMessage(err.Error()))
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
		return errors.New("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		altPathStr := string(altPath)
		// Shell-safe single-quote escaping for paths
		escapedPath := ShellEscapeSingleQuote(altPathStr)

		var checks []string
		checks = append(checks, fmt.Sprintf("test -e '%s'", escapedPath))

		if fp.Readable {
			checks = append(checks, fmt.Sprintf("test -r '%s'", escapedPath))
		}
		if fp.Writable {
			checks = append(checks, fmt.Sprintf("test -w '%s'", escapedPath))
		}
		if fp.Executable {
			checks = append(checks, fmt.Sprintf("test -x '%s'", escapedPath))
		}

		checkScript := strings.Join(checks, " && ")

		validationCtx, _, stderr := NewContainerValidationContext(ctx, checkScript)

		result := rt.Execute(validationCtx)
		if result.Error != nil {
			return fmt.Errorf("  • container validation failed for path %s: %w", altPath, result.Error)
		}
		if err := CheckTransientExitCode(result, altPathStr); err != nil {
			return err
		}
		if result.ExitCode == 0 {
			return nil
		}
		detail := "not found or permission denied in container"
		if stderrStr := strings.TrimSpace(stderr.String()); stderrStr != "" {
			detail = stderrStr
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, detail))
	}

	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s", allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements in container:\n      - %s", strings.Join(allErrors, "\n      - "))
}

// CheckHostFilepathDependencies verifies all required files/directories exist on the HOST filesystem.
// Always uses native validation regardless of selected runtime.
func CheckHostFilepathDependencies(deps *invowkfile.DependsOn, invowkfilePath types.FilesystemPath, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []DependencyMessage
	invowkDir := types.FilesystemPath(filepath.Dir(string(invowkfilePath))) //goplint:ignore -- derived from validated invowkfilePath

	for _, fp := range deps.Filepaths {
		if err := ValidateFilepathAlternatives(fp, invowkDir); err != nil {
			filepathErrors = append(filepathErrors, DependencyMessage(err.Error()))
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
	if len(fp.Alternatives) == 0 {
		return errors.New("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		altPathStr := string(altPath)
		// Resolve path relative to invowkfile if not absolute
		resolvedPath := altPathStr
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(string(invowkDir), resolvedPath)
		}

		if err := ValidateSingleFilepath(types.FilesystemPath(altPathStr), types.FilesystemPath(resolvedPath), fp); err == nil { //goplint:ignore -- path from CUE alternatives list
			// Success! This alternative satisfies the dependency
			return nil
		} else {
			allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, err.Error()))
		}
	}

	// None of the alternatives satisfied the requirements
	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s", allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements:\n      - %s", strings.Join(allErrors, "\n      - "))
}

// ValidateSingleFilepath checks if a single filepath exists and has the required permissions.
// displayPath is used for error messages; resolvedPath is the absolute path for filesystem checks.
func ValidateSingleFilepath(displayPath, resolvedPath types.FilesystemPath, fp invowkfile.FilepathDependency) error {
	resolvedPathStr := string(resolvedPath)

	// Check if path exists
	info, err := os.Stat(resolvedPathStr)
	if os.IsNotExist(err) {
		return errors.New("path does not exist")
	}
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
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
		return fmt.Errorf("missing permissions: %s", strings.Join(permErrors, ", "))
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
		// Directories on Windows: executable means traversable (accessible).
		if info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return false
			}
			_ = f.Close() // Traversability probe; close error non-critical
			return true
		}

		// Files on Windows: check extension (PATHEXT convention) AND readability.
		ext := strings.ToLower(filepath.Ext(path))
		execExts := []string{".exe", ".cmd", ".bat", ".com", ".ps1"}
		hasExecExt := slices.Contains(execExts, ext)
		if !hasExecExt {
			pathext := os.Getenv("PATHEXT")
			if pathext != "" {
				for pathExt := range strings.SplitSeq(strings.ToLower(pathext), ";") {
					if pathExt == "" {
						continue
					}
					if pathExt == ext {
						hasExecExt = true
						break
					}
				}
			}
		}
		if !hasExecExt {
			return false
		}

		// Best-effort accessibility check: a file denied by ACLs will fail here
		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			return false
		}
		_ = f.Close() // Executability probe; close error non-critical
		return true
	}

	// On Unix-like systems, check execute permission bit
	mode := info.Mode()
	return mode&0o111 != 0
}
