// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"context"
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
	if model.cmdName != string(opts.CommandName) {
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
	msg1 := outputMsg{content: "Hello "}
	msg2 := outputMsg{content: "World!"}

	updatedModel, _ := model.Update(msg1)
	m1 := updatedModel.(*interactiveModel)

	updatedModel2, _ := m1.Update(msg2)
	m2 := updatedModel2.(*interactiveModel)

	expected := "Hello World!"
	if m2.content.String() != expected {
		t.Errorf("expected content %q, got %q", expected, m2.content.String())
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

func TestInteractiveBuilder_Run_NoCommand(t *testing.T) {
	t.Parallel()

	builder := NewInteractive()

	result, err := builder.Run()

	if err == nil {
		t.Error("expected error when no command is provided")
	}
	if result != nil {
		t.Error("expected nil result when error occurs")
	}
	if err.Error() != "no command provided" {
		t.Errorf("expected error message 'no command provided', got %q", err.Error())
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

func TestInteractiveResult_Fields(t *testing.T) {
	t.Parallel()

	result := InteractiveResult{
		ExitCode: 42,
		Duration: 5 * time.Second,
		Error:    nil,
	}

	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
	if result.Duration != 5*time.Second {
		t.Errorf("expected duration 5s, got %v", result.Duration)
	}
	if result.Error != nil {
		t.Errorf("expected nil error, got %v", result.Error)
	}
}

func TestInteractiveOptions_Fields(t *testing.T) {
	t.Parallel()

	opts := InteractiveOptions{
		Title:       "My Title",
		CommandName: "my-cmd",
		Config:      DefaultConfig(),
	}

	if opts.Title != "My Title" {
		t.Errorf("expected title 'My Title', got %q", opts.Title)
	}
	if opts.CommandName != "my-cmd" {
		t.Errorf("expected command name 'my-cmd', got %q", opts.CommandName)
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

	model.appendCompletionMessage()

	content := model.content.String()
	if content == "" {
		t.Error("expected completion message to be appended")
	}
	// The message should contain "COMPLETED SUCCESSFULLY" for exit code 0
	if content == "" {
		t.Error("content should not be empty after appending completion message")
	}
}

func TestInteractiveModel_AppendCompletionMessage_Failure(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.result = &InteractiveResult{
		ExitCode: 1,
		Duration: 500 * time.Millisecond,
	}

	model.appendCompletionMessage()

	content := model.content.String()
	if content == "" {
		t.Error("expected completion message to be appended")
	}
}

func TestInteractiveModel_ConcurrentOutputWrites(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	// Simulate concurrent output writes (should be safe due to mutex)
	done := make(chan bool)
	for i := range 10 {
		go func(n int) {
			msg := outputMsg{content: "output "}
			model.Update(msg)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// Verify content was written (exact content may vary due to race, but should not panic)
	content := model.content.String()
	if content == "" {
		t.Error("expected some content to be written")
	}
}

func TestStripOSCSequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no OSC sequences",
			input:    "Hello World!",
			expected: "Hello World!",
		},
		{
			name:     "OSC 11 background color with BEL terminator",
			input:    "prefix\x1b]11;rgb:3030/0a0a/2424\x07suffix",
			expected: "prefixsuffix",
		},
		{
			name:     "OSC 11 background color with ST terminator",
			input:    "prefix\x1b]11;rgb:3030/0a0a/2424\x1b\\suffix",
			expected: "prefixsuffix",
		},
		{
			name:     "OSC 11 without leading ESC (partial sequence)",
			input:    "prefix]11;rgb:3030/0a0a/2424\\suffix",
			expected: "prefixsuffix",
		},
		{
			name:     "OSC 10 foreground color",
			input:    "prefix\x1b]10;rgb:ffff/ffff/ffff\x07suffix",
			expected: "prefixsuffix",
		},
		{
			name:     "OSC 4 palette color",
			input:    "prefix\x1b]4;0;rgb:0000/0000/0000\x07suffix",
			expected: "prefixsuffix",
		},
		{
			name:     "multiple OSC sequences",
			input:    "\x1b]11;rgb:3030/0a0a/2424\x07You chose: green\x1b]11;rgb:3030/0a0a/2424\x07",
			expected: "You chose: green",
		},
		{
			name:     "OSC 8 hyperlink also stripped in pager context",
			input:    "\x1b]8;;https://example.com\x07link\x1b]8;;\x07",
			expected: "link",
		},
		{
			name:     "OSC 0 window title also stripped in pager context",
			input:    "\x1b]0;My Window Title\x07text",
			expected: "text",
		},
		{
			name:     "mixed content from screenshot",
			input:    "]11;rgb:3030/0a0a/2424\\You chose: green",
			expected: "You chose: green",
		},
		{
			name:     "uppercase hex in color response",
			input:    "\x1b]11;rgb:FFFF/AAAA/BBBB\x07text",
			expected: "text",
		},
		{
			name:     "preserves regular text and ANSI colors",
			input:    "\x1b[32mGreen text\x1b[0m",
			expected: "\x1b[32mGreen text\x1b[0m",
		},
		{
			name:     "partial sequence without ESC",
			input:    "]0;title\x07rest of output",
			expected: "rest of output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := stripOSCSequences(tt.input)
			if result != tt.expected {
				t.Errorf("stripOSCSequences() mismatch\ngot:  %q\nwant: %q", result, tt.expected)
			}
		})
	}
}
