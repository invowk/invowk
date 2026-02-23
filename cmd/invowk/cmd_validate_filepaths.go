// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/platform"
)

// checkFilepathDependenciesInContainer verifies all required files/directories exist inside the container.
// Called only for container runtime (caller guards non-container early return).
func checkFilepathDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("container runtime not available for filepath validation")
	}

	var filepathErrors []DependencyMessage

	for _, fp := range deps.Filepaths {
		if err := validateFilepathInContainer(fp, rt, ctx); err != nil {
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

// validateFilepathInContainer validates a filepath dependency within a container.
// The runtime is passed directly (hoisted by caller) to avoid redundant registry lookups.
func validateFilepathInContainer(fp invowkfile.FilepathDependency, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		altPathStr := string(altPath)
		// Shell-safe single-quote escaping for paths
		escapedPath := shellEscapeSingleQuote(altPathStr)

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

		validationCtx, _, stderr := newContainerValidationContext(ctx, checkScript)

		result := rt.Execute(validationCtx)
		if result.Error != nil {
			return fmt.Errorf("  • container validation failed for path %s: %w", altPath, result.Error)
		}
		if err := checkTransientExitCode(result, altPathStr); err != nil {
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

// checkHostFilepathDependencies verifies all required files/directories exist on the HOST filesystem.
// Always uses native validation regardless of selected runtime.
func checkHostFilepathDependencies(deps *invowkfile.DependsOn, invowkfilePath string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []DependencyMessage
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range deps.Filepaths {
		if err := validateFilepathAlternatives(fp, invowkDir); err != nil {
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

// validateFilepathAlternatives checks if any of the alternative paths exists and has the required permissions
// Returns nil (success) if any alternative satisfies all requirements
func validateFilepathAlternatives(fp invowkfile.FilepathDependency, invowkDir string) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		altPathStr := string(altPath)
		// Resolve path relative to invowkfile if not absolute
		resolvedPath := altPathStr
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(invowkDir, resolvedPath)
		}

		if err := validateSingleFilepath(altPathStr, resolvedPath, fp); err == nil {
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

// validateSingleFilepath checks if a single filepath exists and has the required permissions
func validateSingleFilepath(displayPath, resolvedPath string, fp invowkfile.FilepathDependency) error {
	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("path does not exist")
	}
	if err != nil {
		return fmt.Errorf("cannot access path: %w", err)
	}

	var permErrors []string

	// Check readable permission
	if fp.Readable {
		if !isReadable(resolvedPath, info) {
			permErrors = append(permErrors, "read")
		}
	}

	// Check writable permission
	if fp.Writable {
		if !isWritable(resolvedPath, info) {
			permErrors = append(permErrors, "write")
		}
	}

	// Check executable permission
	if fp.Executable {
		if !isExecutable(resolvedPath, info) {
			permErrors = append(permErrors, "execute")
		}
	}

	if len(permErrors) > 0 {
		return fmt.Errorf("missing permissions: %s", strings.Join(permErrors, ", "))
	}

	return nil
}

// isReadable checks if a path is readable (cross-platform).
func isReadable(path string, info os.FileInfo) bool {
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

// isWritable checks if a path is writable (cross-platform).
// For directories, creates a temp file to verify write access.
// For files, opens in write mode.
func isWritable(path string, info os.FileInfo) bool {
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

// isExecutable checks if a path is executable (cross-platform).
//
// On Windows, executability is determined by file extension (PATHEXT convention),
// with a readability probe as a best-effort accessibility check to catch
// obviously ACL-denied files. For directories, Windows treats them as
// "executable" if they are accessible (openable), which is analogous to —
// but not identical to — Unix directory execute (traverse) permission.
//
// On Unix-like systems, checks whether any execute bit (owner, group, or other)
// is set. This is a permissive heuristic — it does not verify that the current
// user specifically has execute permission.
func isExecutable(path string, info os.FileInfo) bool {
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
