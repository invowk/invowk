// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// InputOptions configures the Input component.
type InputOptions struct {
	// Title is the title/prompt displayed above the input.
	Title string
	// Description provides additional context below the title.
	Description string
	// Placeholder is the placeholder text shown when input is empty.
	Placeholder string
	// Value is the initial value of the input.
	Value string
	// CharLimit limits the number of characters (0 for no limit).
	CharLimit int
	// Width sets the width of the input field (0 for auto).
	Width int
	// Password hides the input characters.
	Password bool
	// Prompt is the character(s) shown before the input (default: "> ").
	Prompt string
	// Config holds common TUI configuration.
	Config Config
}

// inputModel implements EmbeddableComponent for text input.
type inputModel struct {
	form      *huh.Form
	result    *string
	done      bool
	cancelled bool
	width     int
	height    int
}

// NewInputModel creates an embeddable input component.
func NewInputModel(opts InputOptions) *inputModel {
	return newInputModelWithTheme(opts, getHuhTheme(opts.Config.Theme))
}

// NewInputModelForModal creates an embeddable input component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewInputModelForModal(opts InputOptions) *inputModel {
	return newInputModelWithTheme(opts, getModalHuhTheme())
}

// newInputModelWithTheme creates an input model with a specific huh theme.
func newInputModelWithTheme(opts InputOptions, theme *huh.Theme) *inputModel {
	var result string
	if opts.Value != "" {
		result = opts.Value
	}

	input := huh.NewInput().
		Title(opts.Title).
		Description(opts.Description).
		Placeholder(opts.Placeholder).
		Value(&result)

	if opts.CharLimit > 0 {
		input = input.CharLimit(opts.CharLimit)
	}

	if opts.Password {
		input = input.EchoMode(huh.EchoModePassword)
	}

	if opts.Prompt != "" {
		input = input.Prompt(opts.Prompt)
	}

	form := huh.NewForm(huh.NewGroup(input)).
		WithTheme(theme).
		WithAccessible(opts.Config.Accessible)

	if opts.Width > 0 {
		form = form.WithWidth(opts.Width)
	} else if opts.Config.Width > 0 {
		form = form.WithWidth(opts.Config.Width)
	}

	return &inputModel{
		form:   form,
		result: &result,
	}
}

// Init implements tea.Model.
func (m *inputModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update implements tea.Model.
func (m *inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle cancel keys before passing to form
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c", "esc":
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
	}

	return m, cmd
}

// View implements tea.Model.
func (m *inputModel) View() string {
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
func (m *inputModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
func (m *inputModel) Result() (interface{}, error) {
	if m.cancelled {
		return nil, nil
	}
	return *m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *inputModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *inputModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.form = m.form.WithWidth(width).WithHeight(height)
}

// Input prompts the user for a single line of text input.
// Returns the entered text or an error if the prompt was cancelled.
func Input(opts InputOptions) (string, error) {
	model := NewInputModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	m := finalModel.(*inputModel)
	if m.cancelled {
		return "", fmt.Errorf("user aborted")
	}
	result, _ := m.Result()
	return result.(string), nil
}

// InputBuilder provides a fluent API for building Input prompts.
type InputBuilder struct {
	opts InputOptions
}

// NewInput creates a new InputBuilder with default options.
func NewInput() *InputBuilder {
	return &InputBuilder{
		opts: InputOptions{
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title of the input prompt.
func (b *InputBuilder) Title(title string) *InputBuilder {
	b.opts.Title = title
	return b
}

// Description sets the description of the input prompt.
func (b *InputBuilder) Description(desc string) *InputBuilder {
	b.opts.Description = desc
	return b
}

// Placeholder sets the placeholder text.
func (b *InputBuilder) Placeholder(placeholder string) *InputBuilder {
	b.opts.Placeholder = placeholder
	return b
}

// Value sets the initial value.
func (b *InputBuilder) Value(value string) *InputBuilder {
	b.opts.Value = value
	return b
}

// CharLimit sets the character limit.
func (b *InputBuilder) CharLimit(limit int) *InputBuilder {
	b.opts.CharLimit = limit
	return b
}

// Width sets the width of the input field.
func (b *InputBuilder) Width(width int) *InputBuilder {
	b.opts.Width = width
	return b
}

// Password enables password mode (hidden input).
func (b *InputBuilder) Password() *InputBuilder {
	b.opts.Password = true
	return b
}

// Prompt sets the prompt character(s).
func (b *InputBuilder) Prompt(prompt string) *InputBuilder {
	b.opts.Prompt = prompt
	return b
}

// Theme sets the visual theme.
func (b *InputBuilder) Theme(theme Theme) *InputBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *InputBuilder) Accessible(accessible bool) *InputBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the input prompt and returns the result.
func (b *InputBuilder) Run() (string, error) {
	return Input(b.opts)
}

// Model returns the embeddable model for composition.
func (b *InputBuilder) Model() EmbeddableComponent {
	return NewInputModel(b.opts)
}
