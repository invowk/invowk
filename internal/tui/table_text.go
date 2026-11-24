// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"
	"strings"

	"charm.land/lipgloss/v2"
)

// tableTextGap separates the columns of WriteTableText.
const tableTextGap = "  "

// WriteTableText writes headers and rows to w as plain text, for a display
// table whose stdout is not a terminal:
//   - the first line holds the headers, then one line per row in order;
//   - each cell is left-aligned and padded with spaces to the widest cell of
//     its column, measured in terminal cells (lipgloss.Width);
//   - columns are separated by two spaces, trailing spaces are trimmed, and
//     every line ends with "\n";
//   - a row with fewer cells than the widest row gets empty cells;
//   - there is no border, header rule or control sequence, and no row or cell
//     is cut.
//
//goplint:ignore -- display rows are transient CLI data, like TableOptions.Rows.
func WriteTableText(w io.Writer, headers []string, rows [][]string) error {
	lines := make([][]string, 0, len(rows)+1)
	lines = append(lines, headers)
	lines = append(lines, rows...)

	var widths []int
	for _, line := range lines {
		for i, cell := range line {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(cell))
		}
	}

	var b strings.Builder
	for _, line := range lines {
		var row strings.Builder
		for i := range widths {
			cell := ""
			if i < len(line) {
				cell = line[i]
			}
			if i > 0 {
				row.WriteString(tableTextGap)
			}
			row.WriteString(cell)
			row.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(cell)))
		}
		b.WriteString(strings.TrimRight(row.String(), " "))
		b.WriteByte('\n')
	}
	_, err := io.WriteString(w, b.String())
	return err
}
