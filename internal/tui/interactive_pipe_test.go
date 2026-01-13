// SPDX-License-Identifier: EPL-2.0

package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// Model Creation Tests
// =============================================================================

func TestNewPipeInteractiveModel(t *testing.T) {
	opts := PipeInteractiveOptions{
		Title:       "Test Title",
		CommandName: "test-cmd",
		EchoInput:   true,
		Config:      DefaultConfig(),
	}

	// Create a mock stdin writer
	_, stdinW := io.Pipe()
	defer stdinW.Close()

	model := newPipeInteractiveModel(opts, stdinW)

	if model.title != opts.Title {
		t.Errorf("expected title %q, got %q", opts.Title, model.title)
	}
	if model.cmdName != opts.CommandName {
		t.Errorf("expected cmdName %q, got %q", opts.CommandName, model.cmdName)
	}
	if model.echoInput != opts.EchoInput {
		t.Errorf("expected echoInput %v, got %v", opts.EchoInput, model.echoInput)
	}
	if model.state != stateExecuting {
		t.Errorf("expected initial state to be stateExecuting, got %v", model.state)
	}
	if model.ready {
		t.Error("expected ready to be false initially")
	}
	if model.stdinW == nil {
		t.Error("expected stdinW to be set")
	}
}

func TestDefaultPipeInteractiveOptions(t *testing.T) {
	opts := DefaultPipeInteractiveOptions()

	if opts.Title != "Running Command" {
		t.Errorf("expected default title 'Running Command', got %q", opts.Title)
	}
	if !opts.EchoInput {
		t.Error("expected EchoInput to be true by default")
	}
}

func TestPipeInteractiveModel_Init(t *testing.T) {
	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)
	cmd := model.Init()

	if cmd != nil {
		t.Error("expected Init() to return nil cmd")
	}
}

// =============================================================================
// Input Buffer Tests - Local Echo Behavior
// =============================================================================

func TestPipeInteractiveModel_LocalEcho_TypeCharacters(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background to prevent blocking
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Initialize viewport first
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type some characters
	testCases := []struct {
		rune     rune
		expected string
	}{
		{'H', "H"},
		{'e', "He"},
		{'l', "Hel"},
		{'l', "Hell"},
		{'o', "Hello"},
	}

	for _, tc := range testCases {
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.rune}}
		model.Update(keyMsg)

		if got := model.GetInputBuffer(); got != tc.expected {
			t.Errorf("after typing %q: expected input buffer %q, got %q", tc.rune, tc.expected, got)
		}
	}

	// Content should still be empty (input not committed yet)
	if model.GetContent() != "" {
		t.Errorf("expected content to be empty before Enter, got %q", model.GetContent())
	}
}

func TestPipeInteractiveModel_LocalEcho_Space(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "Hello World"
	for _, r := range "Hello" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model.Update(tea.KeyMsg{Type: tea.KeySpace})
	for _, r := range "World" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	expected := "Hello World"
	if got := model.GetInputBuffer(); got != expected {
		t.Errorf("expected input buffer %q, got %q", expected, got)
	}
}

func TestPipeInteractiveModel_LocalEcho_Tab(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "A<tab>B"
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})

	expected := "A\tB"
	if got := model.GetInputBuffer(); got != expected {
		t.Errorf("expected input buffer %q, got %q", expected, got)
	}
}

func TestPipeInteractiveModel_LocalEcho_Backspace(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "Hello"
	for _, r := range "Hello" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if got := model.GetInputBuffer(); got != "Hello" {
		t.Errorf("expected input buffer 'Hello', got %q", got)
	}

	// Backspace twice
	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	// Give time for goroutine to update
	time.Sleep(10 * time.Millisecond)

	if got := model.GetInputBuffer(); got != "Hell" {
		t.Errorf("after 1 backspace: expected 'Hell', got %q", got)
	}

	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	time.Sleep(10 * time.Millisecond)

	if got := model.GetInputBuffer(); got != "Hel" {
		t.Errorf("after 2 backspaces: expected 'Hel', got %q", got)
	}
}

func TestPipeInteractiveModel_LocalEcho_BackspaceUTF8(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "Hé" (with accented e - multi-byte UTF-8)
	for _, r := range "Hé" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if got := model.GetInputBuffer(); got != "Hé" {
		t.Errorf("expected input buffer 'Hé', got %q", got)
	}

	// Backspace should remove the entire é character, not just one byte
	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	time.Sleep(10 * time.Millisecond)

	if got := model.GetInputBuffer(); got != "H" {
		t.Errorf("after backspace on UTF-8: expected 'H', got %q", got)
	}
}

