// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"github.com/charmbracelet/huh"
)

// ConfirmOptions configures the Confirm component.
type ConfirmOptions struct {
	// Title is the question/prompt to display.
	Title string
	// Description provides additional context below the title.
	Description string
	// Affirmative is the text for the affirmative option (default: "Yes").
	Affirmative string
	// Negative is the text for the negative option (default: "No").
	Negative string
	// Default is the default value (true for yes, false for no).
	Default bool
	// Config holds common TUI configuration.
	Config Config
}

// Confirm prompts the user to confirm an action (yes/no).
// Returns true for affirmative, false for negative, or an error if cancelled.
func Confirm(opts ConfirmOptions) (bool, error) {
	result := opts.Default

	confirm := huh.NewConfirm().
		Title(opts.Title).
		Description(opts.Description).
		Value(&result)

	if opts.Affirmative != "" {
		confirm = confirm.Affirmative(opts.Affirmative)
	}

	if opts.Negative != "" {
		confirm = confirm.Negative(opts.Negative)
	}

	form := huh.NewForm(huh.NewGroup(confirm)).
		WithTheme(getHuhTheme(opts.Config.Theme)).
		WithAccessible(shouldUseAccessible(opts.Config))

	if err := form.Run(); err != nil {
		return false, err
	}

	return result, nil
}

// ConfirmBuilder provides a fluent API for building Confirm prompts.
type ConfirmBuilder struct {
	opts ConfirmOptions
}

// NewConfirm creates a new ConfirmBuilder with default options.
func NewConfirm() *ConfirmBuilder {
	return &ConfirmBuilder{
		opts: ConfirmOptions{
			Affirmative: "Yes",
			Negative:    "No",
			Default:     true,
			Config:      DefaultConfig(),
		},
	}
}

// Title sets the title/question of the confirm prompt.
func (b *ConfirmBuilder) Title(title string) *ConfirmBuilder {
	b.opts.Title = title
	return b
}

// Description sets the description of the confirm prompt.
func (b *ConfirmBuilder) Description(desc string) *ConfirmBuilder {
	b.opts.Description = desc
	return b
}

// Affirmative sets the text for the affirmative option.
func (b *ConfirmBuilder) Affirmative(text string) *ConfirmBuilder {
	b.opts.Affirmative = text
	return b
}

// Negative sets the text for the negative option.
func (b *ConfirmBuilder) Negative(text string) *ConfirmBuilder {
	b.opts.Negative = text
	return b
}

// Default sets the default value.
func (b *ConfirmBuilder) Default(value bool) *ConfirmBuilder {
	b.opts.Default = value
	return b
}

// Theme sets the visual theme.
func (b *ConfirmBuilder) Theme(theme Theme) *ConfirmBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *ConfirmBuilder) Accessible(accessible bool) *ConfirmBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the confirm prompt and returns the result.
func (b *ConfirmBuilder) Run() (bool, error) {
	return Confirm(b.opts)
}
