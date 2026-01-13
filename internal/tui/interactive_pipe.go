// SPDX-License-Identifier: EPL-2.0

// Package tui provides terminal user interface components for invowk.
//
// This file implements pipe-based interactive mode, which provides local echo
// for user input when running commands through pipes instead of PTY.
//
// # Local Echo Behavior
//
// Unlike PTY-based execution where the terminal driver provides automatic echo,
// pipe-based execution requires explicit local echo. This implementation:
//
//   - Maintains an input buffer that displays typed characters immediately
//   - Clears the input buffer when output arrives (input was consumed)
//   - Clears the input buffer when Enter is pressed (line submitted)
//   - Supports backspace for editing the current input line
//   - Handles concurrent output gracefully without duplicating echoed input
//
// # Smart Buffer Management
//
// The smart buffer prevents double-echo by tracking when input is consumed:
//
//  1. User types characters → displayed immediately via inputBuffer
//  2. User presses Enter → inputBuffer cleared, newline sent to command
//  3. Command produces output → any remaining inputBuffer cleared (consumed)
//
// This approach works well for simple prompts (read, input) but does not
// support complex scenarios like password input (where echo should be disabled)
// or cursor-based editing (readline-style).
package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ExecutorFunc represents a function that executes a command with provided I/O streams.
// This abstraction allows the pipe-based interactive mode to work with any runtime
// (native, virtual, or container) since all runtimes can accept io.Reader/io.Writer.
type ExecutorFunc func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) error

// PipeInteractiveOptions configures the pipe-based interactive execution mode.
type PipeInteractiveOptions struct {
	// Title is displayed at the top of the viewport.
	Title string
	// CommandName is the name of the command being executed.
	CommandName string
	// Config holds common TUI configuration.
	Config Config
	// EchoInput controls whether typed characters are displayed locally.
	// When true (default), characters are echoed to the viewport as typed.
	// Set to false for password prompts or when the command handles its own echo.
	EchoInput bool
}

// DefaultPipeInteractiveOptions returns default options with echo enabled.
func DefaultPipeInteractiveOptions() PipeInteractiveOptions {
	return PipeInteractiveOptions{
		Title:     "Running Command",
		EchoInput: true,
		Config:    DefaultConfig(),
	}
}

// pipeInteractiveModel is the Bubbletea model for pipe-based interactive execution.
// It implements local echo for user input and smart buffer management to handle
// concurrent output from the command.
type pipeInteractiveModel struct {
	viewport viewport.Model
	title    string
	cmdName  string

	// content holds the confirmed output (from command + submitted input lines)
	content strings.Builder

	// inputBuffer holds the current line being typed (not yet submitted)
	// This is displayed but not part of content until Enter is pressed or
	// the command produces output (indicating input was consumed)
	inputBuffer strings.Builder

	// echoInput controls whether typed characters are displayed
	echoInput bool

	result *InteractiveResult
	state  executionState
	ready  bool
	width  int
	height int

	// mu protects content and inputBuffer for concurrent access
	mu sync.Mutex

	// stdinW is the write end of stdin pipe to send input to the command
	stdinW io.WriteCloser
}

// newPipeInteractiveModel creates a new model for pipe-based interactive execution.
func newPipeInteractiveModel(opts PipeInteractiveOptions, stdinW io.WriteCloser) *pipeInteractiveModel {
	return &pipeInteractiveModel{
		title:     opts.Title,
		cmdName:   opts.CommandName,
		echoInput: opts.EchoInput,
		state:     stateExecuting,
		stdinW:    stdinW,
	}
}

