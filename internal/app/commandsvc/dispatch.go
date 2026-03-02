// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/sshserver"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
)

// RuntimeRegistryResult bundles the runtime registry with its cleanup function,
// non-fatal initialization diagnostics, and any container runtime init error
// for fail-fast dispatch.
type RuntimeRegistryResult struct {
	Registry         *runtime.Registry
	Cleanup          func()
	Diagnostics      []discovery.Diagnostic
	ContainerInitErr error
}

// dispatchExecution runs the post-context-build execution pipeline:
//  1. Creates runtime registry.
//  2. Validates timeout string (fail-fast on invalid values).
//  3. Wraps context with timeout.
//  4. Validates dependencies (tools, cmds, filepaths, capabilities, custom checks, env vars).
//  5. Dispatches to interactive mode (alternate screen + TUI server) or standard execution.
//
// It returns ClassifiedError for runtime failures and raw typed errors for
// dependency validation. The CLI adapter handles rendering.
func (s *Service) dispatchExecution(ctx context.Context, req Request, execCtx *runtime.ExecutionContext, cmdInfo *discovery.CommandInfo, cfg *config.Config, diags []discovery.Diagnostic) (Result, []discovery.Diagnostic, error) {
	registryResult := CreateRuntimeRegistry(cfg, s.ssh.current())
	if req.Verbose || execCtx.SelectedRuntime == invowkfile.RuntimeContainer {
		diags = append(diags, registryResult.Diagnostics...)
	}
	defer registryResult.Cleanup()

	// Assign a unique execution ID now that the registry is available.
	// NewExecutionContext leaves ExecutionID empty; it is set here because
	// the registry (which owns the monotonic counter) is created at this point.
	execCtx.ExecutionID = registryResult.Registry.NewExecutionID()

	if registryResult.ContainerInitErr != nil && execCtx.SelectedRuntime == invowkfile.RuntimeContainer {
		issueID, plainMsg := classifyExecutionError(registryResult.ContainerInitErr, req.Verbose)
		return Result{}, diags, &ClassifiedError{
			Err:     registryResult.ContainerInitErr,
			IssueID: issueID,
			Message: plainMsg,
		}
	}

	// Validate timeout early so an invalid timeout (e.g., "not-a-duration")
	// fails fast before dependency validation or execution.
	var timeoutDuration time.Duration
	if execCtx.SelectedImpl != nil {
		var parseErr error
		timeoutDuration, parseErr = execCtx.SelectedImpl.ParseTimeout()
		if parseErr != nil {
			return Result{}, diags, parseErr
		}
	}

	// Apply execution timeout. Duration was already validated above
	// (fail-fast on invalid strings). Cancel is deferred to release timer resources.
	if timeoutDuration > 0 {
		var cancel context.CancelFunc
		execCtx.Context, cancel = context.WithTimeout(execCtx.Context, timeoutDuration)
		defer cancel()
	}

	// Dependency validation needs the registry to check runtime-aware dependencies.
	if err := s.validateDeps(cmdInfo, execCtx, registryResult.Registry, req.UserEnv); err != nil {
		// Return the raw error (e.g., *DependencyError); the CLI adapter wraps it.
		return Result{}, diags, err
	}

	if req.Verbose {
		fmt.Fprintf(s.stdout, "-> Running '%s'...\n", req.Name)
	}

	var result *runtime.Result
	if req.Interactive {
		// Interactive mode is opportunistic and falls back to standard execution
		// when the selected runtime does not implement interactive support.
		rt, err := registryResult.Registry.GetForContext(execCtx)
		if err != nil {
			err = fmt.Errorf("failed to get runtime: %w", err)
			issueID, plainMsg := classifyExecutionError(err, req.Verbose)
			return Result{}, diags, &ClassifiedError{
				Err:     err,
				IssueID: issueID,
				Message: plainMsg,
			}
		}

		interactiveRT := runtime.GetInteractiveRuntime(rt)
		if interactiveRT != nil {
			result = executeInteractive(execCtx, registryResult.Registry, req.Name, interactiveRT)
		} else {
			if req.Verbose {
				fmt.Fprintf(s.stdout, "! Runtime '%s' does not support interactive mode, using standard execution\n",
					rt.Name())
			}
			result = registryResult.Registry.Execute(execCtx)
		}
	} else {
		result = registryResult.Registry.Execute(execCtx)
	}

	if result.Error != nil {
		issueID, plainMsg := classifyExecutionError(result.Error, req.Verbose)
		return Result{}, diags, &ClassifiedError{
			Err:     result.Error,
			IssueID: issueID,
			Message: plainMsg,
		}
	}

	return Result{ExitCode: result.ExitCode}, diags, nil
}

