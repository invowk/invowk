// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewInputModel(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:       "Enter your name",
		Description: "We need your name for identification",
		Placeholder: "John Doe",
		Value:       "Initial",
		CharLimit:   50,
		Width:       40,
		Password:    false,
		Prompt:      "> ",
		Config:      DefaultConfig(),
	}

	model := NewInputModel(opts)

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

func TestNewInputModel_Password(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:    "Enter password",
		Password: true,
		Config:   DefaultConfig(),
	}

	model := NewInputModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Password mode is configured in the underlying form
	// We can't directly test it, but construction should work
}

func TestInputModel_CancelWithEsc(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Test input",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)

	// Simulate Esc key press
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*inputModel)

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

func TestInputModel_CancelWithCtrlC(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Test input",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)

	// Simulate Ctrl+C key press
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*inputModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Ctrl+C")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Ctrl+C")
	}
}

func TestInputModel_SetSize(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)
	model.SetSize(120, 40)

	if model.width != 120 {
		t.Errorf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Errorf("expected height 40, got %d", model.height)
	}
}

func TestInputModel_ViewWhenDone(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)
	model.done = true

	view := model.View()

	if view != "" {
		t.Errorf("expected empty view when done, got %q", view)
	}
}

func TestInputModel_ViewWithWidth(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)
	model.SetSize(50, 10)

	view := model.View()

	// View should be non-empty when not done
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestInputModel_Init(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Test",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)
	cmd := model.Init()

	// Init should return a command from the underlying form
	// We just verify it doesn't panic
	_ = cmd
}

func TestNewInputModelForModal(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Modal Input",
		Config: DefaultConfig(),
	}

	model := NewInputModelForModal(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Modal model should work the same way
	if model.IsDone() {
		t.Error("expected model not to be done initially")
	}
}

func TestInputBuilder_FluentAPI(t *testing.T) {
	t.Parallel()

	builder := NewInput().
		Title("Username").
		Description("Enter your username").
		Placeholder("user123").
		Value("defaultuser").
		CharLimit(20).
		Width(30).
		Prompt(">> ").
		Theme(ThemeCharm).
		Accessible(true)

	if builder.opts.Title != "Username" {
		t.Errorf("expected title 'Username', got %q", builder.opts.Title)
	}
	if builder.opts.Description != "Enter your username" {
		t.Errorf("expected description, got %q", builder.opts.Description)
	}
	if builder.opts.Placeholder != "user123" {
		t.Errorf("expected placeholder 'user123', got %q", builder.opts.Placeholder)
	}
	if builder.opts.Value != "defaultuser" {
		t.Errorf("expected value 'defaultuser', got %q", builder.opts.Value)
	}
	if builder.opts.CharLimit != 20 {
		t.Errorf("expected char limit 20, got %d", builder.opts.CharLimit)
	}
	if builder.opts.Width != 30 {
		t.Errorf("expected width 30, got %d", builder.opts.Width)
	}
	if builder.opts.Prompt != ">> " {
		t.Errorf("expected prompt '>> ', got %q", builder.opts.Prompt)
	}
	if builder.opts.Config.Theme != ThemeCharm {
		t.Errorf("expected theme ThemeCharm, got %v", builder.opts.Config.Theme)
	}
	if !builder.opts.Config.Accessible {
		t.Error("expected accessible mode to be enabled")
	}
}

func TestInputBuilder_Password(t *testing.T) {
	t.Parallel()

	builder := NewInput().
		Title("Enter password").
		Password()

	if !builder.opts.Password {
		t.Error("expected password mode to be enabled")
	}
}

func TestInputBuilder_Model(t *testing.T) {
	t.Parallel()

	builder := NewInput().
		Title("Test")

	model := builder.Model()

	if model == nil {
		t.Fatal("expected non-nil model from builder")
	}
	if model.IsDone() {
		t.Error("expected model not to be done")
	}
}

func TestInputOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:       "Email",
		Description: "Your email address",
		Placeholder: "user@example.com",
		Value:       "test@test.com",
		CharLimit:   100,
		Width:       60,
		Password:    false,
		Prompt:      "→ ",
		Config: Config{
			Theme:      ThemeCatppuccin,
			Accessible: true,
			Width:      80,
		},
	}

	if opts.Title != "Email" {
		t.Errorf("expected title 'Email', got %q", opts.Title)
	}
	if opts.Description != "Your email address" {
		t.Errorf("expected description 'Your email address', got %q", opts.Description)
	}
	if opts.Placeholder != "user@example.com" {
		t.Errorf("expected placeholder 'user@example.com', got %q", opts.Placeholder)
	}
	if opts.Value != "test@test.com" {
		t.Errorf("expected value 'test@test.com', got %q", opts.Value)
	}
	if opts.CharLimit != 100 {
		t.Errorf("expected char limit 100, got %d", opts.CharLimit)
	}
	if opts.Width != 60 {
		t.Errorf("expected width 60, got %d", opts.Width)
	}
	if opts.Password {
		t.Error("expected password to be false")
	}
	if opts.Prompt != "→ " {
		t.Errorf("expected prompt '→ ', got %q", opts.Prompt)
	}
	if opts.Config.Theme != ThemeCatppuccin {
		t.Errorf("expected theme ThemeCatppuccin, got %v", opts.Config.Theme)
	}
	if !opts.Config.Accessible {
		t.Error("expected accessible to be true")
	}
	if opts.Config.Width != 80 {
		t.Errorf("expected config width 80, got %d", opts.Config.Width)
	}
}

func TestNewInputModel_WithInitialValue(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:  "Pre-filled",
		Value:  "Hello World",
		Config: DefaultConfig(),
	}

	model := NewInputModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// The initial value is configured in the underlying form
	// We can't directly access it, but construction should work
}

func TestNewInputModel_WithCharLimit(t *testing.T) {
	t.Parallel()

	opts := InputOptions{
		Title:     "Limited",
		CharLimit: 10,
		Config:    DefaultConfig(),
	}

	model := NewInputModel(opts)

	if model == nil {
		t.Fatal("expected non-nil model")
	}
	// Char limit is configured in the underlying form
}

func TestInputBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	builder := NewInput()

	// Default config should be set
	if builder.opts.Config.Theme != ThemeDefault {
		t.Errorf("expected default theme, got %v", builder.opts.Config.Theme)
	}
	// Password should default to false
	if builder.opts.Password {
		t.Error("expected password to default to false")
	}
}
