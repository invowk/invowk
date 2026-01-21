// SPDX-License-Identifier: MPL-2.0

// Package tui provides a clean API for terminal user interface components.
// It wraps charmbracelet/huh and charmbracelet/bubbles to provide reusable
// TUI elements that can be used both programmatically and via CLI commands.
package tui

import (
	"errors"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Const block placed before var/type (decorder: const → var → type → func).
// Using untyped const pattern for Theme values.
const (
	// ThemeDefault uses the default huh theme.
	ThemeDefault = "default"
	// ThemeCharm uses the Charm theme.
	ThemeCharm = "charm"
	// ThemeDracula uses the Dracula theme.
	ThemeDracula = "dracula"
	// ThemeCatppuccin uses the Catppuccin theme.
	ThemeCatppuccin = "catppuccin"
	// ThemeBase16 uses the Base16 theme.
	ThemeBase16 = "base16"
	// keyCtrlC is the key binding constant for Ctrl+C.
	keyCtrlC = "ctrl+c"
	// ModalBackgroundColor is the background color used for modal overlays.
	// This must match the background in overlayStyle() in embeddable.go.
	ModalBackgroundColor = "#1a1a2e"
)

// Var block placed after const, before type (decorder: const → var → type → func).
var (
	// ErrCancelled is returned when a user cancels a TUI component (e.g., via Ctrl+C or Esc).
	// Callers can check for this error using errors.Is(err, tui.ErrCancelled).
	ErrCancelled = errors.New("user cancelled")
	// modalBgColor is the lipgloss.Color version of ModalBackgroundColor for internal use.
	modalBgColor = lipgloss.Color(ModalBackgroundColor)
)

// All type declarations consolidated in a single block.
type (
	// Theme represents the visual theme for TUI components.
	Theme string

	// Config holds common configuration for TUI components.
	Config struct {
		// Theme specifies the visual theme to use.
		Theme Theme
		// Accessible enables accessible mode for screen readers.
		Accessible bool
		// Width specifies the width of the component (0 for auto).
		Width int
		// Output specifies where to write the component output.
		Output io.Writer
	}

	// Style represents styling options for text output.
	Style struct {
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
)

// DefaultConfig returns the default configuration for TUI components.
func DefaultConfig() Config {
	return Config{
		Theme:      ThemeDefault,
		Accessible: os.Getenv("ACCESSIBLE") != "",
		Width:      0,
		Output:     os.Stdout,
	}
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
	case ThemeDefault:
		return huh.ThemeBase()
	}
	return huh.ThemeBase() // Fallback for any future theme values
}

// modalBaseStyle returns a lipgloss style with the modal background color.
// This is the foundation for ALL modal styles to prevent color bleeding.
func modalBaseStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(modalBgColor)
}

