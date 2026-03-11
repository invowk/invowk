// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"testing"

	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/types"
)

func TestConvertToProtocolResult(t *testing.T) {
	t.Parallel()

	t.Run("input with string", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeInput, "hello")
		ir, ok := result.(tuiserver.InputResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.InputResult", result)
		}
		if ir.Value != "hello" {
			t.Errorf("Value = %q, want %q", ir.Value, "hello")
		}
	})

	t.Run("input with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeInput, 42)
		ir, ok := result.(tuiserver.InputResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.InputResult", result)
		}
		if ir.Value != "" {
			t.Errorf("Value = %q, want empty for wrong type", ir.Value)
		}
	})

	t.Run("textarea with string", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeTextArea, "multi\nline")
		ir, ok := result.(tuiserver.InputResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.InputResult", result)
		}
		if ir.Value != "multi\nline" {
			t.Errorf("Value = %q, want %q", ir.Value, "multi\nline")
		}
	})

	t.Run("write with string", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeWrite, "styled text")
		ir, ok := result.(tuiserver.InputResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.InputResult", result)
		}
		if ir.Value != "styled text" {
			t.Errorf("Value = %q, want %q", ir.Value, "styled text")
		}
	})

	t.Run("confirm with true", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeConfirm, true)
		cr, ok := result.(tuiserver.ConfirmResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.ConfirmResult", result)
		}
		if !cr.Confirmed {
			t.Error("Confirmed = false, want true")
		}
	})

	t.Run("confirm with false", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeConfirm, false)
		cr, ok := result.(tuiserver.ConfirmResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.ConfirmResult", result)
		}
		if cr.Confirmed {
			t.Error("Confirmed = true, want false")
		}
	})

	t.Run("confirm with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeConfirm, "not a bool")
		cr, ok := result.(tuiserver.ConfirmResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.ConfirmResult", result)
		}
		if cr.Confirmed {
			t.Error("Confirmed should be false for wrong type")
		}
	})

	t.Run("choose with slice", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeChoose, []string{"a", "b"})
		cr, ok := result.(tuiserver.ChooseResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.ChooseResult", result)
		}
		selected, ok := cr.Selected.([]string)
		if !ok {
			t.Fatalf("Selected type = %T, want []string", cr.Selected)
		}
		if len(selected) != 2 || selected[0] != "a" {
			t.Errorf("Selected = %v, want [a b]", selected)
		}
	})

	t.Run("choose with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeChoose, 42)
		cr, ok := result.(tuiserver.ChooseResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.ChooseResult", result)
		}
		selected, ok := cr.Selected.([]string)
		if !ok {
			t.Fatalf("Selected type = %T, want []string", cr.Selected)
		}
		if len(selected) != 0 {
			t.Errorf("Selected = %v, want empty slice", selected)
		}
	})

	t.Run("filter with slice", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeFilter, []string{"x", "y"})
		fr, ok := result.(tuiserver.FilterResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.FilterResult", result)
		}
		if len(fr.Selected) != 2 || fr.Selected[0] != "x" {
			t.Errorf("Selected = %v, want [x y]", fr.Selected)
		}
	})

	t.Run("filter with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeFilter, "not a slice")
		fr, ok := result.(tuiserver.FilterResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.FilterResult", result)
		}
		if len(fr.Selected) != 0 {
			t.Errorf("Selected = %v, want empty slice", fr.Selected)
		}
	})

	t.Run("file with string", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeFile, "/tmp/test.txt")
		fr, ok := result.(tuiserver.FileResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.FileResult", result)
		}
		if fr.Path != "/tmp/test.txt" {
			t.Errorf("Path = %q, want %q", fr.Path, "/tmp/test.txt")
		}
	})

	t.Run("file with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeFile, 42)
		fr, ok := result.(tuiserver.FileResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.FileResult", result)
		}
		if fr.Path != "" {
			t.Errorf("Path = %q, want empty for wrong type", fr.Path)
		}
	})

	t.Run("table with TableSelectionResult", func(t *testing.T) {
		t.Parallel()
		input := TableSelectionResult{
			SelectedRow:   []string{"a", "b"},
			SelectedIndex: 1,
		}
		result := convertToProtocolResult(ComponentTypeTable, input)
		tr, ok := result.(tuiserver.TableResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.TableResult", result)
		}
		if tr.SelectedIndex != 1 {
			t.Errorf("SelectedIndex = %d, want 1", tr.SelectedIndex)
		}
		if len(tr.SelectedRow) != 2 || tr.SelectedRow[0] != "a" {
			t.Errorf("SelectedRow = %v, want [a b]", tr.SelectedRow)
		}
	})

	t.Run("table with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeTable, "wrong")
		tr, ok := result.(tuiserver.TableResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.TableResult", result)
		}
		if tr.SelectedIndex != -1 {
			t.Errorf("SelectedIndex = %d, want -1 for wrong type", tr.SelectedIndex)
		}
	})

	t.Run("pager returns empty result", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypePager, nil)
		_, ok := result.(tuiserver.PagerResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.PagerResult", result)
		}
	})

	t.Run("spin with SpinResult", func(t *testing.T) {
		t.Parallel()
		input := SpinResult{
			Stdout:   "output",
			Stderr:   "error",
			ExitCode: types.ExitCode(1),
		}
		result := convertToProtocolResult(ComponentTypeSpin, input)
		sr, ok := result.(tuiserver.SpinResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.SpinResult", result)
		}
		if sr.Stdout != "output" {
			t.Errorf("Stdout = %q, want %q", sr.Stdout, "output")
		}
		if sr.Stderr != "error" {
			t.Errorf("Stderr = %q, want %q", sr.Stderr, "error")
		}
		if sr.ExitCode != 1 {
			t.Errorf("ExitCode = %d, want 1", sr.ExitCode)
		}
	})

	t.Run("spin with wrong type", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentTypeSpin, "not a SpinResult")
		sr, ok := result.(tuiserver.SpinResult)
		if !ok {
			t.Fatalf("result type = %T, want tuiserver.SpinResult", result)
		}
		if sr.Stdout != "" || sr.Stderr != "" {
			t.Error("SpinResult should be zero-valued for wrong type")
		}
	})

	t.Run("unknown component returns as-is", func(t *testing.T) {
		t.Parallel()
		result := convertToProtocolResult(ComponentType("unknown"), "raw")
		s, ok := result.(string)
		if !ok {
			t.Fatalf("result type = %T, want string", result)
		}
		if s != "raw" {
			t.Errorf("result = %q, want %q", s, "raw")
		}
	})
}
