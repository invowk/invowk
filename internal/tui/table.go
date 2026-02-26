// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"strings"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type (
	// TableColumn represents a column in the table.
	TableColumn struct {
		// Title is the column header text.
		Title string
		// Width is the column width (0 for auto).
		Width TerminalDimension
	}

	// TableOptions configures the Table component.
	TableOptions struct {
		// Title is the title displayed above the table.
		Title string
		// Columns defines the table columns.
		Columns []TableColumn
		// Rows contains the table data.
		Rows [][]string
		// Height limits the visible height (0 for auto).
		Height TerminalDimension
		// Width limits the visible width (0 for auto).
		Width TerminalDimension
		// Selectable enables row selection.
		Selectable bool
		// SelectedIndex is the initially selected row index.
		SelectedIndex int
		// Separator is the column separator character.
		Separator string
		// Border enables table border.
		Border bool
		// Config holds common TUI configuration.
		Config Config
	}

	// tableModel is the bubbletea model for the table component.
	// It implements EmbeddableComponent for embedded use.
	tableModel struct {
		table     table.Model
		rows      [][]string
		done      bool
		cancelled bool
		width     int
		height    int
	}

	// TableBuilder provides a fluent API for building Table displays.
	TableBuilder struct {
		opts TableOptions
	}
)

// NewTableModel creates an embeddable table component.
func NewTableModel(opts TableOptions) *tableModel {
	return newTableModelWithStyles(opts, false)
}

// NewTableModelForModal creates an embeddable table component optimized for modal overlays.
// This version uses styles that avoid background color bleeding.
func NewTableModelForModal(opts TableOptions) *tableModel {
	return newTableModelWithStyles(opts, true)
}

func (m *tableModel) Init() tea.Cmd {
	return nil
}

func (m *tableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case keyCtrlC, "esc", "q":
			m.done = true
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *tableModel) View() tea.View {
	if m.done {
		return tea.NewView("")
	}
	// Constrain the table view to the configured width to prevent overflow in modal overlays
	view := m.table.View()
	if m.width > 0 {
		view = lipgloss.NewStyle().MaxWidth(m.width).Render(view)
	}
	return tea.NewView(view)
}

// IsDone implements EmbeddableComponent.
func (m *tableModel) IsDone() bool {
	return m.done
}

// Result implements EmbeddableComponent.
// Returns TableSelectionResult with selected row info.
func (m *tableModel) Result() (any, error) {
	if m.cancelled {
		return TableSelectionResult{SelectedIndex: -1}, nil
	}

	selectedIdx := m.table.Cursor()
	var selectedRow []string
	if selectedIdx >= 0 && selectedIdx < len(m.rows) {
		selectedRow = m.rows[selectedIdx]
	}

	return TableSelectionResult{
		SelectedIndex: selectedIdx,
		SelectedRow:   selectedRow,
	}, nil
}

// Cancelled implements EmbeddableComponent.
func (m *tableModel) Cancelled() bool {
	return m.cancelled
}

// SetSize implements EmbeddableComponent.
func (m *tableModel) SetSize(width, height TerminalDimension) {
	m.width = int(width)
	m.height = int(height)
	if width > 0 {
		m.table.SetWidth(int(width))
	}
	if height > 0 {
		m.table.SetHeight(int(height))
	}
}

// Table displays a table and optionally allows row selection.
// Returns the selected row index (-1 if cancelled) and the selected row values.
func Table(opts TableOptions) (selectedIdx int, row []string, err error) {
	if len(opts.Rows) == 0 {
		return -1, nil, nil
	}

	model := NewTableModel(opts)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return -1, nil, err
	}

	m := finalModel.(*tableModel)
	if m.cancelled {
		return -1, nil, nil
	}

	selectedIdx = m.table.Cursor()
	if selectedIdx >= 0 && selectedIdx < len(opts.Rows) {
		return selectedIdx, opts.Rows[selectedIdx], nil
	}

	return -1, nil, nil
}

// TableFromCSV creates table options from CSV-formatted data.
func TableFromCSV(data, separator string, hasHeader bool) TableOptions {
	if separator == "" {
		separator = ","
	}

	lines := strings.Split(strings.TrimSpace(data), "\n")
	if len(lines) == 0 {
		return TableOptions{}
	}

	var columns []TableColumn
	var rows [][]string

	startIdx := 0
	if hasHeader {
		headers := strings.Split(lines[0], separator)
		columns = make([]TableColumn, len(headers))
		for i, h := range headers {
			columns[i] = TableColumn{Title: strings.TrimSpace(h)}
		}
		startIdx = 1
	} else {
		// Generate column headers
		firstRow := strings.Split(lines[0], separator)
		columns = make([]TableColumn, len(firstRow))
		for i := range firstRow {
			columns[i] = TableColumn{Title: ""}
		}
	}

	for _, line := range lines[startIdx:] {
		fields := strings.Split(line, separator)
		row := make([]string, len(fields))
		for i, f := range fields {
			row[i] = strings.TrimSpace(f)
		}
		rows = append(rows, row)
	}

	return TableOptions{
		Columns:    columns,
		Rows:       rows,
		Selectable: true,
	}
}

// NewTable creates a new TableBuilder with default options.
func NewTable() *TableBuilder {
	return &TableBuilder{
		opts: TableOptions{
			Selectable: true,
			Border:     true,
			Config:     DefaultConfig(),
		},
	}
}

