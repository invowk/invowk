// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestChooseModel_MultiSelectToggle(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select multiple",
		Options: []string{"A", "B", "C"},
		Limit:   3, // Multi-select mode
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Verify initial state
	if len(model.selected) != 0 {
		t.Errorf("expected 0 selections initially, got %d", len(model.selected))
	}

	// Press space to toggle first item
	keyMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*chooseModel)

	if !m.selected[0] {
		t.Error("expected first item to be selected after space")
	}

	// Press space again to deselect
	updatedModel, _ = m.Update(keyMsg)
	m = updatedModel.(*chooseModel)

	if m.selected[0] {
		t.Error("expected first item to be deselected after second space")
	}
}

func TestChooseModel_MultiSelectToggleWithX(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select multiple",
		Options: []string{"A", "B", "C"},
		Limit:   3, // Multi-select mode
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Press 'x' to toggle
	keyMsg := tea.KeyPressMsg{Code: 'x', Text: "x"}
	updatedModel, _ := model.Update(keyMsg)
	m := updatedModel.(*chooseModel)

	if !m.selected[0] {
		t.Error("expected first item to be selected after 'x'")
	}
}

func TestChooseModel_MultiSelectNavigation(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select multiple",
		Options: []string{"A", "B", "C"},
		Limit:   3, // Multi-select mode
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Verify initial cursor position (via list.Index())
	if model.list.Index() != 0 {
		t.Errorf("expected cursor at 0, got %d", model.list.Index())
	}

	// Press down to move cursor
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}
	updatedModel, _ := model.Update(downMsg)
	m := updatedModel.(*chooseModel)

	if m.list.Index() != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", m.list.Index())
	}

	// Press 'j' to move cursor down again
	jMsg := tea.KeyPressMsg{Code: 'j', Text: "j"}
	updatedModel, _ = m.Update(jMsg)
	m = updatedModel.(*chooseModel)

	if m.list.Index() != 2 {
		t.Errorf("expected cursor at 2 after 'j', got %d", m.list.Index())
	}

	// Cursor should not go past last item
	updatedModel, _ = m.Update(downMsg)
	m = updatedModel.(*chooseModel)

	if m.list.Index() != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.list.Index())
	}

	// Press up to move cursor back
	upMsg := tea.KeyPressMsg{Code: tea.KeyUp}
	updatedModel, _ = m.Update(upMsg)
	m = updatedModel.(*chooseModel)

	if m.list.Index() != 1 {
		t.Errorf("expected cursor at 1 after up, got %d", m.list.Index())
	}

	// Press 'k' to move cursor up again
	kMsg := tea.KeyPressMsg{Code: 'k', Text: "k"}
	updatedModel, _ = m.Update(kMsg)
	m = updatedModel.(*chooseModel)

	if m.list.Index() != 0 {
		t.Errorf("expected cursor at 0 after 'k', got %d", m.list.Index())
	}

	// Cursor should not go below 0
	updatedModel, _ = m.Update(upMsg)
	m = updatedModel.(*chooseModel)

	if m.list.Index() != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.list.Index())
	}
}

func TestChooseModel_MultiSelectWithLimit(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select at most 2",
		Options: []string{"A", "B", "C"},
		Limit:   2, // Allow max 2 selections
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}

	// Select first item
	updatedModel, _ := model.Update(spaceMsg)
	m := updatedModel.(*chooseModel)

	if len(m.selected) != 1 {
		t.Errorf("expected 1 selection, got %d", len(m.selected))
	}

	// Move to second and select
	updatedModel, _ = m.Update(downMsg)
	m = updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)

	if len(m.selected) != 2 {
		t.Errorf("expected 2 selections, got %d", len(m.selected))
	}

	// Move to third and try to select (should fail due to limit)
	updatedModel, _ = m.Update(downMsg)
	m = updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)

	if len(m.selected) != 2 {
		t.Errorf("expected selections to stay at 2 due to limit, got %d", len(m.selected))
	}
}

func TestChooseModel_MultiSelectNoLimit(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select any",
		Options: []string{"A", "B", "C"},
		NoLimit: true, // Unlimited selections
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}

	// Select all items
	updatedModel, _ := model.Update(spaceMsg)
	m := updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(downMsg)
	m = updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(downMsg)
	m = updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)

	if len(m.selected) != 3 {
		t.Errorf("expected 3 selections with NoLimit, got %d", len(m.selected))
	}
}

