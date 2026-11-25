// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// scheduleCompletedRebuild asks for one rebuild of the completed view at the
// next frame, however many outputMsgs arrive before it (design D8).
func (m *interactiveModel) scheduleCompletedRebuild() tea.Cmd {
	if m.rebuildPending {
		return nil
	}
	m.rebuildPending = true
	return tea.Tick(interactiveRebuildDelay, func(time.Time) tea.Msg {
		return completedRebuildMsg{}
	})
}

// rebuildCompletedView sets the completed view's content: the history, the
// final main screen and the banner last (design D7). It keeps the user's
// scroll offset, or follows the bottom when the view was at the bottom or
// toBottom is set.
func (m *interactiveModel) rebuildCompletedView(toBottom bool) {
	if !m.ready || m.result == nil {
		return
	}
	atBottom := toBottom || m.viewport.AtBottom()
	offset := m.viewport.YOffset()
	var b strings.Builder
	m.writeCompletedContent(&b)
	m.viewport.SetContent(b.String())
	if atBottom {
		m.viewport.GotoBottom()
		return
	}
	m.viewport.SetYOffset(offset)
}

// writeCompletedContent writes the history lines with their styles, then
// the main screen without its trailing blank rows, then the completion banner.
func (m *interactiveModel) writeCompletedContent(b *strings.Builder) {
	for _, line := range m.screen.emu.Scrollback().Lines() {
		b.WriteString(line.Render())
		b.WriteByte('\n')
	}
	rows := strings.Split(m.screen.emu.Render(), "\n")
	plain := strings.Split(m.screen.emu.String(), "\n")
	used := 0
	for i, row := range plain {
		if strings.TrimSpace(row) != "" {
			used = i + 1
		}
	}
	for _, row := range rows[:min(used, len(rows))] {
		b.WriteString(row)
		b.WriteByte('\n')
	}
	m.appendCompletionMessage(b)
}

// appendCompletionMessage appends the completion status message to b.
func (m *interactiveModel) appendCompletionMessage(b *strings.Builder) {
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
			status = fmt.Sprintf("EXITED WITH CODE %d", int(m.result.ExitCode))
		}
	}

	separator := lipgloss.NewStyle().
		Foreground(modalColorMuted).
		Render(strings.Repeat("-", 50))

	durationStr := lipgloss.NewStyle().
		Foreground(modalColorMuted).
		Render(fmt.Sprintf("Duration: %s", m.result.Duration.Round(time.Millisecond)))

	b.WriteString("\n")
	b.WriteString(separator)
	b.WriteString("\n\n")
	b.WriteString(statusStyle.Render(status))
	b.WriteString("\n")
	b.WriteString(durationStr)
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(modalColorPrimarySoft).
		Italic(true).
		Render("Press Enter to return to terminal..."))
	b.WriteString("\n")
}
