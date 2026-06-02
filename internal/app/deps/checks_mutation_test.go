// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"regexp/syntax"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type checksMutationRuntimeProbe struct {
	envErrors        map[invowkfile.EnvVarName]error
	capabilityErrors map[invowkfile.CapabilityName]error
	commandErrors    map[invowkfile.CommandName]error
	envVars          []invowkfile.EnvVarName
	capabilities     []invowkfile.CapabilityName
	commands         []invowkfile.CommandName
}

func (p *checksMutationRuntimeProbe) CheckTool(invowkfile.BinaryName) error {
	return nil
}

func (p *checksMutationRuntimeProbe) CheckFilepath(invowkfile.FilepathDependency) error {
	return nil
}

func (p *checksMutationRuntimeProbe) CheckEnvVar(envVar invowkfile.EnvVarCheck) error {
	p.envVars = append(p.envVars, envVar.Name)
	if p.envErrors != nil {
		return p.envErrors[envVar.Name]
	}
	return nil
}

func (p *checksMutationRuntimeProbe) CheckCapability(capability invowkfile.CapabilityName) error {
	p.capabilities = append(p.capabilities, capability)
	if p.capabilityErrors != nil {
		return p.capabilityErrors[capability]
	}
	return nil
}

func (p *checksMutationRuntimeProbe) CheckCommand(command invowkfile.CommandName) error {
	p.commands = append(p.commands, command)
	if p.commandErrors != nil {
		return p.commandErrors[command]
	}
	return nil
}

func (p *checksMutationRuntimeProbe) RunCustomCheck(invowkfile.CustomCheck) (CustomCheckResult, error) {
	return CustomCheckResult{}, nil
}

func TestContainerDependencyWrapperMutationContracts(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	commandName := invowkfile.CommandName("build")

	t.Run("env var wrappers skip empty deps and require probe for non-empty deps", func(t *testing.T) {
		t.Parallel()

		if err := CheckEnvVarDependenciesInContainer(nil, nil, ctx); err != nil {
			t.Fatalf("nil env deps error = %v, want nil", err)
		}
		if err := CheckEnvVarDependenciesInContainer(&invowkfile.DependsOn{}, nil, ctx); err != nil {
			t.Fatalf("empty env deps error = %v, want nil", err)
		}
		err := CheckEnvVarDependenciesInContainer(
			&invowkfile.DependsOn{EnvVars: []invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "TOKEN"}}}}},
			nil,
			ctx,
		)
		if !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
			t.Fatalf("non-empty env deps error = %v, want ErrRuntimeDependencyProbeRequired", err)
		}
	})

	t.Run("capability wrappers skip empty deps and require probe for non-empty deps", func(t *testing.T) {
		t.Parallel()

		if err := CheckCapabilityDependenciesInContainer(nil, nil, ctx); err != nil {
			t.Fatalf("nil capability deps error = %v, want nil", err)
		}
		if err := CheckCapabilityDependenciesInContainer(&invowkfile.DependsOn{}, nil, ctx); err != nil {
			t.Fatalf("empty capability deps error = %v, want nil", err)
		}
		err := CheckCapabilityDependenciesInContainer(
			&invowkfile.DependsOn{Capabilities: []invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}}}},
			nil,
			ctx,
		)
		if !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
			t.Fatalf("non-empty capability deps error = %v, want ErrRuntimeDependencyProbeRequired", err)
		}
	})

	t.Run("command wrappers skip empty deps and require probe for non-empty deps", func(t *testing.T) {
		t.Parallel()

		if err := CheckCommandDependenciesInContainer(nil, nil, ctx); err != nil {
			t.Fatalf("nil command deps error = %v, want nil", err)
		}
		if err := CheckCommandDependenciesInContainer(&invowkfile.DependsOn{}, nil, ctx); err != nil {
			t.Fatalf("empty command deps error = %v, want nil", err)
		}
		err := CheckCommandDependenciesInContainer(
			&invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandDependencyRef{"build"}}}},
			nil,
			ctx,
		)
		if !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
			t.Fatalf("non-empty command deps error = %v, want ErrRuntimeDependencyProbeRequired", err)
		}
	})

	t.Run("resolved command wrapper skips empty deps and requires probe for resolved deps", func(t *testing.T) {
		t.Parallel()

		if err := checkResolvedCommandDependenciesInContainer(nil, nil, ctx); err != nil {
			t.Fatalf("nil resolved command deps error = %v, want nil", err)
		}
		err := checkResolvedCommandDependenciesInContainer([]resolvedCommandDependency{{Command: &commandName}}, nil, ctx)
		if !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
			t.Fatalf("non-empty resolved command deps error = %v, want ErrRuntimeDependencyProbeRequired", err)
		}
	})
}

