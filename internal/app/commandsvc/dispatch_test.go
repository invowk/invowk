// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/invowkfiletest"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	stubRuntime struct {
		name          string
		executeCalled int
		executeResult *runtimepkg.Result
		validateErr   error
	}

	stubInteractiveRuntime struct {
		stubRuntime
		supports   bool
		prepareErr error
		prepared   *runtimepkg.PreparedCommand
	}

	stubInteractiveExecutor struct {
		result *runtimepkg.Result
		called int
	}

	recordingExecutionObserver struct {
		startedCommand       invowkfile.CommandName
		interactiveFallback  invowkfile.RuntimeMode
		commandStartingCalls int
		fallbackCalls        int
	}
)

func (s *stubRuntime) Name() string { return s.name }

func (s *stubRuntime) Execute(*runtimepkg.ExecutionContext) *runtimepkg.Result {
	s.executeCalled++
	if s.executeResult != nil {
		return s.executeResult
	}
	return &runtimepkg.Result{ExitCode: 0}
}

func (s *stubRuntime) Available() bool { return true }

func (s *stubRuntime) Validate(*runtimepkg.ExecutionContext) error { return s.validateErr }

func (s *stubInteractiveRuntime) SupportsInteractive() bool { return s.supports }

func (s *stubInteractiveRuntime) PrepareInteractive(execCtx *runtimepkg.ExecutionContext) (*runtimepkg.PreparedCommand, error) {
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	if s.prepared != nil {
		return s.prepared, nil
	}
	shellPath, shellArgs := testutil.FixedShellCommand("exit 0")
	return &runtimepkg.PreparedCommand{Cmd: exec.CommandContext(execCtx.Context, shellPath, shellArgs...)}, nil
}

func (s *stubInteractiveExecutor) Execute(execCtx *runtimepkg.ExecutionContext, _ invowkfile.CommandName, interactiveRT runtimepkg.InteractiveRuntime) *runtimepkg.Result {
	s.called++
	if s.result != nil {
		return s.result
	}
	if err := interactiveRT.Validate(execCtx); err != nil {
		return &runtimepkg.Result{ExitCode: 1, Error: err}
	}
	if _, err := interactiveRT.PrepareInteractive(execCtx); err != nil {
		return &runtimepkg.Result{ExitCode: 1, Error: err}
	}
	return &runtimepkg.Result{ExitCode: 0}
}

func (o *recordingExecutionObserver) CommandStarting(name invowkfile.CommandName) {
	o.startedCommand = name
	o.commandStartingCalls++
}

func (o *recordingExecutionObserver) InteractiveFallback(runtimeName invowkfile.RuntimeMode) {
	o.interactiveFallback = runtimeName
	o.fallbackCalls++
}

func TestDispatchExecution_Success(t *testing.T) {
	t.Parallel()

	svc := &Service{
		hostAccess:      noopHostAccess{},
		registryFactory: defaultRuntimeRegistryFactory{},
		interactive:     defaultInteractiveExecutor{},
	}
	cmdInfo, execCtx, execStdout := commandInfoAndContext(t, "echo hello")

	result, diags, err := svc.dispatchExecution(
		Request{Name: "build", UserEnv: map[string]string{}},
		execCtx,
		cmdInfo,
		config.DefaultConfig(),
		nil,
	)
	if err != nil {
		t.Fatalf("dispatchExecution() error = %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("len(diags) = %d, want 0", len(diags))
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want 0", result.ExitCode)
	}
	if execCtx.ExecutionID == "" {
		t.Fatal("ExecutionID was not assigned")
	}
	if !strings.Contains(execStdout.String(), "hello") {
		t.Fatalf("exec stdout = %q, want output containing hello", execStdout.String())
	}
}

func TestDispatchExecution_DependencyError(t *testing.T) {
	t.Parallel()

	svc := &Service{
		hostAccess:      noopHostAccess{},
		registryFactory: defaultRuntimeRegistryFactory{},
		interactive:     defaultInteractiveExecutor{},
	}
	cmdInfo, execCtx, _ := commandInfoAndContext(t, "echo hello")
	cmdInfo.Invowkfile.DependsOn = &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING"}}}},
	}

	_, _, err := svc.dispatchExecution(
		Request{Name: "build", UserEnv: map[string]string{}},
		execCtx,
		cmdInfo,
		config.DefaultConfig(),
		nil,
	)
	if err == nil {
		t.Fatal("expected dependency error")
	}
	var depErr *deps.DependencyError
	if !errors.As(err, &depErr) {
		t.Fatalf("errors.As(*DependencyError) = false for %T", err)
	}
}

