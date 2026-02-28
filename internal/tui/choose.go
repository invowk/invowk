// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// All type declarations in a single block for decorder compliance.
type (
	// ChooseStringOptions configures the embeddable Choose component for strings.
	// This is used by the TUI server for dynamic component creation.
	// JSON tags must match the snake_case format used by ChooseRequest in protocol.go
	// for proper unmarshaling when options are received via the TUI server JSON protocol.
	ChooseStringOptions struct {
		// Title is the title/prompt displayed above the options.
		Title string `json:"title,omitempty"`
		// Options is the list of string options to choose from.
		Options []string `json:"options"`
		// Limit is the maximum number of selections (0 or 1 for single-select, >1 for multi-select).
		Limit int `json:"limit,omitempty"`
		// NoLimit allows unlimited selections in multi-select mode.
		NoLimit bool `json:"no_limit,omitempty"`
		// Height limits the number of visible options (0 for auto).
		Height TerminalDimension `json:"height,omitempty"`
		// Selected stores pre-selected option indices for multi-select mode (internal only).
		Selected []SelectionIndex `json:"-"`
		// Config holds common TUI configuration (internal only, not from protocol).
		Config Config `json:"-"`
	}

	// chooseModel implements EmbeddableComponent for single and multi-select prompts.
	// This model works specifically with strings for the embeddable interface.
	chooseModel struct {
		list        list.Model
		result      *string
		multiResult *[]string
		isMulti     bool
		done        bool
		cancelled   bool
		width       TerminalDimension
		height      TerminalDimension

		options  []string     // Original options list
		selected map[int]bool // Selection state by index
		limit    int          // Selection limit (0 = unlimited)
		noLimit  bool         // Allow unlimited selections
	}

	// chooseItem implements list.Item for the bubbles list component in multi-select mode.
	chooseItem struct {
		text  string
		index SelectionIndex // Track original index for selection map
	}

	// multiChooseDelegate renders items with selection checkboxes for multi-select mode.
	multiChooseDelegate struct {
		styles     list.DefaultItemStyles
		isSelected func(int) bool // Callback to check if an index is selected
		forModal   bool
	}

	// Option represents a selectable option with a display title and value.
	Option[T comparable] struct {
		// Title is the display text for the option.
		Title string
		// Value is the underlying value of the option.
		Value T
		// Selected indicates if this option is pre-selected (for multi-select).
		Selected bool
	}

	// ChooseOptions configures the Choose component.
	ChooseOptions[T comparable] struct {
		// Title is the title/prompt displayed above the options.
		Title string
		// Description provides additional context below the title.
		Description string
		// Options is the list of options to choose from.
		Options []Option[T]
		// Height limits the number of visible options (0 for auto).
		Height TerminalDimension
		// Cursor is the character used for the cursor (default: "> ").
		Cursor string
		// Config holds common TUI configuration.
		Config Config
	}

	// MultiChooseOptions configures the MultiChoose component.
	MultiChooseOptions[T comparable] struct {
		// Title is the title/prompt displayed above the options.
		Title string
		// Description provides additional context below the title.
		Description string
		// Options is the list of options to choose from.
		Options []Option[T]
		// Limit is the maximum number of selections (0 for no limit).
		Limit int
		// Height limits the number of visible options (0 for auto).
		Height TerminalDimension
		// Config holds common TUI configuration.
		Config Config
	}

	// ChooseBuilder provides a fluent API for building Choose prompts.
	ChooseBuilder[T comparable] struct {
		opts ChooseOptions[T]
	}

	// MultiChooseBuilder provides a fluent API for building MultiChoose prompts.
	MultiChooseBuilder[T comparable] struct {
		opts MultiChooseOptions[T]
	}

	// ChooseStringBuilder provides a fluent API for building string-based Choose prompts
	// that can return an EmbeddableComponent.
	ChooseStringBuilder struct {
		opts ChooseStringOptions
	}
)

// chooseItem implements list.Item interface for bubbles/list.
func (i chooseItem) Title() string       { return i.text }
func (i chooseItem) Description() string { return "" }
func (i chooseItem) FilterValue() string { return i.text }

// multiChooseDelegate implements list.ItemDelegate for rendering items with checkboxes.
func newMultiChooseDelegate(forModal bool, isSelected func(int) bool) multiChooseDelegate {
	d := multiChooseDelegate{
		styles:     list.NewDefaultDelegate().Styles,
		isSelected: isSelected,
		forModal:   forModal,
	}

	if forModal {
		// Modal-specific styles: ALL have EXPLICIT backgrounds to prevent color bleeding
		base := modalBaseStyle()

		// Normal item styles - explicit background on everything
		d.styles.NormalTitle = base.Foreground(lipgloss.Color("#FFFFFF"))
		d.styles.NormalDesc = base.Foreground(lipgloss.Color("#6B7280"))

		// Selected item - use left border indicator WITH explicit background
		d.styles.SelectedTitle = base.
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#7C3AED"))
		d.styles.SelectedDesc = base.
			Foreground(lipgloss.Color("#A78BFA")).
			Padding(0, 0, 0, 1)

		// Dimmed styles - explicit backgrounds
		d.styles.DimmedTitle = base.Foreground(lipgloss.Color("#6B7280"))
		d.styles.DimmedDesc = base.Foreground(lipgloss.Color("#6B7280"))
	} else {
		// Default styles for non-modal usage
		d.styles.NormalTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		d.styles.NormalDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		d.styles.SelectedTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("212"))
		d.styles.SelectedDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Padding(0, 0, 0, 1)
		d.styles.DimmedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		d.styles.DimmedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}

	return d
}

