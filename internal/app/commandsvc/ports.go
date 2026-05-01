// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"

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
		Create(*config.Config, HostAccess, invowkfile.RuntimeMode) RuntimeRegistryResult
	}

	// InteractiveExecutor owns terminal UI execution for runtimes that support
	// interactive mode.
	InteractiveExecutor interface {
		Execute(*runtime.ExecutionContext, invowkfile.CommandName, runtime.InteractiveRuntime) *runtime.Result
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

func (missingRuntimeRegistryFactory) Create(*config.Config, HostAccess, invowkfile.RuntimeMode) RuntimeRegistryResult {
	return RuntimeRegistryResult{
		Registry: runtime.NewRegistry(),
		Cleanup:  func() {},
	}
}

func (defaultInteractiveExecutor) Execute(execCtx *runtime.ExecutionContext, _ invowkfile.CommandName, interactiveRT runtime.InteractiveRuntime) *runtime.Result {
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