func TestChooseModel_SingleSelectIgnoresSpaceToggle(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select one",
		Options: []string{"A", "B", "C"},
		Limit:   0, // Single-select mode
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Press space - should NOT toggle in single-select mode
	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	updatedModel, _ := model.Update(spaceMsg)
	m := updatedModel.(*chooseModel)

	// In single-select mode, selected map should remain empty
	// (selection happens via huh form on Enter)
	if len(m.selected) != 0 {
		t.Errorf("expected no selections in single-select mode, got %d", len(m.selected))
	}
}

func TestChooseModel_SyncSelectionsOrder(t *testing.T) {
	t.Parallel()

	opts := ChooseStringOptions{
		Title:   "Select multiple",
		Options: []string{"A", "B", "C"},
		Limit:   3,
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	downMsg := tea.KeyPressMsg{Code: tea.KeyDown}

	// Select C, then A (out of order)
	// Move to C
	updatedModel, _ := model.Update(downMsg)
	m := updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(downMsg)
	m = updatedModel.(*chooseModel)
	// Select C
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)

	// Move back to A
	upMsg := tea.KeyPressMsg{Code: tea.KeyUp}
	updatedModel, _ = m.Update(upMsg)
	m = updatedModel.(*chooseModel)
	updatedModel, _ = m.Update(upMsg)
	m = updatedModel.(*chooseModel)
	// Select A
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)

	// Verify multiResult is in order (A, C)
	if len(*m.multiResult) != 2 {
		t.Fatalf("expected 2 results, got %d", len(*m.multiResult))
	}
	if (*m.multiResult)[0] != "A" {
		t.Errorf("expected first result 'A', got %q", (*m.multiResult)[0])
	}
	if (*m.multiResult)[1] != "C" {
		t.Errorf("expected second result 'C', got %q", (*m.multiResult)[1])
	}
}

func TestChooseModel_MultiSelectVisualFeedback(t *testing.T) {
	t.Parallel()

	// This test verifies that the multi-select model renders checkbox indicators
	// that change when selections are toggled. This was the core bug: huh.MultiSelect
	// didn't show visual feedback when embedded in modal overlays.
	opts := ChooseStringOptions{
		Title:   "Select multiple",
		Options: []string{"Option A", "Option B", "Option C"},
		Limit:   3,
		Config:  DefaultConfig(),
	}

	model := NewChooseModel(opts)

	// Initial view should show unchecked boxes
	view := model.View().Content
	if !strings.Contains(view, "[ ]") {
		t.Error("expected unchecked boxes [ ] in initial view")
	}

	// Toggle first item
	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	updatedModel, _ := model.Update(spaceMsg)
	m := updatedModel.(*chooseModel)

	// View should now show checked box for first item
	view = m.View().Content
	if !strings.Contains(view, "[x]") {
		t.Error("expected checked box [x] after toggle")
	}

	// Should still have unchecked boxes for other items
	if !strings.Contains(view, "[ ]") {
		t.Error("expected some unchecked boxes [ ] to remain")
	}

	// Toggle again to deselect
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*chooseModel)

	// Should be back to all unchecked
	view = m.View().Content
	// Count occurrences of checked boxes
	checkedCount := strings.Count(view, "[x]")
	if checkedCount != 0 {
		t.Errorf("expected 0 checked boxes after deselect, got %d", checkedCount)
	}
}

func TestChooseModel_MultiSelectForModal(t *testing.T) {
	t.Parallel()

	// Verify modal version also shows visual feedback
	opts := ChooseStringOptions{
		Title:   "Modal Select",
		Options: []string{"A", "B"},
		Limit:   2,
		Config:  DefaultConfig(),
	}

	model := NewChooseModelForModal(opts)

	// Should use bubbles/list for multi-select
	if !model.isMulti {
		t.Error("expected multi-select mode for Limit > 1")
	}

	// Toggle and verify visual feedback
	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	updatedModel, _ := model.Update(spaceMsg)
	m := updatedModel.(*chooseModel)

	view := m.View().Content
	if !strings.Contains(view, "[x]") {
		t.Error("expected checked box [x] in modal view after toggle")
	}
}

