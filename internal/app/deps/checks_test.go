// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"io"
	"os/exec"
	goruntime "runtime"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

func TestCheckCustomCheckDependenciesInContainer(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext()
	registry := runtimepkg.NewRegistry()
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script), "echo ok") {
				_, _ = io.WriteString(ctx.IO.Stdout, "ok")
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1, Error: shellExitError(t)}
		},
	}
	registry.Register(runtimepkg.RuntimeTypeContainer, stub)

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "first", CheckScript: "exit 1"},
					{Name: "second", CheckScript: "echo ok", ExpectedOutput: "^ok$"},
				},
			},
		},
	}

	if err := CheckCustomCheckDependenciesInContainer(deps, registry, ctx); err != nil {
		t.Fatalf("CheckCustomCheckDependenciesInContainer() = %v", err)
	}

	err := CheckCustomCheckDependenciesInContainer(
		&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "first", CheckScript: "exit 1"},
					{Name: "second", CheckScript: "exit 1"},
				},
			}},
		},
		registry,
		ctx,
	)
	if err == nil {
		t.Fatal("expected custom check dependency error")
	}
	var depErr *DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("errors.As(*DependencyError) = false for %T", err)
	}
	if len(depErr.FailedCustomChecks) != 1 || !strings.Contains(string(depErr.FailedCustomChecks[0]), "none of custom checks [first, second] passed") {
		t.Fatalf("depErr.FailedCustomChecks = %v", depErr.FailedCustomChecks)
	}
}

func TestValidateCustomCheckInContainer(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext()
	check := invowkfile.CustomCheck{Name: "demo", CheckScript: "echo ok", ExpectedOutput: "^ok$"}

	err := validateCustomCheckInContainer(check, runtimepkg.NewRegistry(), ctx)
	if err == nil || !strings.Contains(err.Error(), "container runtime not available") {
		t.Fatalf("err = %v", err)
	}

	registry := runtimepkg.NewRegistry()
	registry.Register(runtimepkg.RuntimeTypeContainer, &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			_, _ = io.WriteString(ctx.IO.Stdout, "ok")
			return &runtimepkg.Result{ExitCode: 0}
		},
	})
	if validateErr := validateCustomCheckInContainer(check, registry, ctx); validateErr != nil {
		t.Fatalf("validateCustomCheckInContainer() = %v", validateErr)
	}

	registry.Register(runtimepkg.RuntimeTypeContainer, &filepathStubRuntime{
		execFn: func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
			return &runtimepkg.Result{ExitCode: 1, Error: errors.New("engine down")}
		},
	})
	err = validateCustomCheckInContainer(check, registry, ctx)
	if err == nil || !strings.Contains(err.Error(), "container validation failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestContainerEnvVarValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext()
	registry := runtimepkg.NewRegistry()
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			script := string(ctx.SelectedImpl.Script)
			switch {
			case strings.Contains(script, `printf '%s' "$HOME" | grep -qE '^/home/'`):
				return &runtimepkg.Result{ExitCode: 0}
			case strings.Contains(script, "${MISSING+x}"):
				return &runtimepkg.Result{ExitCode: 1}
			case strings.Contains(script, "${TRANSIENT+x}"):
				return &runtimepkg.Result{ExitCode: 125}
			default:
				return &runtimepkg.Result{ExitCode: 0}
			}
		},
	}
	registry.Register(runtimepkg.RuntimeTypeContainer, stub)

	if validateContainerEnvVar(invowkfile.EnvVarCheck{Name: "", Validation: "^.+$"}, stub, ctx) == nil {
		t.Fatal("expected empty name error")
	}

	if err := validateContainerEnvVar(invowkfile.EnvVarCheck{Name: "HOME", Validation: "^/home/"}, stub, ctx); err != nil {
		t.Fatalf("validateContainerEnvVar() = %v", err)
	}

	err := validateContainerEnvVar(invowkfile.EnvVarCheck{Name: "MISSING"}, stub, ctx)
	if err == nil || !strings.Contains(err.Error(), "not set in container environment") {
		t.Fatalf("err = %v", err)
	}

	err = validateContainerEnvVar(invowkfile.EnvVarCheck{Name: "TRANSIENT"}, stub, ctx)
	if err == nil || !strings.Contains(err.Error(), "container engine failure") {
		t.Fatalf("err = %v", err)
	}

	errorsList := collectContainerEnvVarErrors(
		[]invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING"}, {Name: "TRANSIENT"}}}},
		stub,
		ctx,
	)
	if len(errorsList) != 1 || !strings.Contains(string(errorsList[0]), "none of [MISSING, TRANSIENT] found or passed validation in container") {
		t.Fatalf("errorsList = %v", errorsList)
	}

	err = CheckEnvVarDependenciesInContainer(
		&invowkfile.DependsOn{EnvVars: []invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING"}}}}},
		registry,
		ctx,
	)
	if err == nil {
		t.Fatal("expected env var dependency error")
	}
}

func TestContainerCapabilityValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext()
	registry := runtimepkg.NewRegistry()
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script), "command -v docker") {
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1}
		},
	}
	registry.Register(runtimepkg.RuntimeTypeContainer, stub)

	if validateContainerCapability(invowkfile.CapabilityName("bogus"), stub, ctx) == nil {
		t.Fatal("expected unknown capability error")
	}
	if err := validateContainerCapability(invowkfile.CapabilityContainers, stub, ctx); err != nil {
		t.Fatalf("validateContainerCapability() = %v", err)
	}

	errorsList := collectContainerCapabilityErrors(
		[]invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY, invowkfile.CapabilityLocalAreaNetwork}}},
		stub,
		ctx,
	)
	if len(errorsList) != 1 || !strings.Contains(string(errorsList[0]), "none of capabilities [tty, local-area-network] satisfied in container") {
		t.Fatalf("errorsList = %v", errorsList)
	}

	if string(formatCapabilityAlternatives([]invowkfile.CapabilityName{invowkfile.CapabilityTTY}, false, errors.New("tty - missing"))) != "  • tty - missing" {
		t.Fatal("single capability alternative formatting mismatch")
	}

	err := CheckCapabilityDependenciesInContainer(
		&invowkfile.DependsOn{Capabilities: []invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}}}},
		registry,
		ctx,
	)
	if err == nil {
		t.Fatal("expected capability dependency error")
	}
}

func TestContainerCommandValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext()
	var seenScripts []string
	registry := runtimepkg.NewRegistry()
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			script := string(ctx.SelectedImpl.Script)
			seenScripts = append(seenScripts, script)
			switch {
			case strings.Contains(script, "check-cmd 'build'"):
				return &runtimepkg.Result{ExitCode: 0}
			case strings.Contains(script, "check-cmd 'missing'"):
				_, _ = io.WriteString(ctx.IO.Stderr, "missing")
				return &runtimepkg.Result{ExitCode: 1}
			default:
				return &runtimepkg.Result{ExitCode: 1, Error: errors.New("engine down")}
			}
		},
	}
	registry.Register(runtimepkg.RuntimeTypeContainer, stub)

	if err := validateContainerCommand("build", stub, ctx); err != nil {
		t.Fatalf("validateContainerCommand(build) = %v", err)
	}

	err := validateContainerCommand("missing", stub, ctx)
	if err == nil || !strings.Contains(err.Error(), "command missing not found in container") {
		t.Fatalf("err = %v", err)
	}

	err = validateContainerCommand("broken", stub, ctx)
	if err == nil || !strings.Contains(err.Error(), "container validation failed for command broken") {
		t.Fatalf("err = %v", err)
	}

	errorsList := collectContainerCommandErrors(
		[]invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandName{"missing", "other"}}},
		stub,
		ctx,
	)
	if len(errorsList) != 1 || !strings.Contains(string(errorsList[0]), "none of [missing, other] found in container") {
		t.Fatalf("errorsList = %v", errorsList)
	}

	err = CheckCommandDependenciesInContainer(
		&invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandName{"missing"}}}},
		registry,
		ctx,
	)
	if err == nil {
		t.Fatal("expected command dependency error")
	}

	if len(seenScripts) == 0 || !strings.Contains(seenScripts[0], "invowk internal check-cmd 'build'") {
		t.Fatalf("seenScripts = %v", seenScripts)
	}
}

func TestRequireContainerRuntime(t *testing.T) {
	t.Parallel()

	_, err := requireContainerRuntime(runtimepkg.NewRegistry(), "demo")
	if err == nil || !strings.Contains(err.Error(), "container runtime not available for demo") {
		t.Fatalf("err = %v", err)
	}
}

func newDependencyExecutionContext() *runtimepkg.ExecutionContext {
	return &runtimepkg.ExecutionContext{
		Command: &invowkfile.Command{Name: "build"},
		Context: context.Background(),
	}
}

func shellExitError(t *testing.T) error {
	t.Helper()

	var cmd *exec.Cmd
	if goruntime.GOOS == "windows" {
		cmd = exec.CommandContext(t.Context(), "cmd", "/c", "exit", "/b", "1")
	} else {
		cmd = exec.CommandContext(t.Context(), "sh", "-c", "exit 1")
	}
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit error")
	}
	return err
}