func TestAppendRuntimeRegistryDiagnostics(t *testing.T) {
	t.Parallel()

	diag, err := discovery.NewDiagnostic(discovery.SeverityWarning, discovery.CodeConfigLoadFailed, "warn")
	if err != nil {
		t.Fatalf("NewDiagnostic(): %v", err)
	}

	base := []discovery.Diagnostic{}
	result := appendRuntimeRegistryDiagnostics(base, Request{Verbose: true}, &runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtual}, RuntimeRegistryResult{Diagnostics: []discovery.Diagnostic{diag}})
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	result = appendRuntimeRegistryDiagnostics(base, Request{}, &runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeContainer}, RuntimeRegistryResult{Diagnostics: []discovery.Diagnostic{diag}})
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	result = appendRuntimeRegistryDiagnostics(base, Request{}, &runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtual}, RuntimeRegistryResult{Diagnostics: []discovery.Diagnostic{diag}})
	if len(result) != 0 {
		t.Fatalf("len(result) = %d, want 0", len(result))
	}
}

func TestFailFastContainerInit(t *testing.T) {
	t.Parallel()

	if err := failFastContainerInit(nil, invowkfile.RuntimeContainer); err != nil {
		t.Fatalf("failFastContainerInit(nil) = %v", err)
	}
	if err := failFastContainerInit(errors.New("boom"), invowkfile.RuntimeVirtual); err != nil {
		t.Fatalf("failFastContainerInit(non-container) = %v", err)
	}

	err := failFastContainerInit(errors.New("boom"), invowkfile.RuntimeContainer)
	if err == nil {
		t.Fatal("expected classified error")
	}
	var classified *ClassifiedError
	if !errors.As(err, &classified) {
		t.Fatalf("errors.As(*ClassifiedError) = false for %T", err)
	}
}

func TestApplyExecutionTimeout(t *testing.T) {
	t.Parallel()

	noImplCtx := &runtimepkg.ExecutionContext{Context: t.Context()}
	cancel, err := applyExecutionTimeout(noImplCtx)
	if err != nil {
		t.Fatalf("applyExecutionTimeout(no impl) = %v", err)
	}
	cancel()

	invalidCtx := &runtimepkg.ExecutionContext{
		Context:      t.Context(),
		SelectedImpl: &invowkfile.Implementation{Timeout: "bogus"},
	}
	if _, timeoutErr := applyExecutionTimeout(invalidCtx); timeoutErr == nil {
		t.Fatal("expected invalid timeout error")
	}

	validCtx := &runtimepkg.ExecutionContext{
		Context:      t.Context(),
		SelectedImpl: &invowkfile.Implementation{Timeout: "20ms"},
	}
	cancel, err = applyExecutionTimeout(validCtx)
	if err != nil {
		t.Fatalf("applyExecutionTimeout(valid) = %v", err)
	}
	defer cancel()
	deadline, ok := validCtx.Context.Deadline()
	if !ok {
		t.Fatal("deadline not set")
	}
	if time.Until(deadline) <= 0 {
		t.Fatal("deadline already expired")
	}
}