// Title sets the title of the table.
func (b *TableBuilder) Title(title string) *TableBuilder {
	b.opts.Title = title
	return b
}

// Columns sets the table columns.
func (b *TableBuilder) Columns(columns ...TableColumn) *TableBuilder {
	b.opts.Columns = columns
	return b
}

// ColumnsFromStrings sets columns from string titles.
func (b *TableBuilder) ColumnsFromStrings(titles ...string) *TableBuilder {
	b.opts.Columns = make([]TableColumn, len(titles))
	for i, t := range titles {
		b.opts.Columns[i] = TableColumn{Title: t}
	}
	return b
}

// Rows sets the table rows.
func (b *TableBuilder) Rows(rows ...[]string) *TableBuilder {
	b.opts.Rows = rows
	return b
}

// AddRow adds a row to the table.
func (b *TableBuilder) AddRow(row ...string) *TableBuilder {
	b.opts.Rows = append(b.opts.Rows, row)
	return b
}

// Height sets the visible height.
func (b *TableBuilder) Height(height TerminalDimension) *TableBuilder {
	b.opts.Height = height
	return b
}

// Width sets the visible width.
func (b *TableBuilder) Width(width TerminalDimension) *TableBuilder {
	b.opts.Width = width
	return b
}

// Selectable enables or disables row selection.
func (b *TableBuilder) Selectable(selectable bool) *TableBuilder {
	b.opts.Selectable = selectable
	return b
}

// SelectedIndex sets the initially selected row.
func (b *TableBuilder) SelectedIndex(idx int) *TableBuilder {
	b.opts.SelectedIndex = idx
	return b
}

// Border enables or disables the table border.
func (b *TableBuilder) Border(border bool) *TableBuilder {
	b.opts.Border = border
	return b
}

// FromCSV loads table data from CSV.
func (b *TableBuilder) FromCSV(data, separator string, hasHeader bool) *TableBuilder {
	csvOpts := TableFromCSV(data, separator, hasHeader)
	b.opts.Columns = csvOpts.Columns
	b.opts.Rows = csvOpts.Rows
	return b
}

// Theme sets the visual theme.
func (b *TableBuilder) Theme(theme Theme) *TableBuilder {
	b.opts.Config.Theme = theme
	return b
}

// Accessible enables accessible mode.
func (b *TableBuilder) Accessible(accessible bool) *TableBuilder {
	b.opts.Config.Accessible = accessible
	return b
}

// Run displays the table and returns the selected row.
func (b *TableBuilder) Run() (selectedIdx int, row []string, err error) {
	return Table(b.opts)
}

// Display shows the table without selection (read-only).
func (b *TableBuilder) Display() error {
	_, _, err := Table(b.opts)
	return err
}

// Model returns the embeddable model for composition.
func (b *TableBuilder) Model() EmbeddableComponent {
	return NewTableModel(b.opts)
}

// newTableModelWithStyles creates a table model with optional modal-specific styling.
func newTableModelWithStyles(opts TableOptions, forModal bool) *tableModel {
	if len(opts.Rows) == 0 {
		return &tableModel{
			done: true,
			rows: [][]string{},
		}
	}

	// Build columns
	columns := make([]table.Column, len(opts.Columns))
	for i, col := range opts.Columns {
		colWidth := int(col.Width)
		if colWidth == 0 {
			// Auto-calculate width based on content
			colWidth = len(col.Title)
			for _, row := range opts.Rows {
				if i < len(row) && len(row[i]) > colWidth {
					colWidth = len(row[i])
				}
			}
			colWidth += 2 // Add padding
		}
		columns[i] = table.Column{
			Title: col.Title,
			Width: colWidth,
		}
	}

	// Build rows
	rows := make([]table.Row, len(opts.Rows))
	for i, row := range opts.Rows {
		rows[i] = row
	}

	tableHeight := int(opts.Height)
	if tableHeight == 0 {
		tableHeight = min(len(opts.Rows)+1, 15)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
	)

	s := table.DefaultStyles()

	if forModal {
		// Modal-specific styles: ALL have EXPLICIT backgrounds to prevent color bleeding
		base := modalBaseStyle()
		purple := lipgloss.Color("#7C3AED")
		lightPurple := lipgloss.Color("#A78BFA")
		white := lipgloss.Color("#FFFFFF")
		dimmed := lipgloss.Color("#6B7280")

		// Header style - explicit background with border bottom
		s.Header = base.
			Foreground(purple).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(dimmed).
			BorderBottom(true)

		// Selected row - use left border indicator WITH explicit background
		s.Selected = base.
			Foreground(lightPurple).
			Bold(true).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(purple)

		// Cell style - explicit background
		s.Cell = base.Foreground(white)
	} else {
		// Default styles
		s.Header = s.Header.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			BorderBottom(true).
			Bold(true)
		s.Selected = s.Selected.
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(false)
	}

	t.SetStyles(s)

	if opts.SelectedIndex >= 0 && opts.SelectedIndex < len(opts.Rows) {
		t.SetCursor(opts.SelectedIndex)
	}

	return &tableModel{
		table:  t,
		rows:   opts.Rows,
		width:  int(opts.Width),
		height: tableHeight,
	}
}
