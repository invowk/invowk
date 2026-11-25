// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"io"
	"strings"
	"unicode"

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
//   - every control character in a cell (a newline, a tab, an escape), every
//     format character (Unicode category Cf, such as a bidirectional override
//     or a zero-width space) and the line and paragraph separators U+2028 and
//     U+2029 are written as a space, so a row stays on one line and its text
//     reads in the order it is stored;
//   - every byte sequence that is not valid UTF-8 is written as U+FFFD;
//   - there is no border, header rule or control sequence, and no row or cell
//     is cut.
//
//goplint:ignore -- display rows are transient CLI data, like TableOptions.Rows.
func WriteTableText(w io.Writer, headers []string, rows [][]string) error {
	lines := make([][]string, 0, len(rows)+1)
	lines = append(lines, tableTextCells(headers))
	for _, row := range rows {
		lines = append(lines, tableTextCells(row))
	}

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

// tableTextCells returns a copy of cells with every control character,
// format character and line or paragraph separator written as a space, and
// every invalid UTF-8 byte as U+FFFD (strings.Map decodes it as RuneError).
//
//goplint:ignore -- display cells are transient CLI data, like TableOptions.Rows.
func tableTextCells(cells []string) []string {
	out := make([]string, len(cells))
	for i, cell := range cells {
		out[i] = strings.Map(tableTextRune, cell)
	}
	return out
}

// tableTextRune maps a control character, a format character (Cf) and the
// line and paragraph separators U+2028 (Zl) and U+2029 (Zp) to a space.
//
//goplint:ignore -- strings.Map takes a plain rune mapping.
func tableTextRune(r rune) rune {
	if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) {
		return ' '
	}
	return r
}
