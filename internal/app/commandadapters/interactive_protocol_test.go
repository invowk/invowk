// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiwire"
	"github.com/invowk/invowk/pkg/types"
)

func TestComponentResponseToProtocol(t *testing.T) {
	t.Parallel()

	t.Run("cancelled", func(t *testing.T) {
		t.Parallel()

		got := tui.EncodeComponentResponse(tui.ComponentTypeInput, tui.ComponentResponse{Cancelled: true})
		if !got.Cancelled {
			t.Fatal("Cancelled = false, want true")
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		got := tui.EncodeComponentResponse(tui.ComponentTypeInput, tui.ComponentResponse{Err: errors.New("boom")})
		if got.Error != "boom" {
			t.Fatalf("Error = %q, want boom", got.Error)
		}
	})

	t.Run("input result", func(t *testing.T) {
		t.Parallel()

		got := tui.EncodeComponentResponse(tui.ComponentTypeInput, tui.ComponentResponse{Result: "hello"})
		var result tuiwire.InputResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("json.Unmarshal() = %v", err)
		}
		if result.Value != "hello" {
			t.Fatalf("Value = %q, want hello", result.Value)
		}
	})

	t.Run("table result", func(t *testing.T) {
		t.Parallel()

		got := tui.EncodeComponentResponse(tui.ComponentTypeTable, tui.ComponentResponse{
			Result: tui.TableSelectionResult{
				SelectedRow:   []string{"a", "b"},
				SelectedIndex: 1,
			},
		})
		var result tuiwire.TableResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("json.Unmarshal() = %v", err)
		}
		if result.SelectedIndex != 1 {
			t.Fatalf("SelectedIndex = %d, want 1", result.SelectedIndex)
		}
		if len(result.SelectedRow) != 2 || result.SelectedRow[0] != "a" {
			t.Fatalf("SelectedRow = %v, want [a b]", result.SelectedRow)
		}
	})

	t.Run("spin result", func(t *testing.T) {
		t.Parallel()

		got := tui.EncodeComponentResponse(tui.ComponentTypeSpin, tui.ComponentResponse{
			Result: tuiwire.SpinResult{
				Stdout:   "output",
				Stderr:   "error",
				ExitCode: types.ExitCode(1),
			},
		})
		var result tuiwire.SpinResult
		if err := json.Unmarshal(got.Result, &result); err != nil {
			t.Fatalf("json.Unmarshal() = %v", err)
		}
		if result.Stdout != "output" || result.Stderr != "error" || result.ExitCode != 1 {
			t.Fatalf("SpinResult = %+v, want output/error/1", result)
		}
	})
}
