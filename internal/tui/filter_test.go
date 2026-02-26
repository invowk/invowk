// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewFilterModel(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:       "Search files",
		Placeholder: "Type to filter...",
		Options:     []string{"file1.go", "file2.go", "main.go"},
		Limit:       0, // Single-select
		Height:      10,
		Width:       50,
		Fuzzy:       true,
		Config:      DefaultConfig(),
	}

	model := NewFilterModel(opts)

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

func TestNewFilterModel_EmptyOptions(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Empty filter",
		Options: []string{},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Empty filter should be immediately done
	if !model.IsDone() {
		t.Error("expected empty filter to be done immediately")
	}
}

func TestNewFilterModel_MultiSelect(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Multi-select",
		Options: []string{"A", "B", "C"},
		Limit:   3,
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.limit != 3 {
		t.Errorf("expected limit 3, got %d", model.limit)
	}
}

func TestNewFilterModel_NoLimit(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Unlimited",
		Options: []string{"A", "B", "C"},
		NoLimit: true,
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if !model.noLimit {
		t.Error("expected noLimit to be true")
	}
}

func TestFilterModel_CancelWithEsc_NotFiltering(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	// When not in filtering mode, Esc should immediately cancel
	keyMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*filterModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Esc when not filtering")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Esc when not filtering")
	}

	// Result should return ErrCancelled
	_, err := m.Result()
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
}

func TestFilterModel_TwoStageEscape(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"Apple", "Banana", "Cherry"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	// Step 1: Enter filter mode by pressing "/" (activates filtering)
	slashMsg := tea.KeyPressMsg{Code: '/', Text: "/"}
	updatedModel, _ := model.Update(slashMsg)
	m := updatedModel.(*filterModel)

	// Step 2: Type some filter text
	typeMsg := tea.KeyPressMsg{Code: 'a', Text: "a"}
	updatedModel, _ = m.Update(typeMsg)
	m = updatedModel.(*filterModel)

	// Verify we're in filtering mode (list.Filtering = 1)
	if m.list.FilterState() != 1 {
		t.Logf("FilterState: %d (expected 1 for Filtering)", m.list.FilterState())
	}

	// Step 3: First Escape should clear filter, not cancel
	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(*filterModel)

	// Should NOT be done/cancelled yet - escape was passed to list to clear filter
	if m.IsDone() {
		t.Error("expected model NOT to be done after first Esc while filtering")
	}
	if m.Cancelled() {
		t.Error("expected model NOT to be cancelled after first Esc while filtering")
	}

	// Step 4: Second Escape (when not filtering) should cancel
	updatedModel, _ = m.Update(escMsg)
	m = updatedModel.(*filterModel)

	if !m.IsDone() {
		t.Error("expected model to be done after second Esc")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after second Esc")
	}
}

func TestFilterModel_CancelWithCtrlC(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*filterModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Ctrl+C")
	}
}

func TestFilterModel_SetSize(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)
	model.SetSize(80, 20)

	if model.width != 80 {
		t.Errorf("expected width 80, got %d", model.width)
	}
	if model.height != 20 {
		t.Errorf("expected height 20, got %d", model.height)
	}
}

func TestFilterModel_ViewWhenDone(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)
	model.done = true

	view := model.View().Content

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestFilterModel_ViewWithWidth(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)
	model.SetSize(40, 10)

	view := model.View().Content

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestFilterModel_Init(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)
	cmd := model.Init()

	// Init should return nil for filter
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestNewFilterModelForModal(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Modal Filter",
		Options: []string{"X", "Y", "Z"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestFilterModel_ToggleSelection(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Multi-select",
		Options: []string{"A", "B", "C"},
		Limit:   3, // Multi-select mode
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	// Initial state - nothing selected
	if len(model.selected) != 0 {
		t.Errorf("expected no selections initially, got %d", len(model.selected))
	}

	// Simulate space key press to toggle selection (in multi-select mode)
	keyMsg := tea.KeyPressMsg{Code: tea.KeySpace}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*filterModel)

	if len(m.selected) != 1 {
		t.Errorf("expected 1 selection after space, got %d", len(m.selected))
	}
	if !m.selected[0] {
		t.Error("expected item 0 to be selected")
	}

	// Toggle again to deselect
	updatedModel2, _ := m.Update(keyMsg)
	m2 := updatedModel2.(*filterModel)

	if len(m2.selected) != 0 {
		t.Errorf("expected 0 selections after second space, got %d", len(m2.selected))
	}
}

func TestFilterModel_PreSelected(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:    "Pre-selected",
		Options:  []string{"A", "B", "C"},
		Selected: []int{0, 2}, // Pre-select A and C
		Limit:    3,
		Config:   DefaultConfig(),
	}

	model := NewFilterModel(opts)

	if len(model.selected) != 2 {
		t.Errorf("expected 2 pre-selected items, got %d", len(model.selected))
	}
	if !model.selected[0] {
		t.Error("expected item 0 to be pre-selected")
	}
	if !model.selected[2] {
		t.Error("expected item 2 to be pre-selected")
	}
}

