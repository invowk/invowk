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

func TestCheckCustomCheckDependenciesInContainer(t *testing.T) {
	t.Parallel()

	ctx := newDependencyExecutionContext(t)
	stub := newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			if strings.Contains(string(ctx.SelectedImpl.Script.Content), "echo ok") {
				_, _ = io.WriteString(ctx.IO.Stdout, "ok")
				return &runtimepkg.Result{ExitCode: 0}
			}
			return &runtimepkg.Result{ExitCode: 1, Error: shellExitError(t)}
		})

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

	probe := newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			_, _ = io.WriteString(ctx.IO.Stdout, "ok")
			return &runtimepkg.Result{ExitCode: 0}
		})

	result, validateErr := probe.RunCustomCheck(check)
	if validateErr != nil {
		t.Fatalf("RunCustomCheck() = %v", validateErr)
	}
	if validateErr := ValidateCustomCheckOutput(check, result); validateErr != nil {
		t.Fatalf("ValidateCustomCheckOutput() = %v", validateErr)
	}

	probe = newFilepathStubRuntime(t,
		func(_ *runtimepkg.ExecutionContext) *runtimepkg.Result {
			return &runtimepkg.Result{ExitCode: 1, Error: errors.New("engine down")}
		})

	_, err := probe.RunCustomCheck(check)
	if !errors.Is(err, ErrContainerValidationFailed) {
		t.Fatalf("err = %v, want wrapping ErrContainerValidationFailed", err)
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
			name:   "expected non-zero exit code matches",
			check:  invowkfile.CustomCheck{Name: "demo", ExpectedCode: &expectedTwo},
			result: mustCustomCheckResult(t, "", 2),
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
		if got := depErr.FailedCustomChecks[0].String(); !strings.Contains(got, "invalid custom check dependency") || !strings.Contains(got, "custom check script must set content or file") {
			t.Fatalf("FailedCustomChecks[0] = %q, want validation detail", got)
		}
	})
}

func TestCustomCheckInterpreterTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target customCheckInterpreterTarget
		text   string
		valid  bool
	}{
		{name: "host", target: customCheckInterpreterTargetHost, text: "host", valid: true},
		{name: "runtime", target: customCheckInterpreterTargetRuntime, text: "runtime", valid: true},
		{name: "unknown", target: customCheckInterpreterTarget(99), text: "unknown(99)", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.target.String(); got != tt.text {
				t.Fatalf("String() = %q, want %q", got, tt.text)
			}
			err := tt.target.Validate()
			if tt.valid && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
		})
	}
}

func TestCustomCheckAnalysisRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target customCheckInterpreterTarget
		script invowkfile.ScriptContent
		want   invowkfile.RuntimeMode
	}{
		{
			name:   "runtime checks analyze as container",
			target: customCheckInterpreterTargetRuntime,
			script: "#!/bin/sh\necho ok\n",
			want:   invowkfile.RuntimeContainer,
		},
		{
			name:   "host non-shell shebang analyzes as native",
			target: customCheckInterpreterTargetHost,
			script: "#!/usr/bin/env python3\nprint('ok')\n",
			want:   invowkfile.RuntimeNative,
		},
		{
			name:   "host shell shebang analyzes as virtual shell",
			target: customCheckInterpreterTargetHost,
			script: "#!/bin/sh\necho ok\n",
			want:   invowkfile.RuntimeVirtualSh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := customCheckAnalysisRuntime(invowkfile.CustomCheckScript{}, tt.script, tt.target)
			if got != tt.want {
				t.Fatalf("customCheckAnalysisRuntime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReportCustomCheckInterpreterDiagnosticsNilReporter(t *testing.T) {
	t.Parallel()

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("reportCustomCheckInterpreterDiagnostics panicked with nil reporter: %v", recovered)
		}
	}()

	reportCustomCheckInterpreterDiagnostics(ExecutionContext{}, []invowkfile.ScriptInterpreterDiagnostic{
		{},
	})
}

func TestCustomCheckScriptFileResolution_HostProbeReceivesResolvedContent(t *testing.T) {
	t.Parallel()

	fixture := newCustomCheckScriptFileResolutionFixture(t)
	probe := &recordingHostProbe{
		checkResults: map[invowkfile.CheckName]CustomCheckResult{
			"file-check": mustCustomCheckResult(t, "ok", 0),
		},
	}
	if err := CheckHostCustomCheckDependenciesWithProbe(fixture.deps, fixture.ctx, probe); err != nil {
		t.Fatalf("CheckHostCustomCheckDependenciesWithProbe() = %v", err)
	}
	assertResolvedHostCheck(t, probe)
}

func TestCustomCheckScriptFileResolution_ContainerProbeReceivesResolvedContent(t *testing.T) {
	t.Parallel()

	fixture := newCustomCheckScriptFileResolutionFixture(t)
	seenScript, seenInterp := runContainerScriptFileResolution(t, fixture)
	if seenScript != "echo from file" {
		t.Fatalf("container script = %q, want resolved file content", seenScript)
	}
	if seenInterp != "bash" {
		t.Fatalf("container interpreter = %q, want bash", seenInterp)
	}
}

func newCustomCheckScriptFileResolutionFixture(t *testing.T) customCheckScriptFileResolutionFixture {
	t.Helper()

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
	scriptFile := invowkfile.ScriptFilePath("scripts/check")
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
	return customCheckScriptFileResolutionFixture{deps: deps, ctx: ctx}
}

func assertResolvedHostCheck(t *testing.T, probe *recordingHostProbe) {
	t.Helper()

	if len(probe.checkScripts) != 1 || probe.checkScripts[0] != "echo from file" {
		t.Fatalf("probe.checkScripts = %v, want resolved file content", probe.checkScripts)
	}
	if len(probe.checkInterps) != 1 || probe.checkInterps[0] != "bash" {
		t.Fatalf("probe.checkInterps = %v, want bash", probe.checkInterps)
	}
}

func runContainerScriptFileResolution(t *testing.T, fixture customCheckScriptFileResolutionFixture) (string, invowkfile.InterpreterSpec) {
	t.Helper()

	var seenScript string
	var seenInterp invowkfile.InterpreterSpec
	stub := newFilepathStubRuntime(t,
		func(ctx *runtimepkg.ExecutionContext) *runtimepkg.Result {
			seenScript = string(ctx.SelectedImpl.Script.Content)
			seenInterp = ctx.SelectedImpl.Script.Interpreter
			_, _ = io.WriteString(ctx.IO.Stdout, "ok\n")
			return &runtimepkg.Result{ExitCode: 0}
		})

	if err := CheckCustomCheckDependenciesInContainer(fixture.deps, stub, fixture.ctx); err != nil {
		t.Fatalf("CheckCustomCheckDependenciesInContainer() = %v", err)
	}
	return seenScript, seenInterp
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

func customCheckFileScript(path string) invowkfile.CustomCheckScript {
	scriptFile := invowkfile.ScriptFilePath(path)
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

	_ = evaluateCustomChecks(deps, execCtx, customCheckInterpreterTargetHost, validator)

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

	_ = evaluateCustomChecks(deps, execCtx, customCheckInterpreterTargetHost, validator)

	got := receivedCtx.Load()
	if got == nil {
		t.Fatal("validator did not receive a context")
	}
	// Should not be cancelled (context.Background() has no cancellation).
	if (*got).Err() != nil {
		t.Errorf("expected non-cancelled fallback context, got err: %v", (*got).Err())
	}
}
