// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIConfirmCommand creates the `invowk tui confirm` command.
func newTUIConfirmCommand() *cobra.Command {
	cmd := &cobra.Command{
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

	cmd.Flags().String("title", "", "title displayed above the prompt (alternative to positional arg)")
	cmd.Flags().String("affirmative", "Yes", "label for the affirmative button")
	cmd.Flags().String("negative", "No", "label for the negative button")
	cmd.Flags().Bool("default", false, "default to yes")

	return cmd
}

func runTuiConfirm(cmd *cobra.Command, args []string) error {
	confirmTitle, _ := cmd.Flags().GetString("title")
	confirmAffirmative, _ := cmd.Flags().GetString("affirmative")
	confirmNegative, _ := cmd.Flags().GetString("negative")
	confirmDefault, _ := cmd.Flags().GetBool("default")

	title := confirmTitle
	if len(args) > 0 {
		title = args[0]
	}

	var confirmed bool
	var err error

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		confirmed, err = client.Confirm(tuiserver.ConfirmRequest{
			Title:       title,
			Affirmative: confirmAffirmative,
			Negative:    confirmNegative,
			Default:     confirmDefault,
		})
	} else {
		// Render TUI directly
		confirmed, err = tui.Confirm(tui.ConfirmOptions{
			Title:       title,
			Affirmative: confirmAffirmative,
			Negative:    confirmNegative,
			Default:     confirmDefault,
		})
	}

	if err != nil {
		return err
	}

	if !confirmed {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	return nil
}
