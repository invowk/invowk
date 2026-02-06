// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"
	"strings"

	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIChooseCommand creates the `invowk tui choose` command.
func newTUIChooseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "choose [options...]",
		Short: "Select from a list of options",
		Long: `Select one or more options from a list.

By default, allows selecting a single option. Use --limit to allow multiple
selections. Each selected option is printed on a separate line.

Examples:
  # Single selection
  invowk tui choose "Option 1" "Option 2" "Option 3"

  # With title
  invowk tui choose --title "Pick a color" red green blue

  # Multi-select (up to 3)
  invowk tui choose --limit 3 "One" "Two" "Three" "Four"

  # Unlimited multi-select
  invowk tui choose --no-limit "One" "Two" "Three"

  # Use in shell script
  COLOR=$(invowk tui choose --title "Pick a color" red green blue)
  echo "You picked: $COLOR"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runTuiChoose,
	}

	cmd.Flags().String("title", "", "title displayed above the list")
	cmd.Flags().Int("limit", 1, "maximum number of selections (1 for single select)")
	cmd.Flags().Bool("no-limit", false, "allow unlimited selections")
	cmd.Flags().Int("height", 0, "height of the list (0 for auto)")

	return cmd
}

func runTuiChoose(cmd *cobra.Command, args []string) error {
	chooseTitle, _ := cmd.Flags().GetString("title")
	chooseLimit, _ := cmd.Flags().GetInt("limit")
	chooseNoLimit, _ := cmd.Flags().GetBool("no-limit")
	chooseHeight, _ := cmd.Flags().GetInt("height")

	limit := chooseLimit
	if chooseNoLimit {
		limit = 0 // 0 means unlimited in multi-select
	}

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		if limit == 1 && !chooseNoLimit {
			// Single selection mode
			result, err := client.ChooseSingle(tuiserver.ChooseRequest{
				Title:   chooseTitle,
				Options: args,
				Height:  chooseHeight,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, result)
		} else {
			// Multi-selection mode
			results, err := client.ChooseMultiple(tuiserver.ChooseRequest{
				Title:   chooseTitle,
				Options: args,
				Limit:   limit,
				NoLimit: chooseNoLimit,
				Height:  chooseHeight,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, strings.Join(results, "\n"))
		}
		return nil
	}

	// Render TUI directly
	if limit == 1 && !chooseNoLimit {
		// Single selection mode using ChooseStrings convenience function
		result, err := tui.ChooseStrings(chooseTitle, args, tui.DefaultConfig())
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(os.Stdout, result)
	} else {
		// Multi-selection mode using MultiChooseStrings convenience function
		results, err := tui.MultiChooseStrings(chooseTitle, args, limit, tui.DefaultConfig())
		if err != nil {
			return err
		}

		// Print each selection on a separate line
		_, _ = fmt.Fprintln(os.Stdout, strings.Join(results, "\n"))
	}

	return nil
}
