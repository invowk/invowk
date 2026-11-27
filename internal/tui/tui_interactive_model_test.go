// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type (
	testEmbeddableComponent struct {
		done      bool
		cancelled bool
		result    any
		err       error
		view      string
	}

	resizeRecordingTerminal struct {
		fakeInteractiveTerminal
		sizes [][2]int
	}

	styledTextRequestCase struct {
		name      string
		options   any
		wantErr   bool
		wantText  string
		wantWidth int
	}
)

func (c *testEmbeddableComponent) Init() tea.Cmd { return nil }

func (c *testEmbeddableComponent) Update(tea.Msg) (tea.Model, tea.Cmd) {
	c.done = true
	return c, tea.Quit
}

func (c *testEmbeddableComponent) View() tea.View { return tea.NewView(c.view) }
func (c *testEmbeddableComponent) IsDone() bool   { return c.done }
func (c *testEmbeddableComponent) Result() (any, error) {
	return c.result, c.err
}
func (c *testEmbeddableComponent) Cancelled() bool { return c.cancelled }
func (c *testEmbeddableComponent) SetSize(TerminalDimension, TerminalDimension) {
}

func TestInteractiveModel_Init_ReturnsNilCmd(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	cmd := model.Init()

	if cmd != nil {
		t.Errorf("Init() returned non-nil cmd: %v", cmd)
	}
}

func TestInteractiveModel_Update_WindowSizeMsg_InitializesViewport(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	// Output that arrives before the viewer is ready is kept on the screen.
	model.Update(outputMsg{data: []byte("buffered output")})

	msg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updated, _ := model.Update(msg)
	m := updated.(*interactiveModel)

	if !m.ready {
		t.Fatal("expected ready=true after first WindowSizeMsg")
	}
	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 30 {
		t.Errorf("height = %d, want 30", m.height)
	}
	if got := m.screen.emu.String(); !strings.HasPrefix(got, "buffered output") {
		t.Errorf("screen = %q, want the buffered output", got)
	}
	if w, h := m.screen.emu.Width(), m.screen.emu.Height(); w != 100 || h != 27 {
		t.Errorf("screen = %dx%d, want the 100x27 command area", w, h)
	}
}

func TestInteractiveModel_Update_WindowSizeMsg_ResizesExistingViewport(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)

	// First WindowSizeMsg initializes the viewport.
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Second WindowSizeMsg resizes the existing viewport.
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	m := updated.(*interactiveModel)

	if m.width != 120 {
		t.Errorf("width after resize = %d, want 120", m.width)
	}
	if m.height != 50 {
		t.Errorf("height after resize = %d, want 50", m.height)
	}
}

func TestInteractiveModel_Update_KeyMsg_QuitKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{
			name: "ctrl+c in completed state",
			key:  tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl},
		},
		{
			name: "q in completed state",
			key:  tea.KeyPressMsg{Code: 'q', Text: "q"},
		},
		{
			name: "enter in completed state",
			key:  tea.KeyPressMsg{Code: tea.KeyEnter},
		},
		{
			name: "esc in completed state",
			key:  tea.KeyPressMsg{Code: tea.KeyEscape},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := newInteractiveModel(InteractiveOptions{}, nil)
			model.state = stateCompleted
			model.result = &InteractiveResult{ExitCode: 0}

			// Initialize viewport so handleKeyMsg navigation works.
			model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

			_, cmd := model.Update(tt.key)
			if cmd == nil {
				t.Error("expected quit command, got nil")
			}
		})
	}
}

func TestInteractiveModel_Update_KeyMsg_CtrlBackslash_ForceQuit(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.state = stateExecuting

	msg := tea.KeyPressMsg{Code: '\\', Mod: tea.ModCtrl}
	_, cmd := model.handleKeyMsg(msg)

	if cmd == nil {
		t.Error("expected quit command for ctrl+\\ during execution, got nil")
	}
}

