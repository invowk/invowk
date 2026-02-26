// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
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