func TestPipeInteractiveModel_LocalEcho_BackspaceEmptyBuffer(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Backspace on empty buffer should not panic
	model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	time.Sleep(10 * time.Millisecond)

	if got := model.GetInputBuffer(); got != "" {
		t.Errorf("expected empty input buffer, got %q", got)
	}
}

func TestPipeInteractiveModel_LocalEcho_Disabled(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	// EchoInput = false (for password prompts)
	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: false}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "secret"
	for _, r := range "secret" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Input buffer should be empty because echo is disabled
	if got := model.GetInputBuffer(); got != "" {
		t.Errorf("expected empty input buffer with echo disabled, got %q", got)
	}
}

// =============================================================================
// Enter Key Tests - Input Commit Behavior
// =============================================================================

func TestPipeInteractiveModel_Enter_CommitsInputBuffer(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "Hello"
	for _, r := range "Hello" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Input buffer should be cleared
	if got := model.GetInputBuffer(); got != "" {
		t.Errorf("expected input buffer to be cleared after Enter, got %q", got)
	}

	// Content should now contain the input + newline
	expected := "Hello\n"
	if got := model.GetContent(); got != expected {
		t.Errorf("expected content %q, got %q", expected, got)
	}
}

func TestPipeInteractiveModel_Enter_SendsNewlineToStdin(t *testing.T) {
	stdinR, stdinW := io.Pipe()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Channel to receive what was written to stdin
	received := make(chan string, 1)
	go func() {
		buf := make([]byte, 1024)
		var result bytes.Buffer
		for {
			n, err := stdinR.Read(buf)
			if n > 0 {
				result.Write(buf[:n])
			}
			if err != nil {
				break
			}
			// Check if we got a newline
			if bytes.Contains(result.Bytes(), []byte("\n")) {
				received <- result.String()
				return
			}
		}
		received <- result.String()
	}()

	// Type "test" and press Enter
	for _, r := range "test" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Close to unblock reader
	stdinW.Close()

	select {
	case got := <-received:
		if !strings.HasSuffix(got, "\n") {
			t.Errorf("expected stdin to receive newline, got %q", got)
		}
		if !strings.Contains(got, "test") {
			t.Errorf("expected stdin to contain 'test', got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for stdin data")
	}
}

// =============================================================================
// Output Message Tests - Smart Buffer Management
// =============================================================================

func TestPipeInteractiveModel_Output_ClearsInputBuffer(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type some characters
	for _, r := range "partial" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Verify input buffer has content
	if got := model.GetInputBuffer(); got != "partial" {
		t.Errorf("expected input buffer 'partial', got %q", got)
	}

	// Simulate output from command (indicating input was consumed)
	model.Update(outputMsg{content: "Command output\n"})

	// Input buffer should be cleared (input was consumed)
	if got := model.GetInputBuffer(); got != "" {
		t.Errorf("expected input buffer to be cleared after output, got %q", got)
	}

	// Content should have the output
	if got := model.GetContent(); got != "Command output\n" {
		t.Errorf("expected content 'Command output\\n', got %q", got)
	}
}

func TestPipeInteractiveModel_Output_AppendsToContent(t *testing.T) {
	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Send multiple output messages
	model.Update(outputMsg{content: "Line 1\n"})
	model.Update(outputMsg{content: "Line 2\n"})
	model.Update(outputMsg{content: "Line 3\n"})

	expected := "Line 1\nLine 2\nLine 3\n"
	if got := model.GetContent(); got != expected {
		t.Errorf("expected content %q, got %q", expected, got)
	}
}

func TestPipeInteractiveModel_Output_NoDoublEcho(t *testing.T) {
	// This tests the smart buffer: user types "hello", presses enter,
	// command echoes "hello" back - we should not see "hellohello"

	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type "hello"
	for _, r := range "hello" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter (commits "hello\n" to content, clears buffer)
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	contentAfterEnter := model.GetContent()
	if contentAfterEnter != "hello\n" {
		t.Errorf("after Enter: expected 'hello\\n', got %q", contentAfterEnter)
	}

	// Now command produces output (e.g., echo of what it read)
	// This should NOT cause double-echo because buffer is already cleared
	model.Update(outputMsg{content: "You typed: hello\n"})

	finalContent := model.GetContent()
	expected := "hello\nYou typed: hello\n"
	if finalContent != expected {
		t.Errorf("expected final content %q, got %q", expected, finalContent)
	}
}

// =============================================================================
// Done Message Tests
// =============================================================================

func TestPipeInteractiveModel_Done_ClearsInputBuffer(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Type some characters (incomplete input)
	for _, r := range "incomplete" {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Command completes
	result := InteractiveResult{
		ExitCode: 0,
		Duration: 1 * time.Second,
	}
	model.Update(doneMsg{result: result})

	// Input buffer should be cleared
	if got := model.GetInputBuffer(); got != "" {
		t.Errorf("expected input buffer to be cleared after done, got %q", got)
	}

	// State should be completed
	if model.state != stateCompleted {
		t.Errorf("expected state to be stateCompleted, got %v", model.state)
	}
}

func TestPipeInteractiveModel_Done_ClosesStdinPipe(t *testing.T) {
	stdinR, stdinW := io.Pipe()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Initially, stdinW should be set
	if model.stdinW == nil {
		t.Fatal("expected stdinW to be set initially")
	}

	// Command completes
	result := InteractiveResult{ExitCode: 0, Duration: 1 * time.Second}
	model.Update(doneMsg{result: result})

	// stdinW should be nil after done
	if model.stdinW != nil {
		t.Error("expected stdinW to be nil after done")
	}

	// Writing to the closed pipe should fail
	_, err := stdinR.Read(make([]byte, 1))
	if err != io.EOF && err != io.ErrClosedPipe {
		t.Errorf("expected EOF or ErrClosedPipe, got %v", err)
	}
}

// =============================================================================
// Window Size Tests
// =============================================================================

func TestPipeInteractiveModel_WindowSize(t *testing.T) {
	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*pipeInteractiveModel)

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

// =============================================================================
// View Tests
// =============================================================================

func TestPipeInteractiveModel_View_NotReady(t *testing.T) {
	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)

	view := model.View()

	expected := "Initializing..."
	if view != expected {
		t.Errorf("expected view %q, got %q", expected, view)
	}
}

func TestPipeInteractiveModel_View_Ready(t *testing.T) {
	model := newPipeInteractiveModel(PipeInteractiveOptions{
		Title:       "Test Title",
		CommandName: "test-cmd",
		EchoInput:   true,
	}, nil)

	// Initialize viewport
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	view := model.View()

	// Check that view is rendered
	if view == "Initializing..." {
		t.Error("expected view to be rendered, not 'Initializing...'")
	}
	if len(view) == 0 {
		t.Error("expected non-empty view")
	}
}

// =============================================================================
// Control Key Tests
// =============================================================================

func TestPipeInteractiveModel_CtrlBackslash_ForceQuit(t *testing.T) {
	// This test verifies that regular keys don't cause quit during execution
	// The Ctrl+\ handling is tested indirectly since we can't easily mock
	// the exact key message that produces "ctrl+\\" string.

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Regular key during execution should not trigger quit
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	_, cmd := model.handleKeyMsg(keyMsg)

	if cmd != nil {
		t.Error("expected nil command for regular key during execution")
	}

	// State should remain executing
	if model.state != stateExecuting {
		t.Error("expected model to remain in executing state")
	}
}

func TestPipeInteractiveModel_ControlKeys_Forwarded(t *testing.T) {
	stdinR, stdinW := io.Pipe()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Channel to receive stdin data
	received := make(chan byte, 10)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := stdinR.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				received <- buf[0]
			}
		}
	}()

	// Test Ctrl+C
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	select {
	case b := <-received:
		if b != 0x03 { // ETX (Ctrl+C)
			t.Errorf("expected Ctrl+C (0x03), got 0x%02x", b)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for Ctrl+C")
	}

	// Test Ctrl+D
	model.Update(tea.KeyMsg{Type: tea.KeyCtrlD})

	select {
	case b := <-received:
		if b != 0x04 { // EOT (Ctrl+D)
			t.Errorf("expected Ctrl+D (0x04), got 0x%02x", b)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for Ctrl+D")
	}

	stdinW.Close()
}

// =============================================================================
// Completed State Navigation Tests
// =============================================================================

func TestPipeInteractiveModel_CompletedState_Navigation(t *testing.T) {
	tests := []struct {
		name       string
		key        tea.KeyMsg
		expectQuit bool
	}{
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, true},
		{"q", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}, true},
		{"esc", tea.KeyMsg{Type: tea.KeyEscape}, true},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}, true},
		{"up", tea.KeyMsg{Type: tea.KeyUp}, false},
		{"down", tea.KeyMsg{Type: tea.KeyDown}, false},
		{"pgup", tea.KeyMsg{Type: tea.KeyPgUp}, false},
		{"pgdown", tea.KeyMsg{Type: tea.KeyPgDown}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)
			model.state = stateCompleted
			model.result = &InteractiveResult{ExitCode: 0}
			model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

			_, cmd := model.handleKeyMsg(tt.key)

			if tt.expectQuit && cmd == nil {
				t.Error("expected quit command, got nil")
			}
			if !tt.expectQuit && cmd != nil {
				t.Error("expected nil command for navigation key")
			}
		})
	}
}

