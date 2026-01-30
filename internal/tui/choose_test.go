// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewChooseModel_SingleSelect(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Select an option",
		Options: []string{"Option A", "Option B", "Option C"},
		Limit:   0, // Single-select mode
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if model.isMulti {
		t.Error("expected single-select mode when Limit=0")
	}
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
	if model.Cancelled() {
		t.Error("expected model not to be cancelled initially")
	}
}

func TestNewChooseModel_MultiSelect(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Select multiple",
		Options: []string{"A", "B", "C"},
		Limit:   3, // Multi-select mode
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if !model.isMulti {
		t.Error("expected multi-select mode when Limit>1")
	}
}

func TestNewChooseModel_NoLimit(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Select any",
		Options: []string{"A", "B", "C"},
		NoLimit: true, // Unlimited selections
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	if !model.isMulti {
		t.Error("expected multi-select mode when NoLimit=true")
	}
}

func TestChooseModel_CancelWithEsc(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Simulate Esc key press
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*chooseModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Esc")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Esc")
	}

	// Result should return ErrCancelled
	_, err := m.Result()
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
}

func TestChooseModel_CancelWithCtrlC(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*chooseModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Ctrl+C")
	}
}

func TestChooseModel_SetSize(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)
	model.SetSize(80, 24)

	if model.width != 80 {
		t.Errorf("expected width 80, got %d", model.width)
	}
	if model.height != 24 {
		t.Errorf("expected height 24, got %d", model.height)
	}
}

func TestChooseModel_ViewWhenDone(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)
	model.done = true

	view := model.View()

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestChooseModel_ViewWithWidth(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)
	model.SetSize(40, 10)

	view := model.View()

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestChooseModel_Init(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Test",
		Options: []string{"A", "B"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)
	cmd := model.Init()

	// Init should return a command from the underlying form
	// We just verify it doesn't panic
	_ = cmd
}

func TestNewChooseModelForModal(t *testing.T) {
	opts := ChooseStringOptions{
		Title:   "Modal Select",
		Options: []string{"X", "Y", "Z"},
		Config:  DefaultConfig(),
	}

	model := NewChooseModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Modal model should work the same way
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
}

func TestChooseBuilder_FluentAPI(t *testing.T) {
	builder := NewChoose[string]().
		Title("Pick one").
		Description("Choose wisely").
		Options(
			Option[string]{Title: "First", Value: "1"},
			Option[string]{Title: "Second", Value: "2"},
		).
		Height(5).
		Cursor("> ").
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Title != "Pick one" {
		t.Errorf("expected title 'Pick one', got %q", builder.opts.Title)
	}
	if builder.opts.Description != "Choose wisely" {
		t.Errorf("expected description 'Choose wisely', got %q", builder.opts.Description)
	}
	if len(builder.opts.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(builder.opts.Options))
	}
	if builder.opts.Height != 5 {
		t.Errorf("expected height 5, got %d", builder.opts.Height)
	}
	if builder.opts.Cursor != "> " {
		t.Errorf("expected cursor '> ', got %q", builder.opts.Cursor)
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestChooseBuilder_OptionsFromSlice(t *testing.T) {
	values := []string{"apple", "banana", "cherry"}
	builder := NewChoose[string]().
		OptionsFromSlice(values, func(s string) string {
			return "Fruit: " + s
		})

	if len(builder.opts.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(builder.opts.Options))
	}
	if builder.opts.Options[0].Title != "Fruit: apple" {
		t.Errorf("expected title 'Fruit: apple', got %q", builder.opts.Options[0].Title)
	}
	if builder.opts.Options[0].Value != "apple" {
		t.Errorf("expected value 'apple', got %q", builder.opts.Options[0].Value)
	}
}

func TestMultiChooseBuilder_FluentAPI(t *testing.T) {
	builder := NewMultiChoose[string]().
		Title("Select many").
		Description("Pick multiple").
		Options(
			Option[string]{Title: "A", Value: "a"},
			Option[string]{Title: "B", Value: "b", Selected: true},
		).
		Limit(3).
		Height(8).
		Theme(ThemeDracula).
		Accessible(false)

	if builder.opts.Title != "Select many" {
		t.Errorf("expected title 'Select many', got %q", builder.opts.Title)
	}
	if builder.opts.Description != "Pick multiple" {
		t.Errorf("expected description 'Pick multiple', got %q", builder.opts.Description)
	}
	if len(builder.opts.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(builder.opts.Options))
	}
	if builder.opts.Limit != 3 {
		t.Errorf("expected limit 3, got %d", builder.opts.Limit)
	}
	if builder.opts.Height != 8 {
		t.Errorf("expected height 8, got %d", builder.opts.Height)
	}
	if builder.opts.Config.Theme != ThemeDracula {
		t.Errorf("expected theme ThemeDracula, got %v", builder.opts.Config.Theme)
	}
}

func TestChooseStringBuilder_FluentAPI(t *testing.T) {
	builder := NewChooseString().
		Title("Choose string").
		Options("opt1", "opt2", "opt3").
		Limit(2).
		Height(10).
		Theme(ThemeCatppuccin).
		Accessible(true)

	if builder.opts.Title != "Choose string" {
		t.Errorf("expected title 'Choose string', got %q", builder.opts.Title)
	}
	if len(builder.opts.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(builder.opts.Options))
	}
	if builder.opts.Limit != 2 {
		t.Errorf("expected limit 2, got %d", builder.opts.Limit)
	}
	if builder.opts.Height != 10 {
		t.Errorf("expected height 10, got %d", builder.opts.Height)
	}
	if builder.opts.Config.Theme != ThemeCatppuccin {
		t.Errorf("expected theme ThemeCatppuccin, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestChooseStringBuilder_NoLimit(t *testing.T) {
	builder := NewChooseString().
		Title("Unlimited").
		Options("A", "B", "C").
		NoLimit()

	if !builder.opts.NoLimit {
		t.Error("expected NoLimit to be true")
	}
}

func TestChooseStringBuilder_Model(t *testing.T) {
	builder := NewChooseString().
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

func TestOption_Fields(t *testing.T) {
	opt := Option[int]{
		Title:    "Number One",
		Value:    1,
		Selected: true,
	}

	if opt.Title != "Number One" {
		t.Errorf("expected title 'Number One', got %q", opt.Title)
	}
	if opt.Value != 1 {
		t.Errorf("expected value 1, got %d", opt.Value)
	}
	if !opt.Selected {
		t.Error("expected Selected to be true")
	}
}
