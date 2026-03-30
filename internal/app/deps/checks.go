// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// ValidateCustomCheckOutput validates custom check script output against expected values.
func ValidateCustomCheckOutput(check invowkfile.CustomCheck, outputStr string, execErr error) error {
	// Determine expected exit code (default: 0)
	var expectedCode types.ExitCode
	if check.ExpectedCode != nil {
		expectedCode = *check.ExpectedCode
	}

	// Check exit code
	var actualCode types.ExitCode
	if execErr != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](execErr); ok {
			exitCode := types.ExitCode(exitErr.ExitCode())
			if err := exitCode.Validate(); err != nil {
				return fmt.Errorf("exit code validation: %w", err)
			}
			actualCode = exitCode
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
		matched, err := regexp.MatchString(string(check.ExpectedOutput), outputStr)
		if err != nil {
			return fmt.Errorf("  • %s - invalid regex pattern '%s': %w", check.Name, check.ExpectedOutput.String(), err)
		}
		if !matched {
			return fmt.Errorf("  • %s - check script output '%s' does not match pattern '%s'", check.Name, outputStr, check.ExpectedOutput.String())
		}
	}

	return nil
}

// CheckCustomCheckDependenciesInContainer validates all custom check scripts inside the container.
// Called only for container runtime (caller guards non-container early return).
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func CheckCustomCheckDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	return evaluateCustomChecks(deps, ctx, func(check invowkfile.CustomCheck) error {
		return validateCustomCheckInContainer(check, registry, ctx)
	})
}

// validateCustomCheckNative runs a custom check script using the native shell.
func validateCustomCheckNative(check invowkfile.CustomCheck) error {
	cmd := exec.CommandContext(context.Background(), "sh", "-c", string(check.CheckScript))
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))

	return ValidateCustomCheckOutput(check, outputStr, err)
}

// validateCustomCheckInContainer runs a custom check script within a container.
// Distinguishes infrastructure failures (container engine down) from script exit codes
// to prevent false-positive validation when the container never actually ran.
func validateCustomCheckInContainer(check invowkfile.CustomCheck, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return fmt.Errorf("  • %s - %w", check.Name, ErrContainerRuntimeNotAvailable)
	}

	validationCtx, stdout, stderr := NewContainerValidationContext(ctx, string(check.CheckScript))

	result := rt.Execute(validationCtx)

	// Infrastructure failures must be surfaced immediately -- if the container engine
	// failed, no check ever ran, so we must not fall through to exit code comparison.
	if result.Error != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](result.Error); !ok || exitErr == nil {
			return fmt.Errorf("  • %s - %w: %w", check.Name, ErrContainerValidationFailed, result.Error)
		}
	}
	if err := CheckTransientExitCode(result, string(check.Name)); err != nil {
		return err
	}

	outputStr := strings.TrimSpace(stdout.String() + stderr.String())
	return ValidateCustomCheckOutput(check, outputStr, result.Error)
}

// CheckHostCustomCheckDependencies validates custom checks always using the native shell on the host.
// Host-level custom checks always run in the native shell, regardless of the selected runtime,
// ensuring host-side prerequisites are validated in a consistent, predictable environment.
func CheckHostCustomCheckDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	return evaluateCustomChecks(deps, ctx, validateCustomCheckNative)
}