func TestInteractiveModel_HandleTUIComponentRequest_InvalidOptions(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	responseCh := make(chan ComponentResponse, 1)
	msg := TUIComponentMsg{
		Component:  ComponentType("nonexistent-component"),
		Options:    ConfirmOptions{},
		ResponseCh: responseCh,
	}

	updated, _ := model.handleTUIComponentRequest(msg)
	m := updated.(*interactiveModel)

	// Unknown component type should send an error response and remain in current state.
	if m.state != stateExecuting {
		t.Errorf("state = %v, want stateExecuting after error", m.state)
	}
	if m.activeComponent != nil {
		t.Error("expected activeComponent to be nil after creation failure")
	}

	// Verify the error response was sent.
	resp := <-responseCh
	if resp.Err == nil {
		t.Error("expected non-empty error in response for unknown component type")
	}
}

func TestInteractiveModel_HandleTUIComponentRequest_ValidDispatch(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	responseCh := make(chan ComponentResponse, 1)
	msg := TUIComponentMsg{
		Component:  ComponentTypeConfirm,
		Options:    ConfirmOptions{Title: "Proceed?", Affirmative: "Yes", Negative: "No"},
		ResponseCh: responseCh,
	}

	updated, cmd := model.handleTUIComponentRequest(msg)
	m := updated.(*interactiveModel)

	if m.state != stateTUI {
		t.Errorf("state = %v, want stateTUI", m.state)
	}
	if m.activeComponent == nil {
		t.Error("expected activeComponent to be set")
	}
	if m.activeComponentType != ComponentTypeConfirm {
		t.Errorf("activeComponentType = %q, want %q", m.activeComponentType, ComponentTypeConfirm)
	}
	if m.componentDoneCh == nil {
		t.Error("expected componentDoneCh to be set")
	}
	// Init cmd from the component should be returned.
	_ = cmd // May be nil depending on component Init(); we only verify state was set.
}

func TestInteractiveModel_HandleStyledTextRequest(t *testing.T) {
	t.Parallel()

	tests := []styledTextRequestCase{
		{name: "appends newline", options: StyledTextOptions{Text: types.DescriptionText("status")}, wantText: "status"},
		{name: "preserves newline", options: StyledTextOptions{Text: types.DescriptionText("status\n")}, wantText: "status"},
		{name: "option width fills empty style width", options: StyledTextOptions{Text: types.DescriptionText("x"), Width: 10}, wantText: "x", wantWidth: 10},
		{name: "style width takes precedence", options: StyledTextOptions{Text: types.DescriptionText("x"), Width: 10, Style: Style{Width: 4}}, wantText: "x", wantWidth: 4},
		{name: "rejects option type", options: ConfirmOptions{}, wantErr: true},
		{name: "rejects invalid width", options: StyledTextOptions{Text: types.DescriptionText("status"), Width: -1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := newInteractiveModel(InteractiveOptions{}, nil)
			model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			responseCh := make(chan ComponentResponse, 1)
			updated, cmd := model.handleTUIComponentRequest(TUIComponentMsg{
				Component: ComponentTypeWrite, Options: tt.options, ResponseCh: responseCh,
			})
			if cmd != nil {
				t.Fatal("styled text command != nil, want synchronous model update")
			}
			response := <-responseCh
			got := updated.(*interactiveModel)
			assertStyledTextRequest(t, tt, response, got)
		})
	}
}

func assertStyledTextRequest(t *testing.T, tt styledTextRequestCase, response ComponentResponse, got *interactiveModel) {
	t.Helper()

	if (response.Err != nil) != tt.wantErr {
		t.Fatalf("response error = %v, wantErr %t", response.Err, tt.wantErr)
	}
	if got.state != stateExecuting || got.activeComponent != nil {
		t.Fatalf("styled text state = %v with component %T, want executing without component", got.state, got.activeComponent)
	}
	content := got.screen.emu.String()
	if tt.wantText != "" && !strings.HasPrefix(content, tt.wantText) {
		t.Errorf("screen = %q, want text %q on its first row", content, tt.wantText)
	}
	if tt.wantErr && strings.TrimSpace(content) != "" {
		t.Errorf("screen = %q after rejected options, want empty", content)
	}
	if tt.wantWidth > 0 {
		first, _, _ := strings.Cut(got.screen.emu.Render(), "\n")
		if width := lipgloss.Width(strings.TrimRight(stripANSI(first), " ")); width > tt.wantWidth {
			t.Errorf("styled text width = %d, want at most %d; row %q", width, tt.wantWidth, first)
		}
	}
	// The text moved the cursor below it: it ended in a newline.
	if !tt.wantErr && got.screen.emu.CursorPosition().Y < 1 {
		t.Errorf("cursor row = %d after styled text, want below the text", got.screen.emu.CursorPosition().Y)
	}
}

