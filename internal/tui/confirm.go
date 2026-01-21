// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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
		form      *huh.Form
		result    *bool
		done      bool
		cancelled bool
		width     int
		height    int
	}

	// ConfirmBuilder provides a fluent API for building Confirm prompts.
	ConfirmBuilder struct {
		opts ConfirmOptions
	}
)

// NewConfirmModel creates an embeddable confirm component.
func NewConfirmModel(opts ConfirmOptions) *confirmModel {
	return newConfirmModelWithTheme(opts, getHuhTheme(opts.Config.Theme))
}

// NewConfirmModelForModal creates an embeddable confirm component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewConfirmModelForModal(opts ConfirmOptions) *confirmModel {
	return newConfirmModelWithTheme(opts, getModalHuhTheme())
}

// Init implements tea.Model.
func (m *confirmModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update implements tea.Model.
func (m *confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle cancel keys before passing to form
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, "esc":
			m.done = true
			m.cancelled = true
			return m, nil
		}
	}

	// Pass to huh form
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	// Check if form is complete
	switch m.form.State {
	case huh.StateCompleted:
		m.done = true
	case huh.StateAborted:
		m.done = true
		m.cancelled = true
	case huh.StateNormal:
		// Form still in progress
	}

	return m, cmd
}

// View implements tea.Model.
func (m *confirmModel) View() string {
	if m.done {
		return ""
	}
	// Constrain the form view to the configured width to prevent overflow
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(m.form.View())
	}
	return m.form.View()
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
func (m *confirmModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.form = m.form.WithWidth(width).WithHeight(height)
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

// newConfirmModelWithTheme creates a confirm model with a specific huh theme.
func newConfirmModelWithTheme(opts ConfirmOptions, theme *huh.Theme) *confirmModel {
	result := opts.Default

	confirm := huh.NewConfirm().
		Title(opts.Title).
		Description(opts.Description).
		Value(&result)

	if opts.Affirmative != "" {
		confirm = confirm.Affirmative(opts.Affirmative)
	}

	if opts.Negative != "" {
		confirm = confirm.Negative(opts.Negative)
	}

	form := huh.NewForm(huh.NewGroup(confirm)).
		WithTheme(theme).
		WithAccessible(opts.Config.Accessible)

	return &confirmModel{
		form:   form,
		result: &result,
	}
}
