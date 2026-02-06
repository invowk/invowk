// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/ansi"
	"github.com/muesli/reflow/truncate"
)

// All const declarations in a single block, placed before var/type/func (decorder: const → var → type → func).
// Using untyped const pattern for ComponentType values.
const (
	// modalBorderWidth is the horizontal space taken by the border (1 char each side).
	modalBorderWidth = 2
	// modalBorderHeight is the vertical space taken by the border (1 line each side).
	modalBorderHeight = 2
	// modalPaddingWidth is the horizontal space taken by padding (2 chars each side).
	modalPaddingWidth = 4
	// modalPaddingHeight is the vertical space taken by padding (1 line each side).
	modalPaddingHeight = 2
	// modalOverheadWidth is the total horizontal overhead for the modal frame.
	modalOverheadWidth = modalBorderWidth + modalPaddingWidth // 6
	// modalOverheadHeight is the total vertical overhead for the modal frame.
	modalOverheadHeight = modalBorderHeight + modalPaddingHeight // 4

	// Component type constants for the TUI system.
	ComponentTypeInput    = "input"
	ComponentTypeConfirm  = "confirm"
	ComponentTypeChoose   = "choose"
	ComponentTypeFilter   = "filter"
	ComponentTypeFile     = "file"
	ComponentTypeWrite    = "write"
	ComponentTypeTextArea = "textarea"
	ComponentTypeSpin     = "spin"
	ComponentTypePager    = "pager"
	ComponentTypeTable    = "table"
)

// Modal ANSI variables: modal overlays render on a styled background, but child
// components (huh, bubbles, external tools) may emit bare ANSI resets (\x1b[0m) that
// clear the background color, causing visual "holes." These pre-computed variables
// enable efficient replacement of bare resets with reset+background-restore sequences
// to maintain modal visual continuity.
var (
	// modalBgANSI is the ANSI escape sequence to set the modal background color.
	// It's computed once from ModalBackgroundColor for efficiency.
	modalBgANSI string

	// ansiReset is the standard ANSI reset sequence.
	ansiReset = "\x1b[0m"

	// ansiResetWithBg is the ANSI reset followed by modal background restore.
	// This is what we replace bare resets with.
	ansiResetWithBg string
)

// All type declarations in a single block, placed after var.
type (
	// EmbeddableComponent is a TUI component that can be embedded in a parent Bubbletea model.
	// Unlike standalone components that run their own tea.Program, embeddable components
	// delegate their Update and View to a parent model that owns the terminal.
	//
	// The parent program owns the terminal lifecycle and calls Init() on the component.
	// SetSize must be called before the first Update and on every terminal resize.
	// Cancelled returns true only when the user explicitly dismissed via Esc or Ctrl+C
	// (not on normal submission).
	EmbeddableComponent interface {
		tea.Model

		// IsDone returns true when the component has completed (submitted or cancelled).
		IsDone() bool

		// Result returns the component's result value. Only valid when IsDone() returns true.
		// The type of the result depends on the component:
		// - Input: string
		// - Confirm: bool
		// - Choose (single): string
		// - Choose (multi): []string
		// - Filter: []string
		// - File: string
		// - Write/TextArea: string
		// - Table: TableSelectionResult
		// - Pager: nil
		// - Spin: SpinResult
		Result() (any, error)

		// Cancelled returns true if the user cancelled the component (Esc, Ctrl+C).
		Cancelled() bool

		// SetSize sets the available width and height for the component.
		// This should be called before Init() and when the terminal is resized.
		SetSize(width, height int)
	}

	// TableSelectionResult holds the result of a table selection.
	TableSelectionResult struct {
		SelectedIndex int
		SelectedRow   []string
	}

	// SpinResult holds the result of a spin operation.
	SpinResult struct {
		Stdout   string
		Stderr   string
		ExitCode int
	}

	// ComponentType represents the type of TUI component.
	ComponentType string

	// ModalSize contains the calculated dimensions for a modal overlay.
	ModalSize struct {
		Width  int
		Height int
	}
)

