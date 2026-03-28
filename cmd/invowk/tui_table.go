// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"

	"github.com/spf13/cobra"
)

// errNoDataToDisplay is returned when a table command has no rows to render.
var errNoDataToDisplay = errors.New("no data to display")

type (
	tableConfig struct {
		file       tableFile      //goplint:ignore -- transient CLI flag value bundle for table rendering.
		separator  tableSeparator //goplint:ignore -- transient CLI flag value bundle for table rendering.
		columns    tableColumns   //goplint:ignore -- transient CLI flag value bundle for table rendering.
		widths     tableWidths    //goplint:ignore -- transient CLI flag value bundle for table rendering.
		height     tableHeight    //goplint:ignore -- transient CLI flag value bundle for table rendering.
		selectable bool
	}

	tableFile      string
	tableSeparator string
	tableColumns   []string
	tableWidths    []int
	tableHeight    int
)

func (f tableFile) Validate() error      { return nil }
func (f tableFile) String() string       { return string(f) }
func (s tableSeparator) Validate() error { return nil }
func (s tableSeparator) String() string  { return string(s) }
func (h tableHeight) Validate() error    { return nil }
func (h tableHeight) String() string     { return fmt.Sprintf("%d", h) }

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

func runTuiTable(cmd *cobra.Command, _ []string) error {
	cfg, err := readTableConfig(cmd)
	if err != nil {
		return err
	}
	rows, err := loadTableRows(cfg)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errNoDataToDisplay
	}

	headers, rows := splitHeadersAndData(rows, cfg.columns)
	selectedIdx, selectedRow, err := renderTableSelection(cfg, headers, rows)
	if err != nil {
		return err
	}

	// If selectable and a row was selected, print it
	if cfg.selectable && selectedIdx >= 0 && len(selectedRow) > 0 {
		_, _ = fmt.Fprintln(os.Stdout, strings.Join(selectedRow, string(cfg.separator))) // Terminal output; error non-critical
	}

	return nil
}

func readTableConfig(cmd *cobra.Command) (tableConfig, error) {
	file, _ := cmd.Flags().GetString("file")
	separator, _ := cmd.Flags().GetString("separator")
	columns, _ := cmd.Flags().GetStringSlice("columns")
	widths, _ := cmd.Flags().GetIntSlice("widths")
	height, _ := cmd.Flags().GetInt("height")
	selectable, _ := cmd.Flags().GetBool("selectable")

	fileValue := tableFile(file)
	if err := fileValue.Validate(); err != nil {
		return tableConfig{}, err
	}
	separatorValue := tableSeparator(separator)
	if err := separatorValue.Validate(); err != nil {
		return tableConfig{}, err
	}
	heightValue := tableHeight(height)
	if err := heightValue.Validate(); err != nil {
		return tableConfig{}, err
	}

	cfg := tableConfig{
		file:       fileValue,
		separator:  separatorValue,
		columns:    tableColumns(columns),
		widths:     tableWidths(widths),
		height:     heightValue,
		selectable: selectable,
	}
	if err := cfg.Validate(); err != nil {
		return tableConfig{}, err
	}
	return cfg, nil
}

func (c tableConfig) Validate() error {
	var errs []error
	if err := c.file.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.separator.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.height.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func loadTableRows(cfg tableConfig) ([][]string, error) {
	if cfg.file != "" {
		return loadTableRowsFromFile(cfg)
	}
	return loadTableRowsFromStdin(cfg)
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func loadTableRowsFromFile(cfg tableConfig) ([][]string, error) {
	f, err := os.Open(string(cfg.file))
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }() // Read-only file; close error non-critical

	reader := csv.NewReader(f)
	reader.Comma = rune(cfg.separator[0])
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	return rows, nil
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func loadTableRowsFromStdin(cfg tableConfig) ([][]string, error) {
	if !isStdinPiped() {
		return nil, errors.New("no data provided; use --file or pipe data via stdin")
	}

	var rows [][]string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			rows = append(rows, strings.Split(line, string(cfg.separator)))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stdin: %w", err)
	}
	return rows, nil
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func splitHeadersAndData(rows [][]string, override tableColumns) (headers []string, data [][]string) {
	headers = rows[0]
	if len(override) > 0 {
		return []string(override), rows
	}
	return headers, rows[1:]
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func renderTableSelection(cfg tableConfig, headers []string, rows [][]string) (selectedIdx int, selectedRow []string, err error) {
	if client := tuiserver.NewClientFromEnv(); client != nil {
		return renderTUITableWithClient(cfg, headers, rows, client)
	}
	return renderTUITableDirect(cfg, headers, rows)
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func renderTUITableWithClient(cfg tableConfig, headers []string, rows [][]string, client *tuiserver.Client) (selectedIdx int, selectedRow []string, err error) {
	border := "normal"
	if !cfg.selectable {
		border = "none"
	}
	result, err := client.Table(tuiserver.TableRequest{
		Columns:   headers,
		Rows:      rows,
		Widths:    []int(cfg.widths),
		Height:    int(cfg.height),
		Separator: string(cfg.separator),
		Border:    border,
		Print:     !cfg.selectable,
	})
	if err != nil {
		return 0, nil, err
	}
	return result.SelectedIndex, result.SelectedRow, nil
}

//goplint:ignore -- CLI table helpers operate on transient display rows.
func renderTUITableDirect(cfg tableConfig, headers []string, rows [][]string) (selectedIdx int, selectedRow []string, err error) {
	columns := make([]tui.TableColumn, len(headers))
	for i, header := range headers {
		width := 0
		if i < len(cfg.widths) {
			width = cfg.widths[i]
		}
		columns[i] = tui.TableColumn{
			Title: header,
			Width: tui.TerminalDimension(width), //goplint:ignore -- CLI integer argument
		}
	}

	return tui.Table(tui.TableOptions{
		Columns:    columns,
		Rows:       rows,
		Height:     tui.TerminalDimension(cfg.height), //goplint:ignore -- CLI integer argument
		Selectable: cfg.selectable,
	})
}
