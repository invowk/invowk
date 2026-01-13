// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"github.com/charmbracelet/huh"
)

// Option represents a selectable option with a display title and value.
type Option[T comparable] struct {
	// Title is the display text for the option.
	Title string
	// Value is the underlying value of the option.
	Value T
	// Selected indicates if this option is pre-selected (for multi-select).
	Selected bool
}

// ChooseOptions configures the Choose component.
type ChooseOptions[T comparable] struct {
	// Title is the title/prompt displayed above the options.
	Title string
	// Description provides additional context below the title.
	Description string
	// Options is the list of options to choose from.
	Options []Option[T]
	// Height limits the number of visible options (0 for auto).
	Height int
	// Cursor is the character used for the cursor (default: "> ").
	Cursor string
	// Config holds common TUI configuration.
	Config Config
}

// Choose prompts the user to select one option from a list.
// Returns the selected value or an error if the prompt was cancelled.
func Choose[T comparable](opts ChooseOptions[T]) (T, error) {
	var result T

	huhOpts := make([]huh.Option[T], len(opts.Options))
	for i, opt := range opts.Options {
		huhOpts[i] = huh.NewOption(opt.Title, opt.Value)
	}

	sel := huh.NewSelect[T]().
		Title(opts.Title).
		Description(opts.Description).
		Options(huhOpts...).
		Value(&result)

	if opts.Height > 0 {
		sel = sel.Height(opts.Height)
	}

	form := huh.NewForm(huh.NewGroup(sel)).
		WithTheme(getHuhTheme(opts.Config.Theme)).
		WithAccessible(shouldUseAccessible(opts.Config))

	// Set output writer (stderr when nested to avoid $() capture)
	form = form.WithOutput(getOutputWriter(opts.Config))

	if err := form.Run(); err != nil {
		return result, err
	}

	return result, nil
}

// ChooseStrings is a convenience function for choosing from string options.
// The option titles and values are the same.
func ChooseStrings(title string, options []string, config Config) (string, error) {
	opts := make([]Option[string], len(options))
	for i, o := range options {
		opts[i] = Option[string]{Title: o, Value: o}
	}
	return Choose(ChooseOptions[string]{
		Title:   title,
		Options: opts,
		Config:  config,
	})
}

// MultiChooseOptions configures the MultiChoose component.
type MultiChooseOptions[T comparable] struct {
	// Title is the title/prompt displayed above the options.
	Title string
	// Description provides additional context below the title.
	Description string
	// Options is the list of options to choose from.
	Options []Option[T]
	// Limit is the maximum number of selections (0 for no limit).
	Limit int
	// Height limits the number of visible options (0 for auto).
	Height int
	// Config holds common TUI configuration.
	Config Config
}

// MultiChoose prompts the user to select multiple options from a list.
// Returns the selected values or an error if the prompt was cancelled.
func MultiChoose[T comparable](opts MultiChooseOptions[T]) ([]T, error) {
	var result []T

	huhOpts := make([]huh.Option[T], len(opts.Options))
	for i, opt := range opts.Options {
		o := huh.NewOption(opt.Title, opt.Value)
		if opt.Selected {
			o = o.Selected(true)
		}
		huhOpts[i] = o
	}

	sel := huh.NewMultiSelect[T]().
		Title(opts.Title).
		Description(opts.Description).
		Options(huhOpts...).
		Value(&result)

	if opts.Limit > 0 {
		sel = sel.Limit(opts.Limit)
	}

	if opts.Height > 0 {
		sel = sel.Height(opts.Height)
	}

	form := huh.NewForm(huh.NewGroup(sel)).
		WithTheme(getHuhTheme(opts.Config.Theme)).
		WithAccessible(shouldUseAccessible(opts.Config))

	// Set output writer (stderr when nested to avoid $() capture)
	form = form.WithOutput(getOutputWriter(opts.Config))

	if err := form.Run(); err != nil {
		return nil, err
	}

	return result, nil
}