func TestInteractiveModel_ComponentCompletion(t *testing.T) {
	t.Parallel()

	testErr := errors.New("component failed")
	tests := []struct {
		name      string
		msg       tuiComponentDoneMsg
		want      any
		wantErr   error
		cancelled bool
	}{
		{name: "success", msg: tuiComponentDoneMsg{result: "answer"}, want: "answer"},
		{name: "error", msg: tuiComponentDoneMsg{err: testErr}, wantErr: testErr},
		{name: "cancelled", msg: tuiComponentDoneMsg{cancelled: true}, cancelled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			responseCh := make(chan ComponentResponse, 1)
			model := newInteractiveModel(InteractiveOptions{}, nil)
			model.state = stateTUI
			model.activeComponent = &testEmbeddableComponent{}
			model.activeComponentType = ComponentTypeConfirm
			model.componentDoneCh = responseCh

			updated, cmd := model.handleTUIComponentDone(tt.msg)
			if cmd != nil {
				t.Fatal("handleTUIComponentDone() command != nil")
			}
			response := <-responseCh
			if response.Result != tt.want || !errors.Is(response.Err, tt.wantErr) || response.Cancelled != tt.cancelled {
				t.Errorf("response = %#v, want result %#v, error %v, cancelled %t", response, tt.want, tt.wantErr, tt.cancelled)
			}

			got := updated.(*interactiveModel)
			if got.state != stateExecuting || got.activeComponent != nil || got.activeComponentType != "" || got.componentDoneCh != nil {
				t.Errorf("cleanup state = %v/%T/%q/%v, want executing with cleared component fields", got.state, got.activeComponent, got.activeComponentType, got.componentDoneCh)
			}
		})
	}
}

func TestInteractiveModel_ChildQuitBecomesCompletion(t *testing.T) {
	t.Parallel()

	component := &testEmbeddableComponent{result: "answer"}
	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.state = stateTUI
	model.activeComponent = component
	model.activeComponentType = ComponentTypeConfirm
	responseCh := make(chan ComponentResponse, 1)
	model.componentDoneCh = responseCh

	_, cmd := model.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("child completion command = nil")
	}
	msg := cmd()
	done, ok := msg.(tuiComponentDoneMsg)
	if !ok {
		t.Fatalf("child completion message type = %T, want tuiComponentDoneMsg", msg)
	}
	if done.result != "answer" || done.err != nil || done.cancelled {
		t.Errorf("completion message = %#v, want successful answer", done)
	}
	model.Update(done)
	response := <-responseCh
	if response.Result != "answer" || response.Err != nil || response.Cancelled {
		t.Errorf("response = %#v, want successful answer", response)
	}
	if model.state != stateExecuting {
		t.Errorf("state = %v, want stateExecuting", model.state)
	}
}

func (r *resizeRecordingTerminal) Resize(width, height int) error {
	r.sizes = append(r.sizes, [2]int{width, height})
	return nil
}

func TestInteractiveCommandArea(t *testing.T) {
	t.Parallel()

	tests := []struct{ w, h, wantCols, wantRows TerminalDimension }{
		{w: 100, h: 30, wantCols: 100, wantRows: 27},
		{w: 80, h: 24, wantCols: 80, wantRows: 21},
		{w: 10, h: 3, wantCols: 10, wantRows: 1},
		{w: 10, h: 1, wantCols: 10, wantRows: 1},
	}
	for _, tt := range tests {
		cols, rows := InteractiveCommandArea(tt.w, tt.h)
		if cols != tt.wantCols || rows != tt.wantRows {
			t.Errorf("InteractiveCommandArea(%d, %d) = %dx%d, want %dx%d", tt.w, tt.h, cols, rows, tt.wantCols, tt.wantRows)
		}
	}
}

