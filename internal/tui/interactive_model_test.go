// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package tui

import (
	"encoding/json"
	"testing"

	"github.com/invowk/invowk/internal/tuiserver"

	tea "charm.land/bubbletea/v2"
)

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

	responseCh := make(chan tuiserver.Response, 1)
	msg := TUIComponentMsg{
		Component:  ComponentType("nonexistent-component"),
		Options:    json.RawMessage(`{}`),
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
	if resp.Error == "" {
		t.Error("expected non-empty error in response for unknown component type")
	}
}

func TestInteractiveModel_HandleTUIComponentRequest_ValidDispatch(t *testing.T) {
	t.Parallel()

	model := newInteractiveModel(InteractiveOptions{}, nil)
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	responseCh := make(chan tuiserver.Response, 1)
	msg := TUIComponentMsg{
		Component:  ComponentTypeConfirm,
		Options:    json.RawMessage(`{"title":"Proceed?","affirmative":"Yes","negative":"No"}`),
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
