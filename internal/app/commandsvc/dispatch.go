// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"log/slog"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// dispatchExecution runs the post-context-build execution pipeline:
//  1. Creates a runtime session.
//  2. Validates timeout string (fail-fast on invalid values).
//  3. Wraps context with timeout.
//  4. Validates dependencies (tools, cmds, filepaths, capabilities, custom checks, env vars).
//  5. Dispatches to interactive mode through an adapter port or standard execution.
//
// It returns ClassifiedError for runtime failures and raw typed errors for
// dependency validation. The CLI adapter handles rendering.
func (s *Service) dispatchExecution(req Request, execCtx *runtime.ExecutionContext, cmdInfo *discovery.CommandInfo, cfg *config.Config, diags []Diagnostic) (Result, []Diagnostic, error) {
	session := s.registryFactory.Create(cfg, s.hostAccess, execCtx.SelectedRuntime)
	diags = appendRuntimeSessionDiagnostics(diags, req, execCtx, session)
	defer session.Close()

	// Assign a unique execution ID now that the registry is available.
	// NewExecutionContext leaves ExecutionID empty; it is set here because
	// the registry (which owns the monotonic counter) is created at this point.
	execCtx.ExecutionID = session.NewExecutionID()

	if err := failFastContainerInit(session.ContainerInitErr(), execCtx.SelectedRuntime); err != nil {
		return Result{}, diags, err
	}

	cancel, err := applyExecutionTimeout(execCtx)
	if err != nil {
		return Result{}, diags, err
	}
	defer cancel()

	// Dependency validation uses the runtime session to check runtime-aware dependencies.
	if validateErr := s.validateDeps(cmdInfo, execCtx, session, req.UserEnv); validateErr != nil {
		// Return the raw error (e.g., *DependencyError); the CLI adapter wraps it.
		return Result{}, diags, validateErr
	}

	if req.Verbose {
		cmdName := invowkfile.CommandName(req.Name) //goplint:ignore -- request name was resolved through discovery
		s.observer.CommandStarting(cmdName)
	}

	result, interactiveFallback, err := s.executeWithRequestedMode(req, execCtx, session)
	if err != nil {
		return Result{}, diags, err
	}
	if req.Verbose && interactiveFallback != "" {
		s.observer.InteractiveFallback(interactiveFallback)
	}

	diags = append(diags, BridgeRuntimeDiagnostics(result.Diagnostics)...)
	if result.Error != nil {
		return Result{}, diags, newClassifiedExecutionError(result.Error)
	}

	return Result{ExitCode: result.ExitCode}, diags, nil
}

func appendRuntimeSessionDiagnostics(diags []Diagnostic, req Request, execCtx *runtime.ExecutionContext, session RuntimeSession) []Diagnostic {
	if req.Verbose || execCtx.SelectedRuntime == invowkfile.RuntimeContainer {
		diags = append(diags, session.Diagnostics()...)
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

func (s *Service) executeWithRequestedMode(req Request, execCtx *runtime.ExecutionContext, session RuntimeSession) (*runtime.Result, invowkfile.RuntimeMode, error) {
	cmdName := invowkfile.CommandName(req.Name) //goplint:ignore -- request name was resolved through discovery
	result, interactiveFallback, err := session.Execute(execCtx, cmdName, req.Interactive, s.interactive)
	if err != nil {
		return nil, interactiveFallback, newClassifiedExecutionError(err)
	}
	return result, interactiveFallback, nil
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
