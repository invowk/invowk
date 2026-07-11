// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	fakeCapabilityChecker map[invowkfile.CapabilityName]error

	recordedCapabilityRequest struct {
		ctx        context.Context
		ioCtx      IOContext
		capability invowkfile.CapabilityName
	}

	recordingCapabilityChecker struct {
		requests []recordedCapabilityRequest
	}

	customCheckScriptFileResolutionFixture struct {
		deps *invowkfile.DependsOn
		ctx  ExecutionContext
	}
)

func (f fakeCapabilityChecker) Check(_ context.Context, _ IOContext, capability invowkfile.CapabilityName) error {
	if err, ok := f[capability]; ok {
		return err
	}
	return nil
}

func (r *recordingCapabilityChecker) Check(ctx context.Context, ioCtx IOContext, capability invowkfile.CapabilityName) error {
	r.requests = append(r.requests, recordedCapabilityRequest{ctx: ctx, ioCtx: ioCtx, capability: capability})
	return nil
}

func TestContainerEnvVarValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	stub := newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			script := string(ctx.SelectedImpl.Script.Content)
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
		})

	if stub.CheckEnvVar(invowkfile.EnvVarCheck{Name: "", Validation: "^.+$"}) == nil {
		t.Fatal("expected empty name error")
	}

	if err := stub.CheckEnvVar(invowkfile.EnvVarCheck{Name: "HOME", Validation: "^/home/"}); err != nil {
		t.Fatalf("CheckEnvVar() = %v", err)
	}

	err := stub.CheckEnvVar(invowkfile.EnvVarCheck{Name: "MISSING"})
	if !errors.Is(err, ErrContainerEnvVarNotSet) {
		t.Fatalf("err = %v, want wrapping ErrContainerEnvVarNotSet", err)
	}

	err = stub.CheckEnvVar(invowkfile.EnvVarCheck{Name: "TRANSIENT"})
	if !errors.Is(err, ErrContainerEngineFailure) {
		t.Fatalf("err = %v, want wrapping ErrContainerEngineFailure", err)
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
		stub,
		ctx,
	)
	if err == nil {
		t.Fatal("expected env var dependency error")
	}
}

func TestContainerCapabilityValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	stub := newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script.Content), "command -v docker") {
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1}
		})

	if stub.CheckCapability(invowkfile.CapabilityName("bogus")) == nil {
		t.Fatal("expected unknown capability error")
	}
	if err := stub.CheckCapability(invowkfile.CapabilityContainers); err != nil {
		t.Fatalf("CheckCapability() = %v", err)
	}

	errorsList := collectContainerCapabilityErrors(
		[]invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY, invowkfile.CapabilityLocalAreaNetwork}}},
		stub,
		ctx,
	)
	if len(errorsList) != 1 || !strings.Contains(string(errorsList[0]), "none of capabilities [tty, local-area-network] satisfied in container") {
		t.Fatalf("errorsList = %v", errorsList)
	}

	if string(formatCapabilityAlternatives([]invowkfile.CapabilityName{invowkfile.CapabilityTTY}, false, errors.New("tty - missing"))) != "tty - missing" {
		t.Fatal("single capability alternative formatting mismatch")
	}

	err := CheckCapabilityDependenciesInContainer(
		&invowkfile.DependsOn{Capabilities: []invowkfile.CapabilityDependency{{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}}}},
		stub,
		ctx,
	)
	if err == nil {
		t.Fatal("expected capability dependency error")
	}
}

func TestContainerCommandValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	var seenScripts []string
	stub := newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			script := string(ctx.SelectedImpl.Script.Content)
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
		})

	if err := stub.CheckCommand("build"); err != nil {
		t.Fatalf("CheckCommand(build) = %v", err)
	}

	err := stub.CheckCommand("missing")
	if !errors.Is(err, ErrContainerCommandNotFound) {
		t.Fatalf("err = %v, want wrapping ErrContainerCommandNotFound", err)
	}

	err = stub.CheckCommand("broken")
	if !errors.Is(err, ErrContainerValidationFailed) {
		t.Fatalf("err = %v, want wrapping ErrContainerValidationFailed", err)
	}

	errorsList := collectContainerCommandErrors(
		[]invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandDependencyRef{"missing", "other"}}},
		stub,
		ctx,
	)
	if len(errorsList) != 1 || !strings.Contains(string(errorsList[0]), "none of [missing, other] found in container") {
		t.Fatalf("errorsList = %v", errorsList)
	}

	err = CheckCommandDependenciesInContainer(
		&invowkfile.DependsOn{Commands: []invowkfile.CommandDependency{{Alternatives: []invowkfile.CommandDependencyRef{"missing"}}}},
		stub,
		ctx,
	)
	if err == nil {
		t.Fatal("expected command dependency error")
	}

	if len(seenScripts) == 0 || !strings.Contains(seenScripts[0], "invowk internal check-cmd 'build'") {
		t.Fatalf("seenScripts = %v", seenScripts)
	}
}

func TestRuntimeDependencyProbeRequired(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	err := CheckToolDependenciesInContainer(&invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"go"}}},
	}, nil, ctx)
	if err == nil || !errors.Is(err, ErrRuntimeDependencyProbeRequired) {
		t.Fatalf("err = %v, want wrapping ErrRuntimeDependencyProbeRequired", err)
	}
}

func newDependencyExecutionContext(t *testing.T) ExecutionContext {
	t.Helper()
	return ExecutionContext{
		CommandName: "build",
		Context:     t.Context(),
	}
}

func TestCheckEnvVarDependencies(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)

	t.Run("nil deps", func(t *testing.T) {
		t.Parallel()
		if err := CheckEnvVarDependencies(nil, nil, ctx); err != nil {
			t.Fatalf("CheckEnvVarDependencies() = %v, want nil", err)
		}
	})

	t.Run("empty env vars", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{EnvVars: []invowkfile.EnvVarDependency{}}
		if err := CheckEnvVarDependencies(deps, nil, ctx); err != nil {
			t.Fatalf("CheckEnvVarDependencies() = %v, want nil", err)
		}
	})

	t.Run("existing var", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{
				{Alternatives: []invowkfile.EnvVarCheck{{Name: "HOME"}}},
			},
		}
		userEnv := map[string]string{"HOME": "/home/user"}
		if err := CheckEnvVarDependencies(deps, userEnv, ctx); err != nil {
			t.Fatalf("CheckEnvVarDependencies() = %v, want nil", err)
		}
	})

	t.Run("missing var", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{
				{Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING_VAR"}}},
			},
		}
		userEnv := map[string]string{}
		err := CheckEnvVarDependencies(deps, userEnv, ctx)
		if err == nil {
			t.Fatal("CheckEnvVarDependencies() = nil, want error")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.MissingEnvVars) == 0 {
			t.Fatal("depErr.MissingEnvVars is empty, want at least one entry")
		}
	})

	t.Run("regex match", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{
				{Alternatives: []invowkfile.EnvVarCheck{{Name: "PORT", Validation: "^[0-9]+$"}}},
			},
		}
		userEnv := map[string]string{"PORT": "8080"}
		if err := CheckEnvVarDependencies(deps, userEnv, ctx); err != nil {
			t.Fatalf("CheckEnvVarDependencies() = %v, want nil", err)
		}
	})

	t.Run("regex fail", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			EnvVars: []invowkfile.EnvVarDependency{
				{Alternatives: []invowkfile.EnvVarCheck{{Name: "PORT", Validation: "^[0-9]+$"}}},
			},
		}
		userEnv := map[string]string{"PORT": "not-a-number"}
		err := CheckEnvVarDependencies(deps, userEnv, ctx)
		if err == nil {
			t.Fatal("CheckEnvVarDependencies() = nil, want error")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
	})
}
