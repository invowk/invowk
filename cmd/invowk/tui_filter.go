// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"fmt"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	filterTitle       string
	filterPlaceholder string
	filterWidth       int
	filterHeight      int
	filterLimit       int
	filterNoLimit     bool
	filterReverse     bool
	filterFuzzy       bool

	// tuiFilterCmd provides fuzzy filtering of a list.
	tuiFilterCmd = &cobra.Command{
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
)

func init() {
	tuiCmd.AddCommand(tuiFilterCmd)

	tuiFilterCmd.Flags().StringVar(&filterTitle, "title", "", "title displayed above the filter")
	tuiFilterCmd.Flags().StringVar(&filterPlaceholder, "placeholder", "Type to filter...", "placeholder text in search box")
	tuiFilterCmd.Flags().IntVar(&filterWidth, "width", 0, "width of the filter (0 for auto)")
	tuiFilterCmd.Flags().IntVar(&filterHeight, "height", 0, "height of the list (0 for auto)")
	tuiFilterCmd.Flags().IntVar(&filterLimit, "limit", 1, "maximum selections (1 for single)")
	tuiFilterCmd.Flags().BoolVar(&filterNoLimit, "no-limit", false, "allow unlimited selections")
	tuiFilterCmd.Flags().BoolVar(&filterReverse, "reverse", false, "display list in reverse order")
	tuiFilterCmd.Flags().BoolVar(&filterFuzzy, "fuzzy", true, "enable fuzzy matching")
}

func runTuiFilter(cmd *cobra.Command, args []string) error {
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
			Width:       filterWidth,
			Height:      filterHeight,
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
