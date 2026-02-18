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

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// shellEscapeSingleQuote escapes single quotes for safe use inside shell single-quoted arguments.
// Each embedded single-quote is replaced with the shell idiom that closes the current quoting,
// adds a backslash-escaped literal quote, and reopens single-quoting.
func shellEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// validateCustomCheckOutput validates custom check script output against expected values
func validateCustomCheckOutput(check invowkfile.CustomCheck, outputStr string, execErr error) error {
	// Determine expected exit code (default: 0)
	expectedCode := 0
	if check.ExpectedCode != nil {
		expectedCode = *check.ExpectedCode
	}

	// Check exit code
	actualCode := 0
	if execErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](execErr); ok {
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

// checkCustomCheckDependenciesInContainer validates all custom check scripts inside the container.
// Called only for container runtime (caller guards non-container early return).
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func checkCustomCheckDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range deps.CustomChecks {
		checks := checkDep.GetChecks()
		found, lastErr := evaluateAlternatives(checks, func(check invowkfile.CustomCheck) error {
			return validateCustomCheckInContainer(check, registry, ctx)
		})

		if !found && lastErr != nil {
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
			CommandName:        ctx.Command.Name,
			FailedCustomChecks: checkErrors,
		}
	}

	return nil
}

// validateCustomCheckNative runs a custom check script using the native shell
func validateCustomCheckNative(check invowkfile.CustomCheck) error {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", check.CheckScript)
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	return validateCustomCheckOutput(check, outputStr, err)
}

// validateCustomCheckInContainer runs a custom check script within a container.
// Distinguishes infrastructure failures (container engine down) from script exit codes
// to prevent false-positive validation when the container never actually ran.
func validateCustomCheckInContainer(check invowkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - container runtime not available", check.Name)
	}

	validationCtx, stdout, stderr := newContainerValidationContext(ctx, check.CheckScript)

	result := rt.Execute(validationCtx)

	// Infrastructure failures must be surfaced immediately — if the container engine
	// failed, no check ever ran, so we must not fall through to exit code comparison.
	if result.Error != nil {
		var exitErr *exec.ExitError
		if !errors.As(result.Error, &exitErr) {
			return fmt.Errorf("  • %s - container validation failed: %w", check.Name, result.Error)
		}
	}

	outputStr := strings.TrimSpace(stdout.String() + stderr.String())
	return validateCustomCheckOutput(check, outputStr, result.Error)
}

// checkHostCustomCheckDependencies validates custom checks always using the native shell on the host.
// Host-level custom checks always run in the native shell, regardless of the selected runtime,
// ensuring host-side prerequisites are validated in a consistent, predictable environment.
func checkHostCustomCheckDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range deps.CustomChecks {
		checks := checkDep.GetChecks()
		found, lastErr := evaluateAlternatives(checks, validateCustomCheckNative)

		if !found && lastErr != nil {
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
			CommandName:        ctx.Command.Name,
			FailedCustomChecks: checkErrors,
		}
	}

	return nil
}