// init is the first function in the file (required by decorder).
func init() {
	// Parse the modal background color and pre-compute the ANSI sequences
	modalBgANSI = hexToANSIBackground(ModalBackgroundColor)
	ansiResetWithBg = ansiReset + modalBgANSI
}

// CalculateModalSize calculates appropriate modal content dimensions based on component type
// and available screen space. The returned dimensions are for the INNER content area,
// accounting for the modal frame overhead (border + padding).
// Different component types have different sizing needs.
func CalculateModalSize(componentType ComponentType, screenWidth, screenHeight int) ModalSize {
	// Define margins to leave around the modal (outer)
	const (
		minMarginX = 4  // Minimum horizontal margin (2 on each side)
		minMarginY = 4  // Minimum vertical margin (2 on top/bottom)
		maxWidth   = 70 // Maximum content width for readability (80 - frame overhead)
	)

	// Calculate available space for the OUTER modal after margins
	availableOuterWidth := screenWidth - minMarginX
	availableOuterHeight := screenHeight - minMarginY

	// Calculate available space for INNER content (subtract frame overhead)
	availableContentWidth := availableOuterWidth - modalOverheadWidth
	availableContentHeight := availableOuterHeight - modalOverheadHeight

	if availableContentWidth < 20 {
		availableContentWidth = 20
	}
	if availableContentHeight < 3 {
		availableContentHeight = 3
	}

	// Different components have different ideal content sizes
	switch componentType {
	case ComponentTypeInput, ComponentTypeConfirm:
		// Simple prompts: compact width, minimal height
		width := min(availableContentWidth, maxWidth)
		height := min(availableContentHeight, 4) // Just title + input/buttons
		return ModalSize{Width: width, Height: height}

	case ComponentTypeChoose, ComponentTypeFilter:
		// Selection lists: moderate width, taller height for options
		width := min(availableContentWidth, maxWidth)
		height := min(availableContentHeight, 12)
		return ModalSize{Width: width, Height: height}

	case ComponentTypeFile:
		// File picker: needs more space for directory listing
		width := min(availableContentWidth, maxWidth)
		height := min(availableContentHeight, 16)
		return ModalSize{Width: width, Height: height}

	case ComponentTypeTable:
		// Table: can use more width for columns, moderate height
		width := min(availableContentWidth, 90) // Allow wider for tables
		height := min(availableContentHeight, 16)
		return ModalSize{Width: width, Height: height}

	case ComponentTypePager:
		// Pager: use most of the screen since it's for reading content
		width := min(availableContentWidth, 90)
		height := availableContentHeight // Use all available
		return ModalSize{Width: width, Height: height}

	case ComponentTypeWrite, ComponentTypeTextArea:
		// Text editing: moderate size
		width := min(availableContentWidth, maxWidth)
		height := min(availableContentHeight, 8)
		return ModalSize{Width: width, Height: height}

	case ComponentTypeSpin:
		// Spinner: compact, just shows status
		width := min(availableContentWidth, 50)
		height := min(availableContentHeight, 2)
		return ModalSize{Width: width, Height: height}

	default:
		// Unknown: use reasonable defaults
		width := min(availableContentWidth, maxWidth)
		height := min(availableContentHeight, 8)
		return ModalSize{Width: width, Height: height}
	}
}

