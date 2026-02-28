// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUIFilterCommand creates the `invowk tui filter` command.
func newTUIFilterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filter [options...]",
		Short: "Fuzzy filter a list of options",
		Long: `Filter a list of options using fuzzy matching.

Options can be provided as arguments or piped via stdin (one per line).
The selected option(s) are printed to stdout.

Examples:
  # Filter from arguments
  invowk tui filter "apple" "banana" "cherry" "date"

  # Filter from stdin
  ls | invowk tui filter --title "Select a file"

  # Multi-select filter
  cat files.txt | invowk tui filter --no-limit

  # With custom placeholder
  invowk tui filter --placeholder "Type to search..." opt1 opt2 opt3

  # Strict matching (not fuzzy)
  invowk tui filter --fuzzy=false "one" "two" "three"`,
		RunE: runTuiFilter,
	}

	cmd.Flags().String("title", "", "title displayed above the filter")
	cmd.Flags().String("placeholder", "Type to filter...", "placeholder text in search box")
	cmd.Flags().Int("width", 0, "width of the filter (0 for auto)")
	cmd.Flags().Int("height", 0, "height of the list (0 for auto)")
	cmd.Flags().Int("limit", 1, "maximum selections (1 for single)")
	cmd.Flags().Bool("no-limit", false, "allow unlimited selections")
	cmd.Flags().Bool("reverse", false, "display list in reverse order")
	cmd.Flags().Bool("fuzzy", true, "enable fuzzy matching")

	return cmd
}

func runTuiFilter(cmd *cobra.Command, args []string) error {
	filterTitle, _ := cmd.Flags().GetString("title")
	filterPlaceholder, _ := cmd.Flags().GetString("placeholder")
	filterWidth, _ := cmd.Flags().GetInt("width")
	filterHeight, _ := cmd.Flags().GetInt("height")
	filterLimit, _ := cmd.Flags().GetInt("limit")
	filterNoLimit, _ := cmd.Flags().GetBool("no-limit")
	filterReverse, _ := cmd.Flags().GetBool("reverse")
	filterFuzzy, _ := cmd.Flags().GetBool("fuzzy")

	var options []string

	// Check if we have stdin input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Read from stdin
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				options = append(options, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("error reading stdin: %w", err)
		}
	}

	// Add arguments as options
	options = append(options, args...)

	if len(options) == 0 {
		return fmt.Errorf("no options provided; provide as arguments or pipe via stdin")
	}

	limit := filterLimit
	if filterNoLimit {
		limit = 0
	}

	var results []string
	var err error

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		results, err = client.Filter(tuiserver.FilterRequest{
			Title:       filterTitle,
			Placeholder: filterPlaceholder,
			Options:     options,
			Width:       filterWidth,
			Height:      filterHeight,
			Limit:       limit,
			NoLimit:     filterNoLimit,
			Reverse:     filterReverse,
			Fuzzy:       filterFuzzy,
		})
	} else {
		// Render TUI directly
		results, err = tui.Filter(tui.FilterOptions{
			Title:       filterTitle,
			Placeholder: filterPlaceholder,
			Options:     options,
			Width:       tui.TerminalDimension(filterWidth),  //goplint:ignore -- CLI integer argument
			Height:      tui.TerminalDimension(filterHeight), //goplint:ignore -- CLI integer argument
			Limit:       limit,
			NoLimit:     filterNoLimit,
			Reverse:     filterReverse,
			Fuzzy:       filterFuzzy,
		})
	}

	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout, strings.Join(results, "\n")) // Terminal output; error non-critical
	return nil
}
