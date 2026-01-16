// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"invowk-cli/internal/tui"
)

var (
	formatType     string
	formatLanguage string
	formatTheme    string
)

// tuiFormatCmd formats text with styling.
var tuiFormatCmd = &cobra.Command{
	Use:   "format [text...]",
	Short: "Format text with markdown, code, or emoji",
	Long: `Format and render text with various styling options.

Content can be provided as arguments or piped via stdin.

Format types:
  markdown - Render markdown formatting
  code     - Syntax highlight code
  emoji    - Convert emoji shortcodes (e.g., :smile:)
  template - Apply Go template formatting

Examples:
  # Format markdown
  echo "# Hello World" | invowk tui format --type markdown
  
  # Syntax highlight code
  cat main.go | invowk tui format --type code --language go
  
  # Convert emoji
  echo "Hello :wave: World :smile:" | invowk tui format --type emoji
  
  # Format inline
  invowk tui format --type markdown "**bold** and *italic*"`,
	RunE: runTuiFormat,
}

func init() {
	tuiCmd.AddCommand(tuiFormatCmd)

	tuiFormatCmd.Flags().StringVar(&formatType, "type", "markdown", "format type (markdown, code, emoji, template)")
	tuiFormatCmd.Flags().StringVar(&formatLanguage, "language", "", "language for code highlighting")
	tuiFormatCmd.Flags().StringVar(&formatTheme, "theme", "", "theme for code highlighting (glamour theme)")
}

func runTuiFormat(cmd *cobra.Command, args []string) error {
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
			content = sb.String()
		} else {
			return fmt.Errorf("no content provided; provide as arguments or pipe via stdin")
		}
	}

	var formatTypeEnum tui.FormatType
	switch formatType {
	case "markdown":
		formatTypeEnum = tui.FormatMarkdown
	case "code":
		formatTypeEnum = tui.FormatCode
	case "emoji":
		formatTypeEnum = tui.FormatEmoji
	case "template":
		formatTypeEnum = tui.FormatTemplate
	default:
		return fmt.Errorf("unknown format type: %s (use markdown, code, emoji, or template)", formatType)
	}

	result, err := tui.Format(tui.FormatOptions{
		Content:      content,
		Type:         formatTypeEnum,
		Language:     formatLanguage,
		GlamourTheme: formatTheme,
	})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprint(os.Stdout, result) // Terminal output; error non-critical
	if len(result) > 0 && result[len(result)-1] != '\n' {
		_, _ = fmt.Fprintln(os.Stdout)
	}
	return nil
}
