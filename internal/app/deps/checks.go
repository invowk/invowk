// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

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
		return fmt.Errorf("%s - check script returned exit code %d, expected %d", check.Name, actualCode, expectedCode)
	}

	// Check output pattern if specified
	if check.ExpectedOutput != "" {
		matched, err := regexp.MatchString(string(check.ExpectedOutput), outputStr)
		if err != nil {
			return fmt.Errorf("%s - invalid regex pattern '%s': %w", check.Name, check.ExpectedOutput.String(), err)
		}
		if !matched {
			return fmt.Errorf("%s - check script output '%s' does not match pattern '%s'", check.Name, outputStr, check.ExpectedOutput.String())
		}
	}

	return nil
}

// CheckCustomCheckDependenciesInContainer validates all custom check scripts inside the container.
// Called only for container runtime (caller guards non-container early return).
// Each CustomCheckDependency can be either a direct check or a list of alternatives.
// For alternatives, OR semantics are used (early return on first passing check).
func CheckCustomCheckDependenciesInContainer(deps *invowkfile.DependsOn, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}
	return evaluateCustomChecks(deps, ctx, func(_ context.Context, check invowkfile.CustomCheck) error {
		return probe.RunCustomCheck(check)
	})
}

// CheckHostCustomCheckDependencies validates custom checks always using the native shell on the host.
// Host-level custom checks always run in the native shell, regardless of the selected runtime,
// ensuring host-side prerequisites are validated in a consistent, predictable environment.
func CheckHostCustomCheckDependencies(deps *invowkfile.DependsOn, ctx ExecutionContext) error {
	return CheckHostCustomCheckDependenciesWithProbe(deps, ctx, nil)
}

// CheckHostCustomCheckDependenciesWithProbe validates host custom checks through an injectable probe.
func CheckHostCustomCheckDependenciesWithProbe(deps *invowkfile.DependsOn, ctx ExecutionContext, probe HostProbe) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}
	if probe == nil {
		return ErrHostProbeRequired
	}
	return evaluateCustomChecks(deps, ctx, probe.RunCustomCheck)
}

// evaluateCustomChecks runs custom check dependencies through the provided validator
// and returns a DependencyError if any fail. Each CustomCheckDependency supports
// alternatives with OR semantics (first passing check satisfies the dependency).
// The validator receives the Go context from ExecutionContext for cancellation/timeout.
func evaluateCustomChecks(
	deps *invowkfile.DependsOn,
	ctx ExecutionContext,
	validator func(context.Context, invowkfile.CustomCheck) error,
) error {
	if deps == nil || len(deps.CustomChecks) == 0 {
		return nil
	}

	// Extract the Go context for cancellation/timeout propagation.
	// Nil fallback to context.Background() for backwards compatibility.
	goCtx := ctx.GoContext()
	if goCtx == nil {
		goCtx = context.Background()
	}

	var checkErrors []DependencyMessage

	for _, checkDep := range deps.CustomChecks {
		if err := checkDep.Validate(); err != nil {
			checkErrors = append(checkErrors, dependencyMessageFromDetail(customCheckDependencyValidationMessage(err)))
			continue
		}
		checks := checkDep.GetChecks()
		found, lastErr := EvaluateAlternatives(checks, func(check invowkfile.CustomCheck) error {
			return validator(goCtx, check)
		})

		if !found && lastErr != nil {
			if len(checks) == 1 {
				checkErrors = append(checkErrors, dependencyMessageFromDetail(lastErr.Error()))
			} else {
				names := make([]string, len(checks))
				for i, c := range checks {
					names[i] = string(c.Name)
				}
				checkErrors = append(checkErrors, dependencyMessageFromDetail(fmt.Sprintf("none of custom checks [%s] passed", strings.Join(names, ", "))))
			}
		}
	}

	if len(checkErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			FailedCustomChecks: checkErrors,
			StructuredFailures: dependencyFailures(DependencyFailureCustomCheck, checkErrors),
		}
	}

	return nil
}

