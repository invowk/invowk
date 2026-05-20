// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	customCheckInterpreterTargetHost customCheckInterpreterTarget = iota
	customCheckInterpreterTargetRuntime
)

// customCheckInterpreterTarget identifies host vs runtime custom-check analysis.
//
//goplint:constant-only
type customCheckInterpreterTarget int

func (t customCheckInterpreterTarget) String() string {
	switch t {
	case customCheckInterpreterTargetHost:
		return "host"
	case customCheckInterpreterTargetRuntime:
		return "runtime"
	default:
		return fmt.Sprintf("unknown(%d)", t)
	}
}

func (t customCheckInterpreterTarget) Validate() error {
	switch t {
	case customCheckInterpreterTargetHost, customCheckInterpreterTargetRuntime:
		return nil
	default:
		return fmt.Errorf("invalid custom check interpreter target %s", t)
	}
}

// ValidateCustomCheckOutput validates custom check script output against expected values.
func ValidateCustomCheckOutput(check invowkfile.CustomCheck, result CustomCheckResult) error {
	if err := result.Validate(); err != nil {
		return fmt.Errorf("custom check result: %w", err)
	}

	// Determine expected exit code (default: 0)
	var expectedCode types.ExitCode
	if check.ExpectedCode != nil {
		expectedCode = *check.ExpectedCode
	}

	if result.ExitCode() != expectedCode {
		return fmt.Errorf("%s - check script returned exit code %d, expected %d", check.Name, result.ExitCode(), expectedCode)
	}

	// Check output pattern if specified
	if check.ExpectedOutput != "" {
		matched, err := regexp.MatchString(string(check.ExpectedOutput), result.Output().String())
		if err != nil {
			return fmt.Errorf("%s - invalid regex pattern '%s': %w", check.Name, check.ExpectedOutput.String(), err)
		}
		if !matched {
			return fmt.Errorf("%s - check script output '%s' does not match pattern '%s'", check.Name, result.Output().String(), check.ExpectedOutput.String())
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
	return evaluateCustomChecks(
		deps,
		ctx,
		customCheckInterpreterTargetRuntime,
		func(_ context.Context, check invowkfile.CustomCheck) (CustomCheckResult, error) {
			return probe.RunCustomCheck(check)
		},
	)
}

// CheckHostCustomCheckDependencies validates custom checks on the host.
// Host-level custom checks use the embedded mvdan/sh shell by default and dispatch to
// a host interpreter only when the resolved script selects a non-shell interpreter.
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
	return evaluateCustomChecks(deps, ctx, customCheckInterpreterTargetHost, probe.RunCustomCheck)
}

// evaluateCustomChecks runs custom check dependencies through the provided validator
// and returns a DependencyError if any fail. Each CustomCheckDependency supports
// alternatives with OR semantics (first passing check satisfies the dependency).
// The validator receives the Go context from ExecutionContext for cancellation/timeout.
func evaluateCustomChecks(
	deps *invowkfile.DependsOn,
	ctx ExecutionContext,
	target customCheckInterpreterTarget,
	runner func(context.Context, invowkfile.CustomCheck) (CustomCheckResult, error),
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
			resolvedCheck, diagnostics, err := resolveCustomCheckScript(check, ctx, target)
			if err != nil {
				return err
			}
			emitCustomCheckInterpreterDiagnostics(ctx.IO.Stderr, diagnostics)
			result, err := runner(goCtx, resolvedCheck)
			if err != nil {
				return err
			}
			return ValidateCustomCheckOutput(resolvedCheck, result)
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

func resolveCustomCheckScript(check invowkfile.CustomCheck, ctx ExecutionContext, target customCheckInterpreterTarget) (invowkfile.CustomCheck, []invowkfile.ScriptInterpreterDiagnostic, error) {
	resolvedScript, err := check.Script.ResolveWithFSAndModule(ctx.modulePath(), ctx.scriptFileReader())
	if err != nil {
		return invowkfile.CustomCheck{}, nil, fmt.Errorf("%s - resolve custom check script: %w", check.Name, err)
	}
	analysisRuntime := customCheckAnalysisRuntime(check.Script, resolvedScript, target)
	label := invowkfile.ScriptInterpreterSourceLabel(fmt.Sprintf("custom check %q", check.Name))
	analysis := check.Script.AnalyzeInterpreter(resolvedScript, analysisRuntime, label)
	check.Script = invowkfile.CustomCheckScript{
		Content:     resolvedScript,
		Interpreter: check.Script.Interpreter,
	}
	return check, analysis.Diagnostics(), nil
}

func customCheckAnalysisRuntime(script invowkfile.CustomCheckScript, scriptText invowkfile.ScriptContent, target customCheckInterpreterTarget) invowkfile.RuntimeMode {
	if target == customCheckInterpreterTargetRuntime {
		return invowkfile.RuntimeContainer
	}
	interpInfo := script.ResolveInterpreterFromScript(scriptText.String())
	if interpInfo.Found && !invowkfile.IsShellInterpreter(interpInfo.Interpreter) {
		return invowkfile.RuntimeNative
	}
	return invowkfile.RuntimeVirtualSh
}

func emitCustomCheckInterpreterDiagnostics(stderr io.Writer, diagnostics []invowkfile.ScriptInterpreterDiagnostic) {
	if stderr == nil {
		return
	}
	for i := range diagnostics {
		diagnostic := diagnostics[i]
		_, _ = fmt.Fprintf(stderr, "warning: %s\n", diagnostic.Message())
	}
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
	if scriptErr, ok := errors.AsType[*invowkfile.InvalidCustomCheckScriptError](err); ok {
		for i := range scriptErr.FieldErrors {
			message.WriteString(": ")
			message.WriteString(scriptErr.FieldErrors[i].Error())
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

// checkResolvedCommandDependenciesInContainer validates already-resolved command
// dependencies inside the container using discovery-qualified command names.
func checkResolvedCommandDependenciesInContainer(commands []resolvedCommandDependency, probe RuntimeDependencyProbe, ctx ExecutionContext) error {
	if len(commands) == 0 {
		return nil
	}
	if probe == nil {
		return ErrRuntimeDependencyProbeRequired
	}

	commandErrors := collectResolvedContainerCommandErrors(commands, probe)
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

		found, lastErr := EvaluateAlternatives(alternatives, func(alt commandDependencyAlternative) error {
			return probe.CheckCommand(probeCommandName(alt))
		})
		if !found && lastErr != nil {
			commandErrors = append(commandErrors, formatMissingCommandDependency(alternatives, true))
		}
	}
	return commandErrors
}

func collectResolvedContainerCommandErrors(commands []resolvedCommandDependency, probe RuntimeDependencyProbe) []DependencyMessage {
	var commandErrors []DependencyMessage
	for _, dep := range commands {
		if dep.Command == nil {
			continue
		}
		command := *dep.Command
		if err := probe.CheckCommand(command); err != nil {
			alternatives := dep.Alternatives
			if len(alternatives) == 0 {
				alternatives = []invowkfile.CommandDependencyRef{invowkfile.CommandDependencyRef(command)}
			}
			commandErrors = append(commandErrors, formatMissingCommandDependency(normalizedCommandAlternatives(invowkfile.CommandDependency{Alternatives: alternatives}), true))
		}
	}
	return commandErrors
}

func probeCommandName(alt commandDependencyAlternative) invowkfile.CommandName {
	if alt.Parts.Qualified {
		return invowkfile.CommandName(string(alt.Parts.SourceID) + " " + string(alt.Parts.Command)) //goplint:ignore -- source and command were parsed from a validated dependency ref
	}
	return alt.Parts.Command
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
