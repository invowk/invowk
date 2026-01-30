// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewPagerModel(t *testing.T) {
	opts := PagerOptions{
		Content:         "Line 1\nLine 2\nLine 3",
		Title:           "Test Pager",
		Height:          20,
		Width:           80,
		ShowLineNumbers: true,
		SoftWrap:        true,
		Config:          DefaultConfig(),
	}

	model := NewPagerModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
	if model.title != "Test Pager" {
		t.Errorf("expected title 'Test Pager', got %q", model.title)
	}
}

func TestNewPagerModel_DefaultDimensions(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Title:   "Test",
		// Height and Width default to 0
		Config: DefaultConfig(),
	}

	model := NewPagerModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Defaults should be applied
	if model.height == 0 {
		t.Error("expected default height to be set")
	}
	if model.width == 0 {
		t.Error("expected default width to be set")
	}
}

func TestPagerModel_DismissWithQ(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Simulate 'q' key press
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*pagerModel)

	if !m.IsDone() {
		t.Error("expected model to be done after 'q'")
	}
}

func TestPagerModel_DismissWithEsc(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Simulate Esc key press
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*pagerModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Esc")
	}
}

func TestPagerModel_DismissWithEnter(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Simulate Enter key press
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*pagerModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Enter")
	}
}

func TestPagerModel_DismissWithCtrlC(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*pagerModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
}

func TestPagerModel_Result(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)
	model.done = true

	result, err := model.Result()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Pager has no result value
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestPagerModel_Cancelled(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Pager doesn't have a cancel concept
	if model.Cancelled() {
		t.Error("expected Cancelled to return false")
	}
}

func TestPagerModel_SetSize(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)
	model.SetSize(100, 40)

	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
}

func TestPagerModel_ViewWhenDone(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)
	model.done = true

	view := model.View()

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestPagerModel_ViewWithTitle(t *testing.T) {
	opts := PagerOptions{
		Content: "Line 1\nLine 2",
		Title:   "My Title",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)
	model.SetSize(60, 20)

	view := model.View()

	// View should contain the title
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestPagerModel_ViewWithWidth(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)
	model.SetSize(50, 10)

	view := model.View()

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestPagerModel_Init(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)
	cmd := model.Init()

	// Init should return nil for pager
	if cmd != nil {
		t.Error("expected nil cmd from Init")
	}
}

func TestPagerModel_WindowSizeMsg(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Simulate window resize
	msg := tea.WindowSizeMsg{Width: 120, Height: 50}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*pagerModel)

	// Viewport dimensions should be updated
	// We can't easily verify internal viewport dimensions
	// but the update shouldn't panic
	_ = m
}

func TestNewPagerModelForModal(t *testing.T) {
	opts := PagerOptions{
		Content: "Modal content",
		Title:   "Modal Pager",
		Config:  DefaultConfig(),
	}

	model := NewPagerModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if !model.forModal {
		t.Error("expected forModal to be true")
	}
}

func TestPagerModel_ViewForModal(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Title:   "Title",
		Config:  DefaultConfig(),
	}

	model := NewPagerModelForModal(opts)
	model.SetSize(60, 15)

	view := model.View()

	// Modal view should be rendered
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestPagerBuilder_FluentAPI(t *testing.T) {
	builder := NewPager().
		Content("Long content here").
		Title("Document Title").
		Height(30).
		Width(100).
		ShowLineNumbers(true).
		SoftWrap(true).
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Content != "Long content here" {
		t.Errorf("expected content 'Long content here', got %q", builder.opts.Content)
	}
	if builder.opts.Title != "Document Title" {
		t.Errorf("expected title 'Document Title', got %q", builder.opts.Title)
	}
	if builder.opts.Height != 30 {
		t.Errorf("expected height 30, got %d", builder.opts.Height)
	}
	if builder.opts.Width != 100 {
		t.Errorf("expected width 100, got %d", builder.opts.Width)
	}
	if !builder.opts.ShowLineNumbers {
		t.Error("expected show line numbers to be true")
	}
	if !builder.opts.SoftWrap {
		t.Error("expected soft wrap to be true")
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestPagerBuilder_Model(t *testing.T) {
	builder := NewPager().
		Content("Test content").
		Title("Test")

	model := builder.Model()

	if model == nil {
		t.Fatal("expected non-nil model from builder")
	}
	if model.IsDone() {
		t.Error("expected model not to be done")
	}
}

func TestPagerBuilder_DefaultValues(t *testing.T) {
	builder := NewPager()

	// Default config should be set
	if builder.opts.Config.Theme != ThemeDefault {
		t.Errorf("expected default theme, got %v", builder.opts.Config.Theme)
	}
}

func TestPagerOptions_Fields(t *testing.T) {
	opts := PagerOptions{
		Content:         "Multi\nLine\nContent",
		Title:           "Pager Title",
		Height:          25,
		Width:           90,
		ShowLineNumbers: true,
		SoftWrap:        true,
		Config: Config{
			Theme:      ThemeCatppuccin,
			Accessible: true,
		},
	}

	if opts.Content != "Multi\nLine\nContent" {
		t.Errorf("expected content, got %q", opts.Content)
	}
	if opts.Title != "Pager Title" {
		t.Errorf("expected title 'Pager Title', got %q", opts.Title)
	}
	if opts.Height != 25 {
		t.Errorf("expected height 25, got %d", opts.Height)
	}
	if opts.Width != 90 {
		t.Errorf("expected width 90, got %d", opts.Width)
	}
	if !opts.ShowLineNumbers {
		t.Error("expected show line numbers to be true")
	}
	if !opts.SoftWrap {
		t.Error("expected soft wrap to be true")
	}
	if opts.Config.Theme != ThemeCatppuccin {
		t.Errorf("expected theme ThemeCatppuccin, got %v", opts.Config.Theme)
	}
	if !opts.Config.Accessible {
		t.Error("expected accessible to be true")
	}
}

func TestPagerModel_ReadyState(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Model should start ready (dimensions set in constructor)
	if !model.ready {
		t.Error("expected model to be ready initially")
	}
}

func TestPagerModel_SmallHeight(t *testing.T) {
	opts := PagerOptions{
		Content: "Content",
		Height:  2, // Very small height
		Config:  DefaultConfig(),
	}

	model := NewPagerModel(opts)

	// Should handle small heights gracefully
	if model == nil {
		t.Fatal("expected non-nil model")
	}
}
