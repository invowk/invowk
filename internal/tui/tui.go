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
	"golang.org/x/term"
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
// It automatically enables accessible mode when:
// - Running inside an invowk interactive session (INVOWK_INTERACTIVE=1)
// - Running inside command substitution ($()) where stdin is not a terminal
// - The ACCESSIBLE environment variable is set
//
// When accessible mode is needed, output is directed to stderr so prompts
// aren't captured by command substitution ($() or backticks), which would
// prevent the user from seeing the prompt.
func DefaultConfig() Config {
	nested := IsNestedInteractive()
	noTTY := !isInputTerminal()
	accessible := nested || noTTY || os.Getenv("ACCESSIBLE") != ""

	// When accessible mode is needed, use stderr for output so prompts
	// aren't captured by $() command substitution
	var output io.Writer = os.Stdout
	if accessible {
		output = os.Stderr
	}

	return Config{
		Theme:      ThemeDefault,
		Accessible: accessible,
		Width:      0,
		Output:     output,
	}
}

// IsNestedInteractive returns true if running inside an invowk interactive session.
// This is detected by the INVOWK_INTERACTIVE environment variable, which is set
// by the outer interactive TUI (both PTY and pipe-based modes).
//
// When nested, TUI components should use accessible mode to avoid conflicts
// between the outer and inner TUI rendering.
func IsNestedInteractive() bool {
	return os.Getenv("INVOWK_INTERACTIVE") != ""
}

// isInputTerminal returns true if stdin is connected to a terminal.
// Returns false when running inside command substitution ($()) or pipes.
func isInputTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// shouldUseAccessible returns true if accessible mode should be used.
// This checks:
// - The config.Accessible setting
// - The INVOWK_INTERACTIVE environment variable (nested interactive mode)
// - Whether stdin is a terminal (for command substitution detection)
//
// Even if config.Accessible is false, this returns true when running nested
// or when stdin is not a terminal (e.g., inside $() command substitution).
func shouldUseAccessible(cfg Config) bool {
	return cfg.Accessible || IsNestedInteractive() || !isInputTerminal()
}

// getOutputWriter returns the appropriate output writer for the current context.
// Returns stderr when accessible mode is needed (nested or no TTY) to prevent
// prompts from being captured by command substitution ($()).
// If cfg.Output is already set, it's returned as-is.
func getOutputWriter(cfg Config) io.Writer {
	if cfg.Output != nil {
		return cfg.Output
	}
	if shouldUseAccessible(cfg) {
		return os.Stderr
	}
	return os.Stdout
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
