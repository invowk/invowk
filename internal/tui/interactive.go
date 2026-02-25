// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/xpty"
)

// Const block placed before var/type (decorder: const → var → type → func).
const (
	stateExecuting executionState = iota
	stateCompleted
	stateTUI // Displaying an embedded TUI component
)

// All type declarations consolidated in a single block.
type (
	// InteractiveOptions configures the interactive execution mode.
	InteractiveOptions struct {
		// Title is displayed at the top of the viewport.
		Title string
		// CommandName is the name of the command being executed.
		CommandName invowkfile.CommandName
		// Config holds common TUI configuration.
		Config Config
		// OnProgramReady is called with the *tea.Program after it's created.
		// This allows callers to access the program for terminal control
		// (e.g., ReleaseTerminal/RestoreTerminal for nested TUI components).
		OnProgramReady func(p *tea.Program)
	}

	// InteractiveResult contains the result of interactive execution.
	InteractiveResult struct {
		// ExitCode is the exit code from the command.
		ExitCode types.ExitCode
		// Error contains any execution error.
		Error error
		// Duration is how long the command took to execute.
		Duration time.Duration
	}

	// executionState represents the current state of execution.
	executionState int

	// outputMsg is sent when new output is available from the PTY.
	outputMsg struct {
		content string
	}

	// doneMsg is sent when command execution completes.
	doneMsg struct {
		result InteractiveResult
	}

	// TUIComponentMsg is sent by the bridge goroutine when a child process
	// requests a TUI component to be rendered as an overlay.
	//
	//nolint:revive // TUIComponentMsg is more descriptive than ComponentMsg
	TUIComponentMsg struct {
		// Component is the type of TUI component to render.
		Component ComponentType
		// Options contains the component-specific options as raw JSON.
		Options json.RawMessage
		// ResponseCh is where the result should be sent when the component completes.
		ResponseCh chan<- tuiserver.Response
	}

	// tuiComponentDoneMsg is sent when an embedded TUI component completes.
	tuiComponentDoneMsg struct {
		result    any
		err       error
		cancelled bool
	}

	// interactiveModel is the Bubbletea model for interactive execution.
	interactiveModel struct {
		viewport viewport.Model
		title    string
		cmdName  string
		content  strings.Builder
		result   *InteractiveResult
		state    executionState
		ready    bool
		width    int
		height   int
		mu       sync.Mutex
		pty      xpty.Pty

		// TUI component overlay fields
		activeComponent     EmbeddableComponent
		activeComponentType ComponentType
		componentDoneCh     chan<- tuiserver.Response
	}

	// InteractiveBuilder provides a fluent API for building interactive execution.
	InteractiveBuilder struct {
		opts InteractiveOptions
		cmd  *exec.Cmd
		ctx  context.Context
	}

	// invalidExecutionStateError is returned when an executionState value is not
	// one of the defined states.
	invalidExecutionStateError struct {
		value executionState
	}
)

// String returns the human-readable name of the executionState.
func (s executionState) String() string {
	switch s {
	case stateExecuting:
		return "executing"
	case stateCompleted:
		return "completed"
	case stateTUI:
		return "tui"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// isValid returns whether the executionState is one of the defined states,
// and a list of validation errors if it is not.
func (s executionState) isValid() (bool, []error) {
	switch s {
	case stateExecuting, stateCompleted, stateTUI:
		return true, nil
	default:
		return false, []error{&invalidExecutionStateError{value: s}}
	}
}

func (e *invalidExecutionStateError) Error() string {
	return fmt.Sprintf("invalid execution state: %d", int(e.value))
}

// NewInteractive creates a new InteractiveBuilder with default options.
func NewInteractive() *InteractiveBuilder {
	return &InteractiveBuilder{
		opts: InteractiveOptions{
			Title:  "Running Command",
			Config: DefaultConfig(),
		},
		ctx: context.Background(),
	}
}

// Title sets the title displayed at the top.
func (b *InteractiveBuilder) Title(title string) *InteractiveBuilder {
	b.opts.Title = title
	return b
}

// CommandName sets the command name displayed in the header.
// The string is cast to invowkfile.CommandName internally.
func (b *InteractiveBuilder) CommandName(name string) *InteractiveBuilder {
	b.opts.CommandName = invowkfile.CommandName(name)
	return b
}

// Command sets the exec.Cmd to run.
func (b *InteractiveBuilder) Command(cmd *exec.Cmd) *InteractiveBuilder {
	b.cmd = cmd
	return b
}

// Context sets the context for cancellation.
func (b *InteractiveBuilder) Context(ctx context.Context) *InteractiveBuilder {
	b.ctx = ctx
	return b
}

// Run executes in interactive mode and returns the result.
func (b *InteractiveBuilder) Run() (*InteractiveResult, error) {
	if b.cmd == nil {
		return nil, errNoCommand
	}
	return RunInteractiveCmd(b.ctx, b.opts, b.cmd)
}
