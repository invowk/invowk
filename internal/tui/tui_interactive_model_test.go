// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type testEmbeddableComponent struct {
	done      bool
	cancelled bool
	result    any
	err       error
	view      string
}

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

	// Pre-populate content before the viewport is ready.
	model.content.WriteString("buffered output")

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

	tests := []struct {
		name      string
		options   any
		wantErr   bool
		wantText  string
		wantWidth int
	}{
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
			if (response.Err != nil) != tt.wantErr {
				t.Fatalf("response error = %v, wantErr %t", response.Err, tt.wantErr)
			}
			got := updated.(*interactiveModel)
			if got.state != stateExecuting || got.activeComponent != nil {
				t.Fatalf("styled text state = %v with component %T, want executing without component", got.state, got.activeComponent)
			}
			content := got.content.String()
			if tt.wantText != "" && (!strings.Contains(content, tt.wantText) || !strings.HasSuffix(content, "\n")) {
				t.Errorf("content = %q, want text %q ending in newline", content, tt.wantText)
			}
			if tt.wantErr && content != "" {
				t.Errorf("content = %q after rejected options, want empty", content)
			}
			if tt.wantWidth > 0 {
				line := strings.TrimSuffix(content, "\n")
				if width := lipgloss.Width(line); width != tt.wantWidth {
					t.Errorf("styled text width = %d, want %d; content %q", width, tt.wantWidth, content)
				}
			}
			if !tt.wantErr && !strings.Contains(got.viewport.View(), tt.wantText) {
				t.Errorf("viewport = %q, want styled text", got.viewport.View())
			}
		})
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
