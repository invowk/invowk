// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
)

// Const block placed before var/type (decorder: const → var → type → func).
const (
	// ThemeDefault uses the default style theme.
	ThemeDefault Theme = "default"
	// ThemeCharm uses the Charm theme.
	ThemeCharm Theme = "charm"
	// ThemeDracula uses the Dracula theme.
	ThemeDracula Theme = "dracula"
	// ThemeCatppuccin uses the Catppuccin theme.
	ThemeCatppuccin Theme = "catppuccin"
	// ThemeBase16 uses the Base16 theme.
	ThemeBase16 Theme = "base16"
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
	// errNoCommand is returned when trying to run an interactive session without a command.
	errNoCommand = errors.New("no command provided")
	// ErrInvalidTheme is returned when a Theme value is not one of the defined themes.
	ErrInvalidTheme = errors.New("invalid theme")
	// ErrInvalidTUIConfig is the sentinel error wrapped by InvalidTUIConfigError.
	ErrInvalidTUIConfig = errors.New("invalid TUI config")
	// modalBgColor is the lipgloss.Color version of ModalBackgroundColor for internal use.
	modalBgColor = lipgloss.Color(ModalBackgroundColor)
)

// All type declarations consolidated in a single block.
type (
	// Theme represents the visual theme for TUI components.
	Theme string

	// InvalidThemeError is returned when a Theme value is not recognized.
	// It wraps ErrInvalidTheme for errors.Is() compatibility.
	InvalidThemeError struct {
		Value Theme
	}

	// InvalidTUIConfigError is returned when a TUI Config has invalid fields.
	// It wraps ErrInvalidTUIConfig for errors.Is() compatibility and collects
	// field-level validation errors from Theme.
	InvalidTUIConfigError struct {
		FieldErrors []error
	}

	// Config holds common configuration for TUI components.
	Config struct {
		// Theme specifies the visual theme to use.
		Theme Theme
		// Accessible enables accessible mode for screen readers.
		Accessible bool
		// Width specifies the width of the component (0 for auto).
		Width TerminalDimension
		// Output specifies where to write the component output.
		Output io.Writer
	}

	// Style represents styling options for text output.
	Style struct {
		// Foreground color (CSS hex, ANSI code, or color name).
		Foreground ColorSpec
		// Background color (CSS hex, ANSI code, or color name).
		Background ColorSpec
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
		Border BorderStyle
		// BorderForeground color for the border.
		BorderForeground ColorSpec
		// BorderBackground color for the border.
		BorderBackground ColorSpec
		// Width sets the width of the text block.
		Width TerminalDimension
		// Height sets the height of the text block.
		Height TerminalDimension
		// Align sets text alignment (left, center, right).
		Align TextAlign
	}
)

// Error implements the error interface for InvalidThemeError.
func (e *InvalidThemeError) Error() string {
	return fmt.Sprintf("invalid theme %q (valid: default, charm, dracula, catppuccin, base16)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidThemeError) Unwrap() error {
	return ErrInvalidTheme
}

// String returns the string representation of the Theme.
func (t Theme) String() string {
	return string(t)
}

// IsValid returns whether the Theme is one of the defined themes,
// and a list of validation errors if it is not.
func (t Theme) IsValid() (bool, []error) {
	switch t {
	case ThemeDefault, ThemeCharm, ThemeDracula, ThemeCatppuccin, ThemeBase16:
		return true, nil
	default:
		return false, []error{&InvalidThemeError{Value: t}}
	}
}

// Error implements the error interface for InvalidTUIConfigError.
func (e *InvalidTUIConfigError) Error() string {
	return fmt.Sprintf("invalid TUI config: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidTUIConfig for errors.Is() compatibility.
func (e *InvalidTUIConfigError) Unwrap() error { return ErrInvalidTUIConfig }

// IsValid returns whether the Config has valid fields.
// It delegates to Theme.IsValid() and Width.IsValid().
func (c Config) IsValid() (bool, []error) {
	var errs []error
	if valid, fieldErrs := c.Theme.IsValid(); !valid {
		errs = append(errs, fieldErrs...)
	}
	if valid, fieldErrs := c.Width.IsValid(); !valid {
		errs = append(errs, fieldErrs...)
	}
	if len(errs) > 0 {
		return false, []error{&InvalidTUIConfigError{FieldErrors: errs}}
	}
	return true, nil
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

// Apply applies the style to the given text and returns the styled output.
func (s Style) Apply(text string) string {
	style := lipgloss.NewStyle()

	if s.Foreground != "" {
		style = style.Foreground(lipgloss.Color(string(s.Foreground)))
	}
	if s.Background != "" {
		style = style.Background(lipgloss.Color(string(s.Background)))
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

	if s.Border != "" && s.Border != BorderNone {
		var border lipgloss.Border
		switch s.Border {
		case BorderNone:
			// Already handled by the outer guard; included for exhaustive linter.
		case BorderNormal:
			border = lipgloss.NormalBorder()
		case BorderRounded:
			border = lipgloss.RoundedBorder()
		case BorderThick:
			border = lipgloss.ThickBorder()
		case BorderDouble:
			border = lipgloss.DoubleBorder()
		case BorderHidden:
			border = lipgloss.HiddenBorder()
		default:
			border = lipgloss.NormalBorder()
		}
		style = style.Border(border)
		if s.BorderForeground != "" {
			style = style.BorderForeground(lipgloss.Color(string(s.BorderForeground)))
		}
		if s.BorderBackground != "" {
			style = style.BorderBackground(lipgloss.Color(string(s.BorderBackground)))
		}
	}

	if s.Width > 0 {
		style = style.Width(int(s.Width))
	}
	if s.Height > 0 {
		style = style.Height(int(s.Height))
	}

	switch s.Align {
	case "", AlignLeft:
		style = style.Align(lipgloss.Left)
	case AlignCenter:
		style = style.Align(lipgloss.Center)
	case AlignRight:
		style = style.Align(lipgloss.Right)
	}

	return style.Render(text)
}

// modalBaseStyle returns a lipgloss style with the modal background color.
// This is the foundation for ALL modal styles to prevent color bleeding.
func modalBaseStyle() lipgloss.Style {
	return lipgloss.NewStyle().Background(modalBgColor)
}

// isDarkTheme returns whether the given theme should use dark-oriented defaults.
func isDarkTheme(t Theme) bool {
	switch t {
	case ThemeBase16:
		return false
	case ThemeDefault, ThemeCharm, ThemeDracula, ThemeCatppuccin:
		return true
	default:
		return true
	}
}
