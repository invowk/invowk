// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	// interactiveRebuildDelay is the frame period that batches the rebuilds
	// of the completed view when output arrives after completion (design D8).
	interactiveRebuildDelay = 16 * time.Millisecond

	// leaveAltScreen returns a command that crashed in a full-screen program
	// to the main screen before the completed view is built (design D7).
	leaveAltScreen = "\x1b[?1049l"
)

func (m *interactiveModel) Init() tea.Cmd {
	return nil
}

// Update processes messages for the interactive model, which operates in three states:
//   - stateExecuting: the command runs; its output is applied to the emulated
//     screen shown in the command area, and keys are encoded for its terminal.
//   - stateTUI: modal overlay for a child TUI component (e.g., input, choose, confirm).
//     The overlay renders on top of the command's screen.
//   - stateCompleted: history, final screen and banner browsable with
//     vim-style navigation, exit on keypress.
//
// When a child TUI component completes, we intercept its tea.Quit via tuiComponentDoneMsg
// to prevent it from propagating and closing the parent program.
func (m *interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyMsg(msg)

	case tea.PasteMsg:
		// While the command runs the paste is its input; under a server
		// component overlay it goes to the component below.
		if m.state == stateExecuting {
			m.screen.paste(msg)
			return m, nil
		}

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case outputMsg:
		rebuild := m.applyOutput(msg)
		return m, rebuild

	case completedRebuildMsg:
		m.rebuildPending = false
		m.rebuildCompletedView(false)
		return m, nil

	case doneMsg:
		m.result = &msg.result
		m.state = stateCompleted
		if m.screen.emu.IsAltScreen() {
			m.writeScreen(outputMsg{data: []byte(leaveAltScreen)})
		}
		m.rebuildCompletedView(true)

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
		if doneCmd := m.componentDoneCmd(cmd); doneCmd != nil {
			cmds = append(cmds, doneCmd)
		}
	}

	// The viewport holds the completed view only; while the command runs,
	// keys go to the command and mouse events are not used.
	if m.ready && m.state == stateCompleted {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View implements tea.Model.
func (m *interactiveModel) View() tea.View {
	if !m.ready {
		view := tea.NewView("Initializing...")
		view.AltScreen = true
		return view
	}

	var b strings.Builder
	m.writeHeader(&b)
	b.WriteByte('\n')
	var cursor *tea.Cursor
	if m.state == stateCompleted {
		b.WriteString(m.viewport.View())
	} else {
		b.WriteString(m.screen.emu.Render())
		if m.state == stateExecuting && !m.screen.emu.CursorHidden() {
			pos := m.screen.emu.CursorPosition()
			cursor = tea.NewCursor(pos.X, pos.Y+interactiveHeaderRows)
		}
	}
	b.WriteByte('\n')
	m.writeFooter(&b)
	baseView := b.String()

	// If we're displaying a TUI component, render it as an overlay on top of the base view
	if m.state == stateTUI && m.activeComponent != nil {
		componentView := m.activeComponent.View()
		baseView = RenderOverlay(baseView, componentView.Content, m.width, m.height) //goplint:ignore -- bubbletea View() cannot return error
	}

	view := tea.NewView(baseView)
	view.AltScreen = true
	view.Cursor = cursor
	if m.state == stateCompleted {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

// writeHeader writes the title line and the separator.
func (m *interactiveModel) writeHeader(b *strings.Builder) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(modalColorPrimary)

	cmdStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3B82F6"))

	title := titleStyle.Render(m.title)
	if m.cmdName != "" {
		title += " " + cmdStyle.Render(m.cmdName)
	}

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#374151"))
	b.WriteString(title)
	b.WriteByte('\n')
	b.WriteString(separatorStyle.Render(strings.Repeat("-", int(m.width))))
}

// writeFooter writes the status line.
func (m *interactiveModel) writeFooter(b *strings.Builder) {
	muted := lipgloss.NewStyle().Foreground(modalColorMuted)
	if m.state != stateCompleted {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B")).
			Render("Running..."))
		b.WriteString(muted.Render("  |  Ctrl+\\: force quit"))
		return
	}

	if m.result != nil && m.result.ExitCode == 0 && m.result.Error == nil {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Render("Done"))
	} else {
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Render("Failed"))
	}
	b.WriteString(muted.Render("  |  arrows: scroll  |  Enter/q: exit"))
	b.WriteString(muted.Render(fmt.Sprintf("  %3.f%%", m.viewport.ScrollPercent()*100)))
}

// handleKeyMsg processes keyboard input based on current state.
func (m *interactiveModel) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateExecuting:
		// Emergency escape: force quit, never forwarded to the command.
		if isForceQuitKey(msg) {
			return m, tea.Quit
		}
		if event, ok := encodeInteractiveKey(msg); ok {
			m.screen.emu.SendKey(event)
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
		case "pgdown", "f", "space":
			m.viewport.HalfPageDown()
		case "home", "g":
			m.viewport.GotoTop()
		case "end", "G":
			m.viewport.GotoBottom()
		}

	case stateTUI:
		// When displaying a TUI component, delegate to the component
		if m.activeComponent != nil {
			updatedModel, cmd := m.activeComponent.Update(msg)
			if ec, ok := updatedModel.(EmbeddableComponent); ok {
				m.activeComponent = ec
			}
			doneCmd := m.componentDoneCmd(cmd)
			return m, doneCmd
		}
	}

	return m, nil
}