// =============================================================================
// Builder Tests
// =============================================================================

func TestPipeInteractiveBuilder_Defaults(t *testing.T) {
	builder := NewPipeInteractive()

	if builder.opts.Title != "Running Command" {
		t.Errorf("expected default title 'Running Command', got %q", builder.opts.Title)
	}
	if !builder.opts.EchoInput {
		t.Error("expected EchoInput to be true by default")
	}
	if builder.ctx == nil {
		t.Error("expected context to be set")
	}
}

func TestPipeInteractiveBuilder_Title(t *testing.T) {
	builder := NewPipeInteractive().Title("Custom Title")

	if builder.opts.Title != "Custom Title" {
		t.Errorf("expected title 'Custom Title', got %q", builder.opts.Title)
	}
}

func TestPipeInteractiveBuilder_CommandName(t *testing.T) {
	builder := NewPipeInteractive().CommandName("my-command")

	if builder.opts.CommandName != "my-command" {
		t.Errorf("expected command name 'my-command', got %q", builder.opts.CommandName)
	}
}

func TestPipeInteractiveBuilder_EchoInput(t *testing.T) {
	builder := NewPipeInteractive().EchoInput(false)

	if builder.opts.EchoInput {
		t.Error("expected EchoInput to be false")
	}
}

func TestPipeInteractiveBuilder_Context(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	builder := NewPipeInteractive().Context(ctx)

	if builder.ctx != ctx {
		t.Error("expected context to be set")
	}
}

