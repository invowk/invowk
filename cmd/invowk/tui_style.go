// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	styleForeground string
	styleBackground string
	styleBold       bool
	styleItalic     bool
	styleUnderline  bool
	styleStrike     bool
	styleFaint      bool
	styleBlink      bool
	styleReverse    bool
	styleMarginL    int
	styleMarginR    int
	styleMarginT    int
	styleMarginB    int
	stylePaddingL   int
	stylePaddingR   int
	stylePaddingT   int
	stylePaddingB   int
	styleWidth      int
	styleHeight     int
	styleAlign      string
	styleBorder     string

	// tuiStyleCmd applies styles to text.
	tuiStyleCmd = &cobra.Command{
		Use:   "style [text...]",
		Short: "Apply styles to text",
		Long: `Apply terminal styling to text.

Content can be provided as arguments or piped via stdin.
Uses lipgloss for rendering, supporting colors, borders, and formatting.

Examples:
  # Colored text
  invowk tui style --foreground "#FF0000" "Red text"

  # Bold and italic
  echo "Styled" | invowk tui style --bold --italic

  # With background and padding
  invowk tui style --background "#333" --foreground "#FFF" --padding 1 "Box"

  # Centered with border
  invowk tui style --border rounded --align center --width 40 "Centered Title"

  # Multiple styles
  invowk tui style --bold --foreground "#00FF00" --background "#000" "Matrix"`,
		RunE: runTuiStyle,
	}
)

func init() {
	tuiCmd.AddCommand(tuiStyleCmd)

	tuiStyleCmd.Flags().StringVar(&styleForeground, "foreground", "", "foreground color (hex or ANSI)")
	tuiStyleCmd.Flags().StringVar(&styleBackground, "background", "", "background color (hex or ANSI)")
	tuiStyleCmd.Flags().BoolVar(&styleBold, "bold", false, "bold text")
	tuiStyleCmd.Flags().BoolVar(&styleItalic, "italic", false, "italic text")
	tuiStyleCmd.Flags().BoolVar(&styleUnderline, "underline", false, "underlined text")
	tuiStyleCmd.Flags().BoolVar(&styleStrike, "strikethrough", false, "strikethrough text")
	tuiStyleCmd.Flags().BoolVar(&styleFaint, "faint", false, "faint/dim text")
	tuiStyleCmd.Flags().BoolVar(&styleBlink, "blink", false, "blinking text")
	tuiStyleCmd.Flags().BoolVar(&styleReverse, "reverse", false, "reverse colors")
	tuiStyleCmd.Flags().IntVar(&styleMarginL, "margin-left", 0, "left margin")
	tuiStyleCmd.Flags().IntVar(&styleMarginR, "margin-right", 0, "right margin")
	tuiStyleCmd.Flags().IntVar(&styleMarginT, "margin-top", 0, "top margin")
	tuiStyleCmd.Flags().IntVar(&styleMarginB, "margin-bottom", 0, "bottom margin")
	tuiStyleCmd.Flags().IntVar(&stylePaddingL, "padding-left", 0, "left padding")
	tuiStyleCmd.Flags().IntVar(&stylePaddingR, "padding-right", 0, "right padding")
	tuiStyleCmd.Flags().IntVar(&stylePaddingT, "padding-top", 0, "top padding")
	tuiStyleCmd.Flags().IntVar(&stylePaddingB, "padding-bottom", 0, "bottom padding")
	tuiStyleCmd.Flags().IntVar(&styleWidth, "width", 0, "fixed width (0 for auto)")
	tuiStyleCmd.Flags().IntVar(&styleHeight, "height", 0, "fixed height (0 for auto)")
	tuiStyleCmd.Flags().StringVar(&styleAlign, "align", "", "text alignment (left, center, right)")
	tuiStyleCmd.Flags().StringVar(&styleBorder, "border", "", "border style (normal, rounded, thick, double, hidden, none)")
}

func runTuiStyle(cmd *cobra.Command, args []string) error {
	var content string

	// Get content from args or stdin
	if len(args) > 0 {
		content = strings.Join(args, " ")
	} else {
		// Check if we have stdin input
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Read from stdin
			var sb strings.Builder
			reader := bufio.NewReader(os.Stdin)
			for {
				line, err := reader.ReadString('\n')
				sb.WriteString(line)
				if err != nil {
					if err == io.EOF {
						break
					}
					return fmt.Errorf("error reading stdin: %w", err)
				}
			}
			content = strings.TrimSuffix(sb.String(), "\n")
		} else {
			return fmt.Errorf("no content provided; provide as arguments or pipe via stdin")
		}
	}

	// Build the style
	style := lipgloss.NewStyle()

	if styleForeground != "" {
		style = style.Foreground(lipgloss.Color(styleForeground))
	}
	if styleBackground != "" {
		style = style.Background(lipgloss.Color(styleBackground))
	}
	if styleBold {
		style = style.Bold(true)
	}
	if styleItalic {
		style = style.Italic(true)
	}
	if styleUnderline {
		style = style.Underline(true)
	}
	if styleStrike {
		style = style.Strikethrough(true)
	}
	if styleFaint {
		style = style.Faint(true)
	}
	if styleBlink {
		style = style.Blink(true)
	}
	if styleReverse {
		style = style.Reverse(true)
	}

	// Margins
	if styleMarginL > 0 || styleMarginR > 0 || styleMarginT > 0 || styleMarginB > 0 {
		style = style.Margin(styleMarginT, styleMarginR, styleMarginB, styleMarginL)
	}

	// Padding
	if stylePaddingL > 0 || stylePaddingR > 0 || stylePaddingT > 0 || stylePaddingB > 0 {
		style = style.Padding(stylePaddingT, stylePaddingR, stylePaddingB, stylePaddingL)
	}

	// Dimensions
	if styleWidth > 0 {
		style = style.Width(styleWidth)
	}
	if styleHeight > 0 {
		style = style.Height(styleHeight)
	}

	// Alignment
	switch styleAlign {
	case "center":
		style = style.Align(lipgloss.Center)
	case "right":
		style = style.Align(lipgloss.Right)
	case "left":
		style = style.Align(lipgloss.Left)
	}

	// Border
	switch styleBorder {
	case "normal":
		style = style.Border(lipgloss.NormalBorder())
	case "rounded":
		style = style.Border(lipgloss.RoundedBorder())
	case "thick":
		style = style.Border(lipgloss.ThickBorder())
	case "double":
		style = style.Border(lipgloss.DoubleBorder())
	case "hidden":
		style = style.Border(lipgloss.HiddenBorder())
	}

	_, _ = fmt.Fprintln(os.Stdout, style.Render(content)) // Terminal output; error non-critical
	return nil
}