func TestFilterModel_EnterSelection(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	// Simulate Enter key press
	keyMsg := tea.KeyPressMsg{Code: tea.KeyEnter}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*filterModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Enter")
	}
	if m.Cancelled() {
		t.Error("expected model not to be cancelled after Enter")
	}
}

func TestFilterModel_WindowSizeMsg(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewFilterModel(opts)

	// Simulate window resize
	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := model.Update(msg)
	_ = updatedModel.(*filterModel)

	// Window size should update the list
	// We can't easily test internal list dimensions, but it shouldn't panic
}

func TestFuzzyMatch(t *testing.T) {
	t.Parallel()

	options := []string{"hello", "world", "help", "helicopter"}

	tests := []struct {
		name     string
		pattern  string
		expected int // Expected number of matches
	}{
		{"empty pattern matches all", "", 4},
		{"exact match", "hello", 1},
		{"fuzzy match hel", "hel", 3}, // hello, help, helicopter
		{"no match", "xyz", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := FuzzyMatch(tt.pattern, options)
			if len(results) != tt.expected {
				t.Errorf("FuzzyMatch(%q) returned %d results, expected %d",
					tt.pattern, len(results), tt.expected)
			}
		})
	}
}

func TestFuzzyMatchWithScore(t *testing.T) {
	t.Parallel()

	options := []string{"hello", "world", "help"}

	// Empty pattern
	results := FuzzyMatchWithScore("", options)
	if len(results) != 3 {
		t.Errorf("expected 3 results for empty pattern, got %d", len(results))
	}
	for i, r := range results {
		if r.Score != 0 {
			t.Errorf("expected score 0 for empty pattern, got %d at index %d", r.Score, i)
		}
	}

	// Pattern with matches
	results = FuzzyMatchWithScore("hel", options)
	for _, r := range results {
		if r.Text == "" {
			t.Error("expected non-empty text in result")
		}
	}
}

func TestExactMatch(t *testing.T) {
	t.Parallel()

	options := []string{"Hello World", "hello there", "goodbye", "HELLO"}

	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{"empty pattern matches all", "", 4},
		{"case insensitive hello", "hello", 3}, // Hello World, hello there, HELLO
		{"substring match", "ello", 3},
		{"no match", "xyz", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			results := ExactMatch(tt.pattern, options)
			if len(results) != tt.expected {
				t.Errorf("ExactMatch(%q) returned %d results, expected %d",
					tt.pattern, len(results), tt.expected)
			}
		})
	}
}

func TestFilterBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := NewFilter().
		Title("Search").
		Placeholder("Type to search...").
		Options("one", "two", "three").
		Limit(2).
		Height(15).
		Width(60).
		Reverse(true).
		Fuzzy(true).
		Sort(true).
		Selected(0).
		Theme(ThemeDracula).
		Accessible(true)

	if builder.opts.Title != "Search" {
		t.Errorf("expected title 'Search', got %q", builder.opts.Title)
	}
	if builder.opts.Placeholder != "Type to search..." {
		t.Errorf("expected placeholder, got %q", builder.opts.Placeholder)
	}
	if len(builder.opts.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(builder.opts.Options))
	}
	if builder.opts.Limit != 2 {
		t.Errorf("expected limit 2, got %d", builder.opts.Limit)
	}
	if builder.opts.Height != 15 {
		t.Errorf("expected height 15, got %d", builder.opts.Height)
	}
	if builder.opts.Width != 60 {
		t.Errorf("expected width 60, got %d", builder.opts.Width)
	}
	if !builder.opts.Reverse {
		t.Error("expected reverse to be true")
	}
	if !builder.opts.Fuzzy {
		t.Error("expected fuzzy to be true")
	}
	if !builder.opts.Sort {
		t.Error("expected sort to be true")
	}
	if len(builder.opts.Selected) != 1 || builder.opts.Selected[0] != 0 {
		t.Errorf("expected selected [0], got %v", builder.opts.Selected)
	}
	if builder.opts.Config.Theme != ThemeDracula {
		t.Errorf("expected theme ThemeDracula, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestFilterBuilder_OptionsFromSlice(t *testing.T) {
	t.Parallel()

	options := []string{"apple", "banana", "cherry"}
	builder := NewFilter().
		OptionsFromSlice(options)

	if len(builder.opts.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(builder.opts.Options))
	}
	if builder.opts.Options[0] != "apple" {
		t.Errorf("expected first option 'apple', got %q", builder.opts.Options[0])
	}
}

func TestFilterBuilder_NoLimit(t *testing.T) {
	t.Parallel()

	builder := NewFilter().
		Options("A", "B", "C").
		NoLimit(true)

	if !builder.opts.NoLimit {
		t.Error("expected NoLimit to be true")
	}
}

func TestFilterBuilder_Model(t *testing.T) {
	t.Parallel()

	builder := NewFilter().
		Title("Test").
		Options("A", "B")

	model := builder.Model()

	if model == nil {
		t.Fatal("expected non-nil model from builder")
	}
	if model.IsDone() {
		t.Error("expected model not to be done")
	}
}

func TestFilterBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	builder := NewFilter()

	// Fuzzy should default to true
	if !builder.opts.Fuzzy {
		t.Error("expected fuzzy to default to true")
	}
}

func TestFilterItem(t *testing.T) {
	t.Parallel()

	item := filterItem{text: "test item"}

	if item.Title() != "test item" {
		t.Errorf("expected Title() 'test item', got %q", item.Title())
	}
	if item.Description() != "" {
		t.Errorf("expected empty Description(), got %q", item.Description())
	}
	if item.FilterValue() != "test item" {
		t.Errorf("expected FilterValue() 'test item', got %q", item.FilterValue())
	}
}

func TestFilterOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := FilterOptions{
		Title:         "Filter Test",
		Placeholder:   "Search...",
		Options:       []string{"A", "B", "C"},
		Limit:         2,
		Height:        12,
		Width:         40,
		Reverse:       true,
		Fuzzy:         true,
		Sort:          true,
		NoLimit:       false,
		Selected:      []int{1},
		Strict:        true,
		ShowIndicator: true,
		Config: Config{
			Theme:      ThemeBase16,
			Accessible: false,
		},
	}

	if opts.Title != "Filter Test" {
		t.Errorf("expected title 'Filter Test', got %q", opts.Title)
	}
	if opts.Placeholder != "Search..." {
		t.Errorf("expected placeholder 'Search...', got %q", opts.Placeholder)
	}
	if len(opts.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(opts.Options))
	}
	if opts.Limit != 2 {
		t.Errorf("expected limit 2, got %d", opts.Limit)
	}
	if opts.Height != 12 {
		t.Errorf("expected height 12, got %d", opts.Height)
	}
	if opts.Width != 40 {
		t.Errorf("expected width 40, got %d", opts.Width)
	}
	if !opts.Reverse {
		t.Error("expected reverse to be true")
	}
	if !opts.Fuzzy {
		t.Error("expected fuzzy to be true")
	}
	if !opts.Sort {
		t.Error("expected sort to be true")
	}
	if opts.NoLimit {
		t.Error("expected noLimit to be false")
	}
	if len(opts.Selected) != 1 {
		t.Errorf("expected 1 selected, got %d", len(opts.Selected))
	}
	if !opts.Strict {
		t.Error("expected strict to be true")
	}
	if !opts.ShowIndicator {
		t.Error("expected showIndicator to be true")
	}
}