//goplint:ignore -- returns human-readable validation detail for DependencyMessage.
func customCheckDependencyValidationMessage(err error) string {
	var message strings.Builder
	message.WriteString(err.Error())
	if depErr, ok := errors.AsType[*invowkfile.InvalidCustomCheckDependencyError](err); ok {
		for i := range depErr.FieldErrors {
			message.WriteString(": ")
			message.WriteString(customCheckFieldValidationMessage(depErr.FieldErrors[i]))
		}
	}
	return message.String()
}

//goplint:ignore -- returns human-readable validation detail for DependencyMessage.
func customCheckFieldValidationMessage(err error) string {
	var message strings.Builder
	message.WriteString(err.Error())
	if checkErr, ok := errors.AsType[*invowkfile.InvalidCustomCheckError](err); ok {
		for i := range checkErr.FieldErrors {
			message.WriteString(": ")
			message.WriteString(checkErr.FieldErrors[i].Error())
		}
	}
	return message.String()
}

// CheckEnvVarDependenciesInContainer validates env vars inside the container.
// Called only for container runtime (caller guards non-container early return).
func CheckEnvVarDependenciesInContainer(deps *invowkfile.DependsOn, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}

	envVarErrors := collectContainerEnvVarErrors(deps.EnvVars, probe, ctx)
	if len(envVarErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			MissingEnvVars:     envVarErrors,
			StructuredFailures: dependencyFailures(DependencyFailureEnvVar, envVarErrors),
		}
	}

	return nil
}

