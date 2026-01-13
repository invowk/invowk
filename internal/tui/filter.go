// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// FilterOptions configures the Filter component.
type FilterOptions struct {
	// Title is the title/prompt displayed above the filter.
	Title string
	// Placeholder is the placeholder text for the search input.
	Placeholder string
	// Options is the list of options to filter.
	Options []string
	// Limit is the maximum number of selections (0 for single, >0 for multi).
	Limit int
	// Height limits the visible height (0 for auto).
	Height int
	// Width limits the visible width (0 for auto).
	Width int
	// Reverse reverses the order of results.
	Reverse bool
	// Fuzzy enables fuzzy matching (default: true).
	Fuzzy bool
	// Sort sorts the results by match score.
	Sort bool
	// NoLimit allows unlimited selections.
	NoLimit bool
	// Selected pre-selects these indices.
	Selected []int
	// Strict requires at least one match to be selected.
	Strict bool
	// ShowIndicator shows the selected indicator.
	ShowIndicator bool
	// Config holds common TUI configuration.
	Config Config
}

// filterItem implements list.Item for the bubbles list component.
type filterItem struct {
	text     string
	selected bool
}

func (i filterItem) Title() string       { return i.text }
func (i filterItem) Description() string { return "" }
func (i filterItem) FilterValue() string { return i.text }

// filterModel is the bubbletea model for the filter component.
// It implements EmbeddableComponent for embedded use.
type filterModel struct {
	list      list.Model
	items     []filterItem
	options   []string
	query     string
	selected  map[int]bool
	limit     int
	noLimit   bool
	height    int
	width     int
	done      bool
	cancelled bool
}

// NewFilterModel creates an embeddable filter component.
func NewFilterModel(opts FilterOptions) *filterModel {
	return newFilterModelWithStyles(opts, false)
}

// NewFilterModelForModal creates an embeddable filter component optimized for modal overlays.
// This version uses styles that avoid background color bleeding.
func NewFilterModelForModal(opts FilterOptions) *filterModel {
	return newFilterModelWithStyles(opts, true)
}

// newFilterModelWithStyles creates a filter model with optional modal-specific styling.
func newFilterModelWithStyles(opts FilterOptions, forModal bool) *filterModel {
	if len(opts.Options) == 0 {
		// Empty filter - return a component that's immediately done
		return &filterModel{
			done:    true,
			options: []string{},
		}
	}

	items := make([]list.Item, len(opts.Options))
	for i, opt := range opts.Options {
		items[i] = filterItem{text: opt}
	}

	height := opts.Height
	if height == 0 {
		height = 10
	}

	width := opts.Width
	if width == 0 {
		width = 50
	}

	delegate := list.NewDefaultDelegate()

	if forModal {
		// Modal-specific styles: ALL have EXPLICIT backgrounds to prevent color bleeding
		// Terminal "transparent" doesn't exist - no background = terminal default shows through
		base := modalBaseStyle()

		// Normal item styles - explicit background on everything
		delegate.Styles.NormalTitle = base.Foreground(lipgloss.Color("#FFFFFF"))
		delegate.Styles.NormalDesc = base.Foreground(lipgloss.Color("#6B7280"))

		// Selected item - use left border indicator WITH explicit background
		delegate.Styles.SelectedTitle = base.
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			Padding(0, 0, 0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#7C3AED"))
		delegate.Styles.SelectedDesc = base.
			Foreground(lipgloss.Color("#A78BFA")).
			Padding(0, 0, 0, 1)

		// Dimmed styles - explicit backgrounds
		delegate.Styles.DimmedTitle = base.Foreground(lipgloss.Color("#6B7280"))
		delegate.Styles.DimmedDesc = base.Foreground(lipgloss.Color("#6B7280"))

		// FilterMatch style - used for highlighting matching characters during filtering
		// MUST have explicit background to prevent color bleeding
		delegate.Styles.FilterMatch = base.Foreground(lipgloss.Color("#A78BFA")).Bold(true)
	} else {
		// Default styles for non-modal usage
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
	delegate.ShowDescription = false

	l := list.New(items, delegate, width, height)
	l.Title = opts.Title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)

	if forModal {
		// Modal-specific list styles - ALL have EXPLICIT backgrounds
		// This is critical: every single style must have the modal background
		base := modalBaseStyle()

		l.Styles.Title = base.Bold(true).Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.TitleBar = base.Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.HelpStyle = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.FilterPrompt = base.Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.FilterCursor = base.Foreground(lipgloss.Color("#FFFFFF"))

		// Additional styles - ALL have explicit backgrounds
		l.Styles.NoItems = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.StatusBar = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.StatusEmpty = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.StatusBarActiveFilter = base.Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.StatusBarFilterCount = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.ActivePaginationDot = base.Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.InactivePaginationDot = base.Foreground(lipgloss.Color("#6B7280"))
		l.Styles.DividerDot = base.Foreground(lipgloss.Color("#6B7280"))

		// Additional styles - ALL have explicit backgrounds
		l.Styles.Spinner = base.Foreground(lipgloss.Color("#7C3AED"))
		l.Styles.DefaultFilterCharacterMatch = base.Foreground(lipgloss.Color("#A78BFA")).Bold(true)
		l.Styles.ArabicPagination = base.Foreground(lipgloss.Color("#6B7280"))

		// Customize the filter input - ALL have explicit backgrounds
		l.FilterInput.PromptStyle = base.Foreground(lipgloss.Color("#7C3AED"))
		l.FilterInput.TextStyle = base.Foreground(lipgloss.Color("#FFFFFF"))
		l.FilterInput.Cursor.Style = base.Foreground(lipgloss.Color("#FFFFFF"))
		l.FilterInput.PlaceholderStyle = base.Foreground(lipgloss.Color("#6B7280"))
	} else {
		// Default list styles
		l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 0)
		l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		l.Styles.HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
		l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	}

	if opts.Placeholder != "" {
		l.FilterInput.Placeholder = opts.Placeholder
	}

	filterItems := make([]filterItem, len(opts.Options))
	for i, opt := range opts.Options {
		filterItems[i] = filterItem{text: opt}
	}

	m := &filterModel{
		list:     l,
		items:    filterItems,
		options:  opts.Options,
		selected: make(map[int]bool),
		limit:    opts.Limit,
		noLimit:  opts.NoLimit,
		height:   height,
		width:    width,
	}

	// Pre-select items
	for _, idx := range opts.Selected {
		if idx >= 0 && idx < len(opts.Options) {
			m.selected[idx] = true
		}
	}

	return m
}