// Init implements tea.Model.
func (m *pipeInteractiveModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *pipeInteractiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case outputMsg:
		m.handleOutput(msg.content)

	case doneMsg:
		m.handleDone(msg.result)
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleOutput processes output from the command.
// When output arrives, any pending input in the buffer is considered consumed
// (the command read it), so we clear the input buffer to prevent double-echo.
func (m *pipeInteractiveModel) handleOutput(output string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear input buffer - the command has produced output, meaning
	// it likely consumed any pending input. This prevents double-echo
	// where both our local echo and the command's echo would show.
	m.inputBuffer.Reset()

	// Append the output to confirmed content
	m.content.WriteString(output)

	// Update viewport
	if m.ready {
		m.viewport.SetContent(m.content.String())
		if m.state == stateExecuting {
			m.viewport.GotoBottom()
		}
	}
}

// handleDone processes command completion.
func (m *pipeInteractiveModel) handleDone(result InteractiveResult) {
	m.mu.Lock()

	m.result = &result
	m.state = stateCompleted

	// Clear any remaining input buffer
	m.inputBuffer.Reset()

	m.mu.Unlock()

	// Close stdin pipe when done
	if m.stdinW != nil {
		m.stdinW.Close()
		m.stdinW = nil
	}

	// Add completion message to output
	m.appendCompletionMessage()

	if m.ready {
		m.mu.Lock()
		m.viewport.SetContent(m.content.String())
		m.mu.Unlock()
		m.viewport.GotoBottom()
	}
}

// handleKeyMsg processes keyboard input during execution.
func (m *pipeInteractiveModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateExecuting:
		return m.handleExecutingKeyMsg(msg)

	case stateCompleted:
		return m.handleCompletedKeyMsg(msg)
	}

	return m, nil
}

// handleExecutingKeyMsg processes keyboard input while command is running.
func (m *pipeInteractiveModel) handleExecutingKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+\\":
		// Emergency escape: force quit
		if m.stdinW != nil {
			m.stdinW.Close()
			m.stdinW = nil
		}
		return m, tea.Quit

	default:
		// Forward the key to stdin pipe and handle local echo
		if m.stdinW != nil {
			m.forwardKeyToStdin(msg)
		}
	}

	// After modifying content/viewport, we need to trigger a re-render.
	// Pass the message through viewport.Update to ensure proper rendering.
	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleCompletedKeyMsg processes keyboard input after command completes.
func (m *pipeInteractiveModel) handleCompletedKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc", "enter":
		return m, tea.Quit
	case "up", "k":
		m.viewport.LineUp(1)
	case "down", "j":
		m.viewport.LineDown(1)
	case "pgup", "b":
		m.viewport.HalfViewUp()
	case "pgdown", "f", " ":
		m.viewport.HalfViewDown()
	case "home", "g":
		m.viewport.GotoTop()
	case "end", "G":
		m.viewport.GotoBottom()
	}

	// Pass through viewport.Update to ensure proper rendering after scroll
	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

// forwardKeyToStdin converts a key message to bytes, writes to stdin pipe,
// and updates the local echo buffer.
func (m *pipeInteractiveModel) forwardKeyToStdin(msg tea.KeyMsg) {
	var data []byte
	var echoChar string
	var clearBuffer bool

	switch msg.Type {
	case tea.KeyRunes:
		data = []byte(string(msg.Runes))
		echoChar = string(msg.Runes)

	case tea.KeyEnter:
		data = []byte("\n")
		// On Enter, commit the input buffer to content with a newline
		clearBuffer = true
		m.commitInputBuffer()

	case tea.KeySpace:
		data = []byte(" ")
		echoChar = " "

	case tea.KeyTab:
		data = []byte("\t")
		echoChar = "\t"

	case tea.KeyBackspace:
		data = []byte{0x7f} // DEL character
		m.handleBackspace()

	case tea.KeyCtrlC:
		data = []byte{0x03} // ETX (Ctrl+C)

	case tea.KeyCtrlD:
		data = []byte{0x04} // EOT (Ctrl+D)

	case tea.KeyCtrlZ:
		data = []byte{0x1a} // SUB (Ctrl+Z)

	case tea.KeyEscape:
		data = []byte{0x1b}

	default:
		// For other keys, try to derive from runes
		if len(msg.Runes) > 0 {
			data = []byte(string(msg.Runes))
			echoChar = string(msg.Runes)
		}
	}

	// Write to stdin pipe
	if len(data) > 0 {
		_, _ = m.stdinW.Write(data)
	}

	// Update local echo (if enabled and not a control key)
	if m.echoInput && echoChar != "" && !clearBuffer {
		m.appendToInputBuffer(echoChar)
	}
}

