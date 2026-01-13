// SPDX-License-Identifier: EPL-2.0

// Package tui provides a clean API for terminal user interface components.
// It wraps charmbracelet/huh and charmbracelet/bubbles to provide reusable
// TUI elements that can be used both programmatically and via CLI commands.
package tui

import (
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Theme represents the visual theme for TUI components.
type Theme string

const (
	// ThemeDefault uses the default huh theme.
	ThemeDefault Theme = "default"
	// ThemeCharm uses the Charm theme.
	ThemeCharm Theme = "charm"
	// ThemeDracula uses the Dracula theme.
	ThemeDracula Theme = "dracula"
	// ThemeCatppuccin uses the Catppuccin theme.
	ThemeCatppuccin Theme = "catppuccin"
	// ThemeBase16 uses the Base16 theme.
	ThemeBase16 Theme = "base16"
)

// Config holds common configuration for TUI components.
type Config struct {
	// Theme specifies the visual theme to use.
	Theme Theme
	// Accessible enables accessible mode for screen readers.
	Accessible bool
	// Width specifies the width of the component (0 for auto).
	Width int
	// Output specifies where to write the component output.
	Output io.Writer
}

// DefaultConfig returns the default configuration for TUI components.
func DefaultConfig() Config {
	return Config{
		Theme:      ThemeDefault,
		Accessible: os.Getenv("ACCESSIBLE") != "",
		Width:      0,
		Output:     os.Stdout,
	}
}

// getHuhTheme converts a Theme to a huh.Theme.
func getHuhTheme(t Theme) *huh.Theme {
	switch t {
	case ThemeCharm:
		return huh.ThemeCharm()
	case ThemeDracula:
		return huh.ThemeDracula()
	case ThemeCatppuccin:
		return huh.ThemeCatppuccin()
	case ThemeBase16:
		return huh.ThemeBase16()
	default:
		return huh.ThemeBase()
	}
}

// ModalBackgroundColor is the background color used for modal overlays.
// This must match the background in overlayStyle() in embeddable.go.
const ModalBackgroundColor = "#1a1a2e"

// getModalHuhTheme returns a huh theme customized for modal overlays.
// It uses the modal background color to prevent color bleeding.
// IMPORTANT: All styles must explicitly have NO background set to prevent
// the default huh theme backgrounds from bleeding through the modal overlay.
func getModalHuhTheme() *huh.Theme {
	// Start with a completely fresh theme to avoid inheriting any backgrounds
	t := &huh.Theme{}

	// Define colors that work well on the modal background
	primary := lipgloss.Color("#7C3AED")   // Purple - matches modal border
	secondary := lipgloss.Color("#A78BFA") // Light purple
	text := lipgloss.Color("#FFFFFF")      // White
	dimmed := lipgloss.Color("#6B7280")    // Gray
	errorColor := lipgloss.Color("#EF4444")

	// Create base style with NO background (critical - this prevents color bleeding)
	// By not setting any background, it will be transparent and show the modal bg.
	noBackgroundStyle := lipgloss.NewStyle()

	// === FOCUSED FIELD STYLES ===
	t.Focused.Base = noBackgroundStyle
	t.Focused.Title = noBackgroundStyle.Foreground(primary).Bold(true)
	t.Focused.Description = noBackgroundStyle.Foreground(dimmed)
	t.Focused.ErrorIndicator = noBackgroundStyle.Foreground(errorColor)
	t.Focused.ErrorMessage = noBackgroundStyle.Foreground(errorColor)

	// Text input styles - NO backgrounds anywhere
	t.Focused.TextInput.Cursor = noBackgroundStyle.Foreground(text)
	t.Focused.TextInput.CursorText = noBackgroundStyle.Foreground(text)
	t.Focused.TextInput.Placeholder = noBackgroundStyle.Foreground(dimmed)
	t.Focused.TextInput.Prompt = noBackgroundStyle.Foreground(secondary)
	t.Focused.TextInput.Text = noBackgroundStyle.Foreground(text)

	// Select/option styles - NO backgrounds
	t.Focused.SelectSelector = noBackgroundStyle.Foreground(primary).Bold(true)
	t.Focused.Option = noBackgroundStyle.Foreground(text)
	t.Focused.NextIndicator = noBackgroundStyle.Foreground(dimmed)
	t.Focused.PrevIndicator = noBackgroundStyle.Foreground(dimmed)

	// Multi-select styles - NO backgrounds
	t.Focused.MultiSelectSelector = noBackgroundStyle.Foreground(primary).Bold(true)
	t.Focused.SelectedOption = noBackgroundStyle.Foreground(secondary)
	t.Focused.SelectedPrefix = noBackgroundStyle.Foreground(primary)
	t.Focused.UnselectedOption = noBackgroundStyle.Foreground(text)
	t.Focused.UnselectedPrefix = noBackgroundStyle.Foreground(dimmed)

	// Button styles - ONLY FocusedButton gets a background (it's the active button)
	t.Focused.FocusedButton = lipgloss.NewStyle().
		Foreground(text).
		Background(primary).
		Padding(0, 1)
	t.Focused.BlurredButton = noBackgroundStyle.
		Foreground(dimmed).
		Padding(0, 1)

	// File picker styles - NO backgrounds
	t.Focused.Directory = noBackgroundStyle.Foreground(primary)
	t.Focused.File = noBackgroundStyle.Foreground(text)

	// Card style (wraps each field) - NO background
	t.Focused.Card = noBackgroundStyle

	// Note (for additional info) - NO background
	t.Focused.NoteTitle = noBackgroundStyle.Foreground(primary).Bold(true)

	// === BLURRED FIELD STYLES ===
	t.Blurred.Base = noBackgroundStyle
	t.Blurred.Title = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.Description = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.ErrorIndicator = noBackgroundStyle.Foreground(errorColor)
	t.Blurred.ErrorMessage = noBackgroundStyle.Foreground(errorColor)

	// Blurred text input - NO backgrounds
	t.Blurred.TextInput.Cursor = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.TextInput.CursorText = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.TextInput.Placeholder = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.TextInput.Prompt = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.TextInput.Text = noBackgroundStyle.Foreground(dimmed)

	// Blurred select styles - NO backgrounds
	t.Blurred.SelectSelector = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.Option = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.NextIndicator = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.PrevIndicator = noBackgroundStyle.Foreground(dimmed)

	// Blurred multi-select - NO backgrounds
	t.Blurred.MultiSelectSelector = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.SelectedOption = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.SelectedPrefix = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.UnselectedOption = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.UnselectedPrefix = noBackgroundStyle.Foreground(dimmed)

	// Blurred buttons - NO backgrounds
	t.Blurred.FocusedButton = noBackgroundStyle.Foreground(dimmed).Padding(0, 1)
	t.Blurred.BlurredButton = noBackgroundStyle.Foreground(dimmed).Padding(0, 1)

	// Blurred file picker - NO backgrounds
	t.Blurred.Directory = noBackgroundStyle.Foreground(dimmed)
	t.Blurred.File = noBackgroundStyle.Foreground(dimmed)

	// Blurred card - NO background
	t.Blurred.Card = noBackgroundStyle

	// Blurred note - NO background
	t.Blurred.NoteTitle = noBackgroundStyle.Foreground(dimmed)

	// === HELP STYLES ===
	t.Help.ShortKey = noBackgroundStyle.Foreground(dimmed)
	t.Help.ShortDesc = noBackgroundStyle.Foreground(dimmed)
	t.Help.ShortSeparator = noBackgroundStyle.Foreground(dimmed)
	t.Help.FullKey = noBackgroundStyle.Foreground(dimmed)
	t.Help.FullDesc = noBackgroundStyle.Foreground(dimmed)
	t.Help.FullSeparator = noBackgroundStyle.Foreground(dimmed)
	t.Help.Ellipsis = noBackgroundStyle.Foreground(dimmed)

	// === FORM STYLES ===
	// These are empty to prevent any form-level background bleeding
	t.Form = huh.FormStyles{}

	return t
}

// Style represents styling options for text output.
type Style struct {
	// Foreground color (CSS hex, ANSI code, or color name).
	Foreground string
	// Background color (CSS hex, ANSI code, or color name).
	Background string
	// Bold enables bold text.
	Bold bool
	// Italic enables italic text.
	Italic bool
	// Underline enables underlined text.
	Underline bool
	// Strikethrough enables strikethrough text.
	Strikethrough bool
	// Faint enables faint/dim text.
	Faint bool
	// Blink enables blinking text.
	Blink bool
	// Padding adds padding around the text (top, right, bottom, left or single value for all).
	Padding []int
	// Margin adds margin around the text (top, right, bottom, left or single value for all).
	Margin []int
	// Border type (none, normal, rounded, thick, double, hidden).
	Border string
	// BorderForeground color for the border.
	BorderForeground string
	// BorderBackground color for the border.
	BorderBackground string
	// Width sets the width of the text block.
	Width int
	// Height sets the height of the text block.
	Height int
	// Align sets text alignment (left, center, right).
	Align string
}

// Apply applies the style to the given text and returns the styled output.
func (s Style) Apply(text string) string {
	style := lipgloss.NewStyle()

	if s.Foreground != "" {
		style = style.Foreground(lipgloss.Color(s.Foreground))
	}
	if s.Background != "" {
		style = style.Background(lipgloss.Color(s.Background))
	}
	if s.Bold {
		style = style.Bold(true)
	}
	if s.Italic {
		style = style.Italic(true)
	}
	if s.Underline {
		style = style.Underline(true)
	}
	if s.Strikethrough {
		style = style.Strikethrough(true)
	}
	if s.Faint {
		style = style.Faint(true)
	}
	if s.Blink {
		style = style.Blink(true)
	}

	switch len(s.Padding) {
	case 1:
		style = style.Padding(s.Padding[0])
	case 2:
		style = style.Padding(s.Padding[0], s.Padding[1])
	case 4:
		style = style.Padding(s.Padding[0], s.Padding[1], s.Padding[2], s.Padding[3])
	}

	switch len(s.Margin) {
	case 1:
		style = style.Margin(s.Margin[0])
	case 2:
		style = style.Margin(s.Margin[0], s.Margin[1])
	case 4:
		style = style.Margin(s.Margin[0], s.Margin[1], s.Margin[2], s.Margin[3])
	}

	if s.Border != "" && s.Border != "none" {
		var border lipgloss.Border
		switch s.Border {
		case "normal":
			border = lipgloss.NormalBorder()
		case "rounded":
			border = lipgloss.RoundedBorder()
		case "thick":
			border = lipgloss.ThickBorder()
		case "double":
			border = lipgloss.DoubleBorder()
		case "hidden":
			border = lipgloss.HiddenBorder()
		default:
			border = lipgloss.NormalBorder()
		}
		style = style.Border(border)
		if s.BorderForeground != "" {
			style = style.BorderForeground(lipgloss.Color(s.BorderForeground))
		}
		if s.BorderBackground != "" {
			style = style.BorderBackground(lipgloss.Color(s.BorderBackground))
		}
	}

	if s.Width > 0 {
		style = style.Width(s.Width)
	}
	if s.Height > 0 {
		style = style.Height(s.Height)
	}

	switch s.Align {
	case "center":
		style = style.Align(lipgloss.Center)
	case "right":
		style = style.Align(lipgloss.Right)
	default:
		style = style.Align(lipgloss.Left)
	}

	return style.Render(text)
}