// getModalHuhTheme returns a huh theme customized for modal overlays.
// It uses EXPLICIT background colors on ALL styles to prevent color bleeding.
// This is critical: terminal "transparent" backgrounds don't exist - not setting
// a background causes the terminal's default background to show through after
// any ANSI reset sequence.
func getModalHuhTheme() *huh.Theme {
	// Start with a completely fresh theme to avoid inheriting any backgrounds
	t := &huh.Theme{}

	// Define colors that work well on the modal background
	primary := lipgloss.Color("#7C3AED")   // Purple - matches modal border
	secondary := lipgloss.Color("#A78BFA") // Light purple
	text := lipgloss.Color("#FFFFFF")      // White
	dimmed := lipgloss.Color("#6B7280")    // Gray
	errorColor := lipgloss.Color("#EF4444")

	// Create base style WITH EXPLICIT BACKGROUND (critical - this prevents color bleeding)
	// Every style must have the modal background to ensure consistent rendering.
	base := modalBaseStyle()

	// === FOCUSED FIELD STYLES ===
	t.Focused.Base = base
	t.Focused.Title = base.Foreground(primary).Bold(true)
	t.Focused.Description = base.Foreground(dimmed)
	t.Focused.ErrorIndicator = base.Foreground(errorColor)
	t.Focused.ErrorMessage = base.Foreground(errorColor)

	// Text input styles - ALL have explicit backgrounds
	t.Focused.TextInput.Cursor = base.Foreground(text)
	t.Focused.TextInput.CursorText = base.Foreground(text)
	t.Focused.TextInput.Placeholder = base.Foreground(dimmed)
	t.Focused.TextInput.Prompt = base.Foreground(secondary)
	t.Focused.TextInput.Text = base.Foreground(text)

	// Select/option styles - ALL have explicit backgrounds
	t.Focused.SelectSelector = base.Foreground(primary).Bold(true)
	t.Focused.Option = base.Foreground(text)
	t.Focused.NextIndicator = base.Foreground(dimmed)
	t.Focused.PrevIndicator = base.Foreground(dimmed)

	// Multi-select styles - ALL have explicit backgrounds
	t.Focused.MultiSelectSelector = base.Foreground(primary).Bold(true)
	t.Focused.SelectedOption = base.Foreground(secondary)
	t.Focused.SelectedPrefix = base.Foreground(primary)
	t.Focused.UnselectedOption = base.Foreground(text)
	t.Focused.UnselectedPrefix = base.Foreground(dimmed)

	// Button styles - FocusedButton gets primary background, BlurredButton gets modal background
	t.Focused.FocusedButton = lipgloss.NewStyle().
		Foreground(text).
		Background(primary).
		Padding(0, 1)
	t.Focused.BlurredButton = base.
		Foreground(dimmed).
		Padding(0, 1)

	// File picker styles - ALL have explicit backgrounds
	t.Focused.Directory = base.Foreground(primary)
	t.Focused.File = base.Foreground(text)

	// Card style (wraps each field) - explicit background
	t.Focused.Card = base

	// Note (for additional info) - explicit background
	t.Focused.NoteTitle = base.Foreground(primary).Bold(true)

	// === BLURRED FIELD STYLES ===
	t.Blurred.Base = base
	t.Blurred.Title = base.Foreground(dimmed)
	t.Blurred.Description = base.Foreground(dimmed)
	t.Blurred.ErrorIndicator = base.Foreground(errorColor)
	t.Blurred.ErrorMessage = base.Foreground(errorColor)

	// Blurred text input - ALL have explicit backgrounds
	t.Blurred.TextInput.Cursor = base.Foreground(dimmed)
	t.Blurred.TextInput.CursorText = base.Foreground(dimmed)
	t.Blurred.TextInput.Placeholder = base.Foreground(dimmed)
	t.Blurred.TextInput.Prompt = base.Foreground(dimmed)
	t.Blurred.TextInput.Text = base.Foreground(dimmed)

	// Blurred select styles - ALL have explicit backgrounds
	t.Blurred.SelectSelector = base.Foreground(dimmed)
	t.Blurred.Option = base.Foreground(dimmed)
	t.Blurred.NextIndicator = base.Foreground(dimmed)
	t.Blurred.PrevIndicator = base.Foreground(dimmed)

	// Blurred multi-select - ALL have explicit backgrounds
	t.Blurred.MultiSelectSelector = base.Foreground(dimmed)
	t.Blurred.SelectedOption = base.Foreground(dimmed)
	t.Blurred.SelectedPrefix = base.Foreground(dimmed)
	t.Blurred.UnselectedOption = base.Foreground(dimmed)
	t.Blurred.UnselectedPrefix = base.Foreground(dimmed)

	// Blurred buttons - ALL have explicit backgrounds
	t.Blurred.FocusedButton = base.Foreground(dimmed).Padding(0, 1)
	t.Blurred.BlurredButton = base.Foreground(dimmed).Padding(0, 1)

	// Blurred file picker - ALL have explicit backgrounds
	t.Blurred.Directory = base.Foreground(dimmed)
	t.Blurred.File = base.Foreground(dimmed)

	// Blurred card - explicit background
	t.Blurred.Card = base

	// Blurred note - explicit background
	t.Blurred.NoteTitle = base.Foreground(dimmed)

	// === HELP STYLES ===
	t.Help.ShortKey = base.Foreground(dimmed)
	t.Help.ShortDesc = base.Foreground(dimmed)
	t.Help.ShortSeparator = base.Foreground(dimmed)
	t.Help.FullKey = base.Foreground(dimmed)
	t.Help.FullDesc = base.Foreground(dimmed)
	t.Help.FullSeparator = base.Foreground(dimmed)
	t.Help.Ellipsis = base.Foreground(dimmed)

	// === FORM STYLES ===
	// These are empty to prevent any form-level background bleeding
	t.Form = huh.FormStyles{}

	return t
}
