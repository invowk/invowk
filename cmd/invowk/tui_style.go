// SPDX-License-Identifier: MPL-2.0

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

// newTUIStyleCommand creates the `invowk tui style` command.
func newTUIStyleCommand() *cobra.Command {
	cmd := &cobra.Command{
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

	cmd.Flags().String("foreground", "", "foreground color (hex or ANSI)")
	cmd.Flags().String("background", "", "background color (hex or ANSI)")
	cmd.Flags().Bool("bold", false, "bold text")
	cmd.Flags().Bool("italic", false, "italic text")
	cmd.Flags().Bool("underline", false, "underlined text")
	cmd.Flags().Bool("strikethrough", false, "strikethrough text")
	cmd.Flags().Bool("faint", false, "faint/dim text")
	cmd.Flags().Bool("blink", false, "blinking text")
	cmd.Flags().Bool("reverse", false, "reverse colors")
	cmd.Flags().Int("margin-left", 0, "left margin")
	cmd.Flags().Int("margin-right", 0, "right margin")
	cmd.Flags().Int("margin-top", 0, "top margin")
	cmd.Flags().Int("margin-bottom", 0, "bottom margin")
	cmd.Flags().Int("padding-left", 0, "left padding")
	cmd.Flags().Int("padding-right", 0, "right padding")
	cmd.Flags().Int("padding-top", 0, "top padding")
	cmd.Flags().Int("padding-bottom", 0, "bottom padding")
	cmd.Flags().Int("width", 0, "fixed width (0 for auto)")
	cmd.Flags().Int("height", 0, "fixed height (0 for auto)")
	cmd.Flags().String("align", "", "text alignment (left, center, right)")
	cmd.Flags().String("border", "", "border style (normal, rounded, thick, double, hidden, none)")

	return cmd
}

func runTuiStyle(cmd *cobra.Command, args []string) error {
	styleForeground, _ := cmd.Flags().GetString("foreground")
	styleBackground, _ := cmd.Flags().GetString("background")
	styleBold, _ := cmd.Flags().GetBool("bold")
	styleItalic, _ := cmd.Flags().GetBool("italic")
	styleUnderline, _ := cmd.Flags().GetBool("underline")
	styleStrike, _ := cmd.Flags().GetBool("strikethrough")
	styleFaint, _ := cmd.Flags().GetBool("faint")
	styleBlink, _ := cmd.Flags().GetBool("blink")
	styleReverse, _ := cmd.Flags().GetBool("reverse")
	styleMarginL, _ := cmd.Flags().GetInt("margin-left")
	styleMarginR, _ := cmd.Flags().GetInt("margin-right")
	styleMarginT, _ := cmd.Flags().GetInt("margin-top")
	styleMarginB, _ := cmd.Flags().GetInt("margin-bottom")
	stylePaddingL, _ := cmd.Flags().GetInt("padding-left")
	stylePaddingR, _ := cmd.Flags().GetInt("padding-right")
	stylePaddingT, _ := cmd.Flags().GetInt("padding-top")
	stylePaddingB, _ := cmd.Flags().GetInt("padding-bottom")
	styleWidth, _ := cmd.Flags().GetInt("width")
	styleHeight, _ := cmd.Flags().GetInt("height")
	styleAlign, _ := cmd.Flags().GetString("align")
	styleBorder, _ := cmd.Flags().GetString("border")

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