// TestInteractiveModel_WindowSizeResizesTerminalWhileRunning: the command's
// terminal follows the viewer's command area in every running state, with a
// server component overlay too, and no longer after completion.
func TestInteractiveModel_WindowSizeResizesTerminalWhileRunning(t *testing.T) {
	t.Parallel()

	for _, state := range []executionState{stateExecuting, stateTUI, stateCompleted} {
		t.Run(state.String(), func(t *testing.T) {
			t.Parallel()
			term := &resizeRecordingTerminal{}
			model := newInteractiveModel(InteractiveOptions{}, term)
			model.state = state
			model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			if state == stateCompleted {
				if len(term.sizes) != 0 {
					t.Fatalf("resizes after completion = %v, want none", term.sizes)
				}
				return
			}
			if len(term.sizes) != 1 || term.sizes[0] != [2]int{100, 27} {
				t.Fatalf("resizes = %v, want one to 100x27", term.sizes)
			}
		})
	}
}

func sendStyledText(t *testing.T, model *interactiveModel, opts StyledTextOptions) {
	t.Helper()

	responseCh := make(chan ComponentResponse, 1)
	_, cmd := model.handleTUIComponentRequest(TUIComponentMsg{
		Component:  ComponentTypeWrite,
		Options:    opts,
		ResponseCh: responseCh,
	})
	if cmd != nil {
		t.Fatalf("styled text returned non-nil cmd in state %v", model.state)
	}
	resp := <-responseCh
	if resp.Err != nil {
		t.Fatalf("styled text response error = %v", resp.Err)
	}
}

func TestInteractiveModel_StyledText_InterleavedWithSplitSequences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		chunk1      string
		styled      StyledTextOptions
		chunk2      string
		wantContain []string
		wantAbsent  []string
		checkRedAt  *[2]int
	}{
		{
			name:        "split CSI SGR sequence",
			chunk1:      "before\x1b[3",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "1mRED\x1b[0m",
			wantContain: []string{"before", "MSG", "RED"},
			wantAbsent:  []string{"1m"},
			checkRedAt:  &[2]int{0, 1},
		},
		{
			name:        "lone ESC",
			chunk1:      "before\x1b",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "[32mGREEN\x1b[0m",
			wantContain: []string{"before", "MSG", "GREEN"},
			wantAbsent:  []string{"32m"},
		},
		{
			name:        "split OSC with BEL",
			chunk1:      "before\x1b]0;my",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "title\x07after",
			wantContain: []string{"before", "MSG", "after"},
			wantAbsent:  []string{"mytitle"},
		},
		{
			name:        "split OSC with ST",
			chunk1:      "before\x1b]2;title\x1b",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "\\after",
			wantContain: []string{"before", "MSG", "after"},
			wantAbsent:  []string{"title"},
		},
		{
			name:        "split DCS with ST",
			chunk1:      "before\x1bP1$q",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "secret\x1b\\after",
			wantContain: []string{"before", "MSG", "after"},
			wantAbsent:  []string{"secret"},
		},
		{
			name:        "split SOS with ST",
			chunk1:      "before\x1bXsos",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "\x1b\\after",
			wantContain: []string{"before", "MSG", "after"},
			wantAbsent:  []string{"sos"},
		},
		{
			name:        "split 7-bit SS2",
			chunk1:      "before\x1bN",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "xafter",
			wantContain: []string{"before", "MSG", "xafter"},
		},
		{
			name:        "split 4-byte UTF-8 rune",
			chunk1:      "before \xf0\x9f",
			styled:      StyledTextOptions{Text: "MSG"},
			chunk2:      "\x9a\x80after",
			wantContain: []string{"before", "MSG", "🚀after"},
			wantAbsent:  []string{"\ufffd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model := newSizedInteractiveModel(t, nil)
			model.Update(outputMsg{data: []byte(tt.chunk1)})
			sendStyledText(t, model, tt.styled)
			model.Update(outputMsg{data: []byte(tt.chunk2)})

			screen := screenText(model)
			for _, want := range tt.wantContain {
				if !strings.Contains(screen, want) {
					t.Errorf("screen missing %q:\n%s", want, screen)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(screen, absent) {
					t.Errorf("screen unexpectedly contains %q:\n%s", absent, screen)
				}
			}
			if tt.checkRedAt != nil {
				cell := model.screen.emu.CellAt(tt.checkRedAt[0], tt.checkRedAt[1])
				if cell == nil || cell.Style.Fg == nil {
					t.Fatalf("expected foreground color on cell %v, got %+v", *tt.checkRedAt, cell)
				}
				r, g, b, _ := cell.Style.Fg.RGBA()
				if r <= g || r <= b {
					t.Errorf("expected red foreground on cell %v, got RGBA (%d,%d,%d)", *tt.checkRedAt, r, g, b)
				}
			}
		})
	}
}

