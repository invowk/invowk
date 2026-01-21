// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"invowk-cli/internal/tuiserver"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/xpty"
)

// Const block placed before var/type (decorder: const → var → type → func).
const (
	stateExecuting executionState = iota
	stateCompleted
	stateTUI // Displaying an embedded TUI component
)

// Var block placed after const, before type (decorder: const → var → type → func).
// oscSequenceRe matches all OSC (Operating System Command) escape sequences.
// OSC sequences are used for terminal features like window titles, hyperlinks,
// color queries, clipboard operations, etc.
//
// In the context of invowk's interactive pager, these sequences don't function
// (hyperlinks aren't clickable, window titles don't apply, etc.) and can appear
// as visual garbage when fragmented across PTY read buffers.
//
// Format: ESC ] <content> <terminator>
// Terminators: BEL (\x07), ST (\x1b\\), or backslash alone (\)
//
// Also matches partial/fragmented sequences where leading ESC was consumed.
var oscSequenceRe = regexp.MustCompile(
	`\x1b\][^\x07\x1b\\]*(?:\x07|\x1b\\|\\)` + // Full OSC: ESC ] ... terminator
		`|` +
		`\][^\x07\x1b\\]*(?:\x07|\x1b\\|\\)`, // Partial: ] ... terminator (missing ESC)
)

// All type declarations consolidated in a single block.
type (
	// InteractiveOptions configures the interactive execution mode.
	InteractiveOptions struct {
		// Title is displayed at the top of the viewport.
		Title string
		// CommandName is the name of the command being executed.
		CommandName string
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
		ExitCode int
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
)

func (m *interactiveModel) Init() tea.Cmd {
	return nil
}

func (m *interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case outputMsg:
		m.mu.Lock()
		m.content.WriteString(msg.content)
		m.mu.Unlock()
		if m.ready {
			m.viewport.SetContent(m.content.String())
			// Auto-scroll to bottom during execution
			if m.state == stateExecuting {
				m.viewport.GotoBottom()
			}
		}

	case doneMsg:
		m.result = &msg.result
		m.state = stateCompleted
		// Add completion message to output
		m.appendCompletionMessage()
		if m.ready {
			m.viewport.SetContent(m.content.String())
			m.viewport.GotoBottom()
		}

	case TUIComponentMsg:
		// A child process requested a TUI component to be rendered
		return m.handleTUIComponentRequest(msg)

	case tuiComponentDoneMsg:
		// The embedded TUI component has completed
		return m.handleTUIComponentDone(msg)
	}

	// When in TUI state, also update the active component
	if m.state == stateTUI && m.activeComponent != nil {
		var cmd tea.Cmd
		updatedModel, cmd := m.activeComponent.Update(msg)
		if ec, ok := updatedModel.(EmbeddableComponent); ok {
			m.activeComponent = ec
		}

		// Check if the component is done
		if m.activeComponent.IsDone() {
			// Component is done - send our own completion message
			// and ignore any command from the component (like tea.Quit)
			// to prevent the interactive program from quitting prematurely
			result, err := m.activeComponent.Result()
			cancelled := m.activeComponent.Cancelled()
			cmds = append(cmds, func() tea.Msg {
				return tuiComponentDoneMsg{
					result:    result,
					err:       err,
					cancelled: cancelled,
				}
			})
		} else if cmd != nil {
			// Only add the component's command if it's not done
			cmds = append(cmds, cmd)
		}
	}

	if m.ready && m.state != stateTUI {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m *interactiveModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6"))

	title := titleStyle.Render(m.title)
	if m.cmdName != "" {
		title += " " + cmdStyle.Render(m.cmdName)
	}

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#374151"))
	separator := separatorStyle.Render(strings.Repeat("-", m.width))

	// Footer with status
	var footerContent string
	if m.state == stateCompleted {
		if m.result != nil && m.result.ExitCode == 0 && m.result.Error == nil {
			footerContent = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Render("Done")
		} else {
			footerContent = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				Render("Failed")
		}
		footerContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render("  |  arrows: scroll  |  Enter/q: exit")
	} else {
		footerContent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Render("Running...")
		footerContent += lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Render("  |  Ctrl+\\: force quit")
	}

	scrollPercent := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render(fmt.Sprintf("  %3.f%%", m.viewport.ScrollPercent()*100))

	footer := footerContent + scrollPercent

	baseView := fmt.Sprintf("%s\n%s\n%s\n%s", title, separator, m.viewport.View(), footer)

	// If we're displaying a TUI component, render it as an overlay on top of the base view
	if m.state == stateTUI && m.activeComponent != nil {
		return RenderOverlay(baseView, m.activeComponent.View(), m.width, m.height)
	}

	return baseView
}

// handleKeyMsg processes keyboard input based on current state.
func (m *interactiveModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateExecuting:
		// During execution, forward most keys to the PTY
		switch msg.String() {
		case "ctrl+\\":
			// Emergency escape: force quit
			return m, tea.Quit
		default:
			// Forward the key to the PTY
			if m.pty != nil {
				m.forwardKeyToPty(msg)
			}
		}

	case stateCompleted:
		// After completion, handle UI navigation
		switch msg.String() {
		case keyCtrlC, "q", "esc", "enter":
			return m, tea.Quit
		case "up", "k":
			m.viewport.ScrollUp(1)
		case "down", "j":
			m.viewport.ScrollDown(1)
		case "pgup", "b":
			m.viewport.HalfPageUp()
		case "pgdown", "f", " ":
			m.viewport.HalfPageDown()
		case "home", "g":
			m.viewport.GotoTop()
		case "end", "G":
			m.viewport.GotoBottom()
		}

	case stateTUI:
		// When displaying a TUI component, delegate to the component
		if m.activeComponent != nil {
			var cmds []tea.Cmd
			updatedModel, cmd := m.activeComponent.Update(msg)
			if ec, ok := updatedModel.(EmbeddableComponent); ok {
				m.activeComponent = ec
			}

			// Check if the component is done
			if m.activeComponent.IsDone() {
				// Component is done - send our own completion message
				// and ignore any command from the component (like tea.Quit)
				// to prevent the interactive program from quitting prematurely
				result, err := m.activeComponent.Result()
				cancelled := m.activeComponent.Cancelled()
				cmds = append(cmds, func() tea.Msg {
					return tuiComponentDoneMsg{
						result:    result,
						err:       err,
						cancelled: cancelled,
					}
				})
			} else if cmd != nil {
				// Only add the component's command if it's not done
				cmds = append(cmds, cmd)
			}

			return m, tea.Batch(cmds...)
		}
	}

	return m, nil
}

