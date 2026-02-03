// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type (
	// wcCommand implements the wc (word count) utility.
	wcCommand struct {
		name  string
		flags []FlagInfo
	}

	// wcCounts holds the counts for a file.
	wcCounts struct {
		lines int64
		words int64
		bytes int64
		chars int64
	}
)

func init() {
	RegisterDefault(newWcCommand())
}

// newWcCommand creates a new wc command.
func newWcCommand() *wcCommand {
	return &wcCommand{
		name: "wc",
		flags: []FlagInfo{
			{Name: "l", Description: "print line count"},
			{Name: "w", Description: "print word count"},
			{Name: "c", Description: "print byte count"},
			{Name: "m", Description: "print character count"},
		},
	}
}

// Name returns the command name.
func (c *wcCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *wcCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the wc command.
func (c *wcCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("wc", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	showLines := fs.Bool("l", false, "print line count")
	showWords := fs.Bool("w", false, "print word count")
	showBytes := fs.Bool("c", false, "print byte count")
	showChars := fs.Bool("m", false, "print character count")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	// If no flags specified, show lines, words, and bytes
	if !*showLines && !*showWords && !*showBytes && !*showChars {
		*showLines = true
		*showWords = true
		*showBytes = true
	}

	files := fs.Args()
	var total wcCounts
	var results []struct {
		counts wcCounts
		name   string
	}

	err := ProcessFilesOrStdin(files, hc.Stdin, hc.Dir, c.name,
		func(r io.Reader, filename string, _, _ int) error {
			counts, countErr := c.count(r)
			if countErr != nil {
				return countErr
			}

			// For stdin, use empty name
			name := filename
			if filename == "-" {
				name = ""
			}

			results = append(results, struct {
				counts wcCounts
				name   string
			}{counts, name})

			total.lines += counts.lines
			total.words += counts.words
			total.bytes += counts.bytes
			total.chars += counts.chars

			return nil
		})
	if err != nil {
		return err
	}

	// Print results
	for _, r := range results {
		c.printCounts(hc.Stdout, r.counts, r.name, *showLines, *showWords, *showBytes, *showChars)
	}

	// Print total if multiple files
	if len(files) > 1 {
		c.printCounts(hc.Stdout, total, "total", *showLines, *showWords, *showBytes, *showChars)
	}

	return nil
}

// count reads from r and returns the counts using streaming I/O.
func (c *wcCommand) count(r io.Reader) (wcCounts, error) {
	var counts wcCounts
	reader := bufio.NewReader(r)
	inWord := false

	for {
		ru, size, err := reader.ReadRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return counts, fmt.Errorf("reading input: %w", err)
		}

		counts.bytes += int64(size)
		counts.chars++

		if ru == '\n' {
			counts.lines++
		}

		if unicode.IsSpace(ru) {
			inWord = false
		} else if !inWord {
			inWord = true
			counts.words++
		}
	}

	return counts, nil
}

// printCounts formats and prints the counts.
func (c *wcCommand) printCounts(out io.Writer, counts wcCounts, name string, showLines, showWords, showBytes, showChars bool) {
	var parts []string

	if showLines {
		parts = append(parts, fmt.Sprintf("%7d", counts.lines))
	}
	if showWords {
		parts = append(parts, fmt.Sprintf("%7d", counts.words))
	}
	if showBytes {
		parts = append(parts, fmt.Sprintf("%7d", counts.bytes))
	}
	if showChars && !showBytes { // -m and -c are mutually exclusive, -c takes precedence
		parts = append(parts, fmt.Sprintf("%7d", counts.chars))
	}

	if name != "" {
		fmt.Fprintf(out, "%s %s\n", strings.Join(parts, " "), name)
	} else {
		fmt.Fprintf(out, "%s\n", strings.Join(parts, " "))
	}
}
