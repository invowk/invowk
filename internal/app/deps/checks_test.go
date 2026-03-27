// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"io"
	"os/exec"
	goruntime "runtime"
	"strings"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestCheckCustomCheckDependenciesInContainer(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
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

	ctx := newDependencyExecutionContext(t)
	check := invowkfile.CustomCheck{Name: "demo", CheckScript: "echo ok", ExpectedOutput: "^ok$"}

	err := validateCustomCheckInContainer(check, runtimepkg.NewRegistry(), ctx)
	if err == nil || !errors.Is(err, ErrContainerRuntimeNotAvailable) {
		t.Fatalf("err = %v, want wrapping ErrContainerRuntimeNotAvailable", err)
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

	ctx := newDependencyExecutionContext(t)
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

	ctx := newDependencyExecutionContext(t)
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

	ctx := newDependencyExecutionContext(t)
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
	if err == nil || !errors.Is(err, ErrContainerRuntimeNotAvailable) {
		t.Fatalf("err = %v, want wrapping ErrContainerRuntimeNotAvailable", err)
	}
}

func TestValidateCustomCheckOutput(t *testing.T) {
	t.Parallel()

	expectedZero := types.ExitCode(0)
	expectedTwo := types.ExitCode(2)
	exitErr := shellExitError(t)

	tests := []struct {
		name    string
		check   invowkfile.CustomCheck
		output  string
		execErr error
		wantErr string // empty = expect nil
	}{
		{
			name:  "success with zero exit code and no pattern",
			check: invowkfile.CustomCheck{Name: "demo"},
		},
		{
			name:    "exit code mismatch",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedCode: &expectedZero},
			execErr: exitErr,
			wantErr: "returned exit code",
		},
		{
			name:    "expected non-zero exit code matches",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedCode: &expectedTwo},
			execErr: exitErr, // exit code 1 != expected 2
			wantErr: "returned exit code",
		},
		{
			name:   "output matches regex pattern",
			check:  invowkfile.CustomCheck{Name: "demo", ExpectedOutput: "^ok$"},
			output: "ok",
		},
		{
			name:    "output does not match regex pattern",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedOutput: "^ok$"},
			output:  "fail",
			wantErr: "does not match pattern",
		},
		{
			name:    "invalid regex pattern",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedOutput: "[invalid"},
			output:  "anything",
			wantErr: "invalid regex pattern",
		},
		{
			name:    "non-ExitError defaults to exit code 1",
			check:   invowkfile.CustomCheck{Name: "demo"},
			execErr: errors.New("generic failure"),
			wantErr: "returned exit code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCustomCheckOutput(tt.check, tt.output, tt.execErr)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateCustomCheckOutput() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateCustomCheckOutput() = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateCustomCheckOutput() = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestCheckHostCustomCheckDependencies(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)

	t.Run("nil deps returns nil", func(t *testing.T) {
		t.Parallel()
		if err := CheckHostCustomCheckDependencies(nil, ctx); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("empty custom checks returns nil", func(t *testing.T) {
		t.Parallel()
		if err := CheckHostCustomCheckDependencies(&invowkfile.DependsOn{}, ctx); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("passing check succeeds", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "echo", CheckScript: "echo ok"},
				},
			}},
		}
		if err := CheckHostCustomCheckDependencies(deps, ctx); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("failing check returns dependency error", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "fail", CheckScript: "exit 1"},
				},
			}},
		}
		err := CheckHostCustomCheckDependencies(deps, ctx)
		if err == nil {
			t.Fatal("expected error")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
	})
}

func newDependencyExecutionContext(t *testing.T) *runtimepkg.ExecutionContext {
	t.Helper()
	return &runtimepkg.ExecutionContext{
		Command: &invowkfile.Command{Name: "build"},
		Context: t.Context(),
	}
}

func shellExitError(t *testing.T) error {
	t.Helper()

	script := "exit 1"
	if goruntime.GOOS == "windows" {
		script = "exit /b 1"
	}
	shellPath, shellArgs := testutil.FixedShellCommand(script)
	cmd := exec.CommandContext(t.Context(), shellPath, shellArgs...)
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit error")
	}
	return err
}

func TestCapabilityCheckScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cap          invowkfile.CapabilityName
		wantNonEmpty bool
	}{
		{"internet", invowkfile.CapabilityInternet, true},
		{"containers", invowkfile.CapabilityContainers, true},
		{"lan", invowkfile.CapabilityLocalAreaNetwork, true},
		{"tty", invowkfile.CapabilityTTY, true},
		{"unknown", invowkfile.CapabilityName("bogus"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CapabilityCheckScript(tt.cap)
			if tt.wantNonEmpty && got == "" {
				t.Fatalf("CapabilityCheckScript(%q) = empty, want non-empty script", tt.cap)
			}
			if !tt.wantNonEmpty && got != "" {
				t.Fatalf("CapabilityCheckScript(%q) = %q, want empty string", tt.cap, got)
			}
		})
	}
}

func TestShellEscapeSingleQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no quotes", "hello world", "hello world"},
		{"single quote", "it's", `it'\''s`},
		{"multiple quotes", "a'b'c", `a'\''b'\''c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShellEscapeSingleQuote(tt.input)
			if got != tt.want {
				t.Fatalf("ShellEscapeSingleQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
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

func TestCheckCapabilityDependencies(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)

	t.Run("nil deps", func(t *testing.T) {
		t.Parallel()
		if err := CheckCapabilityDependencies(nil, ctx); err != nil {
			t.Fatalf("CheckCapabilityDependencies() = %v, want nil", err)
		}
	})

	t.Run("empty capabilities", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{Capabilities: []invowkfile.CapabilityDependency{}}
		if err := CheckCapabilityDependencies(deps, ctx); err != nil {
			t.Fatalf("CheckCapabilityDependencies() = %v, want nil", err)
		}
	})
}