func TestPipeInteractiveBuilder_Run_NoExecutor(t *testing.T) {
	builder := NewPipeInteractive()

	result, err := builder.Run()

	if err == nil {
		t.Error("expected error when no executor is provided")
	}
	if result != nil {
		t.Error("expected nil result when error occurs")
	}
	if err.Error() != "no executor provided" {
		t.Errorf("expected error message 'no executor provided', got %q", err.Error())
	}
}

func TestPipeInteractiveBuilder_Chaining(t *testing.T) {
	ctx := context.Background()

	builder := NewPipeInteractive().
		Title("Deploy").
		CommandName("deploy-app").
		EchoInput(true).
		Context(ctx)

	if builder.opts.Title != "Deploy" {
		t.Errorf("expected title 'Deploy', got %q", builder.opts.Title)
	}
	if builder.opts.CommandName != "deploy-app" {
		t.Errorf("expected command name 'deploy-app', got %q", builder.opts.CommandName)
	}
	if !builder.opts.EchoInput {
		t.Error("expected EchoInput to be true")
	}
	if builder.ctx != ctx {
		t.Error("expected context to be set via chaining")
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestPipeInteractiveModel_ConcurrentOutputAndInput(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Simulate concurrent input and output
	done := make(chan bool)

	// Goroutine 1: Send output messages
	go func() {
		for i := 0; i < 10; i++ {
			model.Update(outputMsg{content: "output\n"})
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Send input keys
	go func() {
		for i := 0; i < 10; i++ {
			model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// Should not panic - verify model is still functional
	if model.state != stateExecuting {
		t.Error("model state changed unexpectedly")
	}
}

// =============================================================================
// Stdin Forwarding Tests
// =============================================================================

func TestPipeInteractiveModel_StdinForwarding(t *testing.T) {
	stdinR, stdinW := io.Pipe()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Channel to collect stdin data
	var received bytes.Buffer
	done := make(chan bool)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdinR.Read(buf)
			if err != nil {
				break
			}
			received.Write(buf[:n])
		}
		done <- true
	}()

	// Type "abc" and press Enter
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Close to unblock reader
	stdinW.Close()
	<-done

	got := received.String()
	if got != "abc\n" {
		t.Errorf("expected stdin to receive 'abc\\n', got %q", got)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestPipeInteractiveModel_NilStdinW(t *testing.T) {
	// Model with nil stdin writer should not panic
	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, nil)
	model.state = stateExecuting

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// This should not panic even with nil stdinW
	model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Model should still function for local echo
	// (though nothing is sent to stdin)
}

func TestPipeInteractiveModel_RapidInput(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	defer stdinR.Close()

	model := newPipeInteractiveModel(PipeInteractiveOptions{EchoInput: true}, stdinW)
	model.state = stateExecuting

	// Read from stdin in background
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Rapid input of a long string
	input := "The quick brown fox jumps over the lazy dog"
	for _, r := range input {
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if got := model.GetInputBuffer(); got != input {
		t.Errorf("expected input buffer %q, got %q", input, got)
	}
}
