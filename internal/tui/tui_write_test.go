// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewWriteModel(t *testing.T) {
	t.Parallel()

	opts := WriteOptions{
		Title:       "Write something",
		Description: "Describe your changes",
		Placeholder: "Type here",
		Value:       "Initial value",
		Config:      DefaultConfig(),
	}

	model := NewWriteModel(opts)

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

func TestWriteModel_CancelWithEsc(t *testing.T) {
	t.Parallel()

	opts := WriteOptions{
		Title:  "Test write",
		Config: DefaultConfig(),
	}

	model := NewWriteModel(opts)

	keyMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*writeModel)

	if !m.IsDone() {
		t.Error("expected model to be done after Esc")
	}
	if !m.Cancelled() {
		t.Error("expected model to be cancelled after Esc")
	}

	_, err := m.Result()
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("expected ErrCancelled, got %v", err)
	}
}

func TestWriteModel_SubmitWithCtrlD(t *testing.T) {
	t.Parallel()

	model := NewWriteModel(WriteOptions{Value: "draft\ntext", Config: DefaultConfig()})
	updatedModel, cmd := model.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	updated := updatedModel.(*writeModel)

	if !updated.IsDone() {
		t.Fatal("IsDone() = false after Ctrl+D, want true")
	}
	if updated.Cancelled() {
		t.Fatal("Cancelled() = true after Ctrl+D, want false")
	}
	result, err := updated.Result()
	if err != nil {
		t.Fatalf("Result() error = %v", err)
	}
	if result != "draft\ntext" {
		t.Errorf("Result() = %q, want %q", result, "draft\ntext")
	}
	if cmd == nil {
		t.Fatal("Update(Ctrl+D) command = nil, want tea.Quit")
	}
	if view := updated.View().Content; view != "" {
		t.Errorf("completed View() = %q, want empty", view)
	}
}

func TestWriteModel_CancelWithCtrlC(t *testing.T) {
	t.Parallel()

	model := NewWriteModel(WriteOptions{Config: DefaultConfig()})
	updatedModel, cmd := model.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	updated := updatedModel.(*writeModel)

	if !updated.IsDone() || !updated.Cancelled() {
		t.Fatalf("state after Ctrl+C = done %t, cancelled %t; want both true", updated.IsDone(), updated.Cancelled())
	}
	if _, err := updated.Result(); !errors.Is(err, ErrCancelled) {
		t.Errorf("Result() error = %v, want ErrCancelled", err)
	}
	if cmd == nil {
		t.Fatal("Update(Ctrl+C) command = nil, want tea.Quit")
	}
}

func TestWriteModel_SetSize(t *testing.T) {
	t.Parallel()

	model := NewWriteModel(WriteOptions{Config: DefaultConfig()})
	model.SetSize(72, 11)

	if model.width != 72 || model.height != 11 {
		t.Errorf("model size = %dx%d, want 72x11", model.width, model.height)
	}
	if got := model.textarea.Width(); got <= 0 || got > 72 {
		t.Errorf("textarea content width = %d, want within configured width 72", got)
	}
	if got := model.textarea.Height(); got != 11 {
		t.Errorf("textarea height = %d, want 11", got)
	}
}

func TestNewWriteModelForModal(t *testing.T) {
	t.Parallel()

	model := NewWriteModelForModal(WriteOptions{Title: "Modal", Config: DefaultConfig()})
	if !model.forModal {
		t.Fatal("forModal = false, want true")
	}
	if view := model.View().Content; !strings.Contains(view, "Modal") {
		t.Errorf("View() = %q, want modal title", view)
	}
}

