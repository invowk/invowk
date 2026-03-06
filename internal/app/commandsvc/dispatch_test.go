// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
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

func (s *stubInteractiveRuntime) PrepareInteractive(*runtimepkg.ExecutionContext) (*runtimepkg.PreparedCommand, error) {
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	if s.prepared != nil {
		return s.prepared, nil
	}
	return &runtimepkg.PreparedCommand{Cmd: exec.CommandContext(context.Background(), "sh", "-c", "exit 0")}, nil
}

func TestDispatchExecution_Success(t *testing.T) {
	t.Parallel()

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	svc := &Service{stdout: stdout, stderr: stderr, ssh: &sshServerController{}}
	cmdInfo, execCtx, execStdout := commandInfoAndContext("echo hello")

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

	svc := &Service{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, ssh: &sshServerController{}}
	cmdInfo, execCtx, _ := commandInfoAndContext("echo hello")
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

		svc := &Service{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
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

		var stdout bytes.Buffer
		svc := &Service{stdout: &stdout, stderr: &bytes.Buffer{}}
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
		if !strings.Contains(stdout.String(), "does not support interactive mode") {
			t.Fatalf("stdout = %q", stdout.String())
		}
	})

	t.Run("interactive runtime validation error surfaces in result", func(t *testing.T) {
		t.Parallel()

		registry := runtimepkg.NewRegistry()
		rt := &stubInteractiveRuntime{
			stubRuntime: stubRuntime{name: "interactive", validateErr: errors.New("invalid interactive context")},
			supports:    true,
		}
		registry.Register(runtimepkg.RuntimeTypeVirtual, rt)

		svc := &Service{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
		result, err := svc.executeWithRequestedMode(
			Request{Interactive: true},
			&runtimepkg.ExecutionContext{SelectedRuntime: invowkfile.RuntimeVirtual, Context: t.Context()},
			registry,
		)
		if err != nil {
			t.Fatalf("executeWithRequestedMode() error = %v", err)
		}
		if result.Error == nil || !strings.Contains(result.Error.Error(), "invalid interactive context") {
			t.Fatalf("result.Error = %v", result.Error)
		}
	})

	t.Run("interactive runtime prepare error surfaces in result", func(t *testing.T) {
		t.Parallel()

		registry := runtimepkg.NewRegistry()
		rt := &stubInteractiveRuntime{
			stubRuntime: stubRuntime{name: "interactive"},
			supports:    true,
			prepareErr:  errors.New("prepare failed"),
		}
		registry.Register(runtimepkg.RuntimeTypeVirtual, rt)

		svc := &Service{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
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
		if result.Error == nil || !strings.Contains(result.Error.Error(), "failed to prepare command") {
			t.Fatalf("result.Error = %v", result.Error)
		}
	})
}

func TestNewClassifiedExecutionError(t *testing.T) {
	t.Parallel()

	timedOut := newClassifiedExecutionError(context.DeadlineExceeded)
	if timedOut.IssueID != issue.ScriptExecutionFailedId || timedOut.Message != HintTimedOut {
		t.Fatalf("timedOut = %#v", timedOut)
	}

	cancelled := newClassifiedExecutionError(context.Canceled)
	if cancelled.IssueID != issue.ScriptExecutionFailedId || cancelled.Message != HintCancelled {
		t.Fatalf("cancelled = %#v", cancelled)
	}
}

func commandInfoAndContext(script string) (*discovery.CommandInfo, *runtimepkg.ExecutionContext, *bytes.Buffer) {
	cmd := &invowkfile.Command{
		Name: "build",
		Implementations: []invowkfile.Implementation{{
			Script:    invowkfile.ScriptContent(script),
			Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
			Platforms: invowkfile.AllPlatformConfigs(),
		}},
	}
	inv := &invowkfile.Invowkfile{FilePath: "/tmp/invowkfile.cue"}
	ioCtx, execStdout, _ := runtimepkg.CaptureIO()
	return &discovery.CommandInfo{
			Name:       "build",
			Command:    cmd,
			Invowkfile: inv,
			SourceID:   discovery.SourceIDInvowkfile,
		}, &runtimepkg.ExecutionContext{
			Command:         cmd,
			Invowkfile:      inv,
			Context:         context.Background(),
			SelectedRuntime: invowkfile.RuntimeVirtual,
			SelectedImpl:    &cmd.Implementations[0],
			IO:              ioCtx,
			Env:             runtimepkg.DefaultEnv(),
			TUI:             runtimepkg.TUIContext{ServerURL: "http://127.0.0.1:1", ServerToken: runtimepkg.TUIServerToken("token")},
		}, execStdout
}
