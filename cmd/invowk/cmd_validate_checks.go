// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
)

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
		Context:         ctx.Context,
		IO:              runtime.IOContext{Stdout: &stdout, Stderr: &stderr},
		Env:             runtime.DefaultEnv(),
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
		Context:         ctx.Context,
		IO:              runtime.IOContext{Stdout: &stdout, Stderr: &stderr},
		Env:             runtime.DefaultEnv(),
	}

	result := rt.Execute(validationCtx)
	outputStr := strings.TrimSpace(stdout.String() + stderr.String())

	return validateCustomCheckOutput(check, outputStr, result.Error)
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
