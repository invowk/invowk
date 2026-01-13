// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"github.com/charmbracelet/huh"
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

// Input prompts the user for a single line of text input.
// Returns the entered text or an error if the prompt was cancelled.
func Input(opts InputOptions) (string, error) {
	var result string

	input := huh.NewInput().
		Title(opts.Title).
		Description(opts.Description).
		Placeholder(opts.Placeholder).
		Value(&result)

	if opts.Value != "" {
		result = opts.Value
	}

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
		WithTheme(getHuhTheme(opts.Config.Theme)).
		WithAccessible(opts.Config.Accessible)

	// Apply width at the form level (huh.Input doesn't expose width directly)
	if opts.Width > 0 {
		form = form.WithWidth(opts.Width)
	} else if opts.Config.Width > 0 {
		form = form.WithWidth(opts.Config.Width)
	}

	if err := form.Run(); err != nil {
		return "", err
	}

	return result, nil
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