// handleTUIComponentRequest creates an embedded TUI component and switches to TUI state.
func (m *interactiveModel) handleTUIComponentRequest(msg TUIComponentMsg) (tea.Model, tea.Cmd) {
	if msg.Component == ComponentTypeWrite {
		return m.handleStyledTextRequest(msg)
	}

	// Calculate modal dimensions based on component type
	modalSize := CalculateModalSize(msg.Component, m.width, m.height) //goplint:ignore -- bubbletea Update() cannot return error

	// Create the embeddable component with modal dimensions
	component, err := CreateEmbeddableComponent(msg.Component, msg.Options, modalSize.Width, modalSize.Height)
	if err != nil {
		// Send error response back to the server
		go func() {
			msg.ResponseCh <- ComponentResponse{Err: fmt.Errorf("failed to create component: %w", err)}
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

func (m *interactiveModel) handleStyledTextRequest(msg TUIComponentMsg) (tea.Model, tea.Cmd) {
	opts, err := componentOptions[StyledTextOptions](msg.Options)
	if err != nil {
		go func() {
			msg.ResponseCh <- ComponentResponse{Err: fmt.Errorf("invalid write options: %w", err)}
		}()
		return m, nil
	}
	if err := opts.Validate(); err != nil {
		go func() {
			msg.ResponseCh <- ComponentResponse{Err: fmt.Errorf("invalid write options: %w", err)}
		}()
		return m, nil
	}

	style := opts.Style
	if style.Width == 0 && opts.Width > 0 {
		style.Width = opts.Width
	}
	rendered := style.Apply(opts.Text.String())
	if !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}
	// The text goes where the command's output is: onto its screen, with
	// each line starting at the left margin.
	rebuild := m.applyOutput(outputMsg{data: []byte(strings.ReplaceAll(rendered, "\n", "\r\n"))})
	go func() {
		msg.ResponseCh <- ComponentResponse{}
	}()
	return m, rebuild
}

// applyOutput writes the command's output to its screen and, past
// completion, schedules the rebuild of the completed view that shows it.
func (m *interactiveModel) applyOutput(out outputMsg) tea.Cmd {
	m.writeScreen(out)
	if m.state == stateCompleted {
		return m.scheduleCompletedRebuild()
	}
	return nil
}

// writeScreen applies output to the command's emulated screen. vt's Write
// fails only after Emulator.Close, which the viewer never calls (design D5).
func (m *interactiveModel) writeScreen(out outputMsg) {
	_, _ = m.screen.emu.Write(out.data) //nolint:errcheck // cannot fail: the emulator is never closed
}

// handleTUIComponentDone processes the result from a completed TUI component.
func (m *interactiveModel) handleTUIComponentDone(msg tuiComponentDoneMsg) (tea.Model, tea.Cmd) {
	if m.componentDoneCh == nil {
		return m, nil
	}

	response := ComponentResponse{}
	switch {
	case msg.cancelled:
		response.Cancelled = true
	case msg.err != nil:
		response.Err = msg.err
	default:
		response.Result = msg.result
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

// componentDoneCmd checks if the active component is done and returns either a
// tuiComponentDoneMsg command (to prevent tea.Quit from propagating) or the
// original cmd if the component is still active. Returns nil if neither applies.
func (m *interactiveModel) componentDoneCmd(cmd tea.Cmd) tea.Cmd {
	if m.activeComponent.IsDone() {
		// Component is done - send our own completion message
		// and ignore any command from the component (like tea.Quit)
		// to prevent the interactive program from quitting prematurely
		result, err := m.activeComponent.Result()
		cancelled := m.activeComponent.Cancelled()
		return func() tea.Msg {
			return tuiComponentDoneMsg{
				result:    result,
				err:       err,
				cancelled: cancelled,
			}
		}
	}
	if cmd != nil {
		return cmd
	}
	return nil
}

// handleWindowSize handles terminal resize events.
func (m *interactiveModel) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = validatedWindowDimensions(msg)
	cols, rows := InteractiveCommandArea(m.width, m.height)

	if !m.ready {
		m.viewport = viewport.New(
			viewport.WithWidth(int(cols)),
			viewport.WithHeight(int(rows)),
		)
		m.viewport.YPosition = interactiveHeaderRows
		m.ready = true
	} else {
		m.viewport.SetWidth(int(cols))
		m.viewport.SetHeight(int(rows))
	}

	// Resize the active TUI component if one is displayed
	if m.activeComponent != nil && m.activeComponentType != "" {
		modalSize := CalculateModalSize(m.activeComponentType, m.width, m.height) //goplint:ignore -- bubbletea Update() cannot return error
		m.activeComponent.SetSize(modalSize.Width, modalSize.Height)
	}

	// While the command runs (also under a server component overlay), its
	// screen and its terminal follow the command area together. After
	// completion only the completed view is laid out again.
	if m.state == stateCompleted {
		m.rebuildCompletedView(false)
		return m, nil
	}
	m.screen.emu.Resize(max(int(cols), 1), int(rows))
	if m.terminal != nil {
		_ = m.terminal.Resize(int(cols), int(rows)) //nolint:errcheck // Best-effort resize
	}
	return m, nil
}