func TestWriteBuilder(t *testing.T) {
	t.Parallel()

	builder := NewWrite()
	if builder.opts.Config.Theme != ThemeDefault {
		t.Errorf("default theme = %q, want %q", builder.opts.Config.Theme, ThemeDefault)
	}

	got := builder.Title("Title").
		Description("Description").
		Placeholder("Placeholder").
		Value("Value").
		CharLimit(120).
		Width(64).
		Height(9).
		ShowLineNumbers(true).
		Theme(ThemeDracula).
		Accessible(true)
	if got != builder {
		t.Fatal("fluent setters returned a different builder")
	}

	want := WriteOptions{
		Title: "Title", Description: "Description", Placeholder: "Placeholder", Value: "Value",
		CharLimit: 120, Width: 64, Height: 9, ShowLineNumbers: true,
		Config: Config{Theme: ThemeDracula, Accessible: true, Output: builder.opts.Config.Output},
	}
	if builder.opts != want {
		t.Errorf("builder options = %#v, want %#v", builder.opts, want)
	}

	model, ok := builder.Model().(*writeModel)
	if !ok {
		t.Fatalf("Model() type = %T, want *writeModel", builder.Model())
	}
	if model.title.String() != "Title" || model.description.String() != "Description" {
		t.Errorf("Model() labels = %q/%q, want Title/Description", model.title, model.description)
	}
	if model.textarea.Value() != "Value" || model.textarea.CharLimit != 120 || !model.textarea.ShowLineNumbers {
		t.Errorf("Model() textarea options were not propagated")
	}
	if model.width != 64 || model.height != 9 || model.textarea.Height() != 9 {
		t.Errorf("Model() size = model %dx%d, textarea height %d; want width 64 and textarea height 9", model.width, model.height, model.textarea.Height())
	}
}

func TestWriteModel_WindowSizeMsgDoesNotOverrideExplicitWidth(t *testing.T) {
	t.Parallel()

	opts := WriteOptions{
		Title:  "Test write",
		Width:  42,
		Config: DefaultConfig(),
	}

	model := NewWriteModel(opts)

	if model.width != 42 {
		t.Fatalf("expected model width 42, got %d", model.width)
	}
	initialWidth := model.textarea.Width()
	if initialWidth <= 0 {
		t.Fatalf("expected positive initial textarea width, got %d", initialWidth)
	}

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*writeModel)

	if m.width != 42 {
		t.Errorf("expected model width to stay 42, got %d", m.width)
	}
	if m.textarea.Width() != initialWidth {
		t.Errorf("expected textarea width to stay %d, got %d", initialWidth, m.textarea.Width())
	}
}

func TestWriteModel_WindowSizeMsgDoesNotOverrideConfigWidth(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Width = 55

	opts := WriteOptions{
		Title:  "Test write",
		Config: cfg,
	}

	model := NewWriteModel(opts)

	if model.width != 55 {
		t.Fatalf("expected model width 55, got %d", model.width)
	}
	initialWidth := model.textarea.Width()
	if initialWidth <= 0 {
		t.Fatalf("expected positive initial textarea width, got %d", initialWidth)
	}

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*writeModel)

	if m.width != 55 {
		t.Errorf("expected model width to stay 55, got %d", m.width)
	}
	if m.textarea.Width() != initialWidth {
		t.Errorf("expected textarea width to stay %d, got %d", initialWidth, m.textarea.Width())
	}
}

// TestWriteModel_UnicodeAndLongInputs is a crash-guard test: Init() and View()
// must not panic when the initial value contains non-ASCII or long content.
func TestWriteModel_UnicodeAndLongInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "CJK characters", value: "你好世界\n第二行"},
		{name: "emoji", value: "Hello 🌍🚀✨\nLine 2"},
		{name: "combining marks", value: "e\u0301 a\u0300 u\u0308"},
		{name: "mixed-width", value: "ABCｄｅｆ全角半角"},
		{name: "RTL characters", value: "مرحبا بالعالم"},
		{name: "very long line", value: strings.Repeat("a", 1000)},
		{name: "many lines", value: strings.Repeat("line\n", 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := WriteOptions{
				Title:  "Test",
				Value:  tt.value,
				Config: DefaultConfig(),
			}

			model := NewWriteModel(opts)
			if model == nil {
				t.Fatal("expected non-nil model")
			}

			_ = model.Init()
			view := model.View().Content
			if view == "" {
				t.Error("expected non-empty view")
			}
		})
	}
}
