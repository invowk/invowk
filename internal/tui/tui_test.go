// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestStyleApply(t *testing.T) {
	t.Parallel()

	const text = "hello world"
	unstyled := Style{}.Apply(text)

	t.Run("empty style returns text", func(t *testing.T) {
		t.Parallel()
		result := Style{}.Apply(text)
		if !strings.Contains(result, text) {
			t.Errorf("Apply() = %q, should contain %q", result, text)
		}
	})

	t.Run("foreground changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Foreground: "#ff0000"}.Apply(text)
		if result == unstyled {
			t.Error("foreground style should change output")
		}
		if !strings.Contains(result, text) {
			t.Errorf("Apply() = %q, should contain %q", result, text)
		}
	})

	t.Run("background changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Background: "#0000ff"}.Apply(text)
		if result == unstyled {
			t.Error("background style should change output")
		}
	})

	t.Run("bold changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Bold: true}.Apply(text)
		if result == unstyled {
			t.Error("bold style should change output")
		}
	})

	t.Run("italic changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Italic: true}.Apply(text)
		if result == unstyled {
			t.Error("italic style should change output")
		}
	})

	t.Run("underline changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Underline: true}.Apply(text)
		if result == unstyled {
			t.Error("underline style should change output")
		}
	})

	t.Run("strikethrough changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Strikethrough: true}.Apply(text)
		if result == unstyled {
			t.Error("strikethrough style should change output")
		}
	})

	t.Run("faint changes output", func(t *testing.T) {
		t.Parallel()
		result := Style{Faint: true}.Apply(text)
		if result == unstyled {
			t.Error("faint style should change output")
		}
	})

	t.Run("padding 1 element", func(t *testing.T) {
		t.Parallel()
		result := Style{Padding: []int{1}}.Apply(text)
		if result == unstyled {
			t.Error("padding should change output")
		}
	})

	t.Run("padding 2 elements", func(t *testing.T) {
		t.Parallel()
		result := Style{Padding: []int{1, 2}}.Apply(text)
		if result == unstyled {
			t.Error("padding should change output")
		}
	})

	t.Run("padding 4 elements", func(t *testing.T) {
		t.Parallel()
		result := Style{Padding: []int{1, 2, 1, 2}}.Apply(text)
		if result == unstyled {
			t.Error("padding should change output")
		}
	})

	t.Run("padding 3 elements ignored", func(t *testing.T) {
		t.Parallel()
		result := Style{Padding: []int{1, 2, 3}}.Apply(text)
		if result != unstyled {
			t.Error("3-element padding should be ignored")
		}
	})

	t.Run("margin 1 element", func(t *testing.T) {
		t.Parallel()
		result := Style{Margin: []int{1}}.Apply(text)
		if result == unstyled {
			t.Error("margin should change output")
		}
	})

	t.Run("margin 2 elements", func(t *testing.T) {
		t.Parallel()
		result := Style{Margin: []int{1, 2}}.Apply(text)
		if result == unstyled {
			t.Error("margin should change output")
		}
	})

	t.Run("margin 4 elements", func(t *testing.T) {
		t.Parallel()
		result := Style{Margin: []int{1, 2, 1, 2}}.Apply(text)
		if result == unstyled {
			t.Error("margin should change output")
		}
	})

	t.Run("border normal", func(t *testing.T) {
		t.Parallel()
		result := Style{Border: BorderNormal}.Apply(text)
		if result == unstyled {
			t.Error("border normal should change output")
		}
	})

	t.Run("border rounded", func(t *testing.T) {
		t.Parallel()
		result := Style{Border: BorderRounded}.Apply(text)
		if result == unstyled {
			t.Error("border rounded should change output")
		}
	})

	t.Run("border thick", func(t *testing.T) {
		t.Parallel()
		result := Style{Border: BorderThick}.Apply(text)
		if result == unstyled {
			t.Error("border thick should change output")
		}
	})

	t.Run("border double", func(t *testing.T) {
		t.Parallel()
		result := Style{Border: BorderDouble}.Apply(text)
		if result == unstyled {
			t.Error("border double should change output")
		}
	})

	t.Run("border hidden", func(t *testing.T) {
		t.Parallel()
		result := Style{Border: BorderHidden}.Apply(text)
		if result == unstyled {
			t.Error("border hidden should change output")
		}
	})

	t.Run("border none unchanged", func(t *testing.T) {
		t.Parallel()
		result := Style{Border: BorderNone}.Apply(text)
		if result != unstyled {
			t.Error("border none should not change output")
		}
	})

	t.Run("border with colors", func(t *testing.T) {
		t.Parallel()
		result := Style{
			Border:           BorderNormal,
			BorderForeground: "#ff0000",
			BorderBackground: "#0000ff",
		}.Apply(text)
		if result == unstyled {
			t.Error("border with colors should change output")
		}
	})

	t.Run("width constrains output", func(t *testing.T) {
		t.Parallel()
		result := Style{Width: 80}.Apply(text)
		if result == unstyled {
			t.Error("width should change output")
		}
	})

	t.Run("height constrains output", func(t *testing.T) {
		t.Parallel()
		result := Style{Height: 5}.Apply(text)
		if result == unstyled {
			t.Error("height should change output")
		}
	})

	t.Run("align center", func(t *testing.T) {
		t.Parallel()
		result := Style{Align: AlignCenter, Width: 40}.Apply(text)
		if !strings.Contains(result, text) {
			t.Errorf("Apply() should contain %q", text)
		}
	})

	t.Run("align right", func(t *testing.T) {
		t.Parallel()
		result := Style{Align: AlignRight, Width: 40}.Apply(text)
		if !strings.Contains(result, text) {
			t.Errorf("Apply() should contain %q", text)
		}
	})

	t.Run("combined style", func(t *testing.T) {
		t.Parallel()
		result := Style{
			Foreground: "#ffffff",
			Bold:       true,
			Border:     BorderRounded,
			Padding:    []int{1},
		}.Apply(text)
		if result == unstyled {
			t.Error("combined style should change output")
		}
		if !strings.Contains(result, text) {
			t.Errorf("Apply() should contain %q", text)
		}
	})
}