// handleTUIComponentRequest creates an embedded TUI component and switches to TUI state.
func (m *interactiveModel) handleTUIComponentRequest(msg TUIComponentMsg) (tea.Model, tea.Cmd) {
	// Calculate modal dimensions based on component type
	modalSize := CalculateModalSize(msg.Component, m.width, m.height)

	// Create the embeddable component with modal dimensions
	component, err := CreateEmbeddableComponent(msg.Component, msg.Options, modalSize.Width, modalSize.Height)
	if err != nil {
		// Send error response back to the server
		go func() {
			msg.ResponseCh <- tuiserver.Response{
				Error: fmt.Sprintf("failed to create component: %v", err),
			}
		}()
		return m, nil
	}

	// Store the component, type, and response channel
	m.activeComponent = component
	m.activeComponentType = msg.Component
	m.componentDoneCh = msg.ResponseCh
	m.state = stateTUI

	// Initialize the component
	return m, m.activeComponent.Init()
}

// handleTUIComponentDone processes the result from a completed TUI component.
func (m *interactiveModel) handleTUIComponentDone(msg tuiComponentDoneMsg) (tea.Model, tea.Cmd) {
	if m.componentDoneCh == nil {
		return m, nil
	}

	// Build the response
	var response tuiserver.Response

	switch {
	case msg.cancelled:
		response.Cancelled = true
	case msg.err != nil:
		response.Error = msg.err.Error()
	default:
		// Convert the raw result to a protocol-compliant struct
		protocolResult := convertToProtocolResult(m.activeComponentType, msg.result)

		// Marshal the protocol result to JSON
		resultJSON, err := json.Marshal(protocolResult)
		if err != nil {
			response.Error = fmt.Sprintf("failed to marshal result: %v", err)
		} else {
			response.Result = resultJSON
		}
	}

	// Send the response in a goroutine to avoid blocking
	doneCh := m.componentDoneCh
	go func() {
		doneCh <- response
	}()

	// Clean up and return to previous state
	m.activeComponent = nil
	m.activeComponentType = ""
	m.componentDoneCh = nil
	m.state = stateExecuting

	return m, nil
}

