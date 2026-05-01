// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/tui"
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
		var err error
		content, err = readInputAll(cmd.InOrStdin(), "no content provided; provide as arguments or pipe via stdin")
		if err != nil {
			return err
		}
		// Style command trims trailing newline to avoid blank line after styled output
		content = strings.TrimSuffix(content, "\n")
	}

	style, err := tuiStyleFromFlags(
		styleForeground, styleBackground,
		styleBold, styleItalic, styleUnderline, styleStrike, styleFaint, styleBlink, styleReverse,
		styleMarginT, styleMarginR, styleMarginB, styleMarginL,
		stylePaddingT, stylePaddingR, stylePaddingB, stylePaddingL,
		styleWidth, styleHeight,
		styleAlign, styleBorder,
	)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(cmd.OutOrStdout(), style.Apply(content)); err != nil {
		return fmt.Errorf("failed to write styled output: %w", err)
	}
	return nil
}

//goplint:ignore -- CLI flags are raw Cobra primitives converted and validated before rendering.
func tuiStyleFromFlags(
	foreground, background string,
	bold, italic, underline, strikethrough, faint, blink, reverse bool,
	marginTop, marginRight, marginBottom, marginLeft int,
	paddingTop, paddingRight, paddingBottom, paddingLeft int,
	width, height int,
	alignValue, borderValue string,
) (tui.Style, error) {
	style := tui.Style{
		Foreground:    tui.ColorSpec(foreground), //goplint:ignore -- validated before return by validateTUIStyle.
		Background:    tui.ColorSpec(background), //goplint:ignore -- validated before return by validateTUIStyle.
		Bold:          bold,
		Italic:        italic,
		Underline:     underline,
		Strikethrough: strikethrough,
		Faint:         faint,
		Blink:         blink,
		Reverse:       reverse,
		Width:         tui.TerminalDimension(width),  //goplint:ignore -- validated before return by validateTUIStyle.
		Height:        tui.TerminalDimension(height), //goplint:ignore -- validated before return by validateTUIStyle.
		Align:         tui.TextAlign(alignValue),     //goplint:ignore -- validated before return by validateTUIStyle.
		Border:        tui.BorderStyle(borderValue),  //goplint:ignore -- validated before return by validateTUIStyle.
	}
	if marginLeft > 0 || marginRight > 0 || marginTop > 0 || marginBottom > 0 {
		style.Margin = []int{marginTop, marginRight, marginBottom, marginLeft}
	}
	if paddingLeft > 0 || paddingRight > 0 || paddingTop > 0 || paddingBottom > 0 {
		style.Padding = []int{paddingTop, paddingRight, paddingBottom, paddingLeft}
	}
	if err := validateTUIStyle(style); err != nil {
		return tui.Style{}, err
	}
	return style, nil
}

func validateTUIStyle(style tui.Style) error {
	if err := style.Foreground.Validate(); err != nil {
		return err
	}
	if err := style.Background.Validate(); err != nil {
		return err
	}
	if err := style.Width.Validate(); err != nil {
		return err
	}
	if err := style.Height.Validate(); err != nil {
		return err
	}
	if err := style.Align.Validate(); err != nil {
		return err
	}
	if err := style.Border.Validate(); err != nil {
		return err
	}
	return nil
}
