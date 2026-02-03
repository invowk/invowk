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
)

// headCommand implements the head utility.
type headCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newHeadCommand())
}

// newHeadCommand creates a new head command.
func newHeadCommand() *headCommand {
	return &headCommand{
		name: "head",
		flags: []FlagInfo{
			{Name: "n", Description: "number of lines to output", TakesValue: true},
		},
	}
}

// Name returns the command name.
func (c *headCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *headCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the head command.
func (c *headCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("head", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	numLines := fs.Int("n", 10, "number of lines")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	files := fs.Args()
	if len(files) == 0 {
		// Read from stdin
		return c.processReader(hc.Stdout, hc.Stdin, *numLines)
	}

	// Process files
	for i, file := range files {
		if err := func() error {
			path := file
			if !filepath.IsAbs(path) {
				path = filepath.Join(hc.Dir, path)
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }() // Read-only file; close error non-critical

			// Print header for multiple files
			if len(files) > 1 {
				if i > 0 {
					fmt.Fprintln(hc.Stdout)
				}
				fmt.Fprintf(hc.Stdout, "==> %s <==\n", file)
			}

			return c.processReader(hc.Stdout, f, *numLines)
		}(); err != nil {
			return wrapError(c.name, err)
		}
	}

	return nil
}

// processReader outputs the first n lines from a reader using streaming I/O.
func (c *headCommand) processReader(out io.Writer, in io.Reader, n int) error {
	scanner := bufio.NewScanner(in)
	count := 0

	for scanner.Scan() {
		if count >= n {
			break
		}
		fmt.Fprintln(out, scanner.Text())
		count++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}
	return nil
}
