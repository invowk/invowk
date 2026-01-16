// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ChooseStringOptions configures the embeddable Choose component for strings.
// This is used by the TUI server for dynamic component creation.
type ChooseStringOptions struct {
	// Title is the title/prompt displayed above the options.
	Title string
	// Options is the list of string options to choose from.
	Options []string
	// Limit is the maximum number of selections (0 or 1 for single-select, >1 for multi-select).
	Limit int
	// NoLimit allows unlimited selections in multi-select mode.
	NoLimit bool
	// Height limits the number of visible options (0 for auto).
	Height int
	// Config holds common TUI configuration.
	Config Config
}

// chooseModel implements EmbeddableComponent for single and multi-select prompts.
// This model works specifically with strings for the embeddable interface.
type chooseModel struct {
	form        *huh.Form
	result      *string   // For single-select
	multiResult *[]string // For multi-select
	isMulti     bool
	done        bool
	cancelled   bool
	width       int
	height      int
}

// NewChooseModel creates an embeddable choose component for string options.
func NewChooseModel(opts ChooseStringOptions) *chooseModel {
	// Determine if this is multi-select mode
	isMulti := opts.Limit > 1 || opts.NoLimit

	if isMulti {
		return newMultiChooseModelWithTheme(opts, getHuhTheme(opts.Config.Theme))
	}
	return newSingleChooseModelWithTheme(opts, getHuhTheme(opts.Config.Theme))
}

// NewChooseModelForModal creates an embeddable choose component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewChooseModelForModal(opts ChooseStringOptions) *chooseModel {
	theme := getModalHuhTheme()
	isMulti := opts.Limit > 1 || opts.NoLimit

	if isMulti {
		return newMultiChooseModelWithTheme(opts, theme)
	}
	return newSingleChooseModelWithTheme(opts, theme)
}

// newSingleChooseModelWithTheme creates a single-select choose model with a specific theme.
func newSingleChooseModelWithTheme(opts ChooseStringOptions, theme *huh.Theme) *chooseModel {
	var result string

	huhOpts := make([]huh.Option[string], len(opts.Options))
	for i, opt := range opts.Options {
		huhOpts[i] = huh.NewOption(opt, opt)
	}

	sel := huh.NewSelect[string]().
		Title(opts.Title).
		Options(huhOpts...).
		Value(&result)

	if opts.Height > 0 {
		sel = sel.Height(opts.Height)
	}

	form := huh.NewForm(huh.NewGroup(sel)).
		WithTheme(theme).
		WithAccessible(opts.Config.Accessible)

	return &chooseModel{
		form:   form,
		result: &result,
	}
}

// newMultiChooseModelWithTheme creates a multi-select choose model with a specific theme.
func newMultiChooseModelWithTheme(opts ChooseStringOptions, theme *huh.Theme) *chooseModel {
	var results []string

	huhOpts := make([]huh.Option[string], len(opts.Options))
	for i, opt := range opts.Options {
		huhOpts[i] = huh.NewOption(opt, opt)
	}

	sel := huh.NewMultiSelect[string]().
		Title(opts.Title).
		Options(huhOpts...).
		Value(&results)

	if opts.Limit > 0 {
		sel = sel.Limit(opts.Limit)
	}

	if opts.Height > 0 {
		sel = sel.Height(opts.Height)
	}

	form := huh.NewForm(huh.NewGroup(sel)).
		WithTheme(theme).
		WithAccessible(opts.Config.Accessible)

	return &chooseModel{
		form:        form,
		multiResult: &results,
		isMulti:     true,
	}
}

// Init implements tea.Model.
func (m *chooseModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update implements tea.Model.
func (m *chooseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	switch m.form.State {
	case huh.StateCompleted:
		m.done = true
	case huh.StateAborted:
		m.done = true
		m.cancelled = true
	case huh.StateNormal:
		// Form still in progress
	}

	return m, cmd
}

// View implements tea.Model.
func (m *chooseModel) View() string {
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
func (m *chooseModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns []string for both single and multi-select modes.
func (m *chooseModel) Result() (interface{}, error) {
	if m.cancelled {
		return nil, nil
	}
	if m.isMulti {
		return *m.multiResult, nil
	}
	// Return single result as a slice for consistency
	return []string{*m.result}, nil
}

// Cancelled implements EmbeddableComponent.
func (m *chooseModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *chooseModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.form = m.form.WithWidth(width).WithHeight(height)
}

// ChooseStringsWithModel is a convenience function for choosing from string options
// using the embeddable model internally.
func ChooseStringsWithModel(opts ChooseStringOptions) ([]string, error) {
	model := NewChooseModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := finalModel.(*chooseModel)
	if m.cancelled {
		return nil, fmt.Errorf("user aborted")
	}
	result, _ := m.Result()
	return result.([]string), nil
}

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
		WithAccessible(opts.Config.Accessible)

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
		WithAccessible(opts.Config.Accessible)

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

// ChooseStringBuilder provides a fluent API for building string-based Choose prompts
// that can return an EmbeddableComponent.
type ChooseStringBuilder struct {
	opts ChooseStringOptions
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
func (b *ChooseStringBuilder) Height(height int) *ChooseStringBuilder {
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
