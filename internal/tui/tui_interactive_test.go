// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestNewInteractiveModel(t *testing.T) {
	t.Parallel()

	opts := InteractiveOptions{
		Title:       "Test Title",
		CommandName: "test-cmd",
		Config:      DefaultConfig(),
	}

	model := newInteractiveModel(opts, nil)

	if model.title != opts.Title {
		t.Errorf("expected title %q, got %q", opts.Title, model.title)
	}
	if model.cmdName != opts.CommandName {
		t.Errorf("expected cmdName %q, got %q", opts.CommandName, model.cmdName)
	}
	if model.state != stateExecuting {
		t.Errorf("expected initial state to be stateExecuting, got %v", model.state)
	}
	if model.ready {
		t.Error("expected ready to be false initially")
	}
}

func TestInteractiveModel_Init(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	cmd := model.Init()

	if cmd != nil {
		t.Error("expected Init() to return nil cmd")
	}
}

func TestInteractiveModel_Update_OutputMsg(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	// Send output messages
	msg1 := outputMsg{data: []byte("Hello ")}
	msg2 := outputMsg{data: []byte("World!")}

	updatedModel, _ := model.Update(msg1)
	m1 := updatedModel.(*interactiveModel)

	updatedModel2, _ := m1.Update(msg2)
	m2 := updatedModel2.(*interactiveModel)

	expected := "Hello World!"
	if got := m2.screen.emu.String(); !strings.HasPrefix(got, expected) {
		t.Errorf("expected screen to start with %q, got %q", expected, got)
	}
}

func TestInteractiveModel_Update_DoneMsg(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	result := InteractiveResult{
		ExitCode: 0,
		Duration: 2 * time.Second,
	}

	msg := doneMsg{result: result}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*interactiveModel)

	if m.state != stateCompleted {
		t.Errorf("expected state to be stateCompleted, got %v", m.state)
	}
	if m.result == nil {
		t.Fatal("expected result to be set")
	}
	if m.result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", m.result.ExitCode)
	}
	if m.result.Duration != 2*time.Second {
		t.Errorf("expected duration 2s, got %v", m.result.Duration)
	}
}

func TestInteractiveModel_Update_DoneMsg_WithError(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	result := InteractiveResult{
		ExitCode: 1,
		Duration: 500 * time.Millisecond,
	}

	msg := doneMsg{result: result}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*interactiveModel)

	if m.state != stateCompleted {
		t.Errorf("expected state to be stateCompleted, got %v", m.state)
	}
	if m.result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", m.result.ExitCode)
	}
}

func TestInteractiveModel_Update_WindowSizeMsg(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*interactiveModel)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
	if !m.ready {
		t.Error("expected ready to be true after WindowSizeMsg")
	}
}

func TestInteractiveModel_HandleKeyMsg_CompletedState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        string
		expectQuit bool
	}{
		{"enter", "enter", true},
		{"q", "q", true},
		{"esc", "esc", true},
		{keyCtrlC, keyCtrlC, true},
		{"up", "up", false},
		{"down", "down", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model := newInteractiveModel(InteractiveOptions{}, nil)
			model.state = stateCompleted
			model.result = &InteractiveResult{ExitCode: 0}
			// Initialize viewport by sending WindowSizeMsg first
			model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

			keyMsg := tea.KeyPressMsg{Code: rune(tt.key[0]), Text: tt.key}
			switch tt.key {
			case "enter":
				keyMsg = tea.KeyPressMsg{Code: tea.KeyEnter}
			case "esc":
				keyMsg = tea.KeyPressMsg{Code: tea.KeyEscape}
			case "up":
				keyMsg = tea.KeyPressMsg{Code: tea.KeyUp}
			case "down":
				keyMsg = tea.KeyPressMsg{Code: tea.KeyDown}
			case keyCtrlC:
				keyMsg = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
			}

			_, cmd := model.handleKeyMsg(keyMsg)

			if tt.expectQuit {
				if cmd == nil {
					t.Error("expected quit command, got nil")
				}
			}
		})
	}
}

