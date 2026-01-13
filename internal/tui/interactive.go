// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/xpty"
)

// InteractiveOptions configures the interactive execution mode.
type InteractiveOptions struct {
	// Title is displayed at the top of the viewport.
	Title string
	// CommandName is the name of the command being executed.
	CommandName string
	// Config holds common TUI configuration.
	Config Config
}

// InteractiveResult contains the result of interactive execution.
type InteractiveResult struct {
	// ExitCode is the exit code from the command.
	ExitCode int
	// Error contains any execution error.
	Error error
	// Duration is how long the command took to execute.
	Duration time.Duration
}

// executionState represents the current state of execution.
type executionState int

const (
	stateExecuting executionState = iota
	stateCompleted
)

// outputMsg is sent when new output is available from the PTY.
type outputMsg struct {
	content string
}

// doneMsg is sent when command execution completes.
type doneMsg struct {
	result InteractiveResult
}

// interactiveModel is the Bubbletea model for interactive execution.
type interactiveModel struct {
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
}

func newInteractiveModel(opts InteractiveOptions, pty xpty.Pty) *interactiveModel {
	return &interactiveModel{
		title:   opts.Title,
		cmdName: opts.CommandName,
		state:   stateExecuting,
		pty:     pty,
	}
}

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
		// Return immediately after handling done to ensure the view updates
		return m, nil
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

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
	}

	return m, nil
}

func (m *interactiveModel) forwardKeyToPty(msg tea.KeyMsg) {
	// Convert the key message to bytes and write to PTY
	var data []byte

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

func (m *interactiveModel) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
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
		m.viewport.SetContent(m.content.String())
	} else {
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight
	}

	// Resize the PTY to match
	if m.pty != nil && m.state == stateExecuting {
		_ = m.pty.Resize(msg.Width, viewportHeight)
	}

	return m, nil
}

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

	return fmt.Sprintf("%s\n%s\n%s\n%s", title, separator, m.viewport.View(), footer)
}

// RunInteractiveCmd executes a command in interactive mode with alternate screen buffer.
// It creates a PTY for the command, forwards keyboard input during execution,
// and allows output review after completion.
func RunInteractiveCmd(ctx context.Context, opts InteractiveOptions, cmd *exec.Cmd) (*InteractiveResult, error) {
	// Get terminal size for initial PTY dimensions
	width, height := 80, 24
	if w, h, err := getTerminalSize(); err == nil {
		width, height = w, h
	}

	// Create a PTY using xpty (cross-platform)
	pty, err := xpty.NewPty(width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to create PTY: %w", err)
	}
	defer pty.Close()

	// Start the command on the PTY
	if err := pty.Start(cmd); err != nil {
		return nil, fmt.Errorf("failed to start command on PTY: %w", err)
	}

	// Create the model
	m := newInteractiveModel(opts, pty)

	// Create the Bubbletea program with alternate screen
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Read PTY output in a goroutine and send to the program
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pty.Read(buf)
			if n > 0 {
				p.Send(outputMsg{content: string(buf[:n])})
			}
			if err != nil {
				if err != io.EOF {
					// Log error but don't crash
				}
				break
			}
		}
	}()

	// Wait for the command to complete in a goroutine
	go func() {
		startTime := time.Now()
		err := xpty.WaitProcess(ctx, cmd)
		duration := time.Since(startTime)

		result := InteractiveResult{
			Duration: duration,
		}

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.Error = err
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

// InteractiveBuilder provides a fluent API for building interactive execution.
type InteractiveBuilder struct {
	opts InteractiveOptions
	cmd  *exec.Cmd
	ctx  context.Context
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
