// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/types"
)

func TestHexToANSIBackground(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hex      string
		expected string
	}{
		{
			name:     "valid hex with hash",
			hex:      "#1a1a2e",
			expected: "\x1b[48;2;26;26;46m",
		},
		{
			name:     "valid hex without hash",
			hex:      "1a1a2e",
			expected: "\x1b[48;2;26;26;46m",
		},
		{
			name:     "black",
			hex:      "#000000",
			expected: "\x1b[48;2;0;0;0m",
		},
		{
			name:     "white",
			hex:      "#FFFFFF",
			expected: "\x1b[48;2;255;255;255m",
		},
		{
			name:     "invalid short hex",
			hex:      "#FFF",
			expected: "",
		},
		{
			name:     "invalid characters",
			hex:      "#GGGGGG",
			expected: "",
		},
		{
			name:     "empty string",
			hex:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := hexToANSIBackground(tt.hex)
			if result != tt.expected {
				t.Errorf("hexToANSIBackground(%q) = %q, want %q", tt.hex, result, tt.expected)
			}
		})
	}
}

func TestSanitizeModalBackground(t *testing.T) {
	t.Parallel()

	// Pre-compute expected values based on ModalBackgroundColor
	bgEscape := hexToANSIBackground(ModalBackgroundColor)
	resetWithBg := "\x1b[0m" + bgEscape

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no reset sequences",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "single reset sequence",
			input:    "Hello\x1b[0mWorld",
			expected: "Hello" + resetWithBg + "World",
		},
		{
			name:     "multiple reset sequences",
			input:    "\x1b[0mA\x1b[0mB\x1b[0m",
			expected: resetWithBg + "A" + resetWithBg + "B" + resetWithBg,
		},
		{
			name:     "already sanitized content",
			input:    "Hello" + resetWithBg + "World",
			expected: "Hello" + resetWithBg + "World",
		},
		{
			name:     "mixed sanitized and unsanitized",
			input:    resetWithBg + "\x1b[0m",
			expected: resetWithBg + resetWithBg,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "content with colors but no reset",
			input:    "\x1b[31mRed\x1b[32mGreen",
			expected: "\x1b[31mRed\x1b[32mGreen",
		},
		{
			name:     "background reset sequence",
			input:    "A\x1b[49mB",
			expected: "A\x1b[49m" + bgEscape + "B",
		},
		{
			name:     "combined sgr with background reset",
			input:    "A\x1b[39;49mB",
			expected: "A\x1b[39;49m" + bgEscape + "B",
		},
		{
			name:     "already sanitized background reset sequence",
			input:    "A\x1b[49m" + bgEscape + "B",
			expected: "A\x1b[49m" + bgEscape + "B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := sanitizeModalBackground(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeModalBackground() mismatch\ngot:  %q\nwant: %q", result, tt.expected)
			}
		})
	}
}

func TestShouldRestoreModalBackground(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		params   string
		expected bool
	}{
		{name: "empty params (equivalent to 0m)", params: "", expected: true},
		{name: "reset all", params: "0", expected: true},
		{name: "background reset", params: "49", expected: true},
		{name: "combined with background reset", params: "39;49", expected: true},
		{name: "foreground only", params: "39", expected: false},
		{name: "background set (not reset)", params: "48;5;234", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldRestoreModalBackground(types.DescriptionText(tt.params))
			if got != tt.expected {
				t.Errorf("shouldRestoreModalBackground(%q) = %v, want %v", tt.params, got, tt.expected)
			}
		})
	}
}

func TestModalBgANSIInitialized(t *testing.T) {
	t.Parallel()

	// Verify that the module-level variables are properly initialized
	if modalBgANSI == "" {
		t.Error("modalBgANSI should be initialized to a non-empty value")
	}

	if !strings.HasPrefix(modalBgANSI, "\x1b[48;2;") {
		t.Errorf("modalBgANSI should be a 24-bit background escape, got: %q", modalBgANSI)
	}

	if ansiResetWithBg != ansiReset+modalBgANSI {
		t.Errorf("ansiResetWithBg should be ansiReset + modalBgANSI")
	}
}

func TestModalBaseStyle(t *testing.T) {
	t.Parallel()

	// Verify that modalBaseStyle returns a valid style and preserves content
	style := modalBaseStyle()
	rendered := style.Render("test")

	// Verify the content is preserved (lipgloss may or may not output
	// ANSI escape sequences depending on terminal detection in test env)
	if !strings.Contains(rendered, "test") {
		t.Error("modalBaseStyle should preserve the rendered content")
	}

	// The actual color output depends on terminal capabilities detected by lipgloss.
	// Visual testing validates the colors work correctly in real terminals.
}

func TestCalculateModalSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		componentType ComponentType
		screenWidth   TerminalDimension
		screenHeight  TerminalDimension
		expectWidth   bool // just verify it returns something reasonable
		expectHeight  bool
	}{
		{
			name:          "input component",
			componentType: ComponentTypeInput,
			screenWidth:   120,
			screenHeight:  40,
			expectWidth:   true,
			expectHeight:  true,
		},
		{
			name:          "filter component",
			componentType: ComponentTypeFilter,
			screenWidth:   120,
			screenHeight:  40,
			expectWidth:   true,
			expectHeight:  true,
		},
		{
			name:          "small screen",
			componentType: ComponentTypeInput,
			screenWidth:   30,
			screenHeight:  10,
			expectWidth:   true,
			expectHeight:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			size := CalculateModalSize(tt.componentType, tt.screenWidth, tt.screenHeight)

			if tt.expectWidth && size.Width <= 0 {
				t.Errorf("Expected positive width, got %d", size.Width)
			}
			if tt.expectHeight && size.Height <= 0 {
				t.Errorf("Expected positive height, got %d", size.Height)
			}

			// Verify size doesn't exceed screen bounds (minus overhead)
			if size.Width > tt.screenWidth {
				t.Errorf("Width %d exceeds screen width %d", size.Width, tt.screenWidth)
			}
			if size.Height > tt.screenHeight {
				t.Errorf("Height %d exceeds screen height %d", size.Height, tt.screenHeight)
			}
		})
	}
}
