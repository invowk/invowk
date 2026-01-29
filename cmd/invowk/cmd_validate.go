// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// validateDependencies validates merged dependencies for a command.
// Dependencies are merged from root-level, command-level, and implementation-level, and
// validated according to the selected runtime:
// - native: validated against the native standard shell from the host
// - virtual: validated against invowk's built-in sh interpreter with core utils
// - container: validated against the container's default shell from within the container
//
// Note: `depends_on.cmds` is an existence check only. Invowk validates that referenced
// commands are discoverable (in this invkfile, modules, or configured search paths),
// but it does not execute them automatically.
func validateDependencies(cmdInfo *discovery.CommandInfo, registry *runtime.Registry, parentCtx *runtime.ExecutionContext) error {
	// Merge root-level, command-level, and implementation-level dependencies
	mergedDeps := invkfile.MergeDependsOnAll(cmdInfo.Invkfile.DependsOn, cmdInfo.Command.DependsOn, parentCtx.SelectedImpl.DependsOn)

	if mergedDeps == nil {
		return nil
	}

	// Get the selected runtime for context-aware validation
	selectedRuntime := parentCtx.SelectedRuntime

	// FIRST: Check env var dependencies (host-only, validated BEFORE invowk sets any env vars)
	// We capture the user's environment here to ensure we validate against their actual env,
	// not any variables that invowk might set from the 'env' construct
	if err := checkEnvVarDependencies(mergedDeps, captureUserEnv(), parentCtx); err != nil {
		return err
	}

	// Then check tool dependencies (runtime-aware)
	if err := checkToolDependenciesWithRuntime(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check filepath dependencies (runtime-aware)
	if err := checkFilepathDependenciesWithRuntime(mergedDeps, cmdInfo.Invkfile.FilePath, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check capability dependencies (host-only, not runtime-aware)
	if err := checkCapabilityDependencies(mergedDeps, parentCtx); err != nil {
		return err
	}

	// Then check custom check dependencies (runtime-aware)
	if err := checkCustomCheckDependencies(mergedDeps, selectedRuntime, registry, parentCtx); err != nil {
		return err
	}

	// Then check command dependencies (existence-only; these are not executed automatically)
	// Get module ID from metadata (nil for non-module invkfiles)
	currentModule := ""
	if cmdInfo.Invkfile.Metadata != nil {
		currentModule = cmdInfo.Invkfile.Metadata.Module
	}
	if err := checkCommandDependenciesExist(mergedDeps, currentModule, parentCtx); err != nil {
		return err
	}

	return nil
}

func checkCommandDependenciesExist(deps *invkfile.DependsOn, currentModule string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	cfg := config.Get()
	disc := discovery.New(cfg)

	availableCommands, err := disc.DiscoverCommands()
	if err != nil {
		return fmt.Errorf("failed to discover commands for dependency validation: %w", err)
	}

	available := make(map[string]struct{}, len(availableCommands))
	for _, cmd := range availableCommands {
		available[cmd.Name] = struct{}{}
	}

	var commandErrors []string

	for _, dep := range deps.Commands {
		var alternatives []string
		for _, alt := range dep.Alternatives {
			alt = strings.TrimSpace(alt)
			if alt == "" {
				continue
			}
			alternatives = append(alternatives, alt)
		}
		if len(alternatives) == 0 {
			continue
		}

		// OR semantics: any alternative being discoverable satisfies this dependency.
		found := false
		for _, alt := range alternatives {
			if _, ok := available[alt]; ok {
				found = true
				break
			}

			// Also allow referencing commands from the current invkfile without a module prefix.
			qualified := currentModule + " " + alt
			if _, ok := available[qualified]; ok {
				found = true
				break
			}
		}

		if !found {
			if len(alternatives) == 1 {
				commandErrors = append(commandErrors, fmt.Sprintf("  • %s - command not found", alternatives[0]))
			} else {
				commandErrors = append(commandErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(alternatives, ", ")))
			}
		}
	}

	if len(commandErrors) > 0 {
		return &DependencyError{
			CommandName:     ctx.Command.Name,
			MissingCommands: commandErrors,
		}
	}

	return nil
}

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

// validateCustomCheckOutput validates custom check script output against expected values
func validateCustomCheckOutput(check invkfile.CustomCheck, outputStr string, execErr error) error {
	// Determine expected exit code (default: 0)
	expectedCode := 0
	if check.ExpectedCode != nil {
		expectedCode = *check.ExpectedCode
	}

	// Check exit code
	actualCode := 0
	if execErr != nil {
		var exitErr *exec.ExitError
		if errors.As(execErr, &exitErr) {
			actualCode = exitErr.ExitCode()
		} else {
			// Try to get exit code from error message for non-native runtimes
			actualCode = 1 // Default to 1 for errors
		}
	}

	if actualCode != expectedCode {
		return fmt.Errorf("  • %s - check script returned exit code %d, expected %d", check.Name, actualCode, expectedCode)
	}

	// Check output pattern if specified
	if check.ExpectedOutput != "" {
		matched, err := regexp.MatchString(check.ExpectedOutput, outputStr)
		if err != nil {
			return fmt.Errorf("  • %s - invalid regex pattern '%s': %w", check.Name, check.ExpectedOutput, err)
		}
		if !matched {
			return fmt.Errorf("  • %s - check script output '%s' does not match pattern '%s'", check.Name, outputStr, check.ExpectedOutput)
		}
	}

	return nil
}

// checkCustomCheckDependencies validates all custom check scripts.
// The validation method depends on the runtime:
// - native: executed using the host's native shell
// - virtual: executed using invowk's built-in sh interpreter
// - container: executed within the container environment
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func checkCustomCheckDependencies(deps *invkfile.DependsOn, runtimeMode invkfile.RuntimeMode, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range deps.CustomChecks {
		checks := checkDep.GetChecks()
		var lastErr error
		passed := false

		for _, check := range checks {
			var err error
			switch runtimeMode {
			case invkfile.RuntimeContainer:
				err = validateCustomCheckInContainer(check, registry, ctx)
			case invkfile.RuntimeVirtual:
				err = validateCustomCheckInVirtual(check, registry, ctx)
			case invkfile.RuntimeNative:
				err = validateCustomCheckNative(check)
			}
			if err == nil {
				passed = true
				break // Early return on first passing check
			}
			lastErr = err
		}

		if !passed && lastErr != nil {
			if len(checks) == 1 {
				checkErrors = append(checkErrors, lastErr.Error())
			} else {
				// Collect all check names for the error message
				names := make([]string, len(checks))
				for i, c := range checks {
					names[i] = c.Name
				}
				checkErrors = append(checkErrors, fmt.Sprintf("  • none of custom checks [%s] passed", strings.Join(names, ", ")))
			}
		}
	}

	if len(checkErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.Command.Name,
			FailedCustomChecks: checkErrors,
		}
	}

	return nil
}

