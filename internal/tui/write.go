// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
		textarea    textarea.Model
		result      *string
		done        bool
		cancelled   bool
		width       TerminalDimension
		height      TerminalDimension
		title       types.DescriptionText
		description types.DescriptionText
		forModal    bool
	}

	// WriteBuilder provides a fluent API for building Write prompts.
	WriteBuilder struct {
		opts WriteOptions
	}
)

// NewWriteModel creates an embeddable text area component.
func NewWriteModel(opts WriteOptions) *writeModel {
	return newWriteModel(opts, false)
}

// NewWriteModelForModal creates an embeddable text area component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewWriteModelForModal(opts WriteOptions) *writeModel {
	return newWriteModel(opts, true)
}

// Init implements tea.Model.
func (m *writeModel) Init() tea.Cmd {
	return m.textarea.Focus()
}

// Update implements tea.Model.
func (m *writeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case keyCtrlC, "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		case "ctrl+d":
			*m.result = m.textarea.Value()
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		if m.width == 0 {
			m.textarea.SetWidth(max(1, msg.Width))
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *writeModel) View() tea.View {
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
		m.textarea.View(),
		helpStyle.Render("ctrl+d submit • esc cancel"),
	)

	view := strings.Join(lines, "\n")
	if m.width > 0 {
		view = lipgloss.NewStyle().MaxWidth(int(m.width)).Render(view)
	}

	return tea.NewView(view)
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
func (m *writeModel) SetSize(width, height TerminalDimension) {
	m.width = width
	m.height = height
	if width > 0 {
		m.textarea.SetWidth(int(width))
	}
	if height > 0 {
		m.textarea.SetHeight(int(height))
	}
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

// newWriteModel creates a text area model with component-local styles.
func newWriteModel(opts WriteOptions, forModal bool) *writeModel {
	var result string
	if opts.Value != "" {
		result = opts.Value
	}
	var configuredWidth TerminalDimension

	ta := textarea.New()
	ta.SetVirtualCursor(true)
	ta.Placeholder = opts.Placeholder
	ta.SetValue(result)
	if opts.CharLimit > 0 {
		ta.CharLimit = opts.CharLimit
	}
	ta.ShowLineNumbers = opts.ShowLineNumbers

	if opts.Width > 0 {
		configuredWidth = opts.Width
		ta.SetWidth(int(configuredWidth))
	} else if opts.Config.Width > 0 {
		configuredWidth = opts.Config.Width
		ta.SetWidth(int(configuredWidth))
	}
	if opts.Height > 0 {
		ta.SetHeight(int(opts.Height))
	}
	ta.SetStyles(newWriteStyles(opts.Config.Theme, forModal))

	return &writeModel{
		textarea:    ta,
		result:      &result,
		title:       types.DescriptionText(opts.Title),       //goplint:ignore -- display text from TUI options
		description: types.DescriptionText(opts.Description), //goplint:ignore -- display text from TUI options
		width:       configuredWidth,
		forModal:    forModal,
	}
}

// newWriteStyles returns focused/blurred styles for write rendering.
func newWriteStyles(theme Theme, forModal bool) textarea.Styles {
	styles := textarea.DefaultStyles(isDarkTheme(theme))
	if forModal {
		base := modalBaseStyle()
		styles.Focused.Base = base
		styles.Focused.Text = base.Foreground(lipgloss.Color("#FFFFFF"))
		styles.Focused.Placeholder = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Focused.CursorLine = base
		styles.Focused.CursorLineNumber = base.Foreground(lipgloss.Color("#A78BFA"))
		styles.Focused.LineNumber = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Focused.Prompt = base.Foreground(lipgloss.Color("#7C3AED"))
		styles.Focused.EndOfBuffer = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Cursor.Color = lipgloss.Color("#FFFFFF")
		styles.Blurred.Base = base
		styles.Blurred.Text = base.Foreground(lipgloss.Color("#9CA3AF"))
		styles.Blurred.Placeholder = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Blurred.CursorLine = base
		styles.Blurred.CursorLineNumber = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Blurred.LineNumber = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Blurred.Prompt = base.Foreground(lipgloss.Color("#6B7280"))
		styles.Blurred.EndOfBuffer = base.Foreground(lipgloss.Color("#6B7280"))
		return styles
	}

	styles.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	styles.Cursor.Color = lipgloss.Color("212")
	return styles
}