// checkEnvVarDependenciesInContainer validates env vars inside the container.
// Called only for container runtime (caller guards non-container early return).
func checkEnvVarDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("container runtime not available for env var validation")
	}

	var envVarErrors []string

	for _, envVar := range deps.EnvVars {
		found, lastErr := evaluateAlternatives(envVar.Alternatives, func(alt invowkfile.EnvVarCheck) error {
			name := strings.TrimSpace(alt.Name)
			if name == "" {
				return fmt.Errorf("  • (empty) - environment variable name cannot be empty")
			}

			// Defense-in-depth: validate env var name before shell interpolation
			if err := invowkfile.ValidateEnvVarName(name); err != nil {
				return fmt.Errorf("  • %s - invalid environment variable name", name)
			}

			// Check if env var is set inside container: test -n "${VAR+x}"
			checkScript := fmt.Sprintf("test -n \"${%s+x}\"", name)

			// If validation pattern specified, also check value
			if alt.Validation != "" {
				escapedValidation := shellEscapeSingleQuote(alt.Validation)
				checkScript = fmt.Sprintf("test -n \"${%s+x}\" && printf '%%s' \"$%s\" | grep -qE '%s'", name, name, escapedValidation)
			}

			var stdout, stderr bytes.Buffer
			validationCtx := &runtime.ExecutionContext{
				Command:         ctx.Command,
				Invowkfile:      ctx.Invowkfile,
				SelectedImpl:    &invowkfile.Implementation{Script: checkScript, Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}},
				SelectedRuntime: invowkfile.RuntimeContainer,
				Context:         ctx.Context,
				IO:              runtime.IOContext{Stdout: &stdout, Stderr: &stderr},
				Env:             runtime.DefaultEnv(),
			}

			result := rt.Execute(validationCtx)
			if result.Error != nil {
				return fmt.Errorf("container validation failed for env var %s: %w", name, result.Error)
			}
			if result.ExitCode == 0 {
				return nil
			}
			if alt.Validation != "" {
				return fmt.Errorf("  • %s - not set or value does not match pattern '%s' in container", name, alt.Validation)
			}
			return fmt.Errorf("  • %s - not set in container environment", name)
		})

		if !found && lastErr != nil {
			if len(envVar.Alternatives) == 1 {
				envVarErrors = append(envVarErrors, lastErr.Error())
			} else {
				names := make([]string, len(envVar.Alternatives))
				for i, alt := range envVar.Alternatives {
					names[i] = strings.TrimSpace(alt.Name)
				}
				envVarErrors = append(envVarErrors, fmt.Sprintf("  • none of [%s] found or passed validation in container", strings.Join(names, ", ")))
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

// checkCapabilityDependenciesInContainer validates capabilities inside the container.
// Called only for container runtime (caller guards non-container early return).
func checkCapabilityDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("container runtime not available for capability validation")
	}

	var capabilityErrors []string

	for _, capDep := range deps.Capabilities {
		found, lastErr := evaluateAlternatives(capDep.Alternatives, func(alt invowkfile.CapabilityName) error {
			checkScript := capabilityCheckScript(alt)
			if checkScript == "" {
				return fmt.Errorf("%s - unknown capability", string(alt))
			}

			validationCtx, _, _ := newContainerValidationContext(ctx, checkScript)

			result := rt.Execute(validationCtx)
			if result.Error != nil {
				return fmt.Errorf("container validation failed for capability %s: %w", string(alt), result.Error)
			}
			if result.ExitCode == 0 {
				return nil
			}
			return fmt.Errorf("%s - not available in container", string(alt))
		})

		if !found && lastErr != nil {
			if len(capDep.Alternatives) == 1 {
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • %s", lastErr.Error()))
			} else {
				alts := make([]string, len(capDep.Alternatives))
				for i, alt := range capDep.Alternatives {
					alts[i] = string(alt)
				}
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • none of capabilities [%s] satisfied in container", strings.Join(alts, ", ")))
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

// capabilityCheckScript returns a shell one-liner that checks whether a system capability
// is available. The returned script is suitable for execution in any POSIX shell environment.
func capabilityCheckScript(capName invowkfile.CapabilityName) string {
	switch capName {
	case invowkfile.CapabilityInternet:
		return "ping -c 1 -W 2 8.8.8.8 2>/dev/null || curl -sf --max-time 2 https://google.com >/dev/null 2>&1"
	case invowkfile.CapabilityContainers:
		return "command -v docker >/dev/null 2>&1 || command -v podman >/dev/null 2>&1"
	case invowkfile.CapabilityLocalAreaNetwork:
		// Check for any non-loopback network interface being up
		return "ip route 2>/dev/null | grep -q default || route -n 2>/dev/null | grep -q '^0.0.0.0'"
	case invowkfile.CapabilityTTY:
		return "test -t 0"
	}
	return ""
}

// checkCommandDependenciesInContainer validates command discoverability inside the container.
// Called only for container runtime (caller guards non-container early return).
// Runs `invowk internal check-cmd` inside the container to verify auto-provisioning worked.
func checkCommandDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("container runtime not available for command dependency validation")
	}

	var commandErrors []string

	for _, dep := range deps.Commands {
		var alternatives []string
		for _, alt := range dep.Alternatives {
			alt = strings.TrimSpace(alt)
			if alt != "" {
				alternatives = append(alternatives, alt)
			}
		}
		if len(alternatives) == 0 {
			continue
		}

		found, lastErr := evaluateAlternatives(alternatives, func(alt string) error {
			// Shell-safe single-quote escaping for command name
			escapedAlt := shellEscapeSingleQuote(alt)
			checkScript := fmt.Sprintf("invowk internal check-cmd '%s'", escapedAlt)

			validationCtx, _, stderr := newContainerValidationContext(ctx, checkScript)

			result := rt.Execute(validationCtx)
			if result.Error != nil {
				stderrStr := strings.TrimSpace(stderr.String())
				if stderrStr != "" {
					return fmt.Errorf("container validation failed for command %s: %w (%s)", alt, result.Error, stderrStr)
				}
				return fmt.Errorf("container validation failed for command %s: %w", alt, result.Error)
			}
			if result.ExitCode == 0 {
				return nil
			}
			return fmt.Errorf("command %s not found in container", alt)
		})

		if !found && lastErr != nil {
			if len(alternatives) == 1 {
				commandErrors = append(commandErrors, fmt.Sprintf("  • %s - command not found in container", alternatives[0]))
			} else {
				commandErrors = append(commandErrors, fmt.Sprintf("  • none of [%s] found in container", strings.Join(alternatives, ", ")))
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

// checkCustomChecks verifies all custom check scripts pass (native-only fallback).
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func checkCustomChecks(cmd *invowkfile.Command) error {
	if cmd.DependsOn == nil || len(cmd.DependsOn.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []string

	for _, checkDep := range cmd.DependsOn.CustomChecks {
		checks := checkDep.GetChecks()
		found, lastErr := evaluateAlternatives(checks, validateCustomCheckNative)

		if !found && lastErr != nil {
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
func checkCapabilityDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	var capabilityErrors []string

	// Track seen capability sets to detect duplicates (they're just skipped, not an error)
	seen := make(map[string]bool)

	for _, capDep := range deps.Capabilities {
		// Create a unique key for this set of alternatives
		key := strings.Join(func() []string {
			s := make([]string, len(capDep.Alternatives))
			for i, alt := range capDep.Alternatives {
				s[i] = string(alt)
			}
			return s
		}(), ",")

		// Skip duplicates
		if seen[key] {
			continue
		}
		seen[key] = true

		found, lastErr := evaluateAlternatives(capDep.Alternatives, invowkfile.CheckCapability)

		if !found && lastErr != nil {
			if len(capDep.Alternatives) == 1 {
				capabilityErrors = append(capabilityErrors, fmt.Sprintf("  • %s", lastErr.Error()))
			} else {
				alts := make([]string, len(capDep.Alternatives))
				for i, alt := range capDep.Alternatives {
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
func checkEnvVarDependencies(deps *invowkfile.DependsOn, userEnv map[string]string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	var envVarErrors []string

	for _, envVar := range deps.EnvVars {
		found, lastErr := evaluateAlternatives(envVar.Alternatives, func(alt invowkfile.EnvVarCheck) error {
			// Trim whitespace from name as per schema
			name := strings.TrimSpace(alt.Name)
			if name == "" {
				return fmt.Errorf("  • (empty) - environment variable name cannot be empty")
			}

			// Check if env var exists
			value, exists := userEnv[name]
			if !exists {
				return fmt.Errorf("  • %s - not set in environment", name)
			}

			// If validation pattern is specified, validate the value
			if alt.Validation != "" {
				matched, err := regexp.MatchString(alt.Validation, value)
				if err != nil {
					return fmt.Errorf("  • %s - invalid validation regex '%s': %w", name, alt.Validation, err)
				}
				if !matched {
					return fmt.Errorf("  • %s - value '%s' does not match required pattern '%s'", name, value, alt.Validation)
				}
			}

			// Env var exists and passes validation (if any)
			return nil
		})

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