// appendToInputBuffer adds a character to the input buffer and updates display.
func (m *pipeInteractiveModel) appendToInputBuffer(s string) {
	m.mu.Lock()
	m.inputBuffer.WriteString(s)
	m.mu.Unlock()

	m.updateViewportWithInput()
}

// handleBackspace removes the last character from the input buffer.
func (m *pipeInteractiveModel) handleBackspace() {
	m.mu.Lock()
	current := m.inputBuffer.String()
	if len(current) > 0 {
		// Remove last rune (not byte, to handle UTF-8 correctly)
		_, size := utf8.DecodeLastRuneInString(current)
		m.inputBuffer.Reset()
		m.inputBuffer.WriteString(current[:len(current)-size])
	}
	m.mu.Unlock()

	// Update display after releasing the lock
	m.updateViewportWithInput()
}

// commitInputBuffer moves the current input buffer to content and clears it.
// Called when Enter is pressed to commit the typed line.
func (m *pipeInteractiveModel) commitInputBuffer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.inputBuffer.Len() > 0 {
		// Add the input to content (will be followed by command output)
		m.content.WriteString(m.inputBuffer.String())
	}
	// Add newline for the Enter key
	m.content.WriteString("\n")
	m.inputBuffer.Reset()

	// Update viewport with committed content
	if m.ready {
		m.viewport.SetContent(m.content.String())
		m.viewport.GotoBottom()
	}
}

// updateViewportWithInput updates the viewport to show content + current input buffer.
func (m *pipeInteractiveModel) updateViewportWithInput() {
	if !m.ready {
		return
	}

	m.mu.Lock()
	displayContent := m.content.String() + m.inputBuffer.String()
	m.mu.Unlock()

	m.viewport.SetContent(displayContent)
	m.viewport.GotoBottom()
}

// handleWindowSize processes terminal resize events.
func (m *pipeInteractiveModel) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	headerHeight := 2 // Title + separator
	footerHeight := 1 // Status line
	viewportHeight := msg.Height - headerHeight - footerHeight

	if viewportHeight < 1 {
		viewportHeight = 1
	}

	if !m.ready {
		m.viewport = viewport.New(msg.Width, viewportHeight)
		m.viewport.YPosition = headerHeight
		m.ready = true
		m.mu.Lock()
		m.viewport.SetContent(m.content.String())
		m.mu.Unlock()
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight
	}

	return m, nil
}

// appendCompletionMessage adds a styled completion message to the output.
func (m *pipeInteractiveModel) appendCompletionMessage() {
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

// View implements tea.Model.
func (m *pipeInteractiveModel) View() string {
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

	return fmt.Sprintf("%s\n%s\n%s\n%s", title, separator, m.viewport.View(), footer)
}

// GetInputBuffer returns the current input buffer contents (for testing).
func (m *pipeInteractiveModel) GetInputBuffer() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inputBuffer.String()
}

// GetContent returns the confirmed content (for testing).
func (m *pipeInteractiveModel) GetContent() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.content.String()
}