func TestInteractiveModel_StyledText_SGRAttributeIsolation(t *testing.T) {
	t.Parallel()

	model := newSizedInteractiveModel(t, nil)
	model.Update(outputMsg{data: []byte("\x1b[31;44mCMD1\r\n")})
	sendStyledText(t, model, StyledTextOptions{
		Text:  "STYLED",
		Style: Style{Foreground: "#00ff00"},
	})
	model.Update(outputMsg{data: []byte("CMD2\r\n")})

	cmd1Cell := model.screen.emu.CellAt(0, 0)
	if cmd1Cell == nil || cmd1Cell.Style.Fg == nil || cmd1Cell.Style.Bg == nil {
		t.Fatalf("CMD1 cell missing expected command colors: %+v", cmd1Cell)
	}

	styledCell := model.screen.emu.CellAt(0, 1)
	if styledCell == nil || styledCell.Content != "S" {
		t.Fatalf("unexpected STYLED cell: %+v", styledCell)
	}
	if styledCell.Style.Bg != nil {
		t.Errorf("command background leaked into STYLED cell: %+v", styledCell.Style.Bg)
	}
	if styledCell.Style.Fg == nil {
		t.Fatal("STYLED cell missing its green foreground")
	}
	r, g, b, _ := styledCell.Style.Fg.RGBA()
	if g <= r || g <= b {
		t.Errorf("expected green foreground on STYLED cell, got RGBA (%d,%d,%d)", r, g, b)
	}

	cmd2Cell := model.screen.emu.CellAt(0, 2)
	if cmd2Cell == nil || cmd2Cell.Content != "C" {
		t.Fatalf("unexpected CMD2 cell: %+v", cmd2Cell)
	}
	if cmd2Cell.Style.Fg != nil || cmd2Cell.Style.Bg != nil {
		t.Errorf("STYLED attributes leaked into CMD2 cell: fg=%v bg=%v", cmd2Cell.Style.Fg, cmd2Cell.Style.Bg)
	}
}

func TestInteractiveModel_StyledText_MalformedInputAndDirtyRecovery(t *testing.T) {
	t.Parallel()

	t.Run("unclosed DCS in styled text", func(t *testing.T) {
		t.Parallel()

		model := newSizedInteractiveModel(t, nil)
		sendStyledText(t, model, StyledTextOptions{
			Text:  "warn\x1bP",
			Style: Style{Foreground: "1"},
		})
		model.Update(outputMsg{data: []byte("\x1b[32mAFTER\x1b[0m")})

		screen := screenText(model)
		if !strings.Contains(screen, "warn") || !strings.Contains(screen, "AFTER") {
			t.Fatalf("screen missing expected text:\n%s", screen)
		}
		// Unclosed DCS swallows body's trailing "\r\n", so AFTER starts at col 4 of row 0.
		afterCell := model.screen.emu.CellAt(len("warn"), 0)
		if afterCell == nil || afterCell.Content != "A" || afterCell.Style.Fg == nil {
			t.Fatalf("AFTER lost SGR color after malformed styled text: %+v\nscreen:\n%s", afterCell, screen)
		}
	})

	t.Run("truncated UTF-8 in styled text", func(t *testing.T) {
		t.Parallel()

		model := newSizedInteractiveModel(t, nil)
		sendStyledText(t, model, StyledTextOptions{Text: "u\xf0"})
		model.Update(outputMsg{data: []byte("\x1b[32mAFTER\x1b[0m")})

		if screen := screenText(model); !strings.Contains(screen, "AFTER") {
			t.Fatalf("screen missing AFTER after truncated UTF-8 in styled text:\n%s", screen)
		}
	})

	t.Run("recovery after forced idle flush", func(t *testing.T) {
		t.Parallel()

		model := newSizedInteractiveModel(t, nil)
		model.escapeFlushTimeout = 5 * time.Millisecond
		_, cmd := model.Update(outputMsg{data: []byte("head\x1b")})
		if cmd == nil {
			t.Fatal("expected idle flush command for trailing ESC")
		}
		model.Update(cmd())

		sendStyledText(t, model, StyledTextOptions{Text: "MSG"})
		if screen := screenText(model); !strings.Contains(screen, "headMSG") {
			t.Fatalf("screen = %q, want headMSG intact after dirty-parser recovery", screen)
		}
	})
}