func TestValidateCustomCheckOutputMutationContracts(t *testing.T) {
	t.Parallel()

	err := ValidateCustomCheckOutput(
		invowkfile.CustomCheck{Name: "invalid-result"},
		CustomCheckResult{exitCode: types.ExitCode(-1)},
	)
	if !errors.Is(err, types.ErrInvalidExitCode) {
		t.Fatalf("invalid result error = %v, want ErrInvalidExitCode", err)
	}

	err = ValidateCustomCheckOutput(
		invowkfile.CustomCheck{Name: "bad-regex", ExpectedOutput: "["},
		mustCustomCheckResult(t, "anything", 0),
	)
	var syntaxErr *syntax.Error
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("invalid regex error = %v, want *syntax.Error", err)
	}

	err = ValidateCustomCheckOutput(
		invowkfile.CustomCheck{Name: "output-check", ExpectedOutput: "^ok$"},
		mustCustomCheckResult(t, "fail", 0),
	)
	want := "output-check - check script output 'fail' does not match pattern '^ok$'"
	if err == nil || err.Error() != want {
		t.Fatalf("output mismatch error = %v, want %q", err, want)
	}
}

func TestEvaluateCustomChecksMutationContracts(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	depsWithCheck := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Alternatives: []invowkfile.CustomCheck{{
				Name:   "needs-probe",
				Script: invowkfile.CustomCheckScript{Content: "echo ok"},
			}},
		}},
	}
	if err := CheckHostCustomCheckDependencies(depsWithCheck, ctx); !errors.Is(err, ErrHostProbeRequired) {
		t.Fatalf("CheckHostCustomCheckDependencies() = %v, want ErrHostProbeRequired", err)
	}

	runnerErr := errors.New("runner failed")
	depsWithInvalidAndFailingChecks := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{Name: "invalid-direct"},
			{Alternatives: []invowkfile.CustomCheck{{
				Name:   "runner-fails",
				Script: invowkfile.CustomCheckScript{Content: "echo fail"},
			}}},
		},
	}
	err := evaluateCustomChecks(
		depsWithInvalidAndFailingChecks,
		ctx,
		customCheckInterpreterTargetHost,
		func(context.Context, invowkfile.CustomCheck) (CustomCheckResult, error) {
			return CustomCheckResult{}, runnerErr
		},
	)
	depErr := requireDependencyError(t, err)
	if depErr.CommandName != ctx.CommandName {
		t.Fatalf("CommandName = %q, want %q", depErr.CommandName, ctx.CommandName)
	}
	if len(depErr.FailedCustomChecks) != 2 {
		t.Fatalf("FailedCustomChecks = %v, want two failures", depErr.FailedCustomChecks)
	}
	if !strings.Contains(depErr.FailedCustomChecks[0].String(), "invalid custom check dependency") {
		t.Fatalf("first custom check failure = %q, want validation detail", depErr.FailedCustomChecks[0])
	}
	if depErr.FailedCustomChecks[1].String() != runnerErr.Error() {
		t.Fatalf("second custom check failure = %q, want runner error", depErr.FailedCustomChecks[1])
	}
	if len(depErr.StructuredFailures) != 2 {
		t.Fatalf("StructuredFailures = %v, want two failures", depErr.StructuredFailures)
	}
	for i := range depErr.StructuredFailures {
		if depErr.StructuredFailures[i].Kind() != DependencyFailureCustomCheck {
			t.Fatalf("StructuredFailures[%d].Kind() = %q, want custom_check", i, depErr.StructuredFailures[i].Kind())
		}
	}
}

