// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// RuntimeRegistryResult bundles the runtime registry with its cleanup function,
// non-fatal initialization diagnostics, and any container runtime init error
// for fail-fast dispatch.
type RuntimeRegistryResult struct {
	Registry         *runtime.Registry
	Cleanup          func()
	Diagnostics      []Diagnostic
	ContainerInitErr error
}

// dispatchExecution runs the post-context-build execution pipeline:
//  1. Creates runtime registry.
//  2. Validates timeout string (fail-fast on invalid values).
//  3. Wraps context with timeout.
//  4. Validates dependencies (tools, cmds, filepaths, capabilities, custom checks, env vars).
//  5. Dispatches to interactive mode through an adapter port or standard execution.
//
// It returns ClassifiedError for runtime failures and raw typed errors for
// dependency validation. The CLI adapter handles rendering.
func (s *Service) dispatchExecution(req Request, execCtx *runtime.ExecutionContext, cmdInfo *discovery.CommandInfo, cfg *config.Config, diags []Diagnostic) (Result, []Diagnostic, error) {
	registryResult := s.registryFactory.Create(cfg, s.hostAccess, execCtx.SelectedRuntime)
	diags = appendRuntimeRegistryDiagnostics(diags, req, execCtx, registryResult)
	defer registryResult.Cleanup()

	// Assign a unique execution ID now that the registry is available.
	// NewExecutionContext leaves ExecutionID empty; it is set here because
	// the registry (which owns the monotonic counter) is created at this point.
	execCtx.ExecutionID = registryResult.Registry.NewExecutionID()

	if err := failFastContainerInit(registryResult.ContainerInitErr, execCtx.SelectedRuntime); err != nil {
		return Result{}, diags, err
	}

	cancel, err := applyExecutionTimeout(execCtx)
	if err != nil {
		return Result{}, diags, err
	}
	defer cancel()

	// Dependency validation needs the registry to check runtime-aware dependencies.
	if validateErr := s.validateDeps(cmdInfo, execCtx, registryResult.Registry, req.UserEnv); validateErr != nil {
		// Return the raw error (e.g., *DependencyError); the CLI adapter wraps it.
		return Result{}, diags, validateErr
	}

	if req.Verbose {
		cmdName := invowkfile.CommandName(req.Name) //goplint:ignore -- request name was resolved through discovery
		s.observer.CommandStarting(cmdName)
	}

	result, err := s.executeWithRequestedMode(req, execCtx, registryResult.Registry)
	if err != nil {
		return Result{}, diags, err
	}

	diags = append(diags, BridgeRuntimeDiagnostics(result.Diagnostics)...)
	if result.Error != nil {
		return Result{}, diags, newClassifiedExecutionError(result.Error)
	}

	return Result{ExitCode: result.ExitCode}, diags, nil
}

func appendRuntimeRegistryDiagnostics(diags []Diagnostic, req Request, execCtx *runtime.ExecutionContext, registryResult RuntimeRegistryResult) []Diagnostic {
	if req.Verbose || execCtx.SelectedRuntime == invowkfile.RuntimeContainer {
		diags = append(diags, registryResult.Diagnostics...)
	}
	return diags
}

func failFastContainerInit(containerInitErr error, selectedRuntime invowkfile.RuntimeMode) error {
	if containerInitErr == nil || selectedRuntime != invowkfile.RuntimeContainer {
		return nil
	}
	return newClassifiedExecutionError(containerInitErr)
}

func applyExecutionTimeout(execCtx *runtime.ExecutionContext) (context.CancelFunc, error) {
	noOpCancel := func() { /* no timeout to cancel */ }
	if execCtx.SelectedImpl == nil {
		return noOpCancel, nil
	}

	timeoutDuration, err := execCtx.SelectedImpl.ParseTimeout()
	if err != nil {
		return nil, err
	}
	if timeoutDuration <= 0 {
		return noOpCancel, nil
	}

	ctx, cancel := context.WithTimeout(execCtx.Context, timeoutDuration)
	execCtx.Context = ctx
	return cancel, nil
}

func (s *Service) executeWithRequestedMode(req Request, execCtx *runtime.ExecutionContext, registry *runtime.Registry) (*runtime.Result, error) {
	if !req.Interactive {
		return registry.Execute(execCtx), nil
	}

	rt, err := registry.GetForContext(execCtx)
	if err != nil {
		return nil, newClassifiedExecutionError(fmt.Errorf("failed to get runtime: %w", err))
	}

	interactiveRT := runtime.GetInteractiveRuntime(rt)
	if interactiveRT != nil {
		cmdName := invowkfile.CommandName(req.Name) //goplint:ignore -- request name was resolved through discovery
		return s.interactive.Execute(execCtx, cmdName, interactiveRT), nil
	}

	if req.Verbose {
		runtimeName := invowkfile.RuntimeMode(rt.Name()) //goplint:ignore -- runtime names are registered from runtime mode constants
		s.observer.InteractiveFallback(runtimeName)
	}
	return registry.Execute(execCtx), nil
}

func newClassifiedExecutionError(err error) *ClassifiedError {
	kind, plainMsg := classifyExecutionError(err)
	return &ClassifiedError{
		Err:     err,
		Kind:    kind,
		Message: plainMsg,
	}
}

// BridgeRuntimeDiagnostics converts runtime-layer initialization diagnostics
// into command-service diagnostics.
func BridgeRuntimeDiagnostics(diags []runtime.InitDiagnostic) []Diagnostic {
	result := make([]Diagnostic, 0, len(diags))
	for _, diag := range diags {
		d, err := NewDiagnosticWithCause(
			DiagnosticSeverityWarning,
			DiagnosticCode(diag.Code),
			diag.Message,
			"",
			diag.Cause,
		)
		if err != nil {
			slog.Error("BUG: failed to bridge runtime diagnostic to command diagnostic",
				"code", diag.Code, "error", err)
			continue
		}
		result = append(result, d)
	}

	return result
}
