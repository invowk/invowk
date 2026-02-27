// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIInputCommand creates the `invowk tui input` command.
func newTUIInputCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "input",
		Short: "Prompt for single-line text input",
		Long: `Prompt the user for a single line of text input.

The result is printed to stdout, making it suitable for use in shell pipelines
and variable assignments.

Examples:
  # Basic input
  invowk tui input --title "What is your name?"

  # With placeholder text
  invowk tui input --title "Email" --placeholder "user@example.com"

  # Password input (hidden)
  invowk tui input --title "Password" --password

  # With character limit
  invowk tui input --title "Username" --char-limit 20

  # Use in shell script
  NAME=$(invowk tui input --title "Enter your name:")
  echo "Hello, $NAME!"`,
		RunE: runTuiInput,
	}

	cmd.Flags().String("title", "", "title/prompt displayed above the input")
	cmd.Flags().String("description", "", "description displayed below the title")
	cmd.Flags().String("placeholder", "", "placeholder text when input is empty")
	cmd.Flags().String("value", "", "initial value of the input")
	cmd.Flags().Int("char-limit", 0, "character limit (0 for no limit)")
	cmd.Flags().Int("width", 0, "width of the input field (0 for auto)")
	cmd.Flags().Bool("password", false, "hide input characters (password mode)")
	cmd.Flags().String("prompt", "", "prompt character(s) before input")

	return cmd
}

func runTuiInput(cmd *cobra.Command, args []string) error {
	inputTitle, _ := cmd.Flags().GetString("title")
	inputDescription, _ := cmd.Flags().GetString("description")
	inputPlaceholder, _ := cmd.Flags().GetString("placeholder")
	inputValue, _ := cmd.Flags().GetString("value")
	inputCharLimit, _ := cmd.Flags().GetInt("char-limit")
	inputWidth, _ := cmd.Flags().GetInt("width")
	inputPassword, _ := cmd.Flags().GetBool("password")
	inputPrompt, _ := cmd.Flags().GetString("prompt")

	var result string
	var err error

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		result, err = client.Input(tuiserver.InputRequest{
			Title:       inputTitle,
			Description: inputDescription,
			Placeholder: inputPlaceholder,
			Value:       inputValue,
			CharLimit:   inputCharLimit,
			Width:       inputWidth,
			Password:    inputPassword,
			Prompt:      inputPrompt,
		})
	} else {
		// Render TUI directly
		result, err = tui.Input(tui.InputOptions{
			Title:       inputTitle,
			Description: inputDescription,
			Placeholder: inputPlaceholder,
			Value:       inputValue,
			CharLimit:   inputCharLimit,
			Width:       tui.TerminalDimension(inputWidth), //goplint:ignore -- CLI integer argument
			Password:    inputPassword,
			Prompt:      inputPrompt,
		})
	}

	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout, result) // Terminal output; error non-critical
	return nil
}
