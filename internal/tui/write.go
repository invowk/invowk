// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"github.com/charmbracelet/huh"
)

// WriteOptions configures the Write component (multi-line text input).
type WriteOptions struct {
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
	Width int
	// Height sets the height of the text area in lines (0 for auto).
	Height int
	// ShowLineNumbers enables line number display.
	ShowLineNumbers bool
	// Config holds common TUI configuration.
	Config Config
}

// Write prompts the user for multi-line text input.
// Returns the entered text or an error if the prompt was cancelled.
func Write(opts WriteOptions) (string, error) {
	var result string

	text := huh.NewText().
		Title(opts.Title).
		Description(opts.Description).
		Placeholder(opts.Placeholder).
		Value(&result)

	if opts.Value != "" {
		result = opts.Value
	}

	if opts.CharLimit > 0 {
		text = text.CharLimit(opts.CharLimit)
	}

	if opts.Height > 0 {
		text = text.Lines(opts.Height)
	}

	if opts.ShowLineNumbers {
		text = text.ShowLineNumbers(true)
	}

	form := huh.NewForm(huh.NewGroup(text)).
		WithTheme(getHuhTheme(opts.Config.Theme)).
		WithAccessible(opts.Config.Accessible)

	if err := form.Run(); err != nil {
		return "", err
	}

	return result, nil
}

// WriteBuilder provides a fluent API for building Write prompts.
type WriteBuilder struct {
	opts WriteOptions
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
func (b *WriteBuilder) Width(width int) *WriteBuilder {
	b.opts.Width = width
	return b
}

// Height sets the height in lines.
func (b *WriteBuilder) Height(height int) *WriteBuilder {
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
