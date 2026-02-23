// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIWriteCommand creates the `invowk tui write` command.
func newTUIWriteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write",
		Short: "Multi-line text editor",
		Long: `Open a multi-line text editor for entering longer text.

The result is printed to stdout when the user submits (Ctrl+D or Esc).
Useful for commit messages, descriptions, or any multi-line input.

Examples:
  # Basic text editor
  invowk tui write --title "Enter description"

  # With placeholder and line numbers
  invowk tui write --title "Commit message" --placeholder "Describe your changes..." --show-line-numbers

  # With initial value
  invowk tui write --value "Initial text here"

  # Use in shell script
  MESSAGE=$(invowk tui write --title "Enter commit message")
  git commit -m "$MESSAGE"`,
		RunE: runTuiWrite,
	}

	cmd.Flags().String("title", "", "title displayed above the editor")
	cmd.Flags().String("description", "", "description displayed below the title")
	cmd.Flags().String("placeholder", "", "placeholder text when editor is empty")
	cmd.Flags().String("value", "", "initial value")
	cmd.Flags().Int("char-limit", 0, "character limit (0 for no limit)")
	cmd.Flags().Int("width", 0, "width of the editor (0 for auto)")
	cmd.Flags().Int("height", 0, "height of the editor (0 for auto)")
	cmd.Flags().Bool("show-line-numbers", false, "show line numbers")

	return cmd
}

func runTuiWrite(cmd *cobra.Command, args []string) error {
	writeTitle, _ := cmd.Flags().GetString("title")
	writeDescription, _ := cmd.Flags().GetString("description")
	writePlaceholder, _ := cmd.Flags().GetString("placeholder")
	writeValue, _ := cmd.Flags().GetString("value")
	writeCharLimit, _ := cmd.Flags().GetInt("char-limit")
	writeWidth, _ := cmd.Flags().GetInt("width")
	writeHeight, _ := cmd.Flags().GetInt("height")
	writeShowLineNum, _ := cmd.Flags().GetBool("show-line-numbers")

	var result string
	var err error

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		result, err = client.TextArea(tuiserver.TextAreaRequest{
			Title:           writeTitle,
			Description:     writeDescription,
			Placeholder:     writePlaceholder,
			Value:           writeValue,
			CharLimit:       writeCharLimit,
			Width:           writeWidth,
			Height:          writeHeight,
			ShowLineNumbers: writeShowLineNum,
		})
	} else {
		// Render TUI directly
		result, err = tui.Write(tui.WriteOptions{
			Title:           writeTitle,
			Description:     writeDescription,
			Placeholder:     writePlaceholder,
			Value:           writeValue,
			CharLimit:       writeCharLimit,
			Width:           tui.TerminalDimension(writeWidth),
			Height:          tui.TerminalDimension(writeHeight),
			ShowLineNumbers: writeShowLineNum,
		})
	}

	if err != nil {
		return err
	}

	_, _ = fmt.Fprint(os.Stdout, result) // Terminal output; error non-critical
	if result != "" && result[len(result)-1] != '\n' {
		_, _ = fmt.Fprintln(os.Stdout)
	}
	return nil
}