// MultiChooseStrings is a convenience function for choosing multiple string options.
func MultiChooseStrings(title string, options []string, limit int, config Config) ([]string, error) {
	opts := make([]Option[string], len(options))
	for i, o := range options {
		opts[i] = Option[string]{Title: o, Value: o}
	}
	return MultiChoose(MultiChooseOptions[string]{
		Title:   title,
		Options: opts,
		Limit:   limit,
		Config:  config,
	})
}

// ChooseBuilder provides a fluent API for building Choose prompts.
type ChooseBuilder[T comparable] struct {
	opts ChooseOptions[T]
}

// NewChoose creates a new ChooseBuilder with default options.
func NewChoose[T comparable]() *ChooseBuilder[T] {
	return &ChooseBuilder[T]{
		opts: ChooseOptions[T]{
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title of the choose prompt.
func (b *ChooseBuilder[T]) Title(title string) *ChooseBuilder[T] {
	b.opts.Title = title
	return b
}

// Description sets the description of the choose prompt.
func (b *ChooseBuilder[T]) Description(desc string) *ChooseBuilder[T] {
	b.opts.Description = desc
	return b
}

// Options sets the available options.
func (b *ChooseBuilder[T]) Options(options ...Option[T]) *ChooseBuilder[T] {
	b.opts.Options = options
	return b
}

// OptionsFromSlice creates options from a slice where title equals value.
func (b *ChooseBuilder[T]) OptionsFromSlice(values []T, titleFunc func(T) string) *ChooseBuilder[T] {
	b.opts.Options = make([]Option[T], len(values))
	for i, v := range values {
		b.opts.Options[i] = Option[T]{Title: titleFunc(v), Value: v}
	}
	return b
}

// Height sets the visible height.
func (b *ChooseBuilder[T]) Height(height int) *ChooseBuilder[T] {
	b.opts.Height = height
	return b
}

// Cursor sets the cursor character.
func (b *ChooseBuilder[T]) Cursor(cursor string) *ChooseBuilder[T] {
	b.opts.Cursor = cursor
	return b
}

// Theme sets the visual theme.
func (b *ChooseBuilder[T]) Theme(theme Theme) *ChooseBuilder[T] {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *ChooseBuilder[T]) Accessible(accessible bool) *ChooseBuilder[T] {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the choose prompt and returns the result.
func (b *ChooseBuilder[T]) Run() (T, error) {
	return Choose(b.opts)
}

// MultiChooseBuilder provides a fluent API for building MultiChoose prompts.
type MultiChooseBuilder[T comparable] struct {
	opts MultiChooseOptions[T]
}

// NewMultiChoose creates a new MultiChooseBuilder with default options.
func NewMultiChoose[T comparable]() *MultiChooseBuilder[T] {
	return &MultiChooseBuilder[T]{
		opts: MultiChooseOptions[T]{
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title of the multi-choose prompt.
func (b *MultiChooseBuilder[T]) Title(title string) *MultiChooseBuilder[T] {
	b.opts.Title = title
	return b
}

// Description sets the description of the multi-choose prompt.
func (b *MultiChooseBuilder[T]) Description(desc string) *MultiChooseBuilder[T] {
	b.opts.Description = desc
	return b
}

// Options sets the available options.
func (b *MultiChooseBuilder[T]) Options(options ...Option[T]) *MultiChooseBuilder[T] {
	b.opts.Options = options
	return b
}

// Limit sets the maximum number of selections.
func (b *MultiChooseBuilder[T]) Limit(limit int) *MultiChooseBuilder[T] {
	b.opts.Limit = limit
	return b
}

// Height sets the visible height.
func (b *MultiChooseBuilder[T]) Height(height int) *MultiChooseBuilder[T] {
	b.opts.Height = height
	return b
}

// Theme sets the visual theme.
func (b *MultiChooseBuilder[T]) Theme(theme Theme) *MultiChooseBuilder[T] {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *MultiChooseBuilder[T]) Accessible(accessible bool) *MultiChooseBuilder[T] {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the multi-choose prompt and returns the results.
func (b *MultiChooseBuilder[T]) Run() ([]T, error) {
	return MultiChoose(b.opts)
}
