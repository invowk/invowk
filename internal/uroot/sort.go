// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	gosort "sort"
	"strconv"
	"strings"
)

// sortCommand implements the sort utility.
type sortCommand struct {
	name  string
	flags []FlagInfo
}

// newSortCommand creates a new sort command.
func newSortCommand() *sortCommand {
	return &sortCommand{
		name: "sort",
		flags: []FlagInfo{
			{Name: "r", Description: "reverse the result of comparisons"},
			{Name: "n", Description: "compare according to string numerical value"},
			{Name: "u", Description: "output only unique lines"},
			{Name: "f", Description: "fold lower case to upper case (case insensitive)"},
			{Name: "b", Description: "ignore leading blanks"},
			{Name: "k", Description: "sort by key", TakesValue: true},
			{Name: "t", Description: "field separator", TakesValue: true},
		},
	}
}

// Name returns the command name.
func (c *sortCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *sortCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the sort command.
func (c *sortCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("sort", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	reverse := fs.Bool("r", false, "reverse")
	numeric := fs.Bool("n", false, "numeric sort")
	unique := fs.Bool("u", false, "unique")
	foldCase := fs.Bool("f", false, "case insensitive")
	ignoreBlanks := fs.Bool("b", false, "ignore leading blanks")
	_ = fs.String("k", "", "key") // Accepted but simplified handling
	_ = fs.String("t", "", "separator")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	// Collect all lines from all files
	var lines []string

	err := ProcessFilesOrStdin(fs.Args(), hc.Stdin, hc.Dir, c.name,
		func(r io.Reader, _ string, _, _ int) error {
			fileLines, readErr := c.readLines(r)
			if readErr != nil {
				return readErr
			}
			lines = append(lines, fileLines...)
			return nil
		})
	if err != nil {
		return err
	}

	// Sort the lines
	c.sortLines(lines, *reverse, *numeric, *foldCase, *ignoreBlanks)

	// Output
	if *unique {
		c.outputUnique(hc.Stdout, lines, *foldCase)
	} else {
		for _, line := range lines {
			fmt.Fprintln(hc.Stdout, line)
		}
	}

	return nil
}

// readLines reads all lines from a reader.
func (c *sortCommand) readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	return lines, nil
}

// sortLines sorts the lines according to the given options.
func (c *sortCommand) sortLines(lines []string, reverse, numeric, foldCase, ignoreBlanks bool) {
	gosort.Slice(lines, func(i, j int) bool {
		a, b := lines[i], lines[j]

		if ignoreBlanks {
			a = strings.TrimLeft(a, " \t")
			b = strings.TrimLeft(b, " \t")
		}

		if foldCase {
			a = strings.ToLower(a)
			b = strings.ToLower(b)
		}

		var less bool
		if numeric {
			na := parseNumber(a)
			nb := parseNumber(b)
			less = na < nb
		} else {
			less = a < b
		}

		if reverse {
			return !less
		}
		return less
	})
}

// parseNumber extracts a number from the beginning of a string.
func parseNumber(s string) float64 {
	s = strings.TrimSpace(s)
	// Find the numeric prefix
	end := 0
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '+' {
			end = i + 1
		} else {
			break
		}
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.ParseFloat(s[:end], 64) //nolint:errcheck // Best-effort numeric extraction
	return n
}

// outputUnique outputs only unique lines.
func (c *sortCommand) outputUnique(out io.Writer, lines []string, foldCase bool) {
	var prev string
	first := true

	for _, line := range lines {
		current := line
		if foldCase {
			current = strings.ToLower(line)
		}

		if first || current != prev {
			fmt.Fprintln(out, line)
			prev = current
			first = false
		}
	}
}
