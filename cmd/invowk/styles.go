// SPDX-License-Identifier: MPL-2.0

package cmd

import "github.com/charmbracelet/lipgloss"

// Color palette - shared hex colors for consistent theming across all CLI output.
// These colors are designed for dark terminal backgrounds with good contrast.
const (
	// ColorPrimary is purple - used for titles, headers, and primary emphasis.
	ColorPrimary = lipgloss.Color("#7C3AED")

	// ColorMuted is gray - used for subtitles, secondary text, and de-emphasized content.
	ColorMuted = lipgloss.Color("#6B7280")

	// ColorSuccess is green - used for success states, checkmarks, and positive outcomes.
	ColorSuccess = lipgloss.Color("#10B981")

	// ColorError is red - used for errors, failures, and negative outcomes.
	ColorError = lipgloss.Color("#EF4444")

	// ColorWarning is amber - used for warnings, caution states, and attention-needed items.
	ColorWarning = lipgloss.Color("#F59E0B")

	// ColorHighlight is blue - used for commands, links, and interactive elements.
	ColorHighlight = lipgloss.Color("#3B82F6")

	// ColorVerbose is light gray - used for verbose/debug output and supplementary details.
	ColorVerbose = lipgloss.Color("#9CA3AF")
)

// Base styles - reusable lipgloss styles built from the color palette.
// These can be used directly or extended with additional styling (margins, padding, etc.).
var (
	// TitleStyle is for primary headers and section titles.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	// SubtitleStyle is for secondary headers and descriptions.
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// SuccessStyle is for success messages and positive indicators.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	// ErrorStyle is for error messages and failure indicators.
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	// WarningStyle is for warning messages and caution indicators.
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// CmdStyle is for command names, code, and interactive elements.
	CmdStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight)

	// VerboseStyle is for verbose output and supplementary information.
	VerboseStyle = lipgloss.NewStyle().
			Foreground(ColorVerbose)

	// VerboseHighlightStyle is for emphasized items within verbose output.
	VerboseHighlightStyle = lipgloss.NewStyle().
				Foreground(ColorHighlight)
)