// forwardKeyToPty forwards keyboard input to the PTY.
func (m *interactiveModel) forwardKeyToPty(msg tea.KeyMsg) {
	// Convert the key message to bytes and write to PTY
	var data []byte

	//exhaustive:ignore
	switch msg.Type {
	case tea.KeyRunes:
		data = []byte(string(msg.Runes))
	case tea.KeyEnter:
		data = []byte("\r")
	case tea.KeySpace:
		data = []byte(" ")
	case tea.KeyTab:
		data = []byte("\t")
	case tea.KeyBackspace:
		data = []byte{0x7f} // DEL character
	case tea.KeyDelete:
		data = []byte{0x1b, '[', '3', '~'} // ESC [ 3 ~
	case tea.KeyUp:
		data = []byte{0x1b, '[', 'A'}
	case tea.KeyDown:
		data = []byte{0x1b, '[', 'B'}
	case tea.KeyRight:
		data = []byte{0x1b, '[', 'C'}
	case tea.KeyLeft:
		data = []byte{0x1b, '[', 'D'}
	case tea.KeyHome:
		data = []byte{0x1b, '[', 'H'}
	case tea.KeyEnd:
		data = []byte{0x1b, '[', 'F'}
	case tea.KeyPgUp:
		data = []byte{0x1b, '[', '5', '~'}
	case tea.KeyPgDown:
		data = []byte{0x1b, '[', '6', '~'}
	case tea.KeyCtrlC:
		data = []byte{0x03} // ETX (Ctrl+C)
	case tea.KeyCtrlD:
		data = []byte{0x04} // EOT (Ctrl+D)
	case tea.KeyCtrlZ:
		data = []byte{0x1a} // SUB (Ctrl+Z)
	case tea.KeyEscape:
		data = []byte{0x1b}
	default:
		// For other control keys, try to derive the control character
		if len(msg.Runes) > 0 {
			data = []byte(string(msg.Runes))
		}
	}

	if len(data) > 0 {
		_, _ = m.pty.Write(data)
	}
}

// handleWindowSize handles terminal resize events.
func (m *interactiveModel) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	headerHeight := 2 // Title + separator
	footerHeight := 1 // Status line
	viewportHeight := max(msg.Height-headerHeight-footerHeight, 1)

	if !m.ready {
		m.viewport = viewport.New(msg.Width, viewportHeight)
		m.viewport.YPosition = headerHeight
		m.ready = true
		m.viewport.SetContent(m.content.String())
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight
	}

	// Resize the active TUI component if one is displayed
	if m.activeComponent != nil && m.activeComponentType != "" {
		modalSize := CalculateModalSize(m.activeComponentType, msg.Width, msg.Height)
		m.activeComponent.SetSize(modalSize.Width, modalSize.Height)
	}

	// Resize the PTY to match
	if m.pty != nil && m.state == stateExecuting {
		_ = m.pty.Resize(msg.Width, viewportHeight) //nolint:errcheck // Best-effort resize
	}

	return m, nil
}

// appendCompletionMessage appends the completion status message to the output.
func (m *interactiveModel) appendCompletionMessage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var status string
	var statusStyle lipgloss.Style

	if m.result.ExitCode == 0 && m.result.Error == nil {
		statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true)
		status = "COMPLETED SUCCESSFULLY"
	} else {
		statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true)
		if m.result.Error != nil {
			status = fmt.Sprintf("FAILED: %v", m.result.Error)
		} else {
			status = fmt.Sprintf("EXITED WITH CODE %d", m.result.ExitCode)
		}
	}

	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render(strings.Repeat("-", 50))

	durationStr := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Render(fmt.Sprintf("Duration: %s", m.result.Duration.Round(time.Millisecond)))

	m.content.WriteString("\n\n")
	m.content.WriteString(separator)
	m.content.WriteString("\n\n")
	m.content.WriteString(statusStyle.Render(status))
	m.content.WriteString("\n")
	m.content.WriteString(durationStr)
	m.content.WriteString("\n\n")
	m.content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A78BFA")).
		Italic(true).
		Render("Press Enter to return to terminal..."))
	m.content.WriteString("\n")
}