func TestExecuteWithRequestedMode(t *testing.T) {
	t.Parallel()

	t.Run("non-interactive uses registry execute", func(t *testing.T) {
		t.Parallel()

		registry := runtimepkg.NewRegistry()
		rt := &stubRuntime{name: "stub", executeResult: &runtimepkg.Result{ExitCode: 7}}
		registry.Register(runtimepkg.RuntimeTypeVirtual, rt)

		svc := &Service{}
		result, err := svc.executeWithRequestedMode(
			Request{Interactive: false},
			&runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtual},
			registry,
		)
		if err != nil {
			t.Fatalf("executeWithRequestedMode() error = %v", err)
		}
		if result.ExitCode != 7 || rt.executeCalled != 1 {
			t.Fatalf("result=%v executeCalled=%d", result, rt.executeCalled)
		}
	})

	t.Run("interactive fallback logs and executes normally", func(t *testing.T) {
		t.Parallel()

		registry := runtimepkg.NewRegistry()
		rt := &stubRuntime{name: "stub", executeResult: &runtimepkg.Result{ExitCode: 3}}
		registry.Register(runtimepkg.RuntimeTypeVirtual, rt)

		observer := &recordingExecutionObserver{}
		svc := &Service{observer: observer}
		result, err := svc.executeWithRequestedMode(
			Request{Interactive: true, Verbose: true},
			&runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtual},
			registry,
		)
		if err != nil {
			t.Fatalf("executeWithRequestedMode() error = %v", err)
		}
		if result.ExitCode != 3 || rt.executeCalled != 1 {
			t.Fatalf("result=%v executeCalled=%d", result, rt.executeCalled)
		}
		if observer.fallbackCalls != 1 {
			t.Fatalf("fallback calls = %d, want 1", observer.fallbackCalls)
		}
		if observer.interactiveFallback != "stub" {
			t.Fatalf("interactive fallback runtime = %q, want %q", observer.interactiveFallback, "stub")
		}
	})

	t.Run("interactive runtime validation error surfaces in result", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("invalid interactive context")
		registry := runtimepkg.NewRegistry()
		rt := &stubInteractiveRuntime{
			stubRuntime: stubRuntime{name: "interactive", validateErr: wantErr},
			supports:    true,
		}
		registry.Register(runtimepkg.RuntimeTypeVirtual, rt)

		executor := &stubInteractiveExecutor{}
		svc := &Service{interactive: executor}
		result, err := svc.executeWithRequestedMode(
			Request{Interactive: true},
			&runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtual, Context: t.Context()},
			registry,
		)
		if err != nil {
			t.Fatalf("executeWithRequestedMode() error = %v", err)
		}
		if !errors.Is(result.Error, wantErr) {
			t.Fatalf("result.Error = %v, want wrapped %v", result.Error, wantErr)
		}
		if executor.called != 1 {
			t.Fatalf("interactive executor called %d times, want 1", executor.called)
		}
	})

	t.Run("interactive runtime prepare error surfaces in result", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("prepare failed")
		registry := runtimepkg.NewRegistry()
		rt := &stubInteractiveRuntime{
			stubRuntime: stubRuntime{name: "interactive"},
			supports:    true,
			prepareErr:  wantErr,
		}
		registry.Register(runtimepkg.RuntimeTypeVirtual, rt)

		executor := &stubInteractiveExecutor{}
		svc := &Service{interactive: executor}
		result, err := svc.executeWithRequestedMode(
			Request{Interactive: true},
			&runtimepkg.ExecutionContext{
				SelectedRuntime: invowkfile.RuntimeVirtual,
				Context:         t.Context(),
				TUI:             runtimepkg.TUIContext{},
			},
			registry,
		)
		if err != nil {
			t.Fatalf("executeWithRequestedMode() error = %v", err)
		}
		if !errors.Is(result.Error, wantErr) {
			t.Fatalf("result.Error = %v, want wrapped %v", result.Error, wantErr)
		}
		if executor.called != 1 {
			t.Fatalf("interactive executor called %d times, want 1", executor.called)
		}
	})
}

func TestNewClassifiedExecutionError(t *testing.T) {
	t.Parallel()

	timedOut := newClassifiedExecutionError(context.DeadlineExceeded)
	if timedOut.Kind != ErrorKindScriptExecutionFailed || timedOut.Message != HintTimedOut {
		t.Fatalf("timedOut = %#v", timedOut)
	}

	cancelled := newClassifiedExecutionError(context.Canceled)
	if cancelled.Kind != ErrorKindScriptExecutionFailed || cancelled.Message != HintCancelled {
		t.Fatalf("cancelled = %#v", cancelled)
	}
}

func commandInfoAndContext(t testing.TB, script string) (*discovery.CommandInfo, *runtimepkg.ExecutionContext, *bytes.Buffer) {
	t.Helper()

	cmd := invowkfiletest.NewTestCommand("build",
		invowkfiletest.WithScript(script),
		invowkfiletest.WithRuntime(invowkfile.RuntimeVirtual),
		invowkfiletest.WithAllPlatforms(),
	)
	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(t.TempDir(), "invowkfile.cue"))}
	ioCtx, execStdout, _ := runtimepkg.CaptureIO()
	return &discovery.CommandInfo{
			Name:       "build",
			Command:    cmd,
			Invowkfile: inv,
			SourceID:   discovery.SourceIDInvowkfile,
		}, &runtimepkg.ExecutionContext{
			Command:         cmd,
			Invowkfile:      inv,
			Context:         t.Context(),
			SelectedRuntime: invowkfile.RuntimeVirtual,
			SelectedImpl:    &cmd.Implementations[0],
			IO:              ioCtx,
			Env:             runtimepkg.DefaultEnv(),
			TUI:             runtimepkg.TUIContext{ServerURL: "http://127.0.0.1:1", ServerToken: runtimepkg.TUIServerToken("token")},
		}, execStdout
}
