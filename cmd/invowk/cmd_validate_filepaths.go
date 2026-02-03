// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
)

// checkFilepathDependenciesWithRuntime verifies all required files/directories exist
// The validation method depends on the runtime:
// - native: check against host filesystem
// - virtual: check against host filesystem (virtual shell still uses host fs)
// - container: check within the container filesystem
func checkFilepathDependenciesWithRuntime(deps *invkfile.DependsOn, invkfilePath string, runtimeMode invkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invkfilePath)

	for _, fp := range deps.Filepaths {
		var err error
		switch runtimeMode {
		case invkfile.RuntimeContainer:
			err = validateFilepathInContainer(fp, invowkDir, registry, ctx)
		case invkfile.RuntimeNative, invkfile.RuntimeVirtual:
			// Native and virtual use host filesystem
			err = validateFilepathAlternatives(fp, invowkDir)
		}
		if err != nil {
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

// validateFilepathInContainer validates a filepath dependency within a container
func validateFilepathInContainer(fp invkfile.FilepathDependency, invowkDir string, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • container runtime not available")
	}

	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		// Build a check script for this path
		var checks []string

		// Basic existence check
		checks = append(checks, fmt.Sprintf("test -e '%s'", altPath))

		if fp.Readable {
			checks = append(checks, fmt.Sprintf("test -r '%s'", altPath))
		}
		if fp.Writable {
			checks = append(checks, fmt.Sprintf("test -w '%s'", altPath))
		}
		if fp.Executable {
			checks = append(checks, fmt.Sprintf("test -x '%s'", altPath))
		}

		checkScript := strings.Join(checks, " && ")

		// Create a minimal context for validation
		var stdout, stderr bytes.Buffer
		validationCtx := &runtime.ExecutionContext{
			Command:         ctx.Command,
			Invkfile:        ctx.Invkfile,
			SelectedImpl:    &invkfile.Implementation{Script: checkScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}},
			SelectedRuntime: invkfile.RuntimeContainer,
			Context:         ctx.Context,
			IO:              runtime.IOContext{Stdout: &stdout, Stderr: &stderr},
			Env:             runtime.DefaultEnv(),
		}

		result := rt.Execute(validationCtx)
		if result.ExitCode == 0 {
			// This alternative satisfies the dependency
			return nil
		}
		allErrors = append(allErrors, fmt.Sprintf("%s: not found or permission denied in container", altPath))
	}

	// None of the alternatives satisfied the requirements
	if len(fp.Alternatives) == 1 {
		return fmt.Errorf("  • %s - %s", fp.Alternatives[0], allErrors[0])
	}
	return fmt.Errorf("  • none of the alternatives satisfied the requirements in container:\n      - %s", strings.Join(allErrors, "\n      - "))
}

// checkFilepathDependencies verifies all required files/directories exist with proper permissions (legacy - uses native)
func checkFilepathDependencies(cmd *invkfile.Command, invkfilePath string) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.Filepaths) == 0 {
		return nil
	}

	var filepathErrors []string
	invowkDir := filepath.Dir(invkfilePath)

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
func validateFilepathAlternatives(fp invkfile.FilepathDependency, invowkDir string) error {
	if len(fp.Alternatives) == 0 {
		return fmt.Errorf("  • (no paths specified) - at least one path must be provided in alternatives")
	}

	var allErrors []string

	for _, altPath := range fp.Alternatives {
		// Resolve path relative to invkfile if not absolute
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
func validateSingleFilepath(displayPath, resolvedPath string, fp invkfile.FilepathDependency) error {
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
	if isWindows() {
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
