// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/types"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// All type declarations in a single block for decorder compliance.
type (
	// FileOptions configures the File picker component.
	FileOptions struct {
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
		Height TerminalDimension
		// FileAllowed enables file selection.
		FileAllowed bool
		// DirAllowed enables directory selection.
		DirAllowed bool
		// Config holds common TUI configuration.
		Config Config
	}

	// fileModel implements EmbeddableComponent for file picker.
	fileModel struct {
		picker      filepicker.Model
		result      *string
		done        bool
		cancelled   bool
		width       TerminalDimension
		height      TerminalDimension
		title       types.DescriptionText
		description types.DescriptionText
		forModal    bool
	}

	// FileBuilder provides a fluent API for building File picker prompts.
	FileBuilder struct {
		opts FileOptions
	}
)

// NewFileModel creates an embeddable file picker component.
func NewFileModel(opts FileOptions) *fileModel {
	return newFileModel(opts, false)
}

// NewFileModelForModal creates an embeddable file picker component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewFileModelForModal(opts FileOptions) *fileModel {
	return newFileModel(opts, true)
}

// Init implements tea.Model.
func (m *fileModel) Init() tea.Cmd {
	return m.picker.Init()
}

// Update implements tea.Model.
func (m *fileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		*m.result = path
		m.done = true
		return m, tea.Quit
	}
	_, _ = m.picker.DidSelectDisabledFile(msg)

	return m, cmd
}

// View implements tea.Model.
func (m *fileModel) View() tea.View {
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
		m.picker.View(),
		helpStyle.Render("enter select • esc cancel"),
	)

	view := strings.Join(lines, "\n")
	if m.width > 0 {
		view = lipgloss.NewStyle().MaxWidth(int(m.width)).Render(view)
	}

	return tea.NewView(view)
}

// IsDone implements EmbeddableComponent.
func (m *fileModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns ErrCancelled if the user cancelled the operation.
func (m *fileModel) Result() (any, error) {
	if m.cancelled {
		return nil, ErrCancelled
	}
	return *m.result, nil
}

// Cancelled implements EmbeddableComponent.
func (m *fileModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *fileModel) SetSize(width, height TerminalDimension) {
	m.width = width
	m.height = height
	if height > 0 {
		m.picker.AutoHeight = false
		m.picker.SetHeight(int(height))
	}
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
	result, _ := m.Result() //nolint:errcheck // Result() cannot fail after successful Run()
	return result.(string), nil
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
func (b *FileBuilder) Height(height TerminalDimension) *FileBuilder {
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

// newFileModel creates a file picker model with component-local styles.
func newFileModel(opts FileOptions, forModal bool) *fileModel {
	var result string

	picker := filepicker.New()
	picker.CurrentDirectory = "."
	if opts.CurrentDirectory != "" {
		picker.CurrentDirectory = opts.CurrentDirectory
	}
	if len(opts.AllowedExtensions) > 0 {
		picker.AllowedTypes = opts.AllowedExtensions
	}
	picker.ShowHidden = opts.ShowHidden
	picker.ShowSize = opts.ShowSize
	picker.ShowPermissions = opts.ShowPermissions

	// Default to allowing files if neither is specified.
	fileAllowed := opts.FileAllowed
	dirAllowed := opts.DirAllowed
	if !fileAllowed && !dirAllowed {
		fileAllowed = true
	}
	picker.FileAllowed = fileAllowed
	picker.DirAllowed = dirAllowed
	picker.Styles = newFilePickerStyles(forModal)

	if opts.Height > 0 {
		picker.AutoHeight = false
		picker.SetHeight(int(opts.Height))
	}

	return &fileModel{
		picker:      picker,
		result:      &result,
		title:       types.DescriptionText(opts.Title),       //goplint:ignore -- display text from TUI options
		description: types.DescriptionText(opts.Description), //goplint:ignore -- display text from TUI options
		forModal:    forModal,
	}
}

// newFilePickerStyles returns styles for file picker rendering.
func newFilePickerStyles(forModal bool) filepicker.Styles {
	styles := filepicker.DefaultStyles()
	if !forModal {
		return styles
	}

	base := modalBaseStyle()
	styles.Cursor = base.Foreground(lipgloss.Color("#7C3AED"))
	styles.DisabledCursor = base.Foreground(lipgloss.Color("#6B7280"))
	styles.Directory = base.Foreground(lipgloss.Color("#A78BFA"))
	styles.File = base.Foreground(lipgloss.Color("#FFFFFF"))
	styles.DisabledFile = base.Foreground(lipgloss.Color("#6B7280"))
	styles.Permission = base.Foreground(lipgloss.Color("#6B7280"))
	styles.Selected = base.Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	styles.DisabledSelected = base.Foreground(lipgloss.Color("#6B7280"))
	styles.FileSize = base.Foreground(lipgloss.Color("#6B7280")).Width(7).Align(lipgloss.Right)
	styles.EmptyDirectory = base.Foreground(lipgloss.Color("#6B7280"))
	return styles
}
