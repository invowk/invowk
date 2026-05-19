// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
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

func TestCheckCustomCheckDependenciesInContainer(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script.Content), "echo ok") {
				_, _ = io.WriteString(ctx.IO.Stdout, "ok")
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1, Error: shellExitError(t)}
		},
	}
	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{
			{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "first", Script: invowkfile.CustomCheckScript{Content: "exit 1"}},
					{Name: "second", Script: invowkfile.CustomCheckScript{Content: "echo ok"}, ExpectedOutput: "^ok$"},
				},
			},
		},
	}

	if err := CheckCustomCheckDependenciesInContainer(deps, stub, ctx); err != nil {
		t.Fatalf("CheckCustomCheckDependenciesInContainer() = %v", err)
	}

	err := CheckCustomCheckDependenciesInContainer(
		&invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "first", Script: invowkfile.CustomCheckScript{Content: "exit 1"}},
					{Name: "second", Script: invowkfile.CustomCheckScript{Content: "exit 1"}},
				},
			}},
		},
		stub,
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

	check := invowkfile.CustomCheck{Name: "demo", Script: invowkfile.CustomCheckScript{Content: "echo ok"}, ExpectedOutput: "^ok$"}

	probe := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			_, _ = io.WriteString(ctx.IO.Stdout, "ok")
			return &runtimepkg.Result{ExitCode: 0}
		},
	}
	result, validateErr := probe.RunCustomCheck(check)
	if validateErr != nil {
		t.Fatalf("RunCustomCheck() = %v", validateErr)
	}
	if validateErr := ValidateCustomCheckOutput(check, result); validateErr != nil {
		t.Fatalf("ValidateCustomCheckOutput() = %v", validateErr)
	}

	probe = &filepathStubRuntime{
		execFn: func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
			return &runtimepkg.Result{ExitCode: 1, Error: errors.New("engine down")}
		},
	}
	_, err := probe.RunCustomCheck(check)
	if !errors.Is(err, ErrContainerValidationFailed) {
		t.Fatalf("err = %v, want wrapping ErrContainerValidationFailed", err)
	}
}

func TestContainerEnvVarValidation(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
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
		},
	}
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
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script.Content), "command -v docker") {
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1}
		},
	}
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
	stub := &filepathStubRuntime{
		execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
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
		},
	}
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

func TestValidateCustomCheckOutput(t *testing.T) {
	t.Parallel()

	expectedZero := types.ExitCode(0)
	expectedTwo := types.ExitCode(2)

	tests := []struct {
		name    string
		check   invowkfile.CustomCheck
		result  CustomCheckResult
		wantErr string // empty = expect nil
	}{
		{
			name:  "success with zero exit code and no pattern",
			check: invowkfile.CustomCheck{Name: "demo"},
		},
		{
			name:    "exit code mismatch",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedCode: &expectedZero},
			result:  mustCustomCheckResult(t, "", 1),
			wantErr: "returned exit code",
		},
		{
			name:    "expected non-zero exit code matches",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedCode: &expectedTwo},
			result:  mustCustomCheckResult(t, "", 1), // exit code 1 != expected 2
			wantErr: "returned exit code",
		},
		{
			name:   "output matches regex pattern",
			check:  invowkfile.CustomCheck{Name: "demo", ExpectedOutput: "^ok$"},
			result: mustCustomCheckResult(t, "ok", 0),
		},
		{
			name:    "output does not match regex pattern",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedOutput: "^ok$"},
			result:  mustCustomCheckResult(t, "fail", 0),
			wantErr: "does not match pattern",
		},
		{
			name:    "invalid regex pattern",
			check:   invowkfile.CustomCheck{Name: "demo", ExpectedOutput: "[invalid"},
			result:  mustCustomCheckResult(t, "anything", 0),
			wantErr: "invalid regex pattern",
		},
		{
			name:    "non-zero result fails default expected code",
			check:   invowkfile.CustomCheck{Name: "demo"},
			result:  mustCustomCheckResult(t, "", 1),
			wantErr: "returned exit code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCustomCheckOutput(tt.check, tt.result)
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

func mustCustomCheckResult(t testing.TB, output string, exitCode types.ExitCode) CustomCheckResult {
	t.Helper()

	outputValue := CustomCheckOutput(output)
	if err := outputValue.Validate(); err != nil {
		t.Fatalf("CustomCheckOutput.Validate() = %v", err)
	}
	result, err := NewCustomCheckResult(outputValue, exitCode)
	if err != nil {
		t.Fatalf("NewCustomCheckResult() = %v", err)
	}
	return result
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
					{Name: "echo", Script: invowkfile.CustomCheckScript{Content: "echo ok"}},
				},
			}},
		}
		if err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, &recordingHostProbe{}); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("failing check returns dependency error", func(t *testing.T) {
		t.Parallel()
		deps := &invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Alternatives: []invowkfile.CustomCheck{
					{Name: "fail", Script: invowkfile.CustomCheckScript{Content: "exit 1"}},
				},
			}},
		}
		err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, &recordingHostProbe{
			checkErrors: map[invowkfile.CheckName]error{
				"fail": errors.New("check failed"),
			},
		})
		if err == nil {
			t.Fatal("expected error")
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
	})

	t.Run("invalid check does not execute probe", func(t *testing.T) {
		t.Parallel()

		probe := &recordingHostProbe{}
		deps := &invowkfile.DependsOn{
			CustomChecks: []invowkfile.CustomCheckDependency{{
				Name: "empty-script",
			}},
		}
		err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, probe)
		if err == nil {
			t.Fatal("expected dependency error")
		}
		if len(probe.checks) != 0 {
			t.Fatalf("probe executed %d checks, want 0", len(probe.checks))
		}
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.FailedCustomChecks) != 1 {
			t.Fatalf("FailedCustomChecks = %d, want 1", len(depErr.FailedCustomChecks))
		}
	})
}

