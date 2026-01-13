package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"invowk-cli/internal/tui"
)

var (
	inputTitle       string
	inputDescription string
	inputPlaceholder string
	inputValue       string
	inputCharLimit   int
	inputWidth       int
	inputPassword    bool
	inputPrompt      string
)

// tuiInputCmd provides a single-line text input prompt.
var tuiInputCmd = &cobra.Command{
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

func init() {
	tuiCmd.AddCommand(tuiInputCmd)

	tuiInputCmd.Flags().StringVar(&inputTitle, "title", "", "title/prompt displayed above the input")
	tuiInputCmd.Flags().StringVar(&inputDescription, "description", "", "description displayed below the title")
	tuiInputCmd.Flags().StringVar(&inputPlaceholder, "placeholder", "", "placeholder text when input is empty")
	tuiInputCmd.Flags().StringVar(&inputValue, "value", "", "initial value of the input")
	tuiInputCmd.Flags().IntVar(&inputCharLimit, "char-limit", 0, "character limit (0 for no limit)")
	tuiInputCmd.Flags().IntVar(&inputWidth, "width", 0, "width of the input field (0 for auto)")
	tuiInputCmd.Flags().BoolVar(&inputPassword, "password", false, "hide input characters (password mode)")
	tuiInputCmd.Flags().StringVar(&inputPrompt, "prompt", "", "prompt character(s) before input")
}

func runTuiInput(cmd *cobra.Command, args []string) error {
	result, err := tui.Input(tui.InputOptions{
		Title:       inputTitle,
		Description: inputDescription,
		Placeholder: inputPlaceholder,
		Value:       inputValue,
		CharLimit:   inputCharLimit,
		Width:       inputWidth,
		Password:    inputPassword,
		Prompt:      inputPrompt,
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, result)
	return nil
}