// CreateEmbeddableComponent creates an embeddable component from a component type and options.
// The options should be a JSON-encoded representation of the component-specific options.
// Components created here use a modal-specific theme to ensure proper rendering in overlays.
func CreateEmbeddableComponent(componentType ComponentType, options json.RawMessage, width, height int) (EmbeddableComponent, error) {
	switch componentType {
	case ComponentTypeInput:
		var opts InputOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal input options: %w", err)
		}
		model := NewInputModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeConfirm:
		var opts ConfirmOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal confirm options: %w", err)
		}
		model := NewConfirmModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeChoose:
		var opts ChooseStringOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal choose options: %w", err)
		}
		model := NewChooseModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeFilter:
		var opts FilterOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal filter options: %w", err)
		}
		model := NewFilterModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeFile:
		var opts FileOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal file options: %w", err)
		}
		model := NewFileModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeWrite, ComponentTypeTextArea:
		var opts WriteOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal write options: %w", err)
		}
		model := NewWriteModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypePager:
		var opts PagerOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pager options: %w", err)
		}
		model := NewPagerModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeTable:
		var opts TableOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal table options: %w", err)
		}
		model := NewTableModelForModal(opts)
		model.SetSize(width, height)
		return model, nil

	case ComponentTypeSpin:
		var opts SpinCommandOptions
		if err := json.Unmarshal(options, &opts); err != nil {
			return nil, fmt.Errorf("failed to unmarshal spin options: %w", err)
		}
		model := NewSpinModel(opts)
		model.SetSize(width, height)
		return model, nil

	default:
		return nil, fmt.Errorf("unknown component type: %s", componentType)
	}
}

// RenderOverlay renders an overlay component centered on top of a base view.
// The base view remains visible around the overlay, creating a modal effect.
// This function properly handles ANSI escape sequences in both base and overlay.
//
// The function applies two layers of protection against color bleeding:
// 1. The overlay style applies the modal background to the frame
// 2. sanitizeModalBackground post-processes to catch any bare ANSI resets
func RenderOverlay(base, overlay string, screenWidth, screenHeight int) string {
	// Apply overlay styling (border + padding + background)
	styledOverlay := overlayStyle().Render(overlay)

	// Safety net: sanitize any ANSI reset sequences that might cause color bleeding
	// This catches escapes from third-party components (huh, bubbles) that we might
	// have missed in the explicit background styling.
	styledOverlay = sanitizeModalBackground(styledOverlay)

	// Split into lines
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(styledOverlay, "\n")

	// Ensure base has enough lines
	for len(baseLines) < screenHeight {
		baseLines = append(baseLines, "")
	}

	// Calculate overlay dimensions using ANSI-aware width measurement
	overlayHeight := len(overlayLines)
	overlayWidth := 0
	for _, line := range overlayLines {
		w := lipgloss.Width(line)
		if w > overlayWidth {
			overlayWidth = w
		}
	}

	// Calculate position to center the overlay
	startY := (screenHeight - overlayHeight) / 2
	startX := (screenWidth - overlayWidth) / 2

	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Composite the overlay onto the base
	result := make([]string, len(baseLines))
	for i, baseLine := range baseLines {
		if i >= startY && i < startY+overlayHeight {
			overlayIdx := i - startY
			if overlayIdx < len(overlayLines) {
				result[i] = compositeLineANSI(baseLine, overlayLines[overlayIdx], startX, screenWidth)
			} else {
				result[i] = padLineToWidth(baseLine, screenWidth)
			}
		} else {
			result[i] = padLineToWidth(baseLine, screenWidth)
		}
	}

	return strings.Join(result, "\n")
}

// hexToANSIBackground converts a hex color string to an ANSI 24-bit background escape sequence.
// Supports formats: "#RRGGBB" or "RRGGBB"
func hexToANSIBackground(hex string) string {
	// Remove leading # if present
	hex = strings.TrimPrefix(hex, "#")

	if len(hex) != 6 {
		return "" // Invalid format, return empty
	}

	r, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		return ""
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		return ""
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		return ""
	}

	// ANSI 24-bit color: ESC[48;2;R;G;Bm for background
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

