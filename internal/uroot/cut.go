// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type (
	// cutCommand implements the cut utility.
	cutCommand struct {
		name  string
		flags []FlagInfo
	}

	// cutRange represents a range specification (1-based indexing).
	cutRange struct {
		start int // 1-based start
		end   int // 1-based end, -1 means to end of line
	}
)

// newCutCommand creates a new cut command.
func newCutCommand() *cutCommand {
	return &cutCommand{
		name: "cut",
		flags: []FlagInfo{
			{Name: "d", Description: "use DELIM instead of TAB", TakesValue: true},
			{Name: "f", Description: "select only these fields", TakesValue: true},
			{Name: "c", Description: "select only these characters", TakesValue: true},
			{Name: "s", Description: "do not print lines without delimiters"},
		},
	}
}

// Name returns the command name.
func (c *cutCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *cutCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the cut command.
func (c *cutCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("cut", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	delimiter := fs.String("d", "\t", "delimiter")
	fields := fs.String("f", "", "fields")
	chars := fs.String("c", "", "characters")
	onlyDelimited := fs.Bool("s", false, "only delimited")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	// Must specify either -f or -c
	if *fields == "" && *chars == "" {
		return wrapError(c.name, fmt.Errorf("you must specify a list of bytes, characters, or fields"))
	}

	// Parse field/char specification
	var ranges []cutRange
	var err error
	if *fields != "" {
		ranges, err = parseRanges(*fields)
	} else {
		ranges, err = parseRanges(*chars)
	}
	if err != nil {
		return wrapError(c.name, err)
	}

	return ProcessFilesOrStdin(fs.Args(), hc.Stdin, hc.Dir, c.name,
		func(r io.Reader, _ string, _, _ int) error {
			return c.processReader(hc.Stdout, r, ranges, *delimiter, *fields != "", *onlyDelimited)
		})
}

// parseRanges parses a range specification like "1,3-5,7-".
func parseRanges(spec string) ([]cutRange, error) {
	var ranges []cutRange

	for part := range strings.SplitSeq(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if before, after, found := strings.Cut(part, "-"); found {
			// Range: "3-5" or "3-" or "-5"
			var start, end int
			var err error

			if before == "" {
				start = 1
			} else {
				start, err = strconv.Atoi(before)
				if err != nil || start < 1 {
					return nil, fmt.Errorf("invalid range: %s", part)
				}
			}

			if after == "" {
				end = -1 // to end of line
			} else {
				end, err = strconv.Atoi(after)
				if err != nil || end < 1 {
					return nil, fmt.Errorf("invalid range: %s", part)
				}
			}

			ranges = append(ranges, cutRange{start: start, end: end})
		} else {
			// Single field/char
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid field/character position: %s", part)
			}
			ranges = append(ranges, cutRange{start: n, end: n})
		}
	}

	return ranges, nil
}

// processReader processes input line by line.
func (c *cutCommand) processReader(out io.Writer, in io.Reader, ranges []cutRange, delimiter string, isFields, onlyDelimited bool) error {
	scanner := bufio.NewScanner(in)

	for scanner.Scan() {
		line := scanner.Text()

		if isFields {
			c.cutFields(out, line, ranges, delimiter, onlyDelimited)
		} else {
			c.cutChars(out, line, ranges)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	return nil
}

// cutFields cuts by fields.
func (c *cutCommand) cutFields(out io.Writer, line string, ranges []cutRange, delimiter string, onlyDelimited bool) {
	fields := strings.Split(line, delimiter)

	// If line doesn't contain delimiter and -s is set, skip
	if len(fields) == 1 && onlyDelimited {
		return
	}

	var selected []string
	for _, r := range ranges {
		start := r.start
		end := r.end
		if end == -1 {
			end = len(fields)
		}

		for i := start; i <= end && i <= len(fields); i++ {
			selected = append(selected, fields[i-1]) // 1-based to 0-based
		}
	}

	fmt.Fprintln(out, strings.Join(selected, delimiter))
}

// cutChars cuts by characters.
func (c *cutCommand) cutChars(out io.Writer, line string, ranges []cutRange) {
	runes := []rune(line)
	var selected []rune

	for _, r := range ranges {
		start := r.start
		end := r.end
		if end == -1 {
			end = len(runes)
		}

		for i := start; i <= end && i <= len(runes); i++ {
			selected = append(selected, runes[i-1]) // 1-based to 0-based
		}
	}

	fmt.Fprintln(out, string(selected))
}
