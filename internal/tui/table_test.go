// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewTableModel(t *testing.T) {
	opts := TableOptions{
		Title: "Users",
		Columns: []TableColumn{
			{Title: "Name", Width: 20},
			{Title: "Email", Width: 30},
		},
		Rows: [][]string{
			{"Alice", "alice@example.com"},
			{"Bob", "bob@example.com"},
		},
		Height:        10,
		Width:         60,
		Selectable:    true,
		SelectedIndex: 0,
		Border:        true,
		Config:        DefaultConfig(),
	}

	model := NewTableModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
	if model.Cancelled() {
		t.Error("expected model not to be cancelled initially")
	}
}

func TestNewTableModel_EmptyRows(t *testing.T) {
	opts := TableOptions{
		Title:   "Empty Table",
		Columns: []TableColumn{{Title: "Col1"}},
		Rows:    [][]string{},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Empty table should be immediately done
	if !model.done {
		t.Error("expected empty table to be done immediately")
	}
}

func TestNewTableModel_AutoWidth(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{
			{Title: "Short", Width: 0},   // Auto-width
			{Title: "Medium", Width: 20}, // Fixed width
		},
		Rows: [][]string{
			{"LongerContent", "text"},
		},
		Config: DefaultConfig(),
	}

	model := NewTableModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestTableModel_CancelWithEsc(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}, {"B"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)

	// Simulate Esc key press
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*tableModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Esc")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Esc")
	}
}

func TestTableModel_CancelWithCtrlC(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}, {"B"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*tableModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Ctrl+C")
	}
}

func TestTableModel_CancelWithQ(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}, {"B"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)

	// Simulate 'q' key press
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*tableModel)

	if !m.IsDone() {
		t.Error("expected model to be done after 'q'")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after 'q'")
	}
}

func TestTableModel_SelectWithEnter(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}, {"B"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)

	// Simulate Enter key press
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*tableModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Enter")
	}
	if m.Cancelled() {
		t.Error("expected model not to be cancelled after Enter")
	}
}

func TestTableModel_Result_Selected(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"First"}, {"Second"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)
	model.done = true

	result, err := model.Result()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	tableResult, ok := result.(TableSelectionResult)
	if !ok {
		t.Fatalf("expected TableSelectionResult, got %T", result)
	}

	if tableResult.SelectedIndex < 0 {
		t.Error("expected valid selected index")
	}
}

func TestTableModel_Result_Cancelled(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)
	model.done = true
	model.cancelled = true

	result, err := model.Result()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	tableResult, ok := result.(TableSelectionResult)
	if !ok {
		t.Fatalf("expected TableSelectionResult, got %T", result)
	}

	if tableResult.SelectedIndex != -1 {
		t.Errorf("expected SelectedIndex -1 when cancelled, got %d", tableResult.SelectedIndex)
	}
}

func TestTableModel_SetSize(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)
	model.SetSize(100, 30)

	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}
	if model.height != 30 {
		t.Errorf("expected height 30, got %d", model.height)
	}
}

func TestTableModel_ViewWhenDone(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)
	model.done = true

	view := model.View()

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestTableModel_ViewWithWidth(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}, {"B"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)
	model.SetSize(50, 10)

	view := model.View()

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestTableModel_Init(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModel(opts)
	cmd := model.Init()

	// Init should return nil for table
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestNewTableModelForModal(t *testing.T) {
	opts := TableOptions{
		Columns: []TableColumn{{Title: "Col"}},
		Rows:    [][]string{{"A"}},
		Config:  DefaultConfig(),
	}

	model := NewTableModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestTableFromCSV_WithHeader(t *testing.T) {
	csv := `Name,Age,City
Alice,30,NYC
Bob,25,LA`

	opts := TableFromCSV(csv, ",", true)

	if len(opts.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(opts.Columns))
	}
	if opts.Columns[0].Title != "Name" {
		t.Errorf("expected first column 'Name', got %q", opts.Columns[0].Title)
	}
	if len(opts.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(opts.Rows))
	}
	if opts.Rows[0][0] != "Alice" {
		t.Errorf("expected first cell 'Alice', got %q", opts.Rows[0][0])
	}
}

func TestTableFromCSV_WithoutHeader(t *testing.T) {
	csv := `Alice,30,NYC
Bob,25,LA`

	opts := TableFromCSV(csv, ",", false)

	if len(opts.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(opts.Columns))
	}
	// Columns should have empty titles when no header
	if opts.Columns[0].Title != "" {
		t.Errorf("expected empty column title, got %q", opts.Columns[0].Title)
	}
	if len(opts.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(opts.Rows))
	}
}

func TestTableFromCSV_CustomSeparator(t *testing.T) {
	csv := `Name|Age
Alice|30`

	opts := TableFromCSV(csv, "|", true)

	if len(opts.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(opts.Columns))
	}
	if opts.Columns[0].Title != "Name" {
		t.Errorf("expected column 'Name', got %q", opts.Columns[0].Title)
	}
}

func TestTableFromCSV_EmptyData(t *testing.T) {
	opts := TableFromCSV("", ",", true)

	if len(opts.Columns) != 0 && len(opts.Rows) != 0 {
		t.Error("expected empty table from empty CSV")
	}
}