func TestCustomCheckScriptFileResolution(t *testing.T) {
	t.Parallel()

	expectedCode := types.ExitCode(0)
	moduleDir := t.TempDir()
	scriptsDir := filepath.Join(moduleDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	scriptPath := filepath.Join(scriptsDir, "check")
	if err := os.WriteFile(scriptPath, []byte("echo from file"), 0o644); err != nil {
		t.Fatalf("failed to write script file: %v", err)
	}
	scriptFile := invowkfile.FilesystemPath("scripts/check")
	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Name:           "file-check",
			Script:         invowkfile.CustomCheckScript{File: &scriptFile, Interpreter: "bash"},
			ExpectedCode:   &expectedCode,
			ExpectedOutput: "^ok$",
		}},
	}
	ctx := ExecutionContext{
		CommandName:      "build",
		Context:          t.Context(),
		SourceModulePath: customCheckModulePath(moduleDir),
		ReadScriptFile:   os.ReadFile,
	}

	t.Run("host probe receives resolved content", func(t *testing.T) {
		t.Parallel()

		probe := &recordingHostProbe{
			checkResults: map[invowkfile.CheckName]CustomCheckResult{
				"file-check": mustCustomCheckResult(t, "ok", 0),
			},
		}
		if err := CheckHostCustomCheckDependenciesWithProbe(deps, ctx, probe); err != nil {
			t.Fatalf("CheckHostCustomCheckDependenciesWithProbe() = %v", err)
		}
		if len(probe.checkScripts) != 1 || probe.checkScripts[0] != "echo from file" {
			t.Fatalf("probe.checkScripts = %v, want resolved file content", probe.checkScripts)
		}
		if len(probe.checkInterps) != 1 || probe.checkInterps[0] != "bash" {
			t.Fatalf("probe.checkInterps = %v, want bash", probe.checkInterps)
		}
	})

	t.Run("container probe receives resolved content", func(t *testing.T) {
		t.Parallel()

		var seenScript string
		var seenInterp invowkfile.InterpreterSpec
		stub := &filepathStubRuntime{
			execFn: func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
				seenScript = string(ctx.SelectedImpl.Script.Content)
				seenInterp = ctx.SelectedImpl.Script.Interpreter
				_, _ = io.WriteString(ctx.IO.Stdout, "ok\n")
				return &runtimepkg.Result{ExitCode: 0}
			},
		}
		if err := CheckCustomCheckDependenciesInContainer(deps, stub, ctx); err != nil {
			t.Fatalf("CheckCustomCheckDependenciesInContainer() = %v", err)
		}
		if seenScript != "echo from file" {
			t.Fatalf("container script = %q, want resolved file content", seenScript)
		}
		if seenInterp != "bash" {
			t.Fatalf("container interpreter = %q, want bash", seenInterp)
		}
	})
}

func TestCustomCheckScriptFileResolutionFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ctx    ExecutionContext
		script invowkfile.CustomCheckScript
		want   string
	}{
		{
			name: "non-module file rejected",
			ctx: ExecutionContext{
				CommandName: "build",
				Context:     t.Context(),
			},
			script: customCheckFileScript("scripts/check.sh"),
			want:   "script file requires module invowkfile",
		},
		{
			name: "missing file reports selected path",
			ctx: ExecutionContext{
				CommandName:      "build",
				Context:          t.Context(),
				SourceModulePath: customCheckModulePath(t.TempDir()),
				ReadScriptFile:   os.ReadFile,
			},
			script: customCheckFileScript("scripts/missing.sh"),
			want:   "scripts/missing.sh",
		},
		{
			name:   "invalid resolved content rejected",
			ctx:    customCheckFileContext(t, "   \n\t"),
			script: customCheckFileScript("scripts/check.sh"),
			want:   "invalid script content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := &invowkfile.DependsOn{
				CustomChecks: []invowkfile.CustomCheckDependency{{
					Name:   "file-check",
					Script: tt.script,
				}},
			}
			probe := &recordingHostProbe{}
			err := CheckHostCustomCheckDependenciesWithProbe(deps, tt.ctx, probe)
			if err == nil {
				t.Fatal("CheckHostCustomCheckDependenciesWithProbe() error = nil, want dependency error")
			}
			var depErr *DependencyError
			if !errors.As(err, &depErr) {
				t.Fatalf("errors.As(*DependencyError) = false for %T", err)
			}
			if len(depErr.FailedCustomChecks) != 1 || !strings.Contains(depErr.FailedCustomChecks[0].String(), tt.want) {
				t.Fatalf("FailedCustomChecks = %v, want containing %q", depErr.FailedCustomChecks, tt.want)
			}
			if len(probe.checks) != 0 {
				t.Fatalf("probe executed %d checks, want 0", len(probe.checks))
			}
		})
	}
}

func newDependencyExecutionContext(t *testing.T) ExecutionContext {
	t.Helper()
	return ExecutionContext{
		CommandName: "build",
		Context:     t.Context(),
	}
}

func customCheckFileScript(path string) invowkfile.CustomCheckScript {
	scriptFile := invowkfile.FilesystemPath(path)
	return invowkfile.CustomCheckScript{File: &scriptFile}
}

func customCheckModulePath(path string) *invowkfile.FilesystemPath {
	modulePath := invowkfile.FilesystemPath(path)
	return &modulePath
}

func customCheckFileContext(t *testing.T, content string) ExecutionContext {
	t.Helper()

	moduleDir := t.TempDir()
	scriptsDir := filepath.Join(moduleDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("failed to create scripts dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "check.sh"), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write script file: %v", err)
	}
	return ExecutionContext{
		CommandName:      "build",
		Context:          t.Context(),
		SourceModulePath: customCheckModulePath(moduleDir),
		ReadScriptFile:   os.ReadFile,
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

// TestEvaluateCustomChecks_PropagatesContext verifies that evaluateCustomChecks
// extracts the Go context from ExecutionContext and passes it to the validator (SC-07).
func TestEvaluateCustomChecks_PropagatesContext(t *testing.T) {
	t.Parallel()

	// Create a cancelled context to verify propagation.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	execCtx := ExecutionContext{
		CommandName: "test",
		Context:     ctx,
	}

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Alternatives: []invowkfile.CustomCheck{
				{Name: "check-ctx", Script: invowkfile.CustomCheckScript{Content: "true"}},
			},
		}},
	}

	var receivedCtx atomic.Pointer[context.Context]
	validator := func(goCtx context.Context, _ invowkfile.CustomCheck) (CustomCheckResult, error) {
		receivedCtx.Store(&goCtx)
		return CustomCheckResult{}, nil
	}

	_ = evaluateCustomChecks(deps, execCtx, validator)

	got := receivedCtx.Load()
	if got == nil {
		t.Fatal("validator did not receive a context")
	}

	// The context passed to the validator should be cancelled
	// because we cancelled it above.
	if (*got).Err() == nil {
		t.Error("expected cancelled context to be propagated, but context.Err() is nil")
	}
}

