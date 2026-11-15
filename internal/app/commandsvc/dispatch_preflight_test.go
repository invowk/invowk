// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/containerplan"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// preflightInteractiveExecutor is an interactive executor whose
	// preflight returns err, like a container run without a narrow TUI route.
	preflightInteractiveExecutor struct {
		stubInteractiveExecutor
		err            error
		preflightCalls int
	}
)

func (p *preflightInteractiveExecutor) PreflightInteractive(*runtimepkg.ExecutionContext, RuntimeInteractiveCommand) error {
	p.preflightCalls++
	return p.err
}

func newPreflightService(t *testing.T, executor *preflightInteractiveExecutor) (*Service, *stubInteractiveRuntime) {
	t.Helper()
	rt := &stubInteractiveRuntime{name: string(invowkfile.RuntimeVirtualSh), supports: true}
	registry := runtimepkg.NewRegistry()
	registry.Register(runtimepkg.RuntimeTypeVirtualSh, rt)
	return &Service{
		hostAccess:      noopHostAccess{},
		registryFactory: staticRuntimeRegistryFactory{registry: registry},
		interactive:     executor,
	}, rt
}

func refusedPlanError() error {
	return &containerplan.TUIExposureRefusedError{Row: containerplan.TUIRowPodmanPersistent, Message: "interactive container run refused: test"}
}

// TestDispatchExecution_ExplicitInteractiveRefusedBeforeDependencies pins that
// an explicit -i in a refused mode fails before dependency validation, which
// may start containers for container-scoped depends_on checks.
func TestDispatchExecution_ExplicitInteractiveRefusedBeforeDependencies(t *testing.T) {
	t.Parallel()

	executor := &preflightInteractiveExecutor{err: refusedPlanError()}
	svc, rt := newPreflightService(t, executor)
	cmdInfo, execCtx, _ := commandInfoAndContext(t, "echo hello")
	cmdInfo.Invowkfile.DependsOn = &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{{Alternatives: []invowkfile.EnvVarCheck{{Name: "MISSING"}}}},
	}

	_, _, err := svc.dispatchExecution(
		Request{Name: "build", Interactive: true, InteractiveSet: true, UserEnv: map[string]string{}},
		execCtx, cmdInfo, config.DefaultConfig(), nil,
	)
	classified, ok := errors.AsType[*ClassifiedError](err)
	if !ok || classified.Kind != ErrorKindTUIExposureRefused {
		t.Fatalf("dispatchExecution() error = %v, want a ClassifiedError of kind %s (before the dependency error)", err, ErrorKindTUIExposureRefused)
	}
	if _, isDep := errors.AsType[*deps.DependencyError](err); isDep {
		t.Fatal("dependency validation ran before the interactive preflight")
	}
	if executor.called != 0 || rt.executeCalled != 0 {
		t.Fatalf("executor=%d runtime=%d, want neither to run", executor.called, rt.executeCalled)
	}
}

// TestDispatchExecution_ConfigDefaultInteractiveFallsBack pins that a refused
// mode does not fail a run whose interactive mode only comes from the
// ui.interactive config default: it runs non-interactively with one warning.
func TestDispatchExecution_ConfigDefaultInteractiveFallsBack(t *testing.T) {
	t.Parallel()

	executor := &preflightInteractiveExecutor{err: refusedPlanError()}
	svc, rt := newPreflightService(t, executor)
	cmdInfo, execCtx, _ := commandInfoAndContext(t, "echo hello")
	var stderr bytes.Buffer
	execCtx.IO.Stderr = &stderr

	result, _, err := svc.dispatchExecution(
		Request{Name: "build", Interactive: true, UserEnv: map[string]string{}},
		execCtx, cmdInfo, config.DefaultConfig(), nil,
	)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("dispatchExecution() = %+v, %v; want a successful non-interactive run", result, err)
	}
	if executor.called != 0 || rt.executeCalled != 1 {
		t.Fatalf("executor=%d runtime=%d, want only the non-interactive runtime run", executor.called, rt.executeCalled)
	}
	if got := strings.Count(stderr.String(), "running without interactive mode"); got != 1 {
		t.Fatalf("stderr = %q, want the fallback warning once", stderr.String())
	}
	if !strings.Contains(stderr.String(), "--ivk-interactive=false") {
		t.Fatalf("warning %q does not name --ivk-interactive=false", stderr.String())
	}
}

// TestDispatchExecution_ConfigDefaultInteractiveInvalidPlanFails pins that
// only a refusal falls back: an invalid plan fails even without an explicit -i.
func TestDispatchExecution_ConfigDefaultInteractiveInvalidPlanFails(t *testing.T) {
	t.Parallel()

	executor := &preflightInteractiveExecutor{err: containerplan.ErrInvalidTUIExposurePlan}
	svc, rt := newPreflightService(t, executor)
	cmdInfo, execCtx, _ := commandInfoAndContext(t, "echo hello")

	_, _, err := svc.dispatchExecution(
		Request{Name: "build", Interactive: true, UserEnv: map[string]string{}},
		execCtx, cmdInfo, config.DefaultConfig(), nil,
	)
	if !errors.Is(err, containerplan.ErrInvalidTUIExposurePlan) {
		t.Fatalf("dispatchExecution() error = %v, want ErrInvalidTUIExposurePlan", err)
	}
	if rt.executeCalled != 0 {
		t.Fatal("an invalid plan ran the command")
	}
}

// TestDispatchExecution_PreflightPassesThrough pins that a passing preflight
// keeps the interactive path, and that non-interactive runs skip it.
func TestDispatchExecution_PreflightPassesThrough(t *testing.T) {
	t.Parallel()

	executor := &preflightInteractiveExecutor{}
	svc, _ := newPreflightService(t, executor)
	cmdInfo, execCtx, _ := commandInfoAndContext(t, "echo hello")
	if _, _, err := svc.dispatchExecution(
		Request{Name: "build", Interactive: true, InteractiveSet: true, UserEnv: map[string]string{}},
		execCtx, cmdInfo, config.DefaultConfig(), nil,
	); err != nil {
		t.Fatalf("dispatchExecution(interactive) error = %v", err)
	}
	if executor.preflightCalls != 1 || executor.called != 1 {
		t.Fatalf("preflight=%d executor=%d, want 1 and 1", executor.preflightCalls, executor.called)
	}

	cmdInfo, execCtx, _ = commandInfoAndContext(t, "echo hello")
	if _, _, err := svc.dispatchExecution(
		Request{Name: "build", UserEnv: map[string]string{}},
		execCtx, cmdInfo, config.DefaultConfig(), nil,
	); err != nil {
		t.Fatalf("dispatchExecution(non-interactive) error = %v", err)
	}
	if executor.preflightCalls != 1 {
		t.Fatalf("non-interactive run called the preflight (calls = %d)", executor.preflightCalls)
	}
}