func (m *filterModel) Init() tea.Cmd {
	return nil
}

func (m *filterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		case "tab", " ":
			if m.limit > 1 || m.noLimit {
				// Multi-select mode (limit > 1 or no limit)
				// Note: limit=1 is single-select, space/tab don't toggle
				idx := m.list.Index()
				if m.selected[idx] {
					delete(m.selected, idx)
				} else if m.noLimit || len(m.selected) < m.limit {
					m.selected[idx] = true
				}
			}
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *filterModel) View() string {
	if m.done {
		return ""
	}
	// Constrain the list view to the configured width to prevent overflow in modal overlays
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(m.list.View())
	}
	return m.list.View()
}

// IsDone implements EmbeddableComponent.
func (m *filterModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns []string for selected options.
func (m *filterModel) Result() (interface{}, error) {
	if m.cancelled {
		return nil, nil
	}

	// Handle multi-select (limit > 1 or no limit)
	// Note: limit=1 is single-select mode where Enter selects the highlighted item
	if m.limit > 1 || m.noLimit {
		var results []string
		for idx := range m.selected {
			if idx < len(m.options) {
				results = append(results, m.options[idx])
			}
		}
		return results, nil
	}

	// Single select (limit=0 or limit=1): return the currently highlighted item
	if item, ok := m.list.SelectedItem().(filterItem); ok {
		return []string{item.text}, nil
	}

	return []string{}, nil
}

// Cancelled implements EmbeddableComponent.
func (m *filterModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *filterModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetWidth(width)
	m.list.SetHeight(height - 2)
}

// Filter prompts the user to filter and select from a list of options.
// Returns the selected option(s) or an error if the prompt was cancelled.
func Filter(opts FilterOptions) ([]string, error) {
	if len(opts.Options) == 0 {
		return nil, nil
	}

	model := NewFilterModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	m := finalModel.(*filterModel)
	if m.cancelled {
		return nil, nil
	}

	// Handle multi-select (limit > 1 or no limit)
	if opts.Limit > 1 || opts.NoLimit {
		var results []string
		for idx := range m.selected {
			results = append(results, opts.Options[idx])
		}
		return results, nil
	}

	// Single select (limit=0 or limit=1)
	if item, ok := m.list.SelectedItem().(filterItem); ok {
		return []string{item.text}, nil
	}

	return nil, nil
}

// FilterStrings is a convenience function for filtering string options.
func FilterStrings(options []string, config Config) (string, error) {
	results, err := Filter(FilterOptions{
		Options: options,
		Config:  config,
	})
	if err != nil || len(results) == 0 {
		return "", err
	}
	return results[0], nil
}

// FuzzyMatch performs fuzzy matching on a list of options.
// Returns the matched options sorted by score.
func FuzzyMatch(pattern string, options []string) []string {
	if pattern == "" {
		return options
	}

	matches := fuzzy.Find(pattern, options)
	sort.Sort(matches)

	results := make([]string, len(matches))
	for i, m := range matches {
		results[i] = options[m.Index]
	}

	return results
}

// FuzzyMatchWithScore performs fuzzy matching and returns matches with scores.
func FuzzyMatchWithScore(pattern string, options []string) []struct {
	Text  string
	Score int
} {
	if pattern == "" {
		results := make([]struct {
			Text  string
			Score int
		}, len(options))
		for i, opt := range options {
			results[i] = struct {
				Text  string
				Score int
			}{Text: opt, Score: 0}
		}
		return results
	}

	matches := fuzzy.Find(pattern, options)
	sort.Sort(matches)

	results := make([]struct {
		Text  string
		Score int
	}, len(matches))
	for i, m := range matches {
		results[i] = struct {
			Text  string
			Score int
		}{Text: options[m.Index], Score: m.Score}
	}

	return results
}

// ExactMatch performs exact substring matching on a list of options.
func ExactMatch(pattern string, options []string) []string {
	if pattern == "" {
		return options
	}

	pattern = strings.ToLower(pattern)
	var results []string
	for _, opt := range options {
		if strings.Contains(strings.ToLower(opt), pattern) {
			results = append(results, opt)
		}
	}

	return results
}

// FilterBuilder provides a fluent API for building Filter prompts.
type FilterBuilder struct {
	opts FilterOptions
}

// NewFilter creates a new FilterBuilder with default options.
func NewFilter() *FilterBuilder {
	return &FilterBuilder{
		opts: FilterOptions{
			Fuzzy:  true,
			Config: DefaultConfig(),
		},
	}
}

// Title sets the title of the filter prompt.
func (b *FilterBuilder) Title(title string) *FilterBuilder {
	b.opts.Title = title
	return b
}

// Placeholder sets the placeholder text for the search input.
func (b *FilterBuilder) Placeholder(placeholder string) *FilterBuilder {
	b.opts.Placeholder = placeholder
	return b
}

// Options sets the list of options to filter.
func (b *FilterBuilder) Options(options ...string) *FilterBuilder {
	b.opts.Options = options
	return b
}

// OptionsFromSlice sets the options from a slice.
func (b *FilterBuilder) OptionsFromSlice(options []string) *FilterBuilder {
	b.opts.Options = options
	return b
}

// Limit sets the maximum number of selections for multi-select.
func (b *FilterBuilder) Limit(limit int) *FilterBuilder {
	b.opts.Limit = limit
	return b
}

// NoLimit allows unlimited selections.
func (b *FilterBuilder) NoLimit(noLimit bool) *FilterBuilder {
	b.opts.NoLimit = noLimit
	return b
}

// Height sets the visible height.
func (b *FilterBuilder) Height(height int) *FilterBuilder {
	b.opts.Height = height
	return b
}

// Width sets the visible width.
func (b *FilterBuilder) Width(width int) *FilterBuilder {
	b.opts.Width = width
	return b
}

// Reverse reverses the order of results.
func (b *FilterBuilder) Reverse(reverse bool) *FilterBuilder {
	b.opts.Reverse = reverse
	return b
}

// Fuzzy enables or disables fuzzy matching.
func (b *FilterBuilder) Fuzzy(fuzzy bool) *FilterBuilder {
	b.opts.Fuzzy = fuzzy
	return b
}

// Sort sorts results by match score.
func (b *FilterBuilder) Sort(sort bool) *FilterBuilder {
	b.opts.Sort = sort
	return b
}

// Selected pre-selects items by index.
func (b *FilterBuilder) Selected(indices ...int) *FilterBuilder {
	b.opts.Selected = indices
	return b
}

// Theme sets the visual theme.
func (b *FilterBuilder) Theme(theme Theme) *FilterBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *FilterBuilder) Accessible(accessible bool) *FilterBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run executes the filter prompt and returns the selected option(s).
func (b *FilterBuilder) Run() ([]string, error) {
	return Filter(b.opts)
}

// RunSingle executes the filter prompt and returns a single selected option.
func (b *FilterBuilder) RunSingle() (string, error) {
	results, err := Filter(b.opts)
	if err != nil || len(results) == 0 {
		return "", err
	}
	return results[0], nil
}

// Model returns the embeddable model for composition.
func (b *FilterBuilder) Model() EmbeddableComponent {
	return NewFilterModel(b.opts)
}