// sanitizeModalBackground is a safety net that ensures the modal background
// is restored after any ANSI reset sequences in the rendered content.
// This catches any color bleeding from third-party components that we might
// have missed in the explicit background styling.
//
// It replaces all occurrences of the bare ANSI reset (\x1b[0m) with
// reset + background restore (\x1b[0m\x1b[48;2;R;G;Bm).
func sanitizeModalBackground(content string) string {
	if modalBgANSI == "" {
		return content // No valid background color, skip processing
	}

	// Replace bare resets with reset + background restore
	// We need to be careful not to double-process if ansiResetWithBg is already present
	// First, temporarily replace existing "reset+bg" sequences to protect them
	placeholder := "\x00MODAL_BG_SAFE\x00"
	content = strings.ReplaceAll(content, ansiResetWithBg, placeholder)

	// Now replace all remaining bare resets
	content = strings.ReplaceAll(content, ansiReset, ansiResetWithBg)

	// Restore the protected sequences
	content = strings.ReplaceAll(content, placeholder, ansiResetWithBg)

	return content
}

// overlayStyle returns the style for the overlay border.
func overlayStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 2).
		Background(lipgloss.Color("#1a1a2e"))
}

// compositeLineANSI overlays an overlay line onto a base line at startX position.
// This function properly handles ANSI escape sequences in both strings.
// It uses modal-background-aware resets to prevent color bleeding at boundaries.
func compositeLineANSI(baseLine, overlayLine string, startX, maxWidth int) string {
	baseWidth := lipgloss.Width(baseLine)
	overlayWidth := lipgloss.Width(overlayLine)

	var result strings.Builder

	// Part 1: Base content before the overlay (0 to startX)
	if startX > 0 {
		if baseWidth >= startX {
			// Truncate base to startX width, preserving ANSI codes
			result.WriteString(truncate.String(baseLine, uint(startX)))
		} else {
			// Base is shorter than startX, pad with spaces
			result.WriteString(baseLine)
			result.WriteString(strings.Repeat(" ", startX-baseWidth))
		}
	}

	// Reset ANSI state before overlay and set modal background
	// This ensures the transition from base to overlay doesn't have color bleeding
	result.WriteString(ansiResetWithBg)

	// Part 2: The overlay content
	result.WriteString(overlayLine)

	// Part 3: Base content after the overlay (startX + overlayWidth to end)
	overlayEnd := startX + overlayWidth
	if overlayEnd < maxWidth {
		// Reset ANSI state after overlay (no modal bg needed, we're back to base)
		result.WriteString(ansiReset)

		if baseWidth > overlayEnd {
			// We need to skip the first overlayEnd characters of the base
			// and get the rest. This requires walking the string ANSI-aware.
			suffix := getANSISuffix(baseLine, overlayEnd)
			result.WriteString(suffix)
		} else {
			// Base doesn't extend past the overlay, just pad
			result.WriteString(strings.Repeat(" ", maxWidth-overlayEnd))
		}
	}

	// Ensure line ends with ANSI reset to prevent state leaking to next line
	result.WriteString(ansiReset)

	return result.String()
}

// getANSISuffix returns the portion of a string after skipping `skipWidth` visible characters.
// It properly handles ANSI escape sequences, preserving them in the output.
func getANSISuffix(s string, skipWidth int) string {
	var result strings.Builder
	var visibleCount int
	var inEscape bool
	var escapeSeq strings.Builder

	for _, r := range s {
		if inEscape {
			escapeSeq.WriteRune(r)
			if ansi.IsTerminator(r) {
				inEscape = false
				// If we're past skipWidth, include escape sequences in output
				if visibleCount >= skipWidth {
					result.WriteString(escapeSeq.String())
				}
				escapeSeq.Reset()
			}
			continue
		}

		if r == ansi.Marker {
			inEscape = true
			escapeSeq.WriteRune(r)
			continue
		}

		// Regular visible character
		if visibleCount >= skipWidth {
			result.WriteRune(r)
		}
		visibleCount++
	}

	return result.String()
}

// padLineToWidth ensures a line is at least `width` characters wide (visually).
// It also ensures the line ends with an ANSI reset to prevent state leaking.
func padLineToWidth(line string, width int) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth >= width {
		return line + "\x1b[0m"
	}
	return line + "\x1b[0m" + strings.Repeat(" ", width-lineWidth)
}