func TestInteractiveModel_EscapeFlush_BoundedAndGenerationGated(t *testing.T) {
	t.Parallel()

	t.Run("lone ESC flushes on matching timer", func(t *testing.T) {
		t.Parallel()

		model := newSizedInteractiveModel(t, nil)
		model.escapeFlushTimeout = 5 * time.Millisecond

		_, cmd := model.Update(outputMsg{data: []byte("lead\x1b")})
		if cmd == nil {
			t.Fatal("expected escapeFlushMsg tick command")
		}
		if !model.ansiBuf.hasPending() || model.ansiBuf.emuState != ansiStateGround {
			t.Fatalf("unexpected buffer state before flush: pending=%t emuState=%s", model.ansiBuf.hasPending(), model.ansiBuf.emuState)
		}

		msg := cmd()
		if _, ok := msg.(escapeFlushMsg); !ok {
			t.Fatalf("tick message = %T, want escapeFlushMsg", msg)
		}
		model.Update(msg)
		if model.ansiBuf.hasPending() || model.ansiBuf.emuState != ansiStateEscape {
			t.Fatalf("unexpected buffer state after flush: pending=%t emuState=%s", model.ansiBuf.hasPending(), model.ansiBuf.emuState)
		}
	})

	t.Run("stale escapeFlushMsg does not flush newer sequence", func(t *testing.T) {
		t.Parallel()

		model := newSizedInteractiveModel(t, nil)
		model.escapeFlushTimeout = 5 * time.Millisecond

		_, cmd1 := model.Update(outputMsg{data: []byte("a\x1b")})
		_, cmd2 := model.Update(outputMsg{data: []byte("[31mb\x1b[3")})
		if cmd1 == nil || cmd2 == nil {
			t.Fatal("expected non-nil tick commands for trailing sequences")
		}

		model.Update(cmd1())
		if !model.ansiBuf.hasPending() || string(model.ansiBuf.buf) != "\x1b[3" {
			t.Fatalf("stale flush disturbed newer sequence; pending=%q", string(model.ansiBuf.buf))
		}

		model.Update(cmd2())
		if model.ansiBuf.hasPending() || model.ansiBuf.emuState != ansiStateCSIParam {
			t.Fatalf("fresh flush failed: pending=%t emuState=%s", model.ansiBuf.hasPending(), model.ansiBuf.emuState)
		}
	})

	t.Run("doneMsg flushes pending tail and recovers before leaving alt screen", func(t *testing.T) {
		t.Parallel()

		model := newSizedInteractiveModel(t, nil)
		model.Update(outputMsg{data: []byte("main\r\n\x1b[?1049halt\x1b[3")})
		if !model.screen.emu.IsAltScreen() {
			t.Fatal("expected emulator in alt screen before doneMsg")
		}
		model.Update(doneMsg{result: InteractiveResult{}})
		if model.screen.emu.IsAltScreen() {
			t.Fatal("emulator remained in alt screen after doneMsg")
		}
		lines := completedLines(model)
		if indexOfLine(lines, "main") != 0 || indexOfLine(lines, "alt") != -1 {
			t.Fatalf("completed lines = %q, want main without alt", lines)
		}
	})
}
