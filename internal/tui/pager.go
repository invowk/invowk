// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PagerOptions configures the Pager component.
type PagerOptions struct {
	// Content is the text content to display.
	Content string
	// Title is the title displayed at the top.
	Title string
	// Height limits the visible height (0 for auto).
	Height int
	// Width limits the visible width (0 for auto).
	Width int
	// ShowLineNumbers enables line number display.
	ShowLineNumbers bool
	// SoftWrap enables soft wrapping of long lines.
	SoftWrap bool
	// Config holds common TUI configuration.
	Config Config
}

// pagerModel is the bubbletea model for the pager component.
// It implements EmbeddableComponent for embedded use.
type pagerModel struct {
	viewport viewport.Model
	title    string
	ready    bool
	done     bool
	width    int
	height   int
}

// NewPagerModel creates an embeddable pager component.
func NewPagerModel(opts PagerOptions) *pagerModel {
	height := opts.Height
	if height == 0 {
		height = 20
	}

	width := opts.Width
	if width == 0 {
		width = 80
	}

	vpHeight := height - 4 // Leave room for title and footer
	if vpHeight < 1 {
		vpHeight = 10
	}

	vp := viewport.New(width, vpHeight)
	vp.SetContent(opts.Content)

	return &pagerModel{
		viewport: vp,
		title:    opts.Title,
		ready:    true,
		width:    width,
		height:   height,
	}
}

func (m *pagerModel) Init() tea.Cmd {
	return nil
}

func (m *pagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc", "enter":
			m.done = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2 // Leave room for title and footer
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 2
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *pagerModel) View() string {
	if m.done {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Padding(0, 1)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	title := ""
	if m.title != "" {
		title = titleStyle.Render(m.title) + "\n"
	}

	footer := footerStyle.Render("↑/↓: navigate • q/Enter: close")

	content := title + m.viewport.View() + "\n" + footer

	// Constrain the view to the configured width to prevent overflow in modal overlays
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(content)
	}
	return content
}

// IsDone implements EmbeddableComponent.
func (m *pagerModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Pager has no result value.
func (m *pagerModel) Result() (interface{}, error) {
	return nil, nil
}

// Cancelled implements EmbeddableComponent.
// Pager doesn't have a cancel concept - it's just dismissed.
func (m *pagerModel) Cancelled() bool {
	return false
}

// SetSize implements EmbeddableComponent.
func (m *pagerModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height - 4
}

// Pager displays content in a scrollable viewport.
func Pager(opts PagerOptions) error {
	model := NewPagerModel(opts)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// PagerBuilder provides a fluent API for building Pager displays.
type PagerBuilder struct {
	opts PagerOptions
}

// NewPager creates a new PagerBuilder with default options.
func NewPager() *PagerBuilder {
	return &PagerBuilder{
		opts: PagerOptions{
			Config: DefaultConfig(),
		},
	}
}

// Content sets the text content to display.
func (b *PagerBuilder) Content(content string) *PagerBuilder {
	b.opts.Content = content
	return b
}

// Title sets the title of the pager.
func (b *PagerBuilder) Title(title string) *PagerBuilder {
	b.opts.Title = title
	return b
}

// Height sets the visible height.
func (b *PagerBuilder) Height(height int) *PagerBuilder {
	b.opts.Height = height
	return b
}

// Width sets the visible width.
func (b *PagerBuilder) Width(width int) *PagerBuilder {
	b.opts.Width = width
	return b
}

// ShowLineNumbers enables line number display.
func (b *PagerBuilder) ShowLineNumbers(show bool) *PagerBuilder {
	b.opts.ShowLineNumbers = show
	return b
}

// SoftWrap enables soft wrapping.
func (b *PagerBuilder) SoftWrap(wrap bool) *PagerBuilder {
	b.opts.SoftWrap = wrap
	return b
}

// Theme sets the visual theme.
func (b *PagerBuilder) Theme(theme Theme) *PagerBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *PagerBuilder) Accessible(accessible bool) *PagerBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run displays the pager.
func (b *PagerBuilder) Run() error {
	return Pager(b.opts)
}

// Model returns the embeddable model for composition.
func (b *PagerBuilder) Model() EmbeddableComponent {
	return NewPagerModel(b.opts)
}
