// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// tuiExposurePlanner is implemented by runtimes whose children reach the
	// TUI server from outside the host network view (the container runtime).
	// It runs only for interactive requests, before the TUI server starts.
	tuiExposurePlanner interface {
		PlanTUIExposure(ctx *runtime.ExecutionContext) containerplan.TUIExposurePlan
	}

	// InteractiveExecutor runs commands through the terminal UI adapter.
	//
	//goplint:ignore -- infrastructure adapter; its only field is a test seam for the PTY session.
	InteractiveExecutor struct {
		// runSession runs the prepared command in the interactive session; nil
		// selects runInteractiveCmd. Tests replace it to avoid a real terminal.
		runSession func(context.Context, tui.InteractiveOptions, *exec.Cmd) (*tui.InteractiveResult, error)
	}
)

// NewInteractiveExecutor creates an interactive terminal adapter.
func NewInteractiveExecutor() (InteractiveExecutor, error) {
	executor := InteractiveExecutor{}
	if err := executor.Validate(); err != nil {
		return InteractiveExecutor{}, err
	}
	return executor, nil
}

// Validate returns nil: InteractiveExecutor holds no domain state.
func (InteractiveExecutor) Validate() error {
	return nil
}

// Execute runs a command in interactive mode using Bubble Tea's alternate
// screen buffer. It starts an HTTP-based TUI server for bidirectional component
// requests between the running command and the terminal UI, bound according to
// the runtime's TUI exposure plan. When the plan has no narrow route, or is
// invalid, the run fails before any listener opens or any container starts.
func (e InteractiveExecutor) Execute(ctx *runtime.ExecutionContext, cmdName invowkfile.CommandName, interactiveRT commandsvc.RuntimeInteractiveCommand) *runtime.Result {
	if err := interactiveRT.Validate(ctx); err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: err} //goplint:ignore -- literal exit code for validation failure
	}

	goCtx := ctx.Context
	if goCtx == nil {
		goCtx = context.Background()
	}

	plan := planTUIExposure(ctx, interactiveRT)
	ctx.TUI = runtime.TUIContext{}
	if err := plan.Validate(); err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("refusing to start the TUI server: %w", err)} //goplint:ignore -- literal exit code for an invalid bind plan
	}
	if refusal := plan.RefusalError(); refusal != nil {
		// Before PrepareInteractive: no listener opens and no container starts.
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: refusal} //goplint:ignore -- literal exit code for a refused bind plan
	}
	if plan.Warning() != "" {
		// Before the alternate screen starts, so it cannot hide the warning.
		_, _ = fmt.Fprintln(interactiveStderr(ctx), plan.Warning().String())
	}

	tuiServer, tuiCtx, err := startTUIServer(goCtx, plan)
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: err} //goplint:ignore -- literal exit code for server start failure
	}
	defer func() {
		if stopErr := tuiServer.Stop(); stopErr != nil {
			slog.Debug("TUI server stop failed during cleanup", "error", stopErr)
		}
	}()
	ctx.TUI = tuiCtx

	prepared, err := interactiveRT.PrepareInteractive(ctx)
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("failed to prepare command: %w", err)} //goplint:ignore -- literal exit code for prepare failure
	}

	if prepared.Cleanup != nil {
		defer prepared.Cleanup()
	}

	opts := tui.InteractiveOptions{
		Title:       "Running Command",
		CommandName: cmdName.String(),
	}
	opts.OnProgramReady = func(p *tea.Program) {
		go bridgeTUIRequests(tuiServer, p)
	}
	runSession := e.runSession
	if runSession == nil {
		runSession = runInteractiveCmd
	}
	interactiveResult, err := runSession(goCtx, opts, prepared.Cmd)
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("interactive execution failed: %w", err)} //goplint:ignore -- literal exit code for interactive failure
	}

	return &runtime.Result{
		ExitCode: interactiveResult.ExitCode,
		Error:    interactiveResult.Error,
	}
}

// planTUIExposure asks the runtime for its bind plan; runtimes without a
// planner (native, virtual) get the loopback plan.
func planTUIExposure(ctx *runtime.ExecutionContext, interactiveRT commandsvc.RuntimeInteractiveCommand) containerplan.TUIExposurePlan {
	if planner, ok := interactiveRT.(tuiExposurePlanner); ok {
		return planner.PlanTUIExposure(ctx)
	}
	return containerplan.SelectTUIExposure(containerplan.TUINetworkFacts{}, false)
}

// startTUIServer starts a server bound per plan and returns the TUI context
// the command needs to reach it.
func startTUIServer(goCtx context.Context, plan containerplan.TUIExposurePlan) (*tuiserver.Server, runtime.TUIContext, error) {
	bindPlan := tuiserver.WildcardBindPlan()
	if plan.Kind() != containerplan.TUIPlanWide {
		var err error
		bindPlan, err = tuiserver.NewBindPlan(plan.BindAddrs()...)
		if err != nil {
			return nil, runtime.TUIContext{}, fmt.Errorf("failed to create TUI server: %w", err)
		}
	}
	server, err := tuiserver.NewWithBindPlan(bindPlan)
	if err != nil {
		return nil, runtime.TUIContext{}, fmt.Errorf("failed to create TUI server: %w", err)
	}
	if err := server.Start(goCtx); err != nil {
		return nil, runtime.TUIContext{}, fmt.Errorf("failed to start TUI server: %w", err)
	}
	tuiCtx := runtime.TUIContext{
		ServerURL:   server.URLWithHost(plan.DialHost()),
		ServerToken: runtime.TUIServerToken(server.Token()),
	}
	if plan.PastaForward() {
		mode, modeErr := container.PastaForwardNetworkMode(server.Port())
		if modeErr != nil {
			_ = server.Stop() //nolint:contextcheck // Stop takes no context; best-effort cleanup before reporting the plan error.
			return nil, runtime.TUIContext{}, fmt.Errorf("failed to build pasta forward: %w", modeErr)
		}
		tuiCtx.NetworkMode = mode
	}
	return server, tuiCtx, nil
}

func interactiveStderr(ctx *runtime.ExecutionContext) io.Writer {
	if ctx.IO.Stderr != nil {
		return ctx.IO.Stderr
	}
	return os.Stderr
}

func bridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		responseCh := make(chan tui.ComponentResponse, 1)
		options, err := componentRequestFromProtocol(req.Component, req.Options)
		if err != nil {
			req.ResponseCh <- tuiserver.Response{Error: err.Error()}
			continue
		}
		program.Send(tui.TUIComponentMsg{
			Component:  req.Component,
			Options:    options,
			ResponseCh: responseCh,
		})
		go forwardComponentResponse(req.Component, responseCh, req.ResponseCh)
	}
}

func forwardComponentResponse(componentType tui.ComponentType, from <-chan tui.ComponentResponse, to chan<- tuiserver.Response) {
	componentResponse := <-from
	to <- componentResponseToProtocol(componentType, componentResponse)
}