// evaluateCustomChecks runs custom check dependencies through the provided validator
// and returns a DependencyError if any fail. Each CustomCheckDependency supports
// alternatives with OR semantics (first passing check satisfies the dependency).
func evaluateCustomChecks(
	deps *invowkfile.DependsOn,
	ctx *runtime.ExecutionContext,
	validator func(invowkfile.CustomCheck) error,
) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	var checkErrors []DependencyMessage

	for _, checkDep := range deps.CustomChecks {
		checks := checkDep.GetChecks()
		found, lastErr := EvaluateAlternatives(checks, validator)

		if !found && lastErr != nil {
			if len(checks) == 1 {
				checkErrors = append(checkErrors, DependencyMessage(lastErr.Error()))
			} else {
				names := make([]string, len(checks))
				for i, c := range checks {
					names[i] = string(c.Name)
				}
				checkErrors = append(checkErrors, DependencyMessage(fmt.Sprintf("  • none of custom checks [%s] passed", strings.Join(names, ", "))))
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

// CheckEnvVarDependenciesInContainer validates env vars inside the container.
// Called only for container runtime (caller guards non-container early return).
func CheckEnvVarDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	rt, err := requireContainerRuntime(registry, "env var validation")
	if err != nil {
		return err
	}

	envVarErrors := collectContainerEnvVarErrors(deps.EnvVars, rt, ctx)
	if len(envVarErrors) > 0 {
		return &DependencyError{
			CommandName:    ctx.Command.Name,
			MissingEnvVars: envVarErrors,
		}
	}

	return nil
}

// CheckCapabilityDependenciesInContainer validates capabilities inside the container.
// Called only for container runtime (caller guards non-container early return).
func CheckCapabilityDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	rt, err := requireContainerRuntime(registry, "capability validation")
	if err != nil {
		return err
	}

	capabilityErrors := collectContainerCapabilityErrors(deps.Capabilities, rt, ctx)
	if len(capabilityErrors) > 0 {
		return &DependencyError{
			CommandName:         ctx.Command.Name,
			MissingCapabilities: capabilityErrors,
		}
	}

	return nil
}

// CapabilityCheckScript returns a shell one-liner that checks whether a system capability
// is available. The returned script is suitable for execution in any POSIX shell environment.
func CapabilityCheckScript(capName invowkfile.CapabilityName) string {
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

// CheckCommandDependenciesInContainer validates command discoverability inside the container.
// Called only for container runtime (caller guards non-container early return).
// Runs `invowk internal check-cmd` inside the container to verify auto-provisioning worked.
func CheckCommandDependenciesInContainer(deps *invowkfile.DependsOn, registry *runtime.Registry, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}

	rt, err := requireContainerRuntime(registry, "command dependency validation")
	if err != nil {
		return err
	}

	commandErrors := collectContainerCommandErrors(deps.Commands, rt, ctx)
	if len(commandErrors) > 0 {
		return &DependencyError{
			CommandName:     ctx.Command.Name,
			MissingCommands: commandErrors,
		}
	}

	return nil
}

// CheckCapabilityDependencies verifies all required system capabilities are available.
// Capabilities are always checked against the host system, regardless of the runtime mode.
// For container runtimes, these checks represent the host's capabilities, not the container's.
// Each CapabilityDependency contains a list of alternatives; if any alternative is satisfied, the dependency is met.
func CheckCapabilityDependencies(deps *invowkfile.DependsOn, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}

	var capabilityErrors []DependencyMessage

	for _, capDep := range uniqueCapabilityDependencies(deps.Capabilities) {
		found, lastErr := EvaluateAlternatives(capDep.Alternatives, invowkfile.CheckCapability)
		if !found && lastErr != nil {
			capabilityErrors = append(capabilityErrors, formatCapabilityAlternatives(capDep.Alternatives, false, lastErr))
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

// CheckEnvVarDependencies verifies all required environment variables exist.
// IMPORTANT: This function validates against the provided userEnv map, which should be captured
// at the START of execution before invowk sets any command-level env vars.
// This ensures the check validates the user's actual environment, not variables set by invowk.
// Each EnvVarDependency contains alternatives with OR semantics (early return on first match).
func CheckEnvVarDependencies(deps *invowkfile.DependsOn, userEnv map[string]string, ctx *runtime.ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	envVarErrors := collectHostEnvVarErrors(deps.EnvVars, userEnv)
	if len(envVarErrors) > 0 {
		return &DependencyError{
			CommandName:    ctx.Command.Name,
			MissingEnvVars: envVarErrors,
		}
	}

	return nil
}

//goplint:ignore -- helper formats internal dependency-check labels.
func requireContainerRuntime(registry *runtime.Registry, label string) (runtime.Runtime, error) {
	rt, err := registry.Get(runtime.RuntimeTypeContainer)
	if err != nil {
		return nil, fmt.Errorf("%w for %s", ErrContainerRuntimeNotAvailable, label)
	}
	return rt, nil
}

func collectContainerEnvVarErrors(envVars []invowkfile.EnvVarDependency, rt runtime.Runtime, ctx *runtime.ExecutionContext) []DependencyMessage {
	var envVarErrors []DependencyMessage
	for _, envVar := range envVars {
		found, lastErr := EvaluateAlternatives(envVar.Alternatives, func(alt invowkfile.EnvVarCheck) error {
			return validateContainerEnvVar(alt, rt, ctx)
		})
		if !found && lastErr != nil {
			envVarErrors = append(envVarErrors, formatEnvVarAlternatives(envVar.Alternatives, true, lastErr))
		}
	}
	return envVarErrors
}

func validateContainerEnvVar(alt invowkfile.EnvVarCheck, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	name := strings.TrimSpace(string(alt.Name))
	if name == "" {
		return errors.New("  • (empty) - environment variable name cannot be empty")
	}
	if err := invowkfile.ValidateEnvVarName(name); err != nil {
		return fmt.Errorf("  • %s - %w", name, err)
	}

	checkScript := fmt.Sprintf("test -n \"${%s+x}\"", name)
	if alt.Validation != "" {
		escapedValidation := ShellEscapeSingleQuote(string(alt.Validation))
		checkScript = fmt.Sprintf("test -n \"${%s+x}\" && printf '%%s' \"$%s\" | grep -qE '%s'", name, name, escapedValidation)
	}

	validationCtx, _, _ := NewContainerValidationContext(ctx, checkScript)
	result := rt.Execute(validationCtx)
	if result.Error != nil {
		return fmt.Errorf("%w for env var %s: %w", ErrContainerValidationFailed, name, result.Error)
	}
	if err := CheckTransientExitCode(result, name); err != nil {
		return err
	}
	if result.ExitCode == 0 {
		return nil
	}
	if alt.Validation != "" {
		return fmt.Errorf("  • %s - not set or value does not match pattern '%s' in container", name, alt.Validation.String())
	}
	return fmt.Errorf("  • %s - not set in container environment", name)
}

func collectContainerCapabilityErrors(capabilities []invowkfile.CapabilityDependency, rt runtime.Runtime, ctx *runtime.ExecutionContext) []DependencyMessage {
	var capabilityErrors []DependencyMessage
	for _, capDep := range capabilities {
		found, lastErr := EvaluateAlternatives(capDep.Alternatives, func(alt invowkfile.CapabilityName) error {
			return validateContainerCapability(alt, rt, ctx)
		})
		if !found && lastErr != nil {
			capabilityErrors = append(capabilityErrors, formatCapabilityAlternatives(capDep.Alternatives, true, lastErr))
		}
	}
	return capabilityErrors
}

func validateContainerCapability(alt invowkfile.CapabilityName, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	checkScript := CapabilityCheckScript(alt)
	if checkScript == "" {
		return fmt.Errorf("%s - unknown capability", string(alt))
	}

	validationCtx, _, _ := NewContainerValidationContext(ctx, checkScript)
	result := rt.Execute(validationCtx)
	if result.Error != nil {
		return fmt.Errorf("%w for capability %s: %w", ErrContainerValidationFailed, string(alt), result.Error)
	}
	if err := CheckTransientExitCode(result, string(alt)); err != nil {
		return err
	}
	if result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("%s - not available in container", string(alt))
}

func collectContainerCommandErrors(commands []invowkfile.CommandDependency, rt runtime.Runtime, ctx *runtime.ExecutionContext) []DependencyMessage {
	var commandErrors []DependencyMessage
	for _, dep := range commands {
		alternatives := normalizedCommandAlternatives(dep)
		if len(alternatives) == 0 {
			continue
		}

		found, lastErr := EvaluateAlternatives(alternatives, func(alt string) error {
			return validateContainerCommand(alt, rt, ctx)
		})
		if !found && lastErr != nil {
			commandErrors = append(commandErrors, formatMissingCommandDependency(alternatives, true))
		}
	}
	return commandErrors
}

//goplint:ignore -- helper executes discovered command names as container lookup probes.
func validateContainerCommand(alt string, rt runtime.Runtime, ctx *runtime.ExecutionContext) error {
	checkScript := fmt.Sprintf("invowk internal check-cmd '%s'", ShellEscapeSingleQuote(alt))
	validationCtx, _, stderr := NewContainerValidationContext(ctx, checkScript)

	result := rt.Execute(validationCtx)
	if result.Error != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return fmt.Errorf("%w for command %s: %w (%s)", ErrContainerValidationFailed, alt, result.Error, stderrStr)
		}
		return fmt.Errorf("%w for command %s: %w", ErrContainerValidationFailed, alt, result.Error)
	}
	if err := CheckTransientExitCode(result, alt); err != nil {
		return err
	}
	if result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("command %s not found in container", alt)
}

func uniqueCapabilityDependencies(capabilities []invowkfile.CapabilityDependency) []invowkfile.CapabilityDependency {
	seen := make(map[string]bool)
	var unique []invowkfile.CapabilityDependency
	for _, capDep := range capabilities {
		key := capabilityDependencyKey(capDep)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, capDep)
	}
	return unique
}

