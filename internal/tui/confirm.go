// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// All type declarations in a single block for decorder compliance.
type (
	// ConfirmOptions configures the Confirm component.
	ConfirmOptions struct {
		// Title is the question/prompt to display.
		Title string
		// Description provides additional context below the title.
		Description string
		// Affirmative is the text for the affirmative option (default: "Yes").
		Affirmative string
		// Negative is the text for the negative option (default: "No").
		Negative string
		// Default is the default value (true for yes, false for no).
		Default bool
		// Config holds common TUI configuration.
		Config Config
	}

	// confirmModel implements EmbeddableComponent for confirmation prompts.
	confirmModel struct {
		result      *bool
		done        bool
		cancelled   bool
		width       TerminalDimension
		height      TerminalDimension
		title       types.DescriptionText
		description types.DescriptionText
		affirmative types.DescriptionText
		negative    types.DescriptionText
		selection   bool
		forModal    bool
	}

	// ConfirmBuilder provides a fluent API for building Confirm prompts.
	ConfirmBuilder struct {
		opts ConfirmOptions
	}
)

// NewConfirmModel creates an embeddable confirm component.
func NewConfirmModel(opts ConfirmOptions) *confirmModel {
	return newConfirmModel(opts, false)
}

// NewConfirmModelForModal creates an embeddable confirm component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewConfirmModelForModal(opts ConfirmOptions) *confirmModel {
	return newConfirmModel(opts, true)
}

// Init implements tea.Model.
func (m *confirmModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case keyCtrlC, "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		case "y":
			m.selection = true
			*m.result = true
			m.done = true
			return m, tea.Quit
		case "n":
			m.selection = false
			*m.result = false
			m.done = true
			return m, tea.Quit
		case "left", "h":
			m.selection = true
		case "right", "l":
			m.selection = false
		case "up", "down", "tab", "shift+tab":
			m.selection = !m.selection
		case "enter", "space":
			*m.result = m.selection
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = TerminalDimension(msg.Width)
		m.height = TerminalDimension(msg.Height)
	}

	return m, nil
}

// View implements tea.Model.
func (m *confirmModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	var base lipgloss.Style
	if m.forModal {
		base = modalBaseStyle()
	}

	titleStyle := base.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	descStyle := base.Foreground(lipgloss.Color("#6B7280"))
	activeStyle := base.Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#7C3AED")).Bold(true).Padding(0, 1)
	inactiveStyle := base.Foreground(lipgloss.Color("#9CA3AF")).Padding(0, 1)
	helpStyle := base.Foreground(lipgloss.Color("#6B7280"))

	affirmative := m.affirmative.String()
	negative := m.negative.String()
	if affirmative == "" {
		affirmative = "Yes"
	}
	if negative == "" {
		negative = "No"
	}

	yesView := inactiveStyle.Render(affirmative)
	noView := inactiveStyle.Render(negative)
	if m.selection {
		yesView = activeStyle.Render(affirmative)
	} else {
		noView = activeStyle.Render(negative)
	}

	lines := make([]string, 0, 4)
	if m.title != "" {
		lines = append(lines, titleStyle.Render(m.title.String()))
	}
	if m.description != "" {
		lines = append(lines, descStyle.Render(m.description.String()))
	}
	lines = append(lines,
		yesView+"  "+noView,
		helpStyle.Render("enter submit • y yes • n no • esc cancel"),
	)

	view := strings.Join(lines, "\n")
	if m.width > 0 {
		view = lipgloss.NewStyle().MaxWidth(int(m.width)).Render(view)
	}

	return tea.NewView(view)
}

// IsDone implements EmbeddableComponent.
func (m *confirmModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns ErrCancelled if the user cancelled the operation.
func (m *confirmModel) Result() (any, error) {
	if m.cancelled {
		return nil, ErrCancelled
	}
	return *m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *confirmModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *confirmModel) SetSize(width, height TerminalDimension) {
	m.width = width
	m.height = height
}

// Confirm prompts the user to confirm an action (yes/no).
// Returns true for affirmative, false for negative, or an error if cancelled.
func Confirm(opts ConfirmOptions) (bool, error) {
	model := NewConfirmModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}

	m := finalModel.(*confirmModel)
	if m.cancelled {
		return false, fmt.Errorf("user aborted")
	}
	result, _ := m.Result() //nolint:errcheck // Result() cannot fail after successful Run()
	return result.(bool), nil
}

// NewConfirm creates a new ConfirmBuilder with default options.
func NewConfirm() *ConfirmBuilder {
	return &ConfirmBuilder{
		opts: ConfirmOptions{
			Affirmative: "Yes",
			Negative:    "No",
			Default:     true,
			Config:      DefaultConfig(),
		},
	}
}

// Title sets the title/question of the confirm prompt.
func (b *ConfirmBuilder) Title(title string) *ConfirmBuilder {
	b.opts.Title = title
	return b
}

// Description sets the description of the confirm prompt.
func (b *ConfirmBuilder) Description(desc string) *ConfirmBuilder {
	b.opts.Description = desc
	return b
}

// Affirmative sets the text for the affirmative option.
func (b *ConfirmBuilder) Affirmative(text string) *ConfirmBuilder {
	b.opts.Affirmative = text
	return b
}

// Negative sets the text for the negative option.
func (b *ConfirmBuilder) Negative(text string) *ConfirmBuilder {
	b.opts.Negative = text
	return b
}

// Default sets the default value.
func (b *ConfirmBuilder) Default(value bool) *ConfirmBuilder {
	b.opts.Default = value
	return b
}

// Theme sets the visual theme.
func (b *ConfirmBuilder) Theme(theme Theme) *ConfirmBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *ConfirmBuilder) Accessible(accessible bool) *ConfirmBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the confirm prompt and returns the result.
func (b *ConfirmBuilder) Run() (bool, error) {
	return Confirm(b.opts)
}

// Model returns the embeddable model for composition.
func (b *ConfirmBuilder) Model() EmbeddableComponent {
	return NewConfirmModel(b.opts)
}

// newConfirmModel creates a confirm model configured for standalone or modal use.
func newConfirmModel(opts ConfirmOptions, forModal bool) *confirmModel {
	result := opts.Default

	return &confirmModel{
		result:      &result,
		title:       types.DescriptionText(opts.Title),       //goplint:ignore -- display text from TUI options
		description: types.DescriptionText(opts.Description), //goplint:ignore -- display text from TUI options
		affirmative: types.DescriptionText(opts.Affirmative), //goplint:ignore -- display text from TUI options
		negative:    types.DescriptionText(opts.Negative),    //goplint:ignore -- display text from TUI options
		selection:   opts.Default,
		forModal:    forModal,
	}
}
