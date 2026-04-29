// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// InteractiveExecutor runs commands through the terminal UI adapter.
//
//goplint:ignore -- stateless infrastructure adapter has no domain invariants.
type InteractiveExecutor struct{}

// NewInteractiveExecutor creates an interactive terminal adapter.
func NewInteractiveExecutor() (InteractiveExecutor, error) {
	executor := InteractiveExecutor{}
	if err := executor.Validate(); err != nil {
		return InteractiveExecutor{}, err
	}
	return executor, nil
}

// Validate returns nil because InteractiveExecutor is stateless.
func (InteractiveExecutor) Validate() error {
	return nil
}

// Execute runs a command in interactive mode using Bubble Tea's alternate
// screen buffer. It starts an HTTP-based TUI server for bidirectional component
// requests between the running command and the terminal UI.
func (InteractiveExecutor) Execute(ctx *runtime.ExecutionContext, cmdName invowkfile.CommandName, interactiveRT runtime.InteractiveRuntime) *runtime.Result {
	if err := interactiveRT.Validate(ctx); err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: err} //goplint:ignore -- literal exit code for validation failure
	}

	tuiServer, err := tuiserver.New()
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("failed to create TUI server: %w", err)} //goplint:ignore -- literal exit code for server creation failure
	}

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

	var tuiServerURL types.TUIServerURL
	if containerRT, ok := interactiveRT.(*runtime.ContainerRuntime); ok {
		hostAddr := containerRT.GetHostAddressForContainer()
		tuiServerURL = tuiServer.URLWithHost(hostAddr)
	} else {
		tuiServerURL = tuiServer.URL()
	}

	ctx.TUI.ServerURL = tuiServerURL
	ctx.TUI.ServerToken = runtime.TUIServerToken(tuiServer.Token())

	prepared, err := interactiveRT.PrepareInteractive(ctx)
	if err != nil {
		return &runtime.Result{ExitCode: types.ExitCode(1), Error: fmt.Errorf("failed to prepare command: %w", err)} //goplint:ignore -- literal exit code for prepare failure
	}

	if prepared.Cleanup != nil {
		defer prepared.Cleanup()
	}

	interactiveResult, err := tui.RunInteractiveCmd(
		goCtx,
		tui.InteractiveOptions{
			Title:       "Running Command",
			CommandName: cmdName,
			OnProgramReady: func(p *tea.Program) {
				go bridgeTUIRequests(tuiServer, p)
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

func bridgeTUIRequests(server *tuiserver.Server, program *tea.Program) {
	for req := range server.RequestChannel() {
		responseCh := make(chan tui.ComponentResponse, 1)
		program.Send(tui.TUIComponentMsg{
			Component:  tui.ComponentType(req.Component),
			Options:    req.Options,
			ResponseCh: responseCh,
		})
		go forwardComponentResponse(tui.ComponentType(req.Component), responseCh, req.ResponseCh)
	}
}

func forwardComponentResponse(componentType tui.ComponentType, from <-chan tui.ComponentResponse, to chan<- tuiserver.Response) {
	componentResponse := <-from
	to <- componentResponseToProtocol(componentType, componentResponse)
}

func componentResponseToProtocol(componentType tui.ComponentType, response tui.ComponentResponse) tuiserver.Response {
	switch {
	case response.Cancelled:
		return tuiserver.Response{Cancelled: true}
	case response.Err != nil:
		return tuiserver.Response{Error: response.Err.Error()}
	default:
		resultJSON, err := json.Marshal(componentResultToProtocol(componentType, response.Result))
		if err != nil {
			return tuiserver.Response{Error: fmt.Sprintf("failed to marshal result: %v", err)}
		}
		return tuiserver.Response{Result: resultJSON}
	}
}

func componentResultToProtocol(componentType tui.ComponentType, result any) any {
	switch componentType {
	case tui.ComponentTypeInput, tui.ComponentTypeTextArea, tui.ComponentTypeWrite:
		if s, ok := result.(string); ok {
			return tuiserver.InputResult{Value: s}
		}
		return tuiserver.InputResult{}
	case tui.ComponentTypeConfirm:
		if b, ok := result.(bool); ok {
			return tuiserver.ConfirmResult{Confirmed: b}
		}
		return tuiserver.ConfirmResult{}
	case tui.ComponentTypeChoose:
		if selected, ok := result.([]string); ok {
			return tuiserver.ChooseResult{Selected: selected}
		}
		return tuiserver.ChooseResult{Selected: []string{}}
	case tui.ComponentTypeFilter:
		if selected, ok := result.([]string); ok {
			return tuiserver.FilterResult{Selected: selected}
		}
		return tuiserver.FilterResult{Selected: []string{}}
	case tui.ComponentTypeFile:
		if path, ok := result.(string); ok {
			return tuiserver.FileResult{Path: path}
		}
		return tuiserver.FileResult{}
	case tui.ComponentTypeTable:
		if tableResult, ok := result.(tui.TableSelectionResult); ok {
			return tuiserver.TableResult{
				SelectedRow:   tableResult.SelectedRow,
				SelectedIndex: tableResult.SelectedIndex,
			}
		}
		return tuiserver.TableResult{SelectedIndex: -1}
	case tui.ComponentTypePager:
		return tuiserver.PagerResult{}
	case tui.ComponentTypeSpin:
		if spinResult, ok := result.(tui.SpinResult); ok {
			return tuiserver.SpinResult{
				Stdout:   spinResult.Stdout,
				Stderr:   spinResult.Stderr,
				ExitCode: spinResult.ExitCode,
			}
		}
		return tuiserver.SpinResult{}
	default:
		return result
	}
}