// validateCustomCheckNative runs a custom check script using the native shell
func validateCustomCheckNative(check invkfile.CustomCheck) error {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", check.CheckScript)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	return validateCustomCheckOutput(check, outputStr, err)
}

// validateCustomCheckInVirtual runs a custom check script using the virtual runtime
func validateCustomCheckInVirtual(check invkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeVirtual)
	if err != nil {
		// Fall back to native validation if virtual runtime not available
		return validateCustomCheckNative(check)
	}

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: check.CheckScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeVirtual}}},
		SelectedRuntime: invkfile.RuntimeVirtual,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)
	outputStr := strings.TrimSpace(stdout.String() + stderr.String())

	return validateCustomCheckOutput(check, outputStr, result.Error)
}

// validateCustomCheckInContainer runs a custom check script within a container
func validateCustomCheckInContainer(check invkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", check.Name)
	}

	// Create a minimal context for validation
	var stdout, stderr bytes.Buffer
	validationCtx := &runtime.ExecutionContext{
		Command:         ctx.Command,
		Invkfile:        ctx.Invkfile,
		SelectedImpl:    &invkfile.Implementation{Script: check.CheckScript, Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}}},
		SelectedRuntime: invkfile.RuntimeContainer,
		Stdout:          &stdout,
		Stderr:          &stderr,
		Context:         ctx.Context,
		ExtraEnv:        make(map[string]string),
	}

	result := rt.Execute(validationCtx)
	outputStr := strings.TrimSpace(stdout.String() + stderr.String())

	return validateCustomCheckOutput(check, outputStr, result.Error)
}

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
			Stdout:          &stdout,
			Stderr:          &stderr,
			Context:         ctx.Context,
			ExtraEnv:        make(map[string]string),
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

// checkToolDependencies verifies all required tools are available in PATH (legacy - uses native)
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

