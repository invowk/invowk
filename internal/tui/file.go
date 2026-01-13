// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"github.com/charmbracelet/huh"
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

// File prompts the user to select a file from the filesystem.
// Returns the selected file path or an error if the prompt was cancelled.
func File(opts FileOptions) (string, error) {
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

	picker = picker.FileAllowed(opts.FileAllowed)
	picker = picker.DirAllowed(opts.DirAllowed)

	form := huh.NewForm(huh.NewGroup(picker)).
		WithTheme(getHuhTheme(opts.Config.Theme)).
		WithAccessible(shouldUseAccessible(opts.Config))

	// Set output writer (stderr when nested to avoid $() capture)
	form = form.WithOutput(getOutputWriter(opts.Config))

	if err := form.Run(); err != nil {
		return "", err
	}

	return result, nil
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
