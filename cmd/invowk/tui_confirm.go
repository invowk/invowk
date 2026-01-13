// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"invowk-cli/internal/tui"
)

var (
	confirmTitle       string
	confirmAffirmative string
	confirmNegative    string
	confirmDefault     bool
)

// tuiConfirmCmd provides a yes/no confirmation prompt.
var tuiConfirmCmd = &cobra.Command{
	Use:   "confirm [prompt]",
	Short: "Prompt for yes/no confirmation",
	Long: `Prompt the user for a yes/no confirmation.

Exits with code 0 if confirmed (yes), or code 1 if not confirmed (no).
This makes it suitable for use in shell conditionals.

Examples:
  # Basic confirmation
  invowk tui confirm "Are you sure?"
  
  # With custom button labels
  invowk tui confirm --affirmative "Delete" --negative "Cancel" "Delete this file?"
  
  # Default to yes
  invowk tui confirm --default "Proceed with installation?"
  
  # Use in shell script
  if invowk tui confirm "Continue?"; then
    echo "Continuing..."
  else
    echo "Cancelled."
  fi`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTuiConfirm,
}

func init() {
	tuiCmd.AddCommand(tuiConfirmCmd)

	tuiConfirmCmd.Flags().StringVar(&confirmTitle, "title", "", "title displayed above the prompt (alternative to positional arg)")
	tuiConfirmCmd.Flags().StringVar(&confirmAffirmative, "affirmative", "Yes", "label for the affirmative button")
	tuiConfirmCmd.Flags().StringVar(&confirmNegative, "negative", "No", "label for the negative button")
	tuiConfirmCmd.Flags().BoolVar(&confirmDefault, "default", false, "default to yes")
}

func runTuiConfirm(cmd *cobra.Command, args []string) error {
	title := confirmTitle
	if len(args) > 0 {
		title = args[0]
	}

	confirmed, err := tui.Confirm(tui.ConfirmOptions{
		Title:       title,
		Affirmative: confirmAffirmative,
		Negative:    confirmNegative,
		Default:     confirmDefault,
	})
	if err != nil {
		return err
	}

	if !confirmed {
		os.Exit(1)
	}

	return nil
}