// RunInteractivePipe executes a command using pipe-based I/O in an interactive TUI.
// This mode works with all runtimes (native, virtual, container) because it uses
// standard io.Reader/io.Writer interfaces instead of PTY.
//
// # Local Echo
//
// Since pipes don't provide automatic echo like PTY, this function implements
// local echo: typed characters are displayed immediately in the viewport.
// When the command produces output (indicating it consumed the input), the
// local echo buffer is cleared to prevent double-display.
//
// # Comparison with PTY Mode
//
// Unlike the PTY-based RunInteractiveCmd, this mode:
//   - Does NOT provide full terminal emulation (no cursor control)
//   - Does NOT support full-screen applications (vim, less, top, etc.)
//   - DOES support simple interactive prompts and keyboard input
//   - DOES provide local echo for typed characters
//   - DOES work with the virtual runtime (mvdan/sh)
//   - DOES work with containers without requiring -t flag
//
// # When to Use
//
// Use pipe-based interactive mode (tty: false) for:
//   - Simple prompts with read/input commands
//   - Scripts that need basic user input
//   - Commands that work with all runtimes
//
// Use PTY-based interactive mode (tty: true) for:
//   - Full-screen TUI applications
//   - Commands requiring cursor control
//   - Programs that detect terminal capabilities
func RunInteractivePipe(ctx context.Context, opts PipeInteractiveOptions, executor ExecutorFunc) (*InteractiveResult, error) {
	// Apply defaults if not set
	if opts.Title == "" {
		opts.Title = "Running Command"
	}
	// EchoInput defaults to true (zero value is false, so we check explicitly)
	// The caller should use DefaultPipeInteractiveOptions() or set EchoInput explicitly

	// Create pipes for stdin, stdout, and stderr
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	// Create the model with stdin writer
	m := newPipeInteractiveModel(opts, stdinW)

	// Create the Bubbletea program with alternate screen
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Create a context that can be canceled
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Result channel for executor completion
	resultCh := make(chan InteractiveResult, 1)

	// Run the executor in a goroutine
	go func() {
		defer stdoutW.Close()
		defer stderrW.Close()
		defer stdinR.Close()

		startTime := time.Now()
		err := executor(execCtx, stdinR, stdoutW, stderrW)
		duration := time.Since(startTime)

		result := InteractiveResult{
			Duration: duration,
		}

		if err != nil {
			// Try to extract exit code from error if it's an exit error
			result.Error = err
			result.ExitCode = 1
		}

		resultCh <- result
	}()

	// Read stdout in a goroutine and send to the program
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutR.Read(buf)
			if n > 0 {
				p.Send(outputMsg{content: string(buf[:n])})
			}
			if err != nil {
				break
			}
		}
	}()

	// Read stderr in a goroutine and send to the program
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderrR.Read(buf)
			if n > 0 {
				// Optionally style stderr differently (for now, same as stdout)
				p.Send(outputMsg{content: string(buf[:n])})
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for executor to complete and send done message
	go func() {
		result := <-resultCh
		p.Send(doneMsg{result: result})
	}()

	// Run the TUI
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	// Extract result from final model
	if pm, ok := finalModel.(*pipeInteractiveModel); ok && pm.result != nil {
		return pm.result, nil
	}

	// If we get here without a result, the user force-quit
	return &InteractiveResult{
		ExitCode: 130, // Standard exit code for Ctrl+C
		Error:    fmt.Errorf("execution interrupted"),
	}, nil
}

// PipeInteractiveBuilder provides a fluent API for building pipe-based interactive execution.
type PipeInteractiveBuilder struct {
	opts     PipeInteractiveOptions
	executor ExecutorFunc
	ctx      context.Context
}

// NewPipeInteractive creates a new PipeInteractiveBuilder with default options.
// Echo is enabled by default.
func NewPipeInteractive() *PipeInteractiveBuilder {
	return &PipeInteractiveBuilder{
		opts: DefaultPipeInteractiveOptions(),
		ctx:  context.Background(),
	}
}

// Title sets the title displayed at the top.
func (b *PipeInteractiveBuilder) Title(title string) *PipeInteractiveBuilder {
	b.opts.Title = title
	return b
}

// CommandName sets the command name displayed in the header.
func (b *PipeInteractiveBuilder) CommandName(name string) *PipeInteractiveBuilder {
	b.opts.CommandName = name
	return b
}

// EchoInput sets whether typed characters are displayed locally.
// Default is true. Set to false for password prompts.
func (b *PipeInteractiveBuilder) EchoInput(echo bool) *PipeInteractiveBuilder {
	b.opts.EchoInput = echo
	return b
}

// Executor sets the executor function that runs the command.
func (b *PipeInteractiveBuilder) Executor(executor ExecutorFunc) *PipeInteractiveBuilder {
	b.executor = executor
	return b
}

// Context sets the context for cancellation.
func (b *PipeInteractiveBuilder) Context(ctx context.Context) *PipeInteractiveBuilder {
	b.ctx = ctx
	return b
}

// Run executes in pipe-based interactive mode and returns the result.
func (b *PipeInteractiveBuilder) Run() (*InteractiveResult, error) {
	if b.executor == nil {
		return nil, fmt.Errorf("no executor provided")
	}
	return RunInteractivePipe(b.ctx, b.opts, b.executor)
}