//goplint:ignore -- helper reduces capability alternatives to a dedupe key.
func capabilityDependencyKey(capDep invowkfile.CapabilityDependency) string {
	alts := make([]string, len(capDep.Alternatives))
	for i, alt := range capDep.Alternatives {
		alts[i] = string(alt)
	}
	return strings.Join(alts, ",")
}

func formatCapabilityAlternatives(alternatives []invowkfile.CapabilityName, inContainer bool, lastErr error) DependencyMessage {
	if len(alternatives) == 1 {
		return DependencyMessage("  • " + lastErr.Error())
	}

	alts := make([]string, len(alternatives))
	for i, alt := range alternatives {
		alts[i] = string(alt)
	}
	message := "  • none of capabilities [%s] satisfied"
	if inContainer {
		message = "  • none of capabilities [%s] satisfied in container"
	}
	return DependencyMessage(fmt.Sprintf(message, strings.Join(alts, ", ")))
}

func collectHostEnvVarErrors(envVars []invowkfile.EnvVarDependency, userEnv map[string]string) []DependencyMessage {
	var envVarErrors []DependencyMessage
	for _, envVar := range envVars {
		found, lastErr := EvaluateAlternatives(envVar.Alternatives, func(alt invowkfile.EnvVarCheck) error {
			return validateHostEnvVar(alt, userEnv)
		})
		if !found && lastErr != nil {
			envVarErrors = append(envVarErrors, formatEnvVarAlternatives(envVar.Alternatives, false, lastErr))
		}
	}
	return envVarErrors
}

