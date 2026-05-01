// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiwire"
)

func TestComponentResponseToProtocolStatus(t *testing.T) {
	t.Parallel()

	t.Run("cancelled", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeInput, tui.ComponentResponse{Cancelled: true})
		if !got.Cancelled {
			t.Fatal("Cancelled = false, want true")
		}
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeInput, tui.ComponentResponse{Err: errors.New("boom")})
		if got.Error != "boom" {
			t.Fatalf("Error = %q, want boom", got.Error)
		}
	})
}

func TestComponentResponseToProtocolResults(t *testing.T) {
	t.Parallel()

	t.Run("input result", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeInput, tui.ComponentResponse{Result: "hello"})
		result := decodeComponentResult[tuiwire.InputResult](t, got)
		if result.Value != "hello" {
			t.Fatalf("Value = %q, want hello", result.Value)
		}
	})

	t.Run("textarea result", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeTextArea, tui.ComponentResponse{Result: "hello\nthere"})
		result := decodeComponentResult[tuiwire.TextAreaResult](t, got)
		if result.Value != "hello\nthere" {
			t.Fatalf("Value = %q, want textarea text", result.Value)
		}
	})

	t.Run("write result", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeWrite, tui.ComponentResponse{Result: "ignored"})
		_ = decodeComponentResult[tuiwire.WriteResult](t, got)
	})

	t.Run("table result", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeTable, tui.ComponentResponse{
			Result: tui.TableSelectionResult{
				SelectedRow:   []string{"a", "b"},
				SelectedIndex: 1,
			},
		})
		result := decodeComponentResult[tuiwire.TableResult](t, got)
		if result.SelectedIndex != 1 {
			t.Fatalf("SelectedIndex = %d, want 1", result.SelectedIndex)
		}
		if len(result.SelectedRow) != 2 || result.SelectedRow[0] != "a" {
			t.Fatalf("SelectedRow = %v, want [a b]", result.SelectedRow)
		}
	})

	t.Run("spin result", func(t *testing.T) {
		t.Parallel()

		got := componentResponseToProtocol(tui.ComponentTypeSpin, tui.ComponentResponse{
			Result: tuiwire.SpinResult{},
		})
		_ = decodeComponentResult[tuiwire.SpinResult](t, got)
	})
}

func TestComponentRequestFromProtocolChoose(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(tuiwire.ChooseRequest{
		Title:   "Pick",
		Options: []string{"one", "two"},
		Limit:   2,
		NoLimit: true,
		Height:  7,
	})
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}

	got, err := componentRequestFromProtocol(tui.ComponentTypeChoose, raw)
	if err != nil {
		t.Fatalf("componentRequestFromProtocol() = %v", err)
	}
	opts, ok := got.(tui.ChooseStringOptions)
	if !ok {
		t.Fatalf("got %T, want tui.ChooseStringOptions", got)
	}
	if opts.Title != "Pick" || opts.Limit != 2 || !opts.NoLimit || opts.Height != 7 {
		t.Fatalf("opts = %+v", opts)
	}
	if len(opts.Options) != 2 || opts.Options[0] != "one" || opts.Options[1] != "two" {
		t.Fatalf("Options = %v", opts.Options)
	}
}

func TestComponentRequestFromProtocolChooseRejectsUnsupportedFields(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(tuiwire.ChooseRequest{
		Title:       "Pick",
		Description: "not supported by renderer",
		Options:     []string{"one", "two"},
	})
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}

	if _, err := componentRequestFromProtocol(tui.ComponentTypeChoose, raw); err == nil {
		t.Fatal("componentRequestFromProtocol() error = nil, want unsupported field error")
	}
}

func TestComponentRequestFromProtocolWriteMapsStyle(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(tuiwire.WriteRequest{
		Text:          "hello",
		Foreground:    "212",
		Background:    "#000000",
		Bold:          true,
		Italic:        true,
		Underline:     true,
		Strikethrough: true,
		Faint:         true,
		Blink:         true,
		Border:        "rounded",
		Align:         "center",
		Padding:       []int{1, 2, 3, 4},
		Margin:        []int{4, 3, 2, 1},
		Width:         40,
	})
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}

	got, err := componentRequestFromProtocol(tui.ComponentTypeWrite, raw)
	if err != nil {
		t.Fatalf("componentRequestFromProtocol() = %v", err)
	}
	opts, ok := got.(tui.StyledTextOptions)
	if !ok {
		t.Fatalf("got %T, want tui.StyledTextOptions", got)
	}
	if opts.Text != "hello" || opts.Width != 40 {
		t.Fatalf("opts = %+v", opts)
	}
	if opts.Style.Foreground != "212" || opts.Style.Background != "#000000" || opts.Style.Border != tui.BorderRounded || opts.Style.Align != tui.AlignCenter {
		t.Fatalf("style = %+v", opts.Style)
	}
	if !opts.Style.Bold || !opts.Style.Italic || !opts.Style.Underline || !opts.Style.Strikethrough || !opts.Style.Faint || !opts.Style.Blink {
		t.Fatalf("style booleans = %+v", opts.Style)
	}
	if len(opts.Style.Padding) != 4 || opts.Style.Padding[1] != 2 {
		t.Fatalf("Padding = %v", opts.Style.Padding)
	}
	if len(opts.Style.Margin) != 4 || opts.Style.Margin[1] != 3 {
		t.Fatalf("Margin = %v", opts.Style.Margin)
	}
}

func decodeComponentResult[T any](t testing.TB, got tuiwire.Response) T {
	t.Helper()

	var result T
	if err := json.Unmarshal(got.Result, &result); err != nil {
		t.Fatalf("json.Unmarshal() = %v", err)
	}
	return result
}
