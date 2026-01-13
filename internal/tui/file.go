// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// FileOptions configures the File picker component.
type FileOptions struct {
	// Title is the title/prompt displayed above the file picker.
	Title string
	// Description provides additional context below the title.
	Description string
	// CurrentDirectory is the starting directory (default: current working directory).
	CurrentDirectory string
	// AllowedExtensions limits selection to files with these extensions.
	AllowedExtensions []string
	// ShowHidden enables showing hidden files.
	ShowHidden bool
	// ShowSize enables showing file sizes.
	ShowSize bool
	// ShowPermissions enables showing file permissions.
	ShowPermissions bool
	// Height limits the visible height (0 for auto).
	Height int
	// FileAllowed enables file selection.
	FileAllowed bool
	// DirAllowed enables directory selection.
	DirAllowed bool
	// Config holds common TUI configuration.
	Config Config
}

// fileModel implements EmbeddableComponent for file picker.
type fileModel struct {
	form      *huh.Form
	result    *string
	done      bool
	cancelled bool
	width     int
	height    int
}

// NewFileModel creates an embeddable file picker component.
func NewFileModel(opts FileOptions) *fileModel {
	return newFileModelWithTheme(opts, getHuhTheme(opts.Config.Theme))
}

// NewFileModelForModal creates an embeddable file picker component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewFileModelForModal(opts FileOptions) *fileModel {
	return newFileModelWithTheme(opts, getModalHuhTheme())
}

// newFileModelWithTheme creates a file picker model with a specific huh theme.
func newFileModelWithTheme(opts FileOptions, theme *huh.Theme) *fileModel {
	var result string

	picker := huh.NewFilePicker().
		Title(opts.Title).
		Description(opts.Description).
		Value(&result)

	if opts.CurrentDirectory != "" {
		picker = picker.CurrentDirectory(opts.CurrentDirectory)
	}

	if len(opts.AllowedExtensions) > 0 {
		picker = picker.AllowedTypes(opts.AllowedExtensions)
	}

	if opts.ShowHidden {
		picker = picker.ShowHidden(true)
	}

	if opts.ShowSize {
		picker = picker.ShowSize(true)
	}

	if opts.ShowPermissions {
		picker = picker.ShowPermissions(true)
	}

	if opts.Height > 0 {
		picker = picker.Height(opts.Height)
	}

	// Default to allowing files if neither is specified
	fileAllowed := opts.FileAllowed
	dirAllowed := opts.DirAllowed
	if !fileAllowed && !dirAllowed {
		fileAllowed = true
	}

	picker = picker.FileAllowed(fileAllowed)
	picker = picker.DirAllowed(dirAllowed)

	form := huh.NewForm(huh.NewGroup(picker)).
		WithTheme(theme).
		WithAccessible(opts.Config.Accessible)

	return &fileModel{
		form:   form,
		result: &result,
	}
}

// Init implements tea.Model.
func (m *fileModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update implements tea.Model.
func (m *fileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	if m.form.State == huh.StateCompleted {
		m.done = true
	} else if m.form.State == huh.StateAborted {
		m.done = true
		m.cancelled = true
	}

	return m, cmd
}

// View implements tea.Model.
func (m *fileModel) View() string {
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
func (m *fileModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
func (m *fileModel) Result() (interface{}, error) {
	if m.cancelled {
		return nil, nil
	}
	return *m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *fileModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *fileModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.form = m.form.WithWidth(width).WithHeight(height)
}

// File prompts the user to select a file from the filesystem.
// Returns the selected file path or an error if the prompt was cancelled.
func File(opts FileOptions) (string, error) {
	model := NewFileModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	m := finalModel.(*fileModel)
	if m.cancelled {
		return "", fmt.Errorf("user aborted")
	}
	result, _ := m.Result()
	return result.(string), nil
}

// FileBuilder provides a fluent API for building File picker prompts.
type FileBuilder struct {
	opts FileOptions
}

// NewFile creates a new FileBuilder with default options.
func NewFile() *FileBuilder {
	return &FileBuilder{
		opts: FileOptions{
			FileAllowed: true,
			DirAllowed:  false,
			Config:      DefaultConfig(),
		},
	}
}

// Title sets the title of the file picker.
func (b *FileBuilder) Title(title string) *FileBuilder {
	b.opts.Title = title
	return b
}

// Description sets the description of the file picker.
func (b *FileBuilder) Description(desc string) *FileBuilder {
	b.opts.Description = desc
	return b
}

// CurrentDirectory sets the starting directory.
func (b *FileBuilder) CurrentDirectory(dir string) *FileBuilder {
	b.opts.CurrentDirectory = dir
	return b
}

// AllowedExtensions limits selection to files with these extensions.
func (b *FileBuilder) AllowedExtensions(exts ...string) *FileBuilder {
	b.opts.AllowedExtensions = exts
	return b
}

// ShowHidden enables showing hidden files.
func (b *FileBuilder) ShowHidden(show bool) *FileBuilder {
	b.opts.ShowHidden = show
	return b
}

// ShowSize enables showing file sizes.
func (b *FileBuilder) ShowSize(show bool) *FileBuilder {
	b.opts.ShowSize = show
	return b
}

// ShowPermissions enables showing file permissions.
func (b *FileBuilder) ShowPermissions(show bool) *FileBuilder {
	b.opts.ShowPermissions = show
	return b
}

// Height sets the visible height.
func (b *FileBuilder) Height(height int) *FileBuilder {
	b.opts.Height = height
	return b
}

// FileAllowed enables file selection.
func (b *FileBuilder) FileAllowed(allowed bool) *FileBuilder {
	b.opts.FileAllowed = allowed
	return b
}

// DirAllowed enables directory selection.
func (b *FileBuilder) DirAllowed(allowed bool) *FileBuilder {
	b.opts.DirAllowed = allowed
	return b
}

// Theme sets the visual theme.
func (b *FileBuilder) Theme(theme Theme) *FileBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *FileBuilder) Accessible(accessible bool) *FileBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the file picker and returns the selected path.
func (b *FileBuilder) Run() (string, error) {
	return File(b.opts)
}

// Model returns the embeddable model for composition.
func (b *FileBuilder) Model() EmbeddableComponent {
	return NewFileModel(b.opts)
}
