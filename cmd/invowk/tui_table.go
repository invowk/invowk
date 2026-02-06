// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"

	"github.com/spf13/cobra"
)

// newTUITableCommand creates the `invowk tui table` command.
func newTUITableCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "table",
		Short: "Display and select from a table",
		Long: `Display data in a table format with optional row selection.

Data can be provided via:
- A file with --file
- Piped via stdin (CSV or custom separator)

Examples:
  # Display a CSV file
  invowk tui table --file data.csv

  # Pipe data with custom separator
  echo -e "name|age|city\nAlice|30|NYC\nBob|25|LA" | invowk tui table --separator "|"

  # Selectable rows
  cat data.csv | invowk tui table --selectable

  # Custom column widths
  invowk tui table --file data.csv --widths 20,10,30`,
		RunE: runTuiTable,
	}

	cmd.Flags().String("file", "", "CSV file to display")
	cmd.Flags().String("separator", ",", "column separator")
	cmd.Flags().StringSlice("columns", nil, "column headers (overrides file header)")
	cmd.Flags().IntSlice("widths", nil, "column widths")
	cmd.Flags().Int("height", 0, "table height (0 for auto)")
	cmd.Flags().Bool("selectable", false, "enable row selection")

	return cmd
}

func runTuiTable(cmd *cobra.Command, args []string) error {
	tableFile, _ := cmd.Flags().GetString("file")
	tableSeparator, _ := cmd.Flags().GetString("separator")
	tableColumns, _ := cmd.Flags().GetStringSlice("columns")
	tableWidths, _ := cmd.Flags().GetIntSlice("widths")
	tableHeight, _ := cmd.Flags().GetInt("height")
	tableSelectable, _ := cmd.Flags().GetBool("selectable")

	var rows [][]string

	if tableFile != "" {
		// Read from file
		f, err := os.Open(tableFile)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer func() { _ = f.Close() }() // Read-only file; close error non-critical

		reader := csv.NewReader(f)
		reader.Comma = rune(tableSeparator[0])
		rows, err = reader.ReadAll()
		if err != nil {
			return fmt.Errorf("failed to parse CSV: %w", err)
		}
	} else {
		// Check if we have stdin input
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Read from stdin
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					parts := strings.Split(line, tableSeparator)
					rows = append(rows, parts)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error reading stdin: %w", err)
			}
		} else {
			return fmt.Errorf("no data provided; use --file or pipe data via stdin")
		}
	}

	if len(rows) == 0 {
		return fmt.Errorf("no data to display")
	}

	// Extract headers
	headers := rows[0]
	if len(tableColumns) > 0 {
		headers = tableColumns
	} else {
		rows = rows[1:] // Remove header row from data
	}

	var selectedIdx int
	var selectedRow []string
	var err error

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		borderStr := "normal"
		if !tableSelectable {
			borderStr = "none"
		}
		result, clientErr := client.Table(tuiserver.TableRequest{
			Columns:   headers,
			Rows:      rows,
			Widths:    tableWidths,
			Height:    tableHeight,
			Separator: tableSeparator,
			Border:    borderStr,
			Print:     !tableSelectable,
		})
		if clientErr != nil {
			return clientErr
		}
		selectedIdx = result.SelectedIndex
		selectedRow = result.SelectedRow
	} else {
		// Build columns for direct rendering
		columns := make([]tui.TableColumn, len(headers))
		for i, h := range headers {
			width := 0
			if i < len(tableWidths) {
				width = tableWidths[i]
			}
			columns[i] = tui.TableColumn{
				Title: h,
				Width: width,
			}
		}

		// Use the Table function which returns (int, []string, error)
		selectedIdx, selectedRow, err = tui.Table(tui.TableOptions{
			Columns:    columns,
			Rows:       rows,
			Height:     tableHeight,
			Selectable: tableSelectable,
		})
		if err != nil {
			return err
		}
	}

	// If selectable and a row was selected, print it
	if tableSelectable && selectedIdx >= 0 && len(selectedRow) > 0 {
		_, _ = fmt.Fprintln(os.Stdout, strings.Join(selectedRow, tableSeparator)) // Terminal output; error non-critical
	}

	return nil
}
