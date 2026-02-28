// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type (
	// InputOptions configures the Input component.
	InputOptions struct {
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
		Width TerminalDimension
		// Password hides the input characters.
		Password bool
		// Prompt is the character(s) shown before the input (default: "> ").
		Prompt string
		// Config holds common TUI configuration.
		Config Config
	}

	// inputModel implements EmbeddableComponent for text input.
	inputModel struct {
		input       textinput.Model
		result      *string
		done        bool
		cancelled   bool
		width       TerminalDimension
		height      TerminalDimension
		title       types.DescriptionText
		description types.DescriptionText
		forModal    bool
	}

	// InputBuilder provides a fluent API for building Input prompts.
	InputBuilder struct {
		opts InputOptions
	}
)

// NewInputModel creates an embeddable input component.
func NewInputModel(opts InputOptions) *inputModel {
	return newInputModel(opts, false)
}

// NewInputModelForModal creates an embeddable input component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewInputModelForModal(opts InputOptions) *inputModel {
	return newInputModel(opts, true)
}

// Init implements tea.Model.
func (m *inputModel) Init() tea.Cmd {
	return m.input.Focus()
}

// Update implements tea.Model.
func (m *inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case keyCtrlC, "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			*m.result = m.input.Value()
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		if m.width == 0 {
			m.input.SetWidth(max(1, msg.Width))
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *inputModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	var base lipgloss.Style
	if m.forModal {
		base = modalBaseStyle()
	}

	titleStyle := base.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	descStyle := base.Foreground(lipgloss.Color("#6B7280"))
	helpStyle := base.Foreground(lipgloss.Color("#6B7280"))

	lines := make([]string, 0, 4)
	if m.title != "" {
		lines = append(lines, titleStyle.Render(m.title.String()))
	}
	if m.description != "" {
		lines = append(lines, descStyle.Render(m.description.String()))
	}
	lines = append(lines,
		m.input.View(),
		helpStyle.Render("enter submit • esc cancel"),
	)

	view := strings.Join(lines, "\n")
	if m.width > 0 {
		view = lipgloss.NewStyle().MaxWidth(int(m.width)).Render(view)
	}

	return tea.NewView(view)
}

// IsDone implements EmbeddableComponent.
func (m *inputModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns ErrCancelled if the user cancelled the operation.
func (m *inputModel) Result() (any, error) {
	if m.cancelled {
		return nil, ErrCancelled
	}
	return *m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *inputModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *inputModel) SetSize(width, height TerminalDimension) {
	m.width = width
	m.height = height
	if width > 0 {
		m.input.SetWidth(int(width))
	}
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
	result, _ := m.Result() //nolint:errcheck // Result() cannot fail after successful Run()
	return result.(string), nil
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
func (b *InputBuilder) Width(width TerminalDimension) *InputBuilder {
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

// newInputModel creates an input model with component-local styles.
func newInputModel(opts InputOptions, forModal bool) *inputModel {
	var result string
	if opts.Value != "" {
		result = opts.Value
	}
	var configuredWidth TerminalDimension

	ti := textinput.New()
	ti.Placeholder = opts.Placeholder
	ti.SetValue(result)
	ti.SetVirtualCursor(true)
	if opts.CharLimit > 0 {
		ti.CharLimit = opts.CharLimit
	}
	if opts.Password {
		ti.EchoMode = textinput.EchoPassword
	}
	if opts.Prompt != "" {
		ti.Prompt = opts.Prompt
	}
	if opts.Width > 0 {
		configuredWidth = opts.Width
		ti.SetWidth(int(configuredWidth))
	} else if opts.Config.Width > 0 {
		configuredWidth = opts.Config.Width
		ti.SetWidth(int(configuredWidth))
	}
	ti.SetStyles(newInputStyles(opts.Config.Theme, forModal))

	return &inputModel{
		input:       ti,
		result:      &result,
		title:       types.DescriptionText(opts.Title),       //goplint:ignore -- display text from TUI options
		description: types.DescriptionText(opts.Description), //goplint:ignore -- display text from TUI options
		width:       configuredWidth,
		forModal:    forModal,
	}
}

// newInputStyles returns focused/blurred styles for input rendering.
func newInputStyles(theme Theme, forModal bool) textinput.Styles {
	styles := textinput.DefaultStyles(isDarkTheme(theme))
	if forModal {
		base := modalBaseStyle()
		styles.Focused.Prompt = base.Foreground(lipgloss.Color("#7C3AED"))
		styles.Focused.Text = base.Foreground(lipgloss.Color("#FFFFFF"))
		styles.Focused.Placeholder = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Focused.Suggestion = base.Foreground(lipgloss.Color("#A78BFA"))
		styles.Blurred.Prompt = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Blurred.Text = base.Foreground(lipgloss.Color("#9CA3AF"))
		styles.Blurred.Placeholder = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Blurred.Suggestion = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Cursor.Color = lipgloss.Color("#FFFFFF")
		return styles
	}

	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styles.Focused.Suggestion = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styles.Blurred.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styles.Cursor.Color = lipgloss.Color("212")
	return styles
}