// TestEvaluateCustomChecks_NilContextFallback verifies that evaluateCustomChecks
// falls back to context.Background() when ExecutionContext.Context is nil.
func TestEvaluateCustomChecks_NilContextFallback(t *testing.T) {
	t.Parallel()

	execCtx := ExecutionContext{
		CommandName: "test",
		Context:     nil, // nil context
	}

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Alternatives: []invowkfile.CustomCheck{
				{Name: "check", Script: invowkfile.CustomCheckScript{Content: "true"}},
			},
		}},
	}

	var receivedCtx atomic.Pointer[context.Context]
	validator := func(goCtx context.Context, _ invowkfile.CustomCheck) (CustomCheckResult, error) {
		receivedCtx.Store(&goCtx)
		return CustomCheckResult{}, nil
	}

	_ = evaluateCustomChecks(deps, execCtx, validator)

	got := receivedCtx.Load()
	if got == nil {
		t.Fatal("validator did not receive a context")
	}
	// Should not be cancelled (context.Background() has no cancellation).
	if (*got).Err() != nil {
		t.Errorf("expected non-cancelled fallback context, got err: %v", (*got).Err())
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

	t.Run("injected checker accepts alternative", func(t *testing.T) {
		t.Parallel()

		deps := &invowkfile.DependsOn{
			Capabilities: []invowkfile.CapabilityDependency{
				{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityInternet}},
			},
		}
		if err := CheckCapabilityDependenciesWithChecker(deps, ctx, fakeCapabilityChecker{}); err != nil {
			t.Fatalf("CheckCapabilityDependenciesWithChecker() = %v, want nil", err)
		}
	})

	t.Run("injected checker reports missing alternative", func(t *testing.T) {
		t.Parallel()

		deps := &invowkfile.DependsOn{
			Capabilities: []invowkfile.CapabilityDependency{
				{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityInternet}},
			},
		}
		checker := fakeCapabilityChecker{
			invowkfile.CapabilityInternet: &invowkfile.CapabilityError{
				Capability: invowkfile.CapabilityInternet,
				Message:    "offline",
			},
		}

		err := CheckCapabilityDependenciesWithChecker(deps, ctx, checker)
		var depErr *DependencyError
		if !errors.As(err, &depErr) {
			t.Fatalf("errors.As(*DependencyError) = false for %T", err)
		}
		if len(depErr.MissingCapabilities) != 1 {
			t.Fatalf("missing capabilities = %d, want 1", len(depErr.MissingCapabilities))
		}
	})

	t.Run("injected checker receives request scoped context and io", func(t *testing.T) {
		t.Parallel()

		stdout := &strings.Builder{}
		stderr := &strings.Builder{}
		ioCtx := IOContext{Stdout: stdout, Stderr: stderr}
		ctx := newDependencyExecutionContext(t)
		ctx.Context = t.Context()
		ctx.IO = ioCtx
		deps := &invowkfile.DependsOn{
			Capabilities: []invowkfile.CapabilityDependency{
				{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityTTY}},
			},
		}
		checker := &recordingCapabilityChecker{}

		if err := CheckCapabilityDependenciesWithChecker(deps, ctx, checker); err != nil {
			t.Fatalf("CheckCapabilityDependenciesWithChecker() = %v, want nil", err)
		}
		if len(checker.requests) != 1 {
			t.Fatalf("recorded requests = %d, want 1", len(checker.requests))
		}
		got := checker.requests[0]
		if got.ctx != ctx.Context {
			t.Fatal("capability checker did not receive execution context")
		}
		if got.ioCtx.Stdout != stdout || got.ioCtx.Stderr != stderr {
			t.Fatal("capability checker did not receive execution IO")
		}
		if got.capability != invowkfile.CapabilityTTY {
			t.Fatalf("Capability = %q, want %q", got.capability, invowkfile.CapabilityTTY)
		}
	})
}
