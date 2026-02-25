// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewConfirmModel(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:       "Delete file?",
		Description: "This action cannot be undone",
		Affirmative: "Yes",
		Negative:    "No",
		Default:     true,
		Config:      DefaultConfig(),
	}

	model := NewConfirmModel(opts)

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

func TestNewConfirmModel_DefaultFalse(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:   "Proceed?",
		Default: false,
		Config:  DefaultConfig(),
	}

	model := NewConfirmModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Default value is stored in the result pointer
	// We can't directly test it without form submission
}

func TestConfirmModel_CancelWithEsc(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Confirm?",
		Config: DefaultConfig(),
	}

	model := NewConfirmModel(opts)

	// Simulate Esc key press
	keyMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*confirmModel)

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

func TestConfirmModel_CancelWithCtrlC(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Confirm?",
		Config: DefaultConfig(),
	}

	model := NewConfirmModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*confirmModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Ctrl+C")
	}
}

func TestConfirmModel_SetSize(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewConfirmModel(opts)
	model.SetSize(100, 30)

	if model.width != 100 {
		t.Errorf("expected width 100, got %d", model.width)
	}
	if model.height != 30 {
		t.Errorf("expected height 30, got %d", model.height)
	}
}

func TestConfirmModel_ViewWhenDone(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewConfirmModel(opts)
	model.done = true

	view := model.View().Content

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestConfirmModel_ViewWithWidth(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewConfirmModel(opts)
	model.SetSize(60, 10)

	view := model.View().Content

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestConfirmModel_Init(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewConfirmModel(opts)
	cmd := model.Init()

	// Init should return a command from the underlying form
	// We just verify it doesn't panic
	_ = cmd
}

func TestNewConfirmModelForModal(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:  "Modal Confirm",
		Config: DefaultConfig(),
	}

	model := NewConfirmModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Modal model should work the same way
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
}

func TestConfirmBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := NewConfirm().
		Title("Delete all files?").
		Description("This will permanently remove all data").
		Affirmative("Delete").
		Negative("Cancel").
		Default(false).
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Title != "Delete all files?" {
		t.Errorf("expected title 'Delete all files?', got %q", builder.opts.Title)
	}
	if builder.opts.Description != "This will permanently remove all data" {
		t.Errorf("expected description, got %q", builder.opts.Description)
	}
	if builder.opts.Affirmative != "Delete" {
		t.Errorf("expected affirmative 'Delete', got %q", builder.opts.Affirmative)
	}
	if builder.opts.Negative != "Cancel" {
		t.Errorf("expected negative 'Cancel', got %q", builder.opts.Negative)
	}
	if builder.opts.Default != false {
		t.Error("expected default to be false")
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestConfirmBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	builder := NewConfirm()

	// Check default values set by NewConfirm
	if builder.opts.Affirmative != "Yes" {
		t.Errorf("expected default affirmative 'Yes', got %q", builder.opts.Affirmative)
	}
	if builder.opts.Negative != "No" {
		t.Errorf("expected default negative 'No', got %q", builder.opts.Negative)
	}
	if builder.opts.Default != true {
		t.Error("expected default to be true")
	}
}

func TestConfirmBuilder_Model(t *testing.T) {
	t.Parallel()

	builder := NewConfirm().
		Title("Test")

	model := builder.Model()

	if model == nil {
		t.Fatal("expected non-nil model from builder")
	}
	if model.IsDone() {
		t.Error("expected model not to be done")
	}
}

func TestConfirmOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := ConfirmOptions{
		Title:       "Are you sure?",
		Description: "Please confirm",
		Affirmative: "OK",
		Negative:    "Back",
		Default:     true,
		Config: Config{
			Theme:      ThemeDracula,
			Accessible: false,
		},
	}

	if opts.Title != "Are you sure?" {
		t.Errorf("expected title 'Are you sure?', got %q", opts.Title)
	}
	if opts.Description != "Please confirm" {
		t.Errorf("expected description 'Please confirm', got %q", opts.Description)
	}
	if opts.Affirmative != "OK" {
		t.Errorf("expected affirmative 'OK', got %q", opts.Affirmative)
	}
	if opts.Negative != "Back" {
		t.Errorf("expected negative 'Back', got %q", opts.Negative)
	}
	if !opts.Default {
		t.Error("expected default to be true")
	}
	if opts.Config.Theme != ThemeDracula {
		t.Errorf("expected theme ThemeDracula, got %v", opts.Config.Theme)
	}
}
