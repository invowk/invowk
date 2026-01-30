// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// tailCommand implements the tail utility.
type tailCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newTailCommand())
}

// newTailCommand creates a new tail command.
func newTailCommand() *tailCommand {
	return &tailCommand{
		name: "tail",
		flags: []FlagInfo{
			{Name: "n", Description: "number of lines to output (or +N to start from line N)", TakesValue: true},
		},
	}
}

// Name returns the command name.
func (c *tailCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *tailCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the tail command.
func (c *tailCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("tail", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	numLinesStr := fs.String("n", "10", "number of lines")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	// Parse numLines - handle +N syntax
	numLines, fromStart := parseLineSpec(*numLinesStr)

	files := fs.Args()
	if len(files) == 0 {
		// Read from stdin
		return c.processReader(hc.Stdout, hc.Stdin, numLines, fromStart)
	}

	// Process files
	for i, file := range files {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(hc.Dir, path)
		}

		f, err := os.Open(path)
		if err != nil {
			return wrapError(c.name, err)
		}

		// Print header for multiple files
		if len(files) > 1 {
			if i > 0 {
				fmt.Fprintln(hc.Stdout)
			}
			fmt.Fprintf(hc.Stdout, "==> %s <==\n", file)
		}

		if err := c.processReader(hc.Stdout, f, numLines, fromStart); err != nil {
			f.Close()
			return wrapError(c.name, err)
		}
		f.Close()
	}

	return nil
}

// parseLineSpec parses a line specification like "10" or "+5".
// Returns the number of lines and whether to start from that line number.
func parseLineSpec(s string) (int, bool) {
	if strings.HasPrefix(s, "+") {
		n := 0
		fmt.Sscanf(s[1:], "%d", &n)
		return n, true
	}
	n := 10
	fmt.Sscanf(s, "%d", &n)
	return n, false
}

// processReader outputs the last n lines from a reader using a ring buffer.
// If fromStart is true, it outputs starting from line n instead.
func (c *tailCommand) processReader(out io.Writer, in io.Reader, n int, fromStart bool) error {
	scanner := bufio.NewScanner(in)

	if fromStart {
		// Skip to line n, then output the rest
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if lineNum >= n {
				fmt.Fprintln(out, scanner.Text())
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		return nil
	}

	// Use ring buffer for last n lines
	if n <= 0 {
		return nil
	}

	lines := make([]string, n)
	idx := 0
	count := 0

	for scanner.Scan() {
		lines[idx] = scanner.Text()
		idx = (idx + 1) % n
		count++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	// Output the lines in order
	if count < n {
		// Less than n lines, output all from beginning
		for i := range count {
			fmt.Fprintln(out, lines[i])
		}
	} else {
		// Output from current position (oldest) around the ring
		for i := range n {
			fmt.Fprintln(out, lines[(idx+i)%n])
		}
	}

	return nil
}