// CheckCapabilityDependenciesInContainer validates capabilities inside the container.
// Called only for container runtime (caller guards non-container early return).
func CheckCapabilityDependenciesInContainer(deps *invowkfile.DependsOn, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}

	capabilityErrors := collectContainerCapabilityErrors(deps.Capabilities, probe, ctx)
	if len(capabilityErrors) > 0 {
		return &DependencyError{
			CommandName:         ctx.CommandName,
			MissingCapabilities: capabilityErrors,
			StructuredFailures:  dependencyFailures(DependencyFailureCapability, capabilityErrors),
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
func CheckCommandDependenciesInContainer(deps *invowkfile.DependsOn, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if deps == nil || len(deps.Commands) == 0 {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}

	commandErrors := collectContainerCommandErrors(deps.Commands, probe, ctx)
	if len(commandErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			MissingCommands:    commandErrors,
			StructuredFailures: dependencyFailures(DependencyFailureCommand, commandErrors),
		}
	}

	return nil
}

// CheckCapabilityDependencies verifies all required system capabilities are available.
// Capabilities are always checked against the host system, regardless of the runtime mode.
// For container runtimes, these checks represent the host's capabilities, not the container's.
// Each CapabilityDependency contains a list of alternatives; if any alternative is satisfied, the dependency is met.
func CheckCapabilityDependencies(deps *invowkfile.DependsOn, ctx ExecutionContext) error {
	return CheckCapabilityDependenciesWithChecker(deps, ctx, nil)
}

// CheckCapabilityDependenciesWithChecker verifies capability dependencies with an injected checker.
func CheckCapabilityDependenciesWithChecker(deps *invowkfile.DependsOn, ctx ExecutionContext, checker CapabilityChecker) error {
	if deps == nil || len(deps.Capabilities) == 0 {
		return nil
	}
	if checker == nil {
		return ErrCapabilityCheckerRequired
	}

	var capabilityErrors []DependencyMessage

	for _, capDep := range uniqueCapabilityDependencies(deps.Capabilities) {
		found, lastErr := EvaluateAlternatives(capDep.Alternatives, func(capability invowkfile.CapabilityName) error {
			return checker.Check(ctx.GoContext(), ctx.IO, capability)
		})
		if !found && lastErr != nil {
			capabilityErrors = append(capabilityErrors, formatCapabilityAlternatives(capDep.Alternatives, false, lastErr))
		}
	}

	if len(capabilityErrors) > 0 {
		return &DependencyError{
			CommandName:         ctx.CommandName,
			MissingCapabilities: capabilityErrors,
			StructuredFailures:  dependencyFailures(DependencyFailureCapability, capabilityErrors),
		}
	}

	return nil
}

// CheckEnvVarDependencies verifies all required environment variables exist.
// IMPORTANT: This function validates against the provided userEnv map, which should be captured
// at the START of execution before invowk sets any command-level env vars.
// This ensures the check validates the user's actual environment, not variables set by invowk.
// Each EnvVarDependency contains alternatives with OR semantics (early return on first match).
func CheckEnvVarDependencies(deps *invowkfile.DependsOn, userEnv map[string]string, ctx ExecutionContext) error {
	if deps == nil || len(deps.EnvVars) == 0 {
		return nil
	}

	envVarErrors := collectHostEnvVarErrors(deps.EnvVars, userEnv)
	if len(envVarErrors) > 0 {
		return &DependencyError{
			CommandName:        ctx.CommandName,
			MissingEnvVars:     envVarErrors,
			StructuredFailures: dependencyFailures(DependencyFailureEnvVar, envVarErrors),
		}
	}

	return nil
}

func collectContainerEnvVarErrors(envVars []invowkfile.EnvVarDependency, probe RuntimeDependencyProbe, _ ExecutionContext) []DependencyMessage {
	var envVarErrors []DependencyMessage
	for _, envVar := range envVars {
		found, lastErr := EvaluateAlternatives(envVar.Alternatives, func(alt invowkfile.EnvVarCheck) error {
			return probe.CheckEnvVar(alt)
		})
		if !found && lastErr != nil {
			envVarErrors = append(envVarErrors, formatEnvVarAlternatives(envVar.Alternatives, true, lastErr))
		}
	}
	return envVarErrors
}

func collectContainerCapabilityErrors(capabilities []invowkfile.CapabilityDependency, probe RuntimeDependencyProbe, _ ExecutionContext) []DependencyMessage {
	var capabilityErrors []DependencyMessage
	for _, capDep := range capabilities {
		found, lastErr := EvaluateAlternatives(capDep.Alternatives, func(alt invowkfile.CapabilityName) error {
			return probe.CheckCapability(alt)
		})
		if !found && lastErr != nil {
			capabilityErrors = append(capabilityErrors, formatCapabilityAlternatives(capDep.Alternatives, true, lastErr))
		}
	}
	return capabilityErrors
}

func collectContainerCommandErrors(commands []invowkfile.CommandDependency, probe RuntimeDependencyProbe, _ ExecutionContext) []DependencyMessage {
	var commandErrors []DependencyMessage
	for _, dep := range commands {
		alternatives := normalizedCommandAlternatives(dep)
		if len(alternatives) == 0 {
			continue
		}

		found, lastErr := EvaluateAlternatives(alternatives, func(alt invowkfile.CommandName) error {
			return probe.CheckCommand(alt)
		})
		if !found && lastErr != nil {
			commandErrors = append(commandErrors, formatMissingCommandDependency(alternatives, true))
		}
	}
	return commandErrors
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
		return dependencyMessageFromDetail(lastErr.Error())
	}

	alts := make([]string, len(alternatives))
	for i, alt := range alternatives {
		alts[i] = string(alt)
	}
	message := "none of capabilities [%s] satisfied"
	if inContainer {
		message = "none of capabilities [%s] satisfied in container"
	}
	return dependencyMessageFromDetail(fmt.Sprintf(message, strings.Join(alts, ", ")))
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
		return errors.New("(empty) - environment variable name cannot be empty")
	}

	value, exists := userEnv[name]
	if !exists {
		return fmt.Errorf("%s - not set in environment", name)
	}
	if alt.Validation == "" {
		return nil
	}

	matched, err := regexp.MatchString(string(alt.Validation), value)
	if err != nil {
		return fmt.Errorf("%s - invalid validation regex '%s': %w", name, alt.Validation.String(), err)
	}
	if !matched {
		return fmt.Errorf("%s - value '%s' does not match required pattern '%s'", name, value, alt.Validation.String())
	}
	return nil
}

func formatEnvVarAlternatives(alternatives []invowkfile.EnvVarCheck, inContainer bool, lastErr error) DependencyMessage {
	if len(alternatives) == 1 {
		return dependencyMessageFromDetail(lastErr.Error())
	}

	names := make([]string, len(alternatives))
	for i, alt := range alternatives {
		names[i] = strings.TrimSpace(string(alt.Name))
	}
	message := "none of [%s] found or passed validation"
	if inContainer {
		message = "none of [%s] found or passed validation in container"
	}
	return dependencyMessageFromDetail(fmt.Sprintf(message, strings.Join(names, ", ")))
}
