// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"

	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// HostAccess manages optional host callback infrastructure for runtimes.
	// Implementations may use SSH, HTTP, or another transport; Service only
	// relies on lifecycle semantics.
	HostAccess interface {
		Ensure(context.Context) error
		Running() bool
		Stop()
	}

	// RuntimeRegistryCreator builds a runtime registry for one execution. It
	// receives HostAccess so an adapter can forward concrete transport handles
	// to runtimes without exposing those details to Service.
	RuntimeRegistryCreator interface {
		Create(*config.Config, HostAccess, invowkfile.RuntimeMode) RuntimeSession
	}

	// RuntimeSession owns runtime adapter state for one command execution.
	RuntimeSession interface {
		NewExecutionID() runtime.ExecutionID
		Diagnostics() []Diagnostic
		ContainerInitErr() error
		DependencyProbe(*runtime.ExecutionContext) deps.RuntimeDependencyProbe
		RuntimeForContext(*runtime.ExecutionContext) (runtime.Runtime, error)
		Execute(*runtime.ExecutionContext) *runtime.Result
		Close()
	}

	// RuntimeInteractiveCommand is the minimal runtime-side conversation needed
	// by an interactive execution adapter.
	RuntimeInteractiveCommand interface {
		Validate(*runtime.ExecutionContext) error
		PrepareInteractive(*runtime.ExecutionContext) (*runtime.PreparedCommand, error)
	}

	// InteractiveExecutor owns terminal UI execution for runtimes that support
	// interactive mode.
	InteractiveExecutor interface {
		Execute(*runtime.ExecutionContext, invowkfile.CommandName, RuntimeInteractiveCommand) *runtime.Result
	}

	// RequestScopeFunc attaches per-request service state such as discovery
	// caches. Service calls this at entry points so callers are not required
	// to know adapter-specific context conventions.
	RequestScopeFunc func(context.Context, types.FilesystemPath) context.Context

	// ExecutionObserver receives user-visible execution events. Adapters render
	// these events; Service only describes what happened.
	ExecutionObserver interface {
		CommandStarting(invowkfile.CommandName)
		InteractiveFallback(invowkfile.RuntimeMode)
	}

	noopHostAccess struct{}

	noopExecutionObserver struct{}

	missingRuntimeRegistryFactory struct{}

	emptyRuntimeSession struct {
		registry *runtime.Registry
	}

	defaultInteractiveExecutor struct{}
)

func (noopHostAccess) Ensure(context.Context) error { return nil }

func (noopHostAccess) Running() bool { return false }

func (noopHostAccess) Stop() {
	// No host-access infrastructure was started.
}

func (noopExecutionObserver) CommandStarting(invowkfile.CommandName) {
	// Command lifecycle events are optional for service-only callers.
}

func (noopExecutionObserver) InteractiveFallback(invowkfile.RuntimeMode) {
	// Interactive fallback events are optional for service-only callers.
}

func (missingRuntimeRegistryFactory) Create(*config.Config, HostAccess, invowkfile.RuntimeMode) RuntimeSession {
	return &emptyRuntimeSession{registry: runtime.NewRegistry()}
}

func (s *emptyRuntimeSession) NewExecutionID() runtime.ExecutionID {
	return s.registry.NewExecutionID()
}

func (*emptyRuntimeSession) Diagnostics() []Diagnostic { return nil }

func (*emptyRuntimeSession) ContainerInitErr() error { return nil }

func (*emptyRuntimeSession) DependencyProbe(*runtime.ExecutionContext) deps.RuntimeDependencyProbe {
	return nil
}

func (s *emptyRuntimeSession) RuntimeForContext(execCtx *runtime.ExecutionContext) (runtime.Runtime, error) {
	return s.registry.GetForContext(execCtx)
}

func (s *emptyRuntimeSession) Execute(execCtx *runtime.ExecutionContext) *runtime.Result {
	return s.registry.Execute(execCtx)
}

func (*emptyRuntimeSession) Close() {
	// Missing registry adapters have no infrastructure to clean up.
}

func (defaultInteractiveExecutor) Execute(execCtx *runtime.ExecutionContext, _ invowkfile.CommandName, interactiveRT RuntimeInteractiveCommand) *runtime.Result {
	if err := interactiveRT.Validate(execCtx); err != nil {
		return &runtime.Result{ExitCode: 1, Error: err}
	}
	return &runtime.Result{ExitCode: 1, Error: ErrInteractiveExecutorNotConfigured}
}

func beginNoopRequestScope(ctx context.Context, _ types.FilesystemPath) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
