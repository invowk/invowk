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
)

// checkFilepathDependenciesInContainer verifies all required files/directories exist inside the container.
// Called only for container runtime (caller guards non-container early return).
func checkFilepathDependenciesInContainer(deps *invowkfile.DependsOn, invowkfilePath string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("container runtime not available for filepath validation")
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range deps.Filepaths {
		if err := validateFilepathInContainer(fp, invowkDir, rt, ctx); err != nil {
			filepathErrors = append(filepathErrors, err.Error())
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
func validateFilepathInContainer(fp invowkfile.FilepathDependency, invowkDir string, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		// Shell-safe single-quote escaping for paths
		escapedPath := shellEscapeSingleQuote(altPath)

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

		validationCtx, _, _ := newContainerValidationContext(ctx, checkScript)

		result := rt.Execute(validationCtx)
		if result.Error != nil {
			return fmt.Errorf("  • container validation failed for path %s: %w", altPath, result.Error)
		}
		if result.ExitCode == 0 {
			return nil
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: not found or permission denied in container", altPath))
	}

	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s - %s", fp.Alternatives[0], allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements in container:\n      - %s", strings.Join(allErrors, "\n      - "))
}

// checkHostFilepathDependencies verifies all required files/directories exist on the HOST filesystem.
// Always uses native validation regardless of selected runtime.
func checkHostFilepathDependencies(deps *invowkfile.DependsOn, invowkfilePath string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range deps.Filepaths {
		if err := validateFilepathAlternatives(fp, invowkDir); err != nil {
			filepathErrors = append(filepathErrors, err.Error())
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

// checkFilepathDependencies verifies all required files/directories exist with proper permissions (native-only fallback).
func checkFilepathDependencies(cmd *invowkfile.Command, invowkfilePath string) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invowkfilePath)

	for _, fp := range cmd.DependsOn.Filepaths {
		if err := validateFilepathAlternatives(fp, invowkDir); err != nil {
			filepathErrors = append(filepathErrors, err.Error())
		}
	}

	if len(filepathErrors) > 0 {
		return &DependencyError{
			CommandName:      cmd.Name,
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
		// Resolve path relative to invowkfile if not absolute
		resolvedPath := altPath
		if !filepath.IsAbs(altPath) {
			resolvedPath = filepath.Join(invowkDir, altPath)
		}

		if err := validateSingleFilepath(altPath, resolvedPath, fp); err == nil {
			// Success! This alternative satisfies the dependency
			return nil
		} else {
			allErrors = append(allErrors, fmt.Sprintf("%s: %s", altPath, err.Error()))
		}
	}

	// None of the alternatives satisfied the requirements
	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s - %s", fp.Alternatives[0], allErrors[0])
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

// isReadable checks if a path is readable (cross-platform)
func isReadable(path string, info os.FileInfo) bool {
	// Try to open the file/directory for reading
	if info.IsDir() {
		f, err := os.Open(path)
		if err != nil {
			return false
		}
		_ = f.Close() // Readability check; close error non-critical
		return true
	}
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Readability check; close error non-critical
	return true
}

// isWritable checks if a path is writable (cross-platform)
func isWritable(path string, info os.FileInfo) bool {
	// For directories, try to create a temp file
	if info.IsDir() {
		testFile := filepath.Join(path, ".invowk_write_test")
		f, err := os.Create(testFile)
		if err != nil {
			return false
		}
		_ = f.Close()           // Test file; error non-critical
		_ = os.Remove(testFile) // Cleanup test file; error non-critical
		return true
	}
	// For files, try to open for writing
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close() // Probe only; error non-critical
	return true
}

// isExecutable checks if a path is executable (cross-platform)
func isExecutable(path string, info os.FileInfo) bool {
	// On Windows, check file extension
	if goruntime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		execExts := []string{".exe", ".cmd", ".bat", ".com", ".ps1"}
		if slices.Contains(execExts, ext) {
			return true
		}
		// Also check PATHEXT environment variable
		pathext := os.Getenv("PATHEXT")
		if pathext != "" {
			pathExtList := strings.Split(strings.ToLower(pathext), ";")
			if slices.Contains(pathExtList, ext) {
				return true
			}
		}
		return false
	}

	// On Unix-like systems, check execute permission bit
	mode := info.Mode()
	return mode&0o111 != 0
}