func TestTableBuilder_FluentAPI(t *testing.T) {
	builder := NewTable().
		Title("Products").
		Columns(
			TableColumn{Title: "ID", Width: 10},
			TableColumn{Title: "Name", Width: 30},
		).
		AddRow("1", "Widget").
		AddRow("2", "Gadget").
		Height(15).
		Width(50).
		Selectable(true).
		SelectedIndex(1).
		Border(true).
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Title != "Products" {
		t.Errorf("expected title 'Products', got %q", builder.opts.Title)
	}
	if len(builder.opts.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(builder.opts.Columns))
	}
	if len(builder.opts.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(builder.opts.Rows))
	}
	if builder.opts.Height != 15 {
		t.Errorf("expected height 15, got %d", builder.opts.Height)
	}
	if builder.opts.Width != 50 {
		t.Errorf("expected width 50, got %d", builder.opts.Width)
	}
	if !builder.opts.Selectable {
		t.Error("expected selectable to be true")
	}
	if builder.opts.SelectedIndex != 1 {
		t.Errorf("expected selected index 1, got %d", builder.opts.SelectedIndex)
	}
	if !builder.opts.Border {
		t.Error("expected border to be true")
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestTableBuilder_ColumnsFromStrings(t *testing.T) {
	builder := NewTable().
		ColumnsFromStrings("Col1", "Col2", "Col3")

	if len(builder.opts.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(builder.opts.Columns))
	}
	if builder.opts.Columns[0].Title != "Col1" {
		t.Errorf("expected column title 'Col1', got %q", builder.opts.Columns[0].Title)
	}
}

func TestTableBuilder_Rows(t *testing.T) {
	builder := NewTable().
		Rows(
			[]string{"A", "B"},
			[]string{"C", "D"},
		)

	if len(builder.opts.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(builder.opts.Rows))
	}
}

func TestTableBuilder_FromCSV(t *testing.T) {
	csv := `Name,Value
Test,123`

	builder := NewTable().
		FromCSV(csv, ",", true)

	if len(builder.opts.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(builder.opts.Columns))
	}
	if len(builder.opts.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(builder.opts.Rows))
	}
}

func TestTableBuilder_Model(t *testing.T) {
	builder := NewTable().
		ColumnsFromStrings("Col").
		AddRow("A")

	model := builder.Model()

	if model == nil {
		t.Fatal("expected non-nil model from builder")
	}
	if model.IsDone() {
		t.Error("expected model not to be done")
	}
}

func TestTableBuilder_DefaultValues(t *testing.T) {
	builder := NewTable()

	if !builder.opts.Selectable {
		t.Error("expected selectable to default to true")
	}
	if !builder.opts.Border {
		t.Error("expected border to default to true")
	}
}

func TestTableSelectionResult_Fields(t *testing.T) {
	result := TableSelectionResult{
		SelectedIndex: 2,
		SelectedRow:   []string{"A", "B", "C"},
	}

	if result.SelectedIndex != 2 {
		t.Errorf("expected SelectedIndex 2, got %d", result.SelectedIndex)
	}
	if len(result.SelectedRow) != 3 {
		t.Errorf("expected 3 cells in row, got %d", len(result.SelectedRow))
	}
}

func TestTableColumn_Fields(t *testing.T) {
	col := TableColumn{
		Title: "Test Column",
		Width: 25,
	}

	if col.Title != "Test Column" {
		t.Errorf("expected title 'Test Column', got %q", col.Title)
	}
	if col.Width != 25 {
		t.Errorf("expected width 25, got %d", col.Width)
	}
}

func TestTableOptions_Fields(t *testing.T) {
	opts := TableOptions{
		Title: "My Table",
		Columns: []TableColumn{
			{Title: "A", Width: 10},
		},
		Rows: [][]string{
			{"1"},
		},
		Height:        20,
		Width:         80,
		Selectable:    true,
		SelectedIndex: 0,
		Separator:     "|",
		Border:        true,
		Config: Config{
			Theme:      ThemeDracula,
			Accessible: true,
		},
	}

	if opts.Title != "My Table" {
		t.Errorf("expected title 'My Table', got %q", opts.Title)
	}
	if len(opts.Columns) != 1 {
		t.Errorf("expected 1 column, got %d", len(opts.Columns))
	}
	if len(opts.Rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(opts.Rows))
	}
	if opts.Height != 20 {
		t.Errorf("expected height 20, got %d", opts.Height)
	}
	if opts.Width != 80 {
		t.Errorf("expected width 80, got %d", opts.Width)
	}
	if !opts.Selectable {
		t.Error("expected selectable to be true")
	}
	if opts.SelectedIndex != 0 {
		t.Errorf("expected selected index 0, got %d", opts.SelectedIndex)
	}
	if opts.Separator != "|" {
		t.Errorf("expected separator '|', got %q", opts.Separator)
	}
	if !opts.Border {
		t.Error("expected border to be true")
	}
	if opts.Config.Theme != ThemeDracula {
		t.Errorf("expected theme ThemeDracula, got %v", opts.Config.Theme)
	}
	if !opts.Config.Accessible {
		t.Error("expected accessible to be true")
	}
}