func TestChooseStringOptions_JSONUnmarshal(t *testing.T) {
	t.Parallel()

	// This test verifies that ChooseStringOptions can be unmarshaled from JSON
	// using snake_case field names, which matches the protocol in protocol.go.
	// This was a critical bug: the TUI server sends JSON with snake_case fields
	// (e.g., "no_limit"), but Go's json package doesn't convert snake_case to
	// PascalCase without explicit tags.
	tests := []struct {
		name        string
		jsonInput   string
		wantTitle   string
		wantOptions []string
		wantLimit   int
		wantNoLimit bool
		wantHeight  TerminalDimension
	}{
		{
			name:        "single select with all fields",
			jsonInput:   `{"title":"Pick one","options":["A","B","C"],"limit":1,"height":10}`,
			wantTitle:   "Pick one",
			wantOptions: []string{"A", "B", "C"},
			wantLimit:   1,
			wantNoLimit: false,
			wantHeight:  10,
		},
		{
			name:        "multi-select with limit",
			jsonInput:   `{"title":"Pick multiple","options":["X","Y"],"limit":2}`,
			wantTitle:   "Pick multiple",
			wantOptions: []string{"X", "Y"},
			wantLimit:   2,
			wantNoLimit: false,
		},
		{
			name:        "multi-select with no_limit (snake_case)",
			jsonInput:   `{"title":"Pick any","options":["1","2","3"],"no_limit":true}`,
			wantTitle:   "Pick any",
			wantOptions: []string{"1", "2", "3"},
			wantLimit:   0,
			wantNoLimit: true, // Critical: this MUST be true after unmarshal
		},
		{
			name:        "minimal options only",
			jsonInput:   `{"options":["only","options"]}`,
			wantOptions: []string{"only", "options"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var opts ChooseStringOptions
			if err := json.Unmarshal([]byte(tt.jsonInput), &opts); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}

			if opts.Title != tt.wantTitle {
				t.Errorf("Title: got %q, want %q", opts.Title, tt.wantTitle)
			}
			if len(opts.Options) != len(tt.wantOptions) {
				t.Errorf("Options length: got %d, want %d", len(opts.Options), len(tt.wantOptions))
			} else {
				for i, want := range tt.wantOptions {
					if opts.Options[i] != want {
						t.Errorf("Options[%d]: got %q, want %q", i, opts.Options[i], want)
					}
				}
			}
			if opts.Limit != tt.wantLimit {
				t.Errorf("Limit: got %d, want %d", opts.Limit, tt.wantLimit)
			}
			if opts.NoLimit != tt.wantNoLimit {
				t.Errorf("NoLimit: got %v, want %v", opts.NoLimit, tt.wantNoLimit)
			}
			if opts.Height != tt.wantHeight {
				t.Errorf("Height: got %d, want %d", opts.Height, tt.wantHeight)
			}
		})
	}
}

func TestChooseStringOptions_JSONUnmarshalEnablesMultiSelect(t *testing.T) {
	t.Parallel()

	// This test verifies the complete flow: JSON unmarshal -> model creation.
	// When "no_limit": true is sent via JSON (as the TUI server does), the
	// resulting model MUST be in multi-select mode with checkboxes.
	jsonInput := `{"title":"Select any","options":["A","B","C"],"no_limit":true}`

	var opts ChooseStringOptions
	if err := json.Unmarshal([]byte(jsonInput), &opts); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Create model from unmarshaled options
	model := NewChooseModelForModal(opts)

	// CRITICAL: Model must be in multi-select mode
	if !model.isMulti {
		t.Fatal("expected multi-select mode when no_limit=true is unmarshaled from JSON")
	}

	// Verify checkboxes render correctly
	view := model.View().Content
	if !strings.Contains(view, "[ ]") {
		t.Error("expected unchecked boxes [ ] in multi-select view")
	}

	// Toggle and verify visual feedback
	spaceMsg := tea.KeyPressMsg{Code: ' ', Text: " "}
	updatedModel, _ := model.Update(spaceMsg)
	m := updatedModel.(*chooseModel)

	view = m.View().Content
	if !strings.Contains(view, "[x]") {
		t.Error("expected checked box [x] after toggle in multi-select mode")
	}
}
