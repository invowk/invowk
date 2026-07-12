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
	tests := []struct {
		name         string
		style        Style
		wantChanged  bool
		wantContains bool
	}{
		{name: "empty style returns text", wantContains: true},
		{name: "foreground", style: Style{Foreground: "#ff0000"}, wantChanged: true, wantContains: true},
		{name: "background", style: Style{Background: "#0000ff"}, wantChanged: true},
		{name: "bold", style: Style{Bold: true}, wantChanged: true},
		{name: "italic", style: Style{Italic: true}, wantChanged: true},
		{name: "underline", style: Style{Underline: true}, wantChanged: true},
		{name: "strikethrough", style: Style{Strikethrough: true}, wantChanged: true},
		{name: "faint", style: Style{Faint: true}, wantChanged: true},
		{name: "padding 1", style: Style{Padding: []int{1}}, wantChanged: true},
		{name: "padding 2", style: Style{Padding: []int{1, 2}}, wantChanged: true},
		{name: "padding 4", style: Style{Padding: []int{1, 2, 1, 2}}, wantChanged: true},
		{name: "padding 3 ignored", style: Style{Padding: []int{1, 2, 3}}},
		{name: "margin 1", style: Style{Margin: []int{1}}, wantChanged: true},
		{name: "margin 2", style: Style{Margin: []int{1, 2}}, wantChanged: true},
		{name: "margin 4", style: Style{Margin: []int{1, 2, 1, 2}}, wantChanged: true},
		{name: "border normal", style: Style{Border: BorderNormal}, wantChanged: true},
		{name: "border rounded", style: Style{Border: BorderRounded}, wantChanged: true},
		{name: "border thick", style: Style{Border: BorderThick}, wantChanged: true},
		{name: "border double", style: Style{Border: BorderDouble}, wantChanged: true},
		{name: "border hidden", style: Style{Border: BorderHidden}, wantChanged: true},
		{name: "border none", style: Style{Border: BorderNone}},
		{name: "border colors", style: Style{Border: BorderNormal, BorderForeground: "#ff0000", BorderBackground: "#0000ff"}, wantChanged: true},
		{name: "width", style: Style{Width: 80}, wantChanged: true},
		{name: "height", style: Style{Height: 5}, wantChanged: true},
		{name: "align center", style: Style{Align: AlignCenter, Width: 40}, wantChanged: true, wantContains: true},
		{name: "align right", style: Style{Align: AlignRight, Width: 40}, wantChanged: true, wantContains: true},
		{name: "combined", style: Style{Foreground: "#ffffff", Bold: true, Border: BorderRounded, Padding: []int{1}}, wantChanged: true, wantContains: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.style.Apply(text)
			if got := result != unstyled; got != tt.wantChanged {
				t.Errorf("Apply() changed = %t, want %t; output %q", got, tt.wantChanged, result)
			}
			if tt.wantContains && !strings.Contains(result, text) {
				t.Errorf("Apply() = %q, want content %q", result, text)
			}
		})
	}
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

	tests := []struct {
		name         string
		params       componentViewParams
		wantContains []string
		wantEmpty    bool
	}{
		{name: "done", params: componentViewParams{done: true}, wantEmpty: true},
		{name: "title and description", params: componentViewParams{title: types.DescriptionText("My Title"), description: types.DescriptionText("My Description"), componentView: "component", helpText: "press enter"}, wantContains: []string{"My Title", "My Description", "component", "press enter"}},
		{name: "without title", params: componentViewParams{componentView: "content", helpText: "help"}, wantContains: []string{"content", "help"}},
		{name: "width constraint", params: componentViewParams{componentView: "content", helpText: "help", width: 40}, wantContains: []string{"content"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			view := renderComponentView(tt.params).Content
			if tt.wantEmpty && view != "" {
				t.Errorf("view = %q, want empty", view)
			}
			for _, text := range tt.wantContains {
				if !strings.Contains(view, text) {
					t.Errorf("view = %q, want content %q", view, text)
				}
			}
		})
	}

	nonModal := renderComponentView(componentViewParams{title: "Title", componentView: "body", helpText: "help"})
	modal := renderComponentView(componentViewParams{title: "Title", componentView: "body", helpText: "help", forModal: true})
	if nonModal.Content == modal.Content {
		t.Error("modal and non-modal views should differ")
	}
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
