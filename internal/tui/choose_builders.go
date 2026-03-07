// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"charm.land/bubbles/v2/list"
)

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
func (b *ChooseBuilder[T]) Height(height TerminalDimension) *ChooseBuilder[T] {
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
func (b *MultiChooseBuilder[T]) Height(height TerminalDimension) *MultiChooseBuilder[T] {
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

// NewChooseString creates a new ChooseStringBuilder with default options.
func NewChooseString() *ChooseStringBuilder {
	return &ChooseStringBuilder{
		opts: ChooseStringOptions{
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title of the choose prompt.
func (b *ChooseStringBuilder) Title(title string) *ChooseStringBuilder {
	b.opts.Title = title
	return b
}

// Options sets the available string options.
func (b *ChooseStringBuilder) Options(options ...string) *ChooseStringBuilder {
	b.opts.Options = options
	return b
}

// Limit sets the selection limit (1 for single-select, >1 for multi-select).
func (b *ChooseStringBuilder) Limit(limit int) *ChooseStringBuilder {
	b.opts.Limit = limit
	return b
}

// NoLimit enables unlimited selections in multi-select mode.
func (b *ChooseStringBuilder) NoLimit() *ChooseStringBuilder {
	b.opts.NoLimit = true
	return b
}

// Height sets the visible height.
func (b *ChooseStringBuilder) Height(height TerminalDimension) *ChooseStringBuilder {
	b.opts.Height = height
	return b
}

// Theme sets the visual theme.
func (b *ChooseStringBuilder) Theme(theme Theme) *ChooseStringBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *ChooseStringBuilder) Accessible(accessible bool) *ChooseStringBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the choose prompt and returns the result.
func (b *ChooseStringBuilder) Run() ([]string, error) {
	return ChooseStringsWithModel(b.opts)
}

// Model returns the embeddable model for composition.
func (b *ChooseStringBuilder) Model() EmbeddableComponent {
	return NewChooseModel(b.opts)
}

// newSingleChooseModel creates a single-select choose model using bubbles/list.
func newSingleChooseModel(opts ChooseStringOptions, forModal bool) *chooseModel {
	var result string

	items := make([]list.Item, len(opts.Options))
	for i, opt := range opts.Options {
		items[i] = chooseItem{text: opt, index: SelectionIndex(i)}
	}

	height := int(opts.Height)
	if height == 0 {
		height = 10
	}
	width := 50

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)
	applyDelegateStyles(&delegate, forModal)

	l := list.New(items, delegate, width, height)
	l.Title = opts.Title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	applyListStyles(&l, forModal)

	return &chooseModel{
		list:      l,
		result:    &result,
		isMulti:   false,
		options:   opts.Options,
		selected:  map[int]bool{},
		limit:     1,
		width:     TerminalDimension(width),
		height:    TerminalDimension(height),
		noLimit:   false,
		cancelled: false,
	}
}

// newMultiChooseModelWithTheme creates a multi-select choose model using bubbles/list.
// This replaces huh.MultiSelect because huh doesn't provide visual feedback for toggles
// when embedded within invowk's modal overlay system. Following the proven pattern from
// filter.go, we use bubbles/list with a custom delegate for full rendering control.
func newMultiChooseModelWithTheme(opts ChooseStringOptions, forModal bool) *chooseModel {
	results := make([]string, 0, len(opts.Selected))

	// Create list items
	items := make([]list.Item, len(opts.Options))
	for i, opt := range opts.Options {
		items[i] = chooseItem{text: opt, index: SelectionIndex(i)}
	}

	height := int(opts.Height)
	if height == 0 {
		height = 10
	}

	width := 50

	// Create selection map first - the delegate will reference this via closure
	selected := make(map[int]bool)
	for _, idx := range opts.Selected {
		idxInt := int(idx)
		if idxInt < 0 || idxInt >= len(opts.Options) {
			continue
		}
		selected[idxInt] = true
	}
	for i, opt := range opts.Options {
		if selected[i] {
			results = append(results, opt)
		}
	}

	// Create custom delegate with a closure that checks the selection map.
	// This closure captures 'selected' by reference, so the delegate always
	// sees the current selection state when rendering.
	delegate := newMultiChooseDelegate(forModal, func(idx int) bool {
		return selected[idx]
	})

	// Create list
	l := list.New(items, delegate, width, height)
	l.Title = opts.Title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false) // Disable filtering for choose (use filter.go for that)
	l.SetShowHelp(false)
	applyListStyles(&l, forModal)

	return &chooseModel{
		list:        l,
		multiResult: &results,
		isMulti:     true,
		options:     opts.Options,
		selected:    selected,
		limit:       opts.Limit,
		noLimit:     opts.NoLimit,
		width:       TerminalDimension(width),
		height:      TerminalDimension(height),
	}
}