func TestContainerCollectorsMutationContracts(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)

	t.Run("env alternatives stop after first successful probe", func(t *testing.T) {
		t.Parallel()

		probe := &checksMutationRuntimeProbe{
			envErrors: map[invowkfile.EnvVarName]error{
				"MISSING": errors.New("should not be called"),
			},
		}
		errs := collectContainerEnvVarErrors(
			[]invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "PRESENT"}, {Name: "MISSING"}}}},
			probe,
			ctx,
		)
		if len(errs) != 0 {
			t.Fatalf("container env errors = %v, want none", errs)
		}
		if len(probe.envVars) != 1 || probe.envVars[0] != "PRESENT" {
			t.Fatalf("checked env vars = %v, want only PRESENT", probe.envVars)
		}
	})

	t.Run("qualified command dependencies use source-qualified probe names", func(t *testing.T) {
		t.Parallel()

		probe := &checksMutationRuntimeProbe{
			commandErrors: map[invowkfile.CommandName]error{
				"tools lint": errors.New("missing command"),
			},
		}
		errs := collectContainerCommandErrors(
			[]invowkfile.CommandDependency{
				{},
				{Alternatives: []invowkfile.CommandDependencyRef{"@tools lint"}},
			},
			probe,
			ctx,
		)
		if len(probe.commands) != 1 || probe.commands[0] != "tools lint" {
			t.Fatalf("checked commands = %v, want tools lint", probe.commands)
		}
		want := "@tools lint - command not found in container"
		if len(errs) != 1 || errs[0].String() != want {
			t.Fatalf("container command errors = %v, want %q", errs, want)
		}
	})

	t.Run("resolved command dependencies skip nil commands and format fallback alternatives", func(t *testing.T) {
		t.Parallel()

		build := invowkfile.CommandName("build")
		deploy := invowkfile.CommandName("deploy")
		probe := &checksMutationRuntimeProbe{
			commandErrors: map[invowkfile.CommandName]error{
				build:  errors.New("missing build"),
				deploy: errors.New("missing deploy"),
			},
		}
		errs := collectResolvedContainerCommandErrors(
			[]resolvedCommandDependency{
				{},
				{Command: &build},
				{Command: &deploy, Alternatives: []invowkfile.CommandDependencyRef{"custom-deploy"}},
			},
			probe,
		)
		if len(probe.commands) != 2 || probe.commands[0] != build || probe.commands[1] != deploy {
			t.Fatalf("checked resolved commands = %v, want build and deploy", probe.commands)
		}
		wants := []string{
			"build - command not found in container",
			"custom-deploy - command not found in container",
		}
		if len(errs) != len(wants) {
			t.Fatalf("resolved command errors = %v, want %d errors", errs, len(wants))
		}
		for i := range wants {
			if errs[i].String() != wants[i] {
				t.Fatalf("resolved command error %d = %q, want %q", i, errs[i], wants[i])
			}
		}
	})
}

func TestHostEnvCapabilityMutationContracts(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)

	t.Run("capability wrapper requires checker for non-empty deps", func(t *testing.T) {
		t.Parallel()

		err := CheckCapabilityDependencies(&invowkfile.DependsOn{
			Capabilities: []invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}}},
		}, ctx)
		if !errors.Is(err, ErrCapabilityCheckerRequired) {
			t.Fatalf("CheckCapabilityDependencies() = %v, want ErrCapabilityCheckerRequired", err)
		}
	})

	t.Run("duplicate capability dependencies are checked once", func(t *testing.T) {
		t.Parallel()

		checker := &recordingCapabilityChecker{}
		deps := &invowkfile.DependsOn{
			Capabilities: []invowkfile.CapabilityDependency{
				{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}},
				{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}},
			},
		}
		if err := CheckCapabilityDependenciesWithChecker(deps, ctx, checker); err != nil {
			t.Fatalf("CheckCapabilityDependenciesWithChecker() = %v, want nil", err)
		}
		if len(checker.requests) != 1 {
			t.Fatalf("capability requests = %d, want one de-duplicated request", len(checker.requests))
		}
	})

	t.Run("host multi-capability failure records host-specific message and structured kind", func(t *testing.T) {
		t.Parallel()

		err := CheckCapabilityDependenciesWithChecker(
			&invowkfile.DependsOn{Capabilities: []invowkfile.CapabilityDependency{{
				Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY, invowkfile.CapabilityInternet},
			}}},
			ctx,
			fakeCapabilityChecker{
				invowkfile.CapabilityTTY:      errors.New("tty unavailable"),
				invowkfile.CapabilityInternet: errors.New("internet unavailable"),
			},
		)
		depErr := requireDependencyError(t, err)
		want := "none of capabilities [tty, internet] satisfied"
		if len(depErr.MissingCapabilities) != 1 || depErr.MissingCapabilities[0].String() != want {
			t.Fatalf("MissingCapabilities = %v, want %q", depErr.MissingCapabilities, want)
		}
		if len(depErr.StructuredFailures) != 1 || depErr.StructuredFailures[0].Kind() != DependencyFailureCapability {
			t.Fatalf("StructuredFailures = %v, want one capability failure", depErr.StructuredFailures)
		}
	})

	t.Run("host env formatting trims alternatives and invalid regex remains wrapped", func(t *testing.T) {
		t.Parallel()

		errs := collectHostEnvVarErrors(
			[]invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: " FIRST "}, {Name: "SECOND"}}}},
			map[string]string{},
		)
		want := "none of [FIRST, SECOND] found or passed validation"
		if len(errs) != 1 || errs[0].String() != want {
			t.Fatalf("host env errors = %v, want %q", errs, want)
		}

		err := validateHostEnvVar(invowkfile.EnvVarCheck{Name: "PORT", Validation: "["}, map[string]string{"PORT": "8080"})
		var syntaxErr *syntax.Error
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("invalid host env regex error = %v, want *syntax.Error", err)
		}
	})
}