// CreateRuntimeRegistry creates and populates the runtime registry.
// Native and virtual runtimes are always registered because they execute in-process.
// The container runtime is conditionally registered based on engine availability
// (Docker or Podman). When an SSH server is active for host access, it is forwarded
// to the container runtime so containers can reach back into the host.
//
// INVARIANT: This function creates exactly one ContainerRuntime instance per call.
// The ContainerRuntime.runMu mutex provides intra-process serialization as a fallback
// when flock-based cross-process locking is unavailable (non-Linux platforms).
// Creating multiple ContainerRuntime instances would give each its own mutex,
// defeating the serialization and reintroducing the ping_group_range race.
// See TestCreateRuntimeRegistry_SingleContainerInstance for the enforcement test.
//
// The returned result includes the runtime registry, cleanup function, and
// non-fatal diagnostics produced during runtime initialization.
func CreateRuntimeRegistry(cfg *config.Config, sshServer *sshserver.Server) RuntimeRegistryResult {
	built := runtime.BuildRegistry(runtime.BuildRegistryOptions{
		Config:    cfg,
		SSHServer: sshServer,
	})

	result := RuntimeRegistryResult{
		Registry:         built.Registry,
		Cleanup:          built.Cleanup,
		ContainerInitErr: built.ContainerInitErr,
	}

	for _, diag := range built.Diagnostics {
		d, err := discovery.NewDiagnosticWithCause(
			discovery.SeverityWarning,
			discovery.DiagnosticCode(diag.Code),
			diag.Message,
			"",
			diag.Cause,
		)
		if err != nil {
			slog.Error("BUG: failed to bridge runtime diagnostic to discovery diagnostic",
				"code", diag.Code, "error", err)
			continue
		}
		result.Diagnostics = append(result.Diagnostics, d)
	}

	return result
}

// BridgeTUIRequests bridges TUI component requests from the HTTP-based TUI server
// to the Bubble Tea event loop. It runs as a goroutine that reads from the server's
// request channel until closed, converting each HTTP request into a tea.Msg for
// the interactive model to handle.
func BridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		program.Send(tui.TUIComponentMsg{
			Component:  tui.ComponentType(req.Component),
			Options:    req.Options,
			ResponseCh: req.ResponseCh,
		})
	}
}

// executeInteractive runs a command in interactive mode using Bubble Tea's alternate
// screen buffer. It starts an HTTP-based TUI server for bidirectional component requests
// between the running command and the terminal UI. For container runtimes, the TUI server
// URL is rewritten to use the host-reachable address so containers can call back.
func executeInteractive(ctx *runtime.ExecutionContext, registry *runtime.Registry, cmdName string, interactiveRT runtime.InteractiveRuntime) *runtime.Result {
	if err := interactiveRT.Validate(ctx); err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: err} //goplint:ignore -- literal exit code for validation failure
	}

	tuiServer, err := tuiserver.New()
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("failed to create TUI server: %w", err)} //goplint:ignore -- literal exit code for server creation failure
	}

	// Resolve the Go context once for reuse by both the TUI server and
	// the interactive command. Parent cancellation (e.g., Ctrl+C) propagates
	// to server goroutines and the subprocess. The nil guard is defensive —
	// BuildExecutionContext guarantees non-nil, but executeInteractive is a
	// package-level function that could be called from other paths.
	goCtx := ctx.Context
	if goCtx == nil {
		goCtx = context.Background()
	}

	if err = tuiServer.Start(goCtx); err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("failed to start TUI server: %w", err)} //goplint:ignore -- literal exit code for server start failure
	}
	defer func() {
		if stopErr := tuiServer.Stop(); stopErr != nil {
			slog.Debug("TUI server stop failed during cleanup", "error", stopErr)
		}
	}()

	// Rewrite the TUI server URL for container runtimes. The TUI server
	// listens on the host's localhost, but a container's network namespace
	// isolates it from the host — "localhost" inside the container refers to
	// the container itself, not the host. We replace localhost with the
	// engine-specific host-reachable address (e.g., "host.docker.internal"
	// for Docker, or the host gateway IP for Podman) so the containerized
	// command can call back to the TUI server over the bridge network.
	var tuiServerURL string
	if containerRT, ok := interactiveRT.(*runtime.ContainerRuntime); ok {
		hostAddr := containerRT.GetHostAddressForContainer()
		tuiServerURL = tuiServer.URLWithHost(hostAddr)
	} else {
		tuiServerURL = tuiServer.URL()
	}

	ctx.TUI.ServerURL = runtime.TUIServerURL(tuiServerURL)                  //goplint:ignore -- from tuiserver.URL(), validated internally
	ctx.TUI.ServerToken = runtime.TUIServerToken(string(tuiServer.Token())) //goplint:ignore -- from tuiserver.Token(), validated internally

	prepared, err := interactiveRT.PrepareInteractive(ctx)
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("failed to prepare command: %w", err)} //goplint:ignore -- literal exit code for prepare failure
	}

	if prepared.Cleanup != nil {
		defer prepared.Cleanup()
	}

	prepared.Cmd.Env = append(prepared.Cmd.Env,
		// Native/virtual runtimes read TUI server coordinates from process env.
		fmt.Sprintf("%s=%s", tuiserver.EnvTUIAddr, tuiServerURL),
		fmt.Sprintf("%s=%s", tuiserver.EnvTUIToken, tuiServer.Token()),
	)

	interactiveResult, err := tui.RunInteractiveCmd(
		goCtx,
		tui.InteractiveOptions{
			Title:       "Running Command",
			CommandName: invowkfile.CommandName(cmdName), //goplint:ignore -- from Cobra command name, validated by discovery
			OnProgramReady: func(p *tea.Program) {
				go BridgeTUIRequests(tuiServer, p)
			},
		},
		prepared.Cmd,
	)
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("interactive execution failed: %w", err)} //goplint:ignore -- literal exit code for interactive failure
	}

	return &runtime.Result{
		ExitCode: interactiveResult.ExitCode,
		Error:    interactiveResult.Error,
	}
}
