// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// All type declarations consolidated in a single block.
type (
	// WriteOptions configures the Write component (multi-line text input).
	WriteOptions struct {
		// Title is the title/prompt displayed above the text area.
		Title string
		// Description provides additional context below the title.
		Description string
		// Placeholder is the placeholder text shown when input is empty.
		Placeholder string
		// Value is the initial value of the text area.
		Value string
		// CharLimit limits the number of characters (0 for no limit).
		CharLimit int
		// Width sets the width of the text area (0 for auto).
		Width TerminalDimension
		// Height sets the height of the text area in lines (0 for auto).
		Height TerminalDimension
		// ShowLineNumbers enables line number display.
		ShowLineNumbers bool
		// Config holds common TUI configuration.
		Config Config
	}

	// writeModel implements EmbeddableComponent for multi-line text input.
	writeModel struct {
		form      *huh.Form
		result    *string
		done      bool
		cancelled bool
		width     int
		height    int
	}

	// WriteBuilder provides a fluent API for building Write prompts.
	WriteBuilder struct {
		opts WriteOptions
	}
)

// NewWriteModel creates an embeddable text area component.
func NewWriteModel(opts WriteOptions) *writeModel {
	return newWriteModelWithTheme(opts, getHuhTheme(opts.Config.Theme))
}

// NewWriteModelForModal creates an embeddable text area component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewWriteModelForModal(opts WriteOptions) *writeModel {
	return newWriteModelWithTheme(opts, getModalHuhTheme())
}

// Init implements tea.Model.
func (m *writeModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update implements tea.Model.
func (m *writeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m *writeModel) View() string {
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
func (m *writeModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns ErrCancelled if the user cancelled the operation.
func (m *writeModel) Result() (any, error) {
	if m.cancelled {
		return nil, ErrCancelled
	}
	return *m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *writeModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *writeModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.form = m.form.WithWidth(width).WithHeight(height)
}

// Write prompts the user for multi-line text input.
// Returns the entered text or an error if the prompt was cancelled.
func Write(opts WriteOptions) (string, error) {
	model := NewWriteModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	m := finalModel.(*writeModel)
	if m.cancelled {
		return "", fmt.Errorf("user aborted")
	}
	result, _ := m.Result() //nolint:errcheck // Result() cannot fail after successful Run()
	return result.(string), nil
}

// NewWrite creates a new WriteBuilder with default options.
func NewWrite() *WriteBuilder {
	return &WriteBuilder{
		opts: WriteOptions{
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title of the write prompt.
func (b *WriteBuilder) Title(title string) *WriteBuilder {
	b.opts.Title = title
	return b
}

// Description sets the description of the write prompt.
func (b *WriteBuilder) Description(desc string) *WriteBuilder {
	b.opts.Description = desc
	return b
}

// Placeholder sets the placeholder text.
func (b *WriteBuilder) Placeholder(placeholder string) *WriteBuilder {
	b.opts.Placeholder = placeholder
	return b
}

// Value sets the initial value.
func (b *WriteBuilder) Value(value string) *WriteBuilder {
	b.opts.Value = value
	return b
}

// CharLimit sets the character limit.
func (b *WriteBuilder) CharLimit(limit int) *WriteBuilder {
	b.opts.CharLimit = limit
	return b
}

// Width sets the width of the text area.
func (b *WriteBuilder) Width(width TerminalDimension) *WriteBuilder {
	b.opts.Width = width
	return b
}

// Height sets the height in lines.
func (b *WriteBuilder) Height(height TerminalDimension) *WriteBuilder {
	b.opts.Height = height
	return b
}

// ShowLineNumbers enables line number display.
func (b *WriteBuilder) ShowLineNumbers(show bool) *WriteBuilder {
	b.opts.ShowLineNumbers = show
	return b
}

// Theme sets the visual theme.
func (b *WriteBuilder) Theme(theme Theme) *WriteBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *WriteBuilder) Accessible(accessible bool) *WriteBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the write prompt and returns the result.
func (b *WriteBuilder) Run() (string, error) {
	return Write(b.opts)
}

// Model returns the embeddable model for composition.
func (b *WriteBuilder) Model() EmbeddableComponent {
	return NewWriteModel(b.opts)
}

// newWriteModelWithTheme creates a text area model with a specific huh theme.
func newWriteModelWithTheme(opts WriteOptions, theme *huh.Theme) *writeModel {
	var result string
	if opts.Value != "" {
		result = opts.Value
	}

	text := huh.NewText().
		Title(opts.Title).
		Description(opts.Description).
		Placeholder(opts.Placeholder).
		Value(&result)

	if opts.CharLimit > 0 {
		text = text.CharLimit(opts.CharLimit)
	}

	if opts.Height > 0 {
		text = text.Lines(int(opts.Height))
	}

	if opts.ShowLineNumbers {
		text = text.ShowLineNumbers(true)
	}

	form := huh.NewForm(huh.NewGroup(text)).
		WithTheme(theme).
		WithAccessible(opts.Config.Accessible)

	if opts.Width > 0 {
		form = form.WithWidth(int(opts.Width))
	} else if opts.Config.Width > 0 {
		form = form.WithWidth(int(opts.Config.Width))
	}

	return &writeModel{
		form:   form,
		result: &result,
	}
}