func TestIsDarkTheme(t *testing.T) {
	t.Parallel()

	tests := []struct {
		theme Theme
		want  bool
	}{
		{ThemeDefault, true},
		{ThemeCharm, true},
		{ThemeDracula, true},
		{ThemeCatppuccin, true},
		{ThemeBase16, false},
		{"unknown", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.theme), func(t *testing.T) {
			t.Parallel()
			if got := isDarkTheme(tt.theme); got != tt.want {
				t.Errorf("isDarkTheme(%q) = %v, want %v", tt.theme, got, tt.want)
			}
		})
	}
}

func TestRenderComponentView(t *testing.T) {
	t.Parallel()

	t.Run("done returns empty", func(t *testing.T) {
		t.Parallel()
		v := renderComponentView(componentViewParams{done: true})
		if v.Content != "" {
			t.Errorf("done view should be empty, got %q", v.Content)
		}
	})

	t.Run("with title and description", func(t *testing.T) {
		t.Parallel()
		v := renderComponentView(componentViewParams{
			title:         types.DescriptionText("My Title"),
			description:   types.DescriptionText("My Description"),
			componentView: "component",
			helpText:      "press enter",
		})
		if !strings.Contains(v.Content, "My Title") {
			t.Errorf("view should contain title, got %q", v.Content)
		}
		if !strings.Contains(v.Content, "My Description") {
			t.Errorf("view should contain description, got %q", v.Content)
		}
		if !strings.Contains(v.Content, "component") {
			t.Errorf("view should contain component view, got %q", v.Content)
		}
	})

	t.Run("without title", func(t *testing.T) {
		t.Parallel()
		v := renderComponentView(componentViewParams{
			componentView: "content",
			helpText:      "help",
		})
		if !strings.Contains(v.Content, "content") {
			t.Errorf("view should contain component, got %q", v.Content)
		}
	})

	t.Run("modal style", func(t *testing.T) {
		t.Parallel()
		nonModal := renderComponentView(componentViewParams{
			title:         "Title",
			componentView: "body",
			helpText:      "help",
		})
		modal := renderComponentView(componentViewParams{
			title:         "Title",
			componentView: "body",
			helpText:      "help",
			forModal:      true,
		})
		// Modal and non-modal should produce different ANSI styles
		if nonModal.Content == modal.Content {
			t.Error("modal and non-modal views should differ")
		}
	})

	t.Run("width constraint", func(t *testing.T) {
		t.Parallel()
		v := renderComponentView(componentViewParams{
			componentView: "content",
			helpText:      "help",
			width:         40,
		})
		if v.Content == "" {
			t.Error("view should not be empty")
		}
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	if cfg.Theme != ThemeDefault {
		t.Errorf("Theme = %q, want %q", cfg.Theme, ThemeDefault)
	}
	if cfg.Width != 0 {
		t.Errorf("Width = %d, want 0", cfg.Width)
	}
	if cfg.Output != os.Stdout {
		t.Errorf("Output should be os.Stdout")
	}
}