func validateHostEnvVar(alt invowkfile.EnvVarCheck, userEnv map[string]string) error {
	name := strings.TrimSpace(string(alt.Name))
	if name == "" {
		return errors.New("  • (empty) - environment variable name cannot be empty")
	}

	value, exists := userEnv[name]
	if !exists {
		return fmt.Errorf("  • %s - not set in environment", name)
	}
	if alt.Validation == "" {
		return nil
	}

	matched, err := regexp.MatchString(string(alt.Validation), value)
	if err != nil {
		return fmt.Errorf("  • %s - invalid validation regex '%s': %w", name, alt.Validation.String(), err)
	}
	if !matched {
		return fmt.Errorf("  • %s - value '%s' does not match required pattern '%s'", name, value, alt.Validation.String())
	}
	return nil
}

func formatEnvVarAlternatives(alternatives []invowkfile.EnvVarCheck, inContainer bool, lastErr error) DependencyMessage {
	if len(alternatives) == 1 {
		return DependencyMessage(lastErr.Error())
	}

	names := make([]string, len(alternatives))
	for i, alt := range alternatives {
		names[i] = strings.TrimSpace(string(alt.Name))
	}
	message := "  • none of [%s] found or passed validation"
	if inContainer {
		message = "  • none of [%s] found or passed validation in container"
	}
	return DependencyMessage(fmt.Sprintf(message, strings.Join(names, ", ")))
}