// Height returns the height of a single item.
func (d multiChooseDelegate) Height() int { return 1 }

// Spacing returns the spacing between items.
func (d multiChooseDelegate) Spacing() int { return 0 }

// Update handles item-level updates (not used, handled at model level).
func (d multiChooseDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render renders a single item with a checkbox prefix based on selection state.
// The checkbox is always rendered with NormalTitle style (left-aligned), while
// the text uses SelectedTitle style when focused to show the emphasis indicator.
func (d multiChooseDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(chooseItem)
	if !ok {
		return
	}

	// Determine checkbox state using the callback
	checkbox := "[ ] "
	if d.isSelected(int(i.index)) {
		checkbox = "[x] "
	}

	// Determine if this is the cursor position
	isCursor := index == m.Index()

	// Render checkbox and text separately so focus styling only affects text.
	// This ensures checkboxes stay left-aligned while the focus indicator (border/padding)
	// only shifts the text portion.
	if isCursor {
		// For focused item: checkbox unstyled (uses NormalTitle without border/padding),
		// text with emphasis style (SelectedTitle includes left border indicator)
		checkboxStyle := d.styles.NormalTitle.
			Padding(0).
			UnsetBorderLeft()
		fmt.Fprint(w, checkboxStyle.Render(checkbox)+d.styles.SelectedTitle.Render(i.text))
	} else {
		// For unfocused items: both checkbox and text with normal style
		fmt.Fprint(w, d.styles.NormalTitle.Render(checkbox+i.text))
	}
}

// NewChooseModel creates an embeddable choose component for string options.
func NewChooseModel(opts ChooseStringOptions) *chooseModel {
	// Determine if this is multi-select mode
	isMulti := opts.Limit > 1 || opts.NoLimit

	if isMulti {
		return newMultiChooseModelWithTheme(opts, false) // not modal
	}
	return newSingleChooseModel(opts, false)
}

// NewChooseModelForModal creates an embeddable choose component with modal-specific styling.
// This uses a theme that matches the modal overlay background to prevent color bleeding.
func NewChooseModelForModal(opts ChooseStringOptions) *chooseModel {
	isMulti := opts.Limit > 1 || opts.NoLimit

	if isMulti {
		return newMultiChooseModelWithTheme(opts, true) // modal mode
	}
	return newSingleChooseModel(opts, true)
}

// Init implements tea.Model.
func (m *chooseModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *chooseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle key events
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		case "space", "x":
			// Toggle handling for multi-select mode using bubbles/list.
			if m.isMulti {
				idx := m.list.Index() // Get cursor position from list
				if m.selected[idx] {
					delete(m.selected, idx)
				} else if m.noLimit || m.limit <= 0 || len(m.selected) < m.limit {
					m.selected[idx] = true
				}
				m.syncSelections()
				return m, nil // Don't pass to list (we handled the toggle)
			}
		case "enter":
			if m.isMulti {
				m.syncSelections()
				// Make sure single-result pointer is not stale from prior usage.
				if m.result != nil {
					*m.result = ""
				}
			} else if m.result != nil {
				if item, ok := m.list.SelectedItem().(chooseItem); ok {
					*m.result = item.text
				}
			}
			if m.done {
				return m, tea.Quit
			}
			if m.isMulti || m.result != nil {
				m.done = true
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m *chooseModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}

	view := m.list.View()
	help := "↑/k up • ↓/j down • enter submit • esc cancel"
	if m.isMulti {
		help = "↑/k up • ↓/j down • space toggle • enter submit • esc cancel"
	}
	view = lipgloss.JoinVertical(lipgloss.Left, view, m.list.Styles.HelpStyle.Render(help))

	if m.width > 0 {
		view = lipgloss.NewStyle().MaxWidth(int(m.width)).Render(view)
	}
	return tea.NewView(view)
}

// IsDone implements EmbeddableComponent.
func (m *chooseModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns []string for both single and multi-select modes.
// Returns ErrCancelled if the user cancelled the operation.
func (m *chooseModel) Result() (any, error) {
	if m.cancelled {
		return nil, ErrCancelled
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
func (m *chooseModel) SetSize(width, height TerminalDimension) {
	m.width = width
	m.height = height
	m.list.SetWidth(int(width))
	listHeight := max(1, int(height)-3) // Reserve one line for keybinding hints footer.
	m.list.SetHeight(listHeight)
}

// syncSelections updates multiResult to match our tracked selection state.
func (m *chooseModel) syncSelections() {
	if m.multiResult == nil {
		return
	}
	results := make([]string, 0, len(m.selected))
	for i := 0; i < len(m.options); i++ {
		if m.selected[i] {
			results = append(results, m.options[i])
		}
	}
	*m.multiResult = results
}

// selectedIndices returns selected indices in deterministic order.
func (m *chooseModel) selectedIndices() []SelectionIndex {
	if !m.isMulti {
		item, ok := m.list.SelectedItem().(chooseItem)
		if !ok {
			return nil
		}
		return []SelectionIndex{item.index}
	}

	indices := make([]SelectionIndex, 0, len(m.selected))
	for i := 0; i < len(m.options); i++ {
		if m.selected[i] {
			indices = append(indices, SelectionIndex(i)) //goplint:ignore -- bounded loop index over known-length slice
		}
	}

	return indices
}

func chooseIndicesWithModel(opts ChooseStringOptions) ([]SelectionIndex, error) {
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

	return m.selectedIndices(), nil
}

func selectedIndicesFromOptions[T comparable](options []Option[T]) []SelectionIndex {
	indices := make([]SelectionIndex, 0, len(options))
	for i, opt := range options {
		if opt.Selected {
			indices = append(indices, SelectionIndex(i)) //goplint:ignore -- bounded loop index over known-length slice
		}
	}
	return indices
}

func selectedValuesByIndex[T comparable](options []Option[T], indices []SelectionIndex) []T {
	values := make([]T, 0, len(indices))
	for _, idx := range indices {
		idxInt := int(idx)
		if idxInt < 0 || idxInt >= len(options) {
			continue
		}
		values = append(values, options[idxInt].Value)
	}
	return values
}

// Choose prompts the user to select one option from a list.
// Returns the selected value or an error if the prompt was cancelled.
func Choose[T comparable](opts ChooseOptions[T]) (T, error) {
	var result T

	if len(opts.Options) == 0 {
		return result, nil
	}

	titles := make([]string, len(opts.Options))
	for i, opt := range opts.Options {
		titles[i] = opt.Title
	}

	indices, err := chooseIndicesWithModel(ChooseStringOptions{
		Title:   opts.Title,
		Options: titles,
		Limit:   1,
		Height:  opts.Height,
		Config:  opts.Config,
	})
	if err != nil {
		return result, err
	}

	if len(indices) == 0 {
		return result, nil
	}

	idx := int(indices[0])
	if idx < 0 || idx >= len(opts.Options) {
		return result, nil
	}

	result = opts.Options[idx].Value
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
	result, _ := m.Result() //nolint:errcheck // Result() cannot fail after successful Run()
	return result.([]string), nil
}

// MultiChoose prompts the user to select multiple options from a list.
// Returns the selected values or an error if the prompt was cancelled.
func MultiChoose[T comparable](opts MultiChooseOptions[T]) ([]T, error) {
	if len(opts.Options) == 0 {
		return nil, nil
	}

	titles := make([]string, len(opts.Options))
	for i, opt := range opts.Options {
		titles[i] = opt.Title
	}

	indices, err := chooseIndicesWithModel(ChooseStringOptions{
		Title:    opts.Title,
		Options:  titles,
		Limit:    opts.Limit,
		NoLimit:  opts.Limit <= 0,
		Height:   opts.Height,
		Selected: selectedIndicesFromOptions(opts.Options),
		Config:   opts.Config,
	})
	if err != nil {
		return nil, err
	}

	return selectedValuesByIndex(opts.Options, indices), nil
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

	if forModal {
		base := modalBaseStyle()
		delegate.Styles.NormalTitle = base.Foreground(lipgloss.Color("#FFFFFF"))
		delegate.Styles.NormalDesc = base.Foreground(lipgloss.Color("#6B7280"))
		delegate.Styles.SelectedTitle = base.
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#7C3AED"))
		delegate.Styles.SelectedDesc = base.
			Foreground(lipgloss.Color("#A78BFA")).
			Padding(0, 0, 0, 1)
		delegate.Styles.DimmedTitle = base.Foreground(lipgloss.Color("#6B7280"))
		delegate.Styles.DimmedDesc = base.Foreground(lipgloss.Color("#6B7280"))
	} else {
		delegate.Styles.NormalTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		delegate.Styles.NormalDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		delegate.Styles.SelectedTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("212"))
		delegate.Styles.SelectedDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Padding(0, 0, 0, 1)
		delegate.Styles.DimmedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		delegate.Styles.DimmedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}

	l := list.New(items, delegate, width, height)
	l.Title = opts.Title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	if forModal {
		base := modalBaseStyle()
		l.Styles.Title = base.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.TitleBar = base.Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.HelpStyle = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.NoItems = base.Foreground(lipgloss.Color("#6B7280"))
	} else {
		l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		l.Styles.HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}

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

	if forModal {
		// Modal-specific list styles - ALL have EXPLICIT backgrounds
		base := modalBaseStyle()

		l.Styles.Title = base.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.TitleBar = base.Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.HelpStyle = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.NoItems = base.Foreground(lipgloss.Color("#6B7280"))
	} else {
		// Default list styles
		l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		l.Styles.HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	}

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
