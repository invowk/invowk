// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
)

// uniqCommand implements the uniq utility.
type uniqCommand struct {
	name  string
	flags []FlagInfo
}

// newUniqCommand creates a new uniq command.
func newUniqCommand() *uniqCommand {
	return &uniqCommand{
		name: "uniq",
		flags: []FlagInfo{
			{Name: "c", Description: "prefix lines by the number of occurrences"},
			{Name: "d", Description: "only print duplicate lines"},
			{Name: "u", Description: "only print unique lines"},
			{Name: "i", Description: "ignore case when comparing"},
		},
	}
}

// Name returns the command name.
func (c *uniqCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *uniqCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the uniq command.
func (c *uniqCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("uniq", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	showCount := fs.Bool("c", false, "show count")
	duplicatesOnly := fs.Bool("d", false, "duplicates only")
	uniqueOnly := fs.Bool("u", false, "unique only")
	ignoreCase := fs.Bool("i", false, "ignore case")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	// uniq only processes the first file (or stdin)
	files := fs.Args()
	if len(files) > 1 {
		files = files[:1]
	}

	return ProcessFilesOrStdin(files, hc.Stdin, hc.Dir, c.name,
		func(r io.Reader, _ string, _, _ int) error {
			return c.processInput(hc.Stdout, r, *showCount, *duplicatesOnly, *uniqueOnly, *ignoreCase)
		})
}

// processInput processes input and writes unique adjacent lines to output.
func (c *uniqCommand) processInput(out io.Writer, in io.Reader, showCount, duplicatesOnly, uniqueOnly, ignoreCase bool) error {
	scanner := bufio.NewScanner(in)

	var prevLine string
	var prevOriginal string
	count := 0
	first := true

	outputLine := func(line string, cnt int) {
		if duplicatesOnly && cnt <= 1 {
			return
		}
		if uniqueOnly && cnt > 1 {
			return
		}

		if showCount {
			fmt.Fprintf(out, "%7d %s\n", cnt, line)
		} else {
			fmt.Fprintln(out, line)
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		compareLine := line
		if ignoreCase {
			compareLine = strings.ToLower(line)
		}

		if first {
			prevLine = compareLine
			prevOriginal = line
			count = 1
			first = false
			continue
		}

		if compareLine == prevLine {
			count++
		} else {
			outputLine(prevOriginal, count)
			prevLine = compareLine
			prevOriginal = line
			count = 1
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	// Output the last group
	if !first {
		outputLine(prevOriginal, count)
	}

	return nil
}