func TestInteractiveModel_HandleKeyMsg_ExecutingState(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.state = stateExecuting

	// Regular keys during execution should not cause quit (without PTY, they're just ignored)
	keyMsg := tea.KeyPressMsg{Code: 'a', Text: "a"}

	_, cmd := model.handleKeyMsg(keyMsg)

	// Regular keys should not trigger quit
	if cmd != nil {
		t.Error("expected nil command for regular key during execution (no PTY)")
	}

	// State should remain executing
	if model.state != stateExecuting {
		t.Error("expected model to remain in executing state")
	}
}

func TestInteractiveModel_View_NotReady(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	view := model.View()
	viewContent := view.Content

	expected := "Initializing..."
	if viewContent != expected {
		t.Errorf("expected view %q, got %q", expected, viewContent)
	}
}

func TestInteractiveModel_View_Ready(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{
		Title:       "Test Title",
		CommandName: "test-cmd",
	}, nil)

	// Initialize viewport
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := model.View()
	viewContent := view.Content

	// Check that view contains expected elements
	if viewContent == "Initializing..." {
		t.Error("expected view to be rendered, not 'Initializing...'")
	}
	// The view should contain the title
	if viewContent == "" {
		t.Error("expected non-empty view")
	}
}

func TestInteractiveBuilder(t *testing.T) {
	t.Parallel()

	builder := NewInteractive()

	if builder.opts.Title != "Running Command" {
		t.Errorf("expected default title 'Running Command', got %q", builder.opts.Title)
	}
	if builder.ctx == nil {
		t.Error("expected context to be set")
	}
}

func TestInteractiveBuilder_Title(t *testing.T) {
	t.Parallel()

	builder := NewInteractive().Title("Custom Title")

	if builder.opts.Title != "Custom Title" {
		t.Errorf("expected title 'Custom Title', got %q", builder.opts.Title)
	}
}

func TestInteractiveBuilder_CommandName(t *testing.T) {
	t.Parallel()

	builder := NewInteractive().CommandName("my-command")

	if builder.opts.CommandName != "my-command" {
		t.Errorf("expected command name 'my-command', got %q", builder.opts.CommandName)
	}
}

func TestInteractiveBuilder_Context(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	builder := NewInteractive().Context(ctx)

	if builder.ctx != ctx {
		t.Error("expected context to be set")
	}
}

func TestInteractiveBuilder_Chaining(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	builder := NewInteractive().
		Title("Deploy").
		CommandName("deploy-app").
		Context(ctx)

	if builder.opts.Title != "Deploy" {
		t.Errorf("expected title 'Deploy', got %q", builder.opts.Title)
	}
	if builder.opts.CommandName != "deploy-app" {
		t.Errorf("expected command name 'deploy-app', got %q", builder.opts.CommandName)
	}
	if builder.ctx != ctx {
		t.Error("expected context to be set via chaining")
	}
}

func TestExecutionState_Constants(t *testing.T) {
	t.Parallel()

	if stateExecuting != 0 {
		t.Errorf("expected stateExecuting to be 0, got %d", stateExecuting)
	}
	if stateCompleted != 1 {
		t.Errorf("expected stateCompleted to be 1, got %d", stateCompleted)
	}
}

func TestInteractiveModel_AppendCompletionMessage_Success(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.result = &InteractiveResult{
		ExitCode: 0,
		Duration: 1 * time.Second,
	}

	var b strings.Builder
	model.appendCompletionMessage(&b)

	content := b.String()
	for _, want := range []string{"COMPLETED SUCCESSFULLY", "Duration: 1s", "Press Enter to return to terminal..."} {
		if !strings.Contains(content, want) {
			t.Errorf("completion message lacks %q:\n%s", want, content)
		}
	}
}

func TestInteractiveModel_AppendCompletionMessage_Failure(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.result = &InteractiveResult{
		ExitCode: 1,
		Duration: 500 * time.Millisecond,
	}

	var b strings.Builder
	model.appendCompletionMessage(&b)

	if content := b.String(); !strings.Contains(content, "EXITED WITH CODE 1") {
		t.Errorf("completion message lacks the exit code:\n%s", content)
	}
}
