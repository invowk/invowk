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
	"github.com/invowk/invowk/pkg/types"
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

	staticRuntimeRegistryFactory struct {
		registry *runtimepkg.Registry
	}

	testRuntimeSession struct {
		registry         *runtimepkg.Registry
		diagnostics      []Diagnostic
		containerInitErr error
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

func (s *stubInteractiveExecutor) Execute(execCtx *runtimepkg.ExecutionContext, _ invowkfile.CommandName, interactiveRT RuntimeInteractiveCommand) *runtimepkg.Result {
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

func (f staticRuntimeRegistryFactory) Create(*config.Config, HostAccess, invowkfile.RuntimeMode) RuntimeSession {
	registry := f.registry
	if registry == nil {
		registry = runtimepkg.NewRegistry()
		registry.Register(runtimepkg.RuntimeTypeNative, runtimepkg.NewNativeRuntime())
		registry.Register(runtimepkg.RuntimeTypeVirtualSh, runtimepkg.NewShRuntime(true))
		registry.Register(runtimepkg.RuntimeTypeContainer, &stubRuntime{name: string(invowkfile.RuntimeContainer)})
	}
	return &testRuntimeSession{registry: registry}
}

func (s *testRuntimeSession) NewExecutionID() runtimepkg.ExecutionID {
	return s.registry.NewExecutionID()
}

func (s *testRuntimeSession) Diagnostics() []Diagnostic {
	return s.diagnostics
}

func (s *testRuntimeSession) ContainerInitErr() error {
	return s.containerInitErr
}

func (*testRuntimeSession) DependencyProbe(*runtimepkg.ExecutionContext) deps.RuntimeDependencyProbe {
	return nil
}

func (s *testRuntimeSession) RuntimeForContext(execCtx *runtimepkg.ExecutionContext) (runtimepkg.Runtime, error) {
	return s.registry.GetForContext(execCtx)
}

func (s *testRuntimeSession) Execute(execCtx *runtimepkg.ExecutionContext) *runtimepkg.Result {
	return s.registry.Execute(execCtx)
}

func (*testRuntimeSession) Close() {}

func TestDispatchExecution_Success(t *testing.T) {
	t.Parallel()

	svc := &Service{
		hostAccess:      noopHostAccess{},
		registryFactory: staticRuntimeRegistryFactory{},
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
		registryFactory: staticRuntimeRegistryFactory{},
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

func TestAppendRuntimeSessionDiagnostics(t *testing.T) {
	t.Parallel()

	diag, err := NewDiagnosticWithCause(DiagnosticSeverityWarning, DiagnosticCodeConfigLoadFailed, "warn", "", nil)
	if err != nil {
		t.Fatalf("NewDiagnostic(): %v", err)
	}

	base := []Diagnostic{}
	result := appendRuntimeSessionDiagnostics(base, Request{Verbose: true}, &runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtualSh}, &testRuntimeSession{diagnostics: []Diagnostic{diag}})
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	result = appendRuntimeSessionDiagnostics(base, Request{}, &runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeContainer}, &testRuntimeSession{diagnostics: []Diagnostic{diag}})
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}

	result = appendRuntimeSessionDiagnostics(base, Request{}, &runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtualSh}, &testRuntimeSession{diagnostics: []Diagnostic{diag}})
	if len(result) != 0 {
		t.Fatalf("len(result) = %d, want 0", len(result))
	}
}

func TestFailFastContainerInit(t *testing.T) {
	t.Parallel()

	if err := failFastContainerInit(nil, invowkfile.RuntimeContainer); err != nil {
		t.Fatalf("failFastContainerInit(nil) = %v", err)
	}
	if err := failFastContainerInit(errors.New("boom"), invowkfile.RuntimeVirtualSh); err != nil {
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
		SelectedImpl: &invowkfile.Implementation{Timeout: "5s"},
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

	tests := []struct {
		name          string
		interactive   bool
		verbose       bool
		interactiveRT bool
		exitCode      types.ExitCode
		validateErr   error
		prepareErr    error
		wantFallback  string
		wantExecCalls int
		wantTUICalls  int
	}{
		{name: "non-interactive uses registry execute", exitCode: 7, wantExecCalls: 1},
		{name: "interactive fallback logs and executes normally", interactive: true, verbose: true, exitCode: 3, wantFallback: "stub", wantExecCalls: 1},
		{name: "interactive runtime validation error surfaces in result", interactive: true, interactiveRT: true, validateErr: errors.New("invalid interactive context"), wantTUICalls: 1},
		{name: "interactive runtime prepare error surfaces in result", interactive: true, interactiveRT: true, prepareErr: errors.New("prepare failed"), wantTUICalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			registry := runtimepkg.NewRegistry()
			base := &stubRuntime{name: "stub", executeResult: &runtimepkg.Result{ExitCode: tt.exitCode}}
			var runtime runtimepkg.Runtime = base
			if tt.interactiveRT {
				runtime = &stubInteractiveRuntime{
					stubRuntime: stubRuntime{name: "interactive", validateErr: tt.validateErr},
					supports:    true,
					prepareErr:  tt.prepareErr,
				}
			}
			registry.Register(runtimepkg.RuntimeTypeVirtualSh, runtime)
			executor := &stubInteractiveExecutor{}
			svc := &Service{interactive: executor}
			result, fallback, err := svc.executeWithRequestedMode(
				Request{Interactive: tt.interactive, Verbose: tt.verbose},
				&runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtualSh, Context: t.Context()},
				&testRuntimeSession{registry: registry},
			)
			if err != nil {
				t.Fatalf("executeWithRequestedMode() error = %v", err)
			}
			wantResultErr := tt.validateErr
			if wantResultErr == nil {
				wantResultErr = tt.prepareErr
			}
			if wantResultErr != nil && !errors.Is(result.Error, wantResultErr) {
				t.Fatalf("result.Error = %v, want wrapped %v", result.Error, wantResultErr)
			}
			if wantResultErr == nil && result.ExitCode != tt.exitCode {
				t.Fatalf("ExitCode = %v, want %v", result.ExitCode, tt.exitCode)
			}
			if string(fallback) != tt.wantFallback {
				t.Fatalf("interactive fallback runtime = %q, want %q", fallback, tt.wantFallback)
			}
			if base.executeCalled != tt.wantExecCalls {
				t.Fatalf("execute called %d times, want %d", base.executeCalled, tt.wantExecCalls)
			}
			if executor.called != tt.wantTUICalls {
				t.Fatalf("interactive executor called %d times, want %d", executor.called, tt.wantTUICalls)
			}
		})
	}
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
		invowkfiletest.WithRuntime(invowkfile.RuntimeVirtualSh),
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
			SelectedRuntime: invowkfile.RuntimeVirtualSh,
			SelectedImpl:    &cmd.Implementations[0],
			IO:              ioCtx,
			Env:             runtimepkg.DefaultEnv(),
			TUI:             runtimepkg.TUIContext{ServerURL: "http://127.0.0.1:1", ServerToken: runtimepkg.TUIServerToken("token")},
		}, execStdout
}