// RunInteractiveCmd executes a command in interactive mode with alternate screen buffer.
// It creates a PTY for the command, forwards keyboard input during execution,
// and allows output review after completion.
func RunInteractiveCmd(ctx context.Context, opts InteractiveOptions, cmd *exec.Cmd) (result *InteractiveResult, err error) {
	// Get terminal size for initial PTY dimensions
	width, height := 80, 24
	if w, h, termErr := getTerminalSize(); termErr == nil {
		width, height = w, h
	}

	// Create a PTY using xpty (cross-platform)
	pty, err := xpty.NewPty(width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}
	defer func() {
		if closeErr := pty.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// Start the command on the PTY
	if err = pty.Start(cmd); err != nil {
		return nil, fmt.Errorf("failed to start command on PTY: %w", err)
	}

	// Create the model
	m := newInteractiveModel(opts, pty)

	// Create the Bubbletea program with alternate screen
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Notify the caller that the program is ready.
	// This allows them to access the program for terminal control.
	if opts.OnProgramReady != nil {
		opts.OnProgramReady(p)
	}

	// Read PTY output in a goroutine and send to the program
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := pty.Read(buf)
			if n > 0 {
				// Strip OSC sequences that don't function in the pager context
				// and appear as visual garbage when fragmented across buffers.
				content := stripOSCSequences(string(buf[:n]))
				if content != "" {
					p.Send(outputMsg{content: content})
				}
			}
			if readErr != nil {
				// Non-EOF errors are ignored; PTY read errors during command
				// execution are typically transient and don't warrant crashing
				break
			}
		}
	}()

	// Wait for the command to complete in a goroutine
	go func() {
		startTime := time.Now()
		waitErr := xpty.WaitProcess(ctx, cmd)
		duration := time.Since(startTime)

		result := InteractiveResult{
			Duration: duration,
		}

		if waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.Error = waitErr
				result.ExitCode = 1
			}
		}

		p.Send(doneMsg{result: result})
	}()

	// Run the TUI
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if im, ok := finalModel.(*interactiveModel); ok && im.result != nil {
		return im.result, nil
	}

	// If we get here without a result, the user force-quit
	return &InteractiveResult{
		ExitCode: 130, // Standard exit code for Ctrl+C
		Error:    fmt.Errorf("execution interrupted"),
	}, nil
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
func (b *InteractiveBuilder) CommandName(name string) *InteractiveBuilder {
	b.opts.CommandName = name
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
		return nil, fmt.Errorf("no command provided")
	}
	return RunInteractiveCmd(b.ctx, b.opts, b.cmd)
}

// stripOSCSequences removes all OSC escape sequences from output.
// In invowk's interactive pager context, OSC features don't function anyway
// (the pager is a text viewport, not a full terminal emulator), and fragmented
// sequences appear as visual garbage.
func stripOSCSequences(s string) string {
	return oscSequenceRe.ReplaceAllString(s, "")
}

func newInteractiveModel(opts InteractiveOptions, pty xpty.Pty) *interactiveModel {
	return &interactiveModel{
		title:   opts.Title,
		cmdName: opts.CommandName,
		state:   stateExecuting,
		pty:     pty,
	}
}

// convertToProtocolResult converts a raw component result to a protocol-compliant struct.
// The tuiserver client expects specific JSON structures for each component type.
func convertToProtocolResult(componentType ComponentType, result any) any {
	switch componentType {
	case ComponentTypeInput, ComponentTypeTextArea, ComponentTypeWrite:
		// Input, TextArea, and Write return a string
		if s, ok := result.(string); ok {
			return tuiserver.InputResult{Value: s}
		}
		return tuiserver.InputResult{}

	case ComponentTypeConfirm:
		// Confirm returns a bool
		if b, ok := result.(bool); ok {
			return tuiserver.ConfirmResult{Confirmed: b}
		}
		return tuiserver.ConfirmResult{}

	case ComponentTypeChoose:
		// Choose returns []string
		if selected, ok := result.([]string); ok {
			return tuiserver.ChooseResult{Selected: selected}
		}
		return tuiserver.ChooseResult{Selected: []string{}}

	case ComponentTypeFilter:
		// Filter returns []string
		if selected, ok := result.([]string); ok {
			return tuiserver.FilterResult{Selected: selected}
		}
		return tuiserver.FilterResult{Selected: []string{}}

	case ComponentTypeFile:
		// File returns a string path
		if path, ok := result.(string); ok {
			return tuiserver.FileResult{Path: path}
		}
		return tuiserver.FileResult{}

	case ComponentTypeTable:
		// Table returns TableSelectionResult
		if tableResult, ok := result.(TableSelectionResult); ok {
			return tuiserver.TableResult{
				SelectedRow:   tableResult.SelectedRow,
				SelectedIndex: tableResult.SelectedIndex,
			}
		}
		return tuiserver.TableResult{SelectedIndex: -1}

	case ComponentTypePager:
		// Pager has no result
		return tuiserver.PagerResult{}

	case ComponentTypeSpin:
		// Spin returns SpinResult
		if spinResult, ok := result.(SpinResult); ok {
			return tuiserver.SpinResult{
				Stdout:   spinResult.Stdout,
				Stderr:   spinResult.Stderr,
				ExitCode: spinResult.ExitCode,
			}
		}
		return tuiserver.SpinResult{}

	default:
		// Unknown component type, return as-is
		return result
	}
}

// getTerminalSize attempts to get the current terminal size.
func getTerminalSize() (width, height int, err error) {
	// Try to get size from stdout
	fd := int(os.Stdout.Fd())
	width, height, err = getTerminalSizeFromFd(fd)
	if err == nil {
		return width, height, nil
	}

	// Fallback: try stderr
	fd = int(os.Stderr.Fd())
	width, height, err = getTerminalSizeFromFd(fd)
	if err == nil {
		return width, height, nil
	}

	// Fallback: try stdin
	fd = int(os.Stdin.Fd())
	return getTerminalSizeFromFd(fd)
}