// checkCustomChecks verifies all custom check scripts pass (legacy - uses native).
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func checkCustomChecks(cmd *invkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range cmd.DependsOn.CustomChecks {
		checks := checkDep.GetChecks()
		var lastErr error
		passed := false

		for _, check := range checks {
			if err := validateCustomCheckNative(check); err == nil {
				passed = true
				break // Early return on first passing check
			} else {
				lastErr = err
			}
		}

		if !passed && lastErr != nil {
			if len(checks) == 1 {
				checkErrors = append(checkErrors, lastErr.Error())
			} else {
				names := make([]string, len(checks))
				for i, c := range checks {
					names[i] = c.Name
				}
				checkErrors = append(checkErrors, fmt.Sprintf("  • none of custom checks [%s] passed", strings.Join(names, ", ")))
			}
		}
	}

	if len(checkErrors) > 0 {
		return &DependencyError{
			CommandName:        cmd.Name,
			FailedCustomChecks: checkErrors,
		}
	}

	return nil
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

// checkCapabilityDependencies verifies all required system capabilities are available.
// Capabilities are always checked against the host system, regardless of the runtime mode.
// For container runtimes, these checks represent the host's capabilities, not the container's.
// Each CapabilityDependency contains a list of alternatives; if any alternative is satisfied, the dependency is met.
func checkCapabilityDependencies(deps *invkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	var capabilityErrors []string

	// Track seen capability sets to detect duplicates (they're just skipped, not an error)
	seen := make(map[string]bool)

	for _, cap := range deps.Capabilities {
		// Create a unique key for this set of alternatives
		key := strings.Join(func() []string {
			s := make([]string, len(cap.Alternatives))
			for i, alt := range cap.Alternatives {
				s[i] = string(alt)
			}
			return s
		}(), ",")

		// Skip duplicates
		if seen[key] {
			continue
		}
		seen[key] = true

		var lastErr error
		found := false
		for _, alt := range cap.Alternatives {
			if err := invkfile.CheckCapability(alt); err == nil {
				found = true
				break // Early return on first match
			} else {
				lastErr = err
			}
		}

		if !found && lastErr != nil {
			if len(cap.Alternatives) == 1 {
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • %s", lastErr.Error()))
			} else {
				alts := make([]string, len(cap.Alternatives))
				for i, alt := range cap.Alternatives {
					alts[i] = string(alt)
				}
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • none of capabilities [%s] satisfied", strings.Join(alts, ", ")))
			}
		}
	}

	if len(capabilityErrors) > 0 {
		return &DependencyError{
			CommandName:         ctx.Command.Name,
			MissingCapabilities: capabilityErrors,
		}
	}

	return nil
}

// checkEnvVarDependencies verifies all required environment variables exist.
// IMPORTANT: This function validates against the provided userEnv map, which should be captured
// at the START of execution before invowk sets any command-level env vars.
// This ensures the check validates the user's actual environment, not variables set by invowk.
// Each EnvVarDependency contains alternatives with OR semantics (early return on first match).
func checkEnvVarDependencies(deps *invkfile.DependsOn, userEnv map[string]string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	var envVarErrors []string

	for _, envVar := range deps.EnvVars {
		var lastErr error
		found := false

		for _, alt := range envVar.Alternatives {
			// Trim whitespace from name as per schema
			name := strings.TrimSpace(alt.Name)
			if name == "" {
				lastErr = fmt.Errorf("  • (empty) - environment variable name cannot be empty")
				continue
			}

			// Check if env var exists
			value, exists := userEnv[name]
			if !exists {
				lastErr = fmt.Errorf("  • %s - not set in environment", name)
				continue
			}

			// If validation pattern is specified, validate the value
			if alt.Validation != "" {
				matched, err := regexp.MatchString(alt.Validation, value)
				if err != nil {
					lastErr = fmt.Errorf("  • %s - invalid validation regex '%s': %w", name, alt.Validation, err)
					continue
				}
				if !matched {
					lastErr = fmt.Errorf("  • %s - value '%s' does not match required pattern '%s'", name, value, alt.Validation)
					continue
				}
			}

			// Env var exists and passes validation (if any)
			found = true
			break // Early return on first match
		}

		if !found && lastErr != nil {
			if len(envVar.Alternatives) == 1 {
				envVarErrors = append(envVarErrors, lastErr.Error())
			} else {
				// Collect all alternative names for the error message
				names := make([]string, len(envVar.Alternatives))
				for i, alt := range envVar.Alternatives {
					names[i] = strings.TrimSpace(alt.Name)
				}
				envVarErrors = append(envVarErrors, fmt.Sprintf("  • none of [%s] found or passed validation", strings.Join(names, ", ")))
			}
		}
	}

	if len(envVarErrors) > 0 {
		return &DependencyError{
			CommandName:    ctx.Command.Name,
			MissingEnvVars: envVarErrors,
		}
	}

	return nil
}
