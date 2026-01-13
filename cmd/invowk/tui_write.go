// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"invowk-cli/internal/tui"
)

var (
	writeTitle       string
	writeDescription string
	writePlaceholder string
	writeValue       string
	writeCharLimit   int
	writeWidth       int
	writeHeight      int
	writeShowLineNum bool
)

// tuiWriteCmd provides a multi-line text editor.
var tuiWriteCmd = &cobra.Command{
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

func init() {
	tuiCmd.AddCommand(tuiWriteCmd)

	tuiWriteCmd.Flags().StringVar(&writeTitle, "title", "", "title displayed above the editor")
	tuiWriteCmd.Flags().StringVar(&writeDescription, "description", "", "description displayed below the title")
	tuiWriteCmd.Flags().StringVar(&writePlaceholder, "placeholder", "", "placeholder text when editor is empty")
	tuiWriteCmd.Flags().StringVar(&writeValue, "value", "", "initial value")
	tuiWriteCmd.Flags().IntVar(&writeCharLimit, "char-limit", 0, "character limit (0 for no limit)")
	tuiWriteCmd.Flags().IntVar(&writeWidth, "width", 0, "width of the editor (0 for auto)")
	tuiWriteCmd.Flags().IntVar(&writeHeight, "height", 0, "height of the editor (0 for auto)")
	tuiWriteCmd.Flags().BoolVar(&writeShowLineNum, "show-line-numbers", false, "show line numbers")
}

func runTuiWrite(cmd *cobra.Command, args []string) error {
	result, err := tui.Write(tui.WriteOptions{
		Title:           writeTitle,
		Description:     writeDescription,
		Placeholder:     writePlaceholder,
		Value:           writeValue,
		CharLimit:       writeCharLimit,
		Width:           writeWidth,
		Height:          writeHeight,
		ShowLineNumbers: writeShowLineNum,
	})
	if err != nil {
		return err
	}

	fmt.Fprint(os.Stdout, result)
	if len(result) > 0 && result[len(result)-1] != '\n' {
		fmt.Fprintln(os.Stdout)
	}
	return nil
}
