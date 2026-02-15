// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"regexp"
)

// grepCommand implements the grep utility.
type grepCommand struct {
	name  string
	flags []FlagInfo
}

// newGrepCommand creates a new grep command.
func newGrepCommand() *grepCommand {
	return &grepCommand{
		name: "grep",
		flags: []FlagInfo{
			{Name: "i", Description: "ignore case distinctions"},
			{Name: "v", Description: "invert match (select non-matching lines)"},
			{Name: "n", Description: "prefix each line with line number"},
			{Name: "c", Description: "print only a count of matching lines"},
			{Name: "l", Description: "print only names of files with matches"},
			{Name: "h", Description: "suppress file name prefix"},
			{Name: "H", Description: "print file name for each match"},
		},
	}
}

// Name returns the command name.
func (c *grepCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *grepCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the grep command.
func (c *grepCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("grep", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	ignoreCase := fs.Bool("i", false, "ignore case")
	invertMatch := fs.Bool("v", false, "invert match")
	showLineNumbers := fs.Bool("n", false, "show line numbers")
	countOnly := fs.Bool("c", false, "count only")
	filesWithMatches := fs.Bool("l", false, "files with matches only")
	noFilename := fs.Bool("h", false, "suppress filename")
	withFilename := fs.Bool("H", false, "print filename")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	remaining := fs.Args()
	if len(remaining) == 0 {
		return wrapError(c.name, fmt.Errorf("missing pattern"))
	}

	pattern := remaining[0]
	files := remaining[1:]

	// Compile regex
	if *ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return wrapError(c.name, fmt.Errorf("invalid pattern: %w", err))
	}

	// Determine if we should show filenames
	showFilename := len(files) > 1 || *withFilename
	if *noFilename {
		showFilename = false
	}

	matchFound := false

	err = ProcessFilesOrStdin(files, hc.Stdin, hc.Dir, c.name,
		func(r io.Reader, filename string, _, _ int) error {
			// For stdin, use empty string for filename display
			displayName := filename
			if filename == "-" {
				displayName = ""
			}

			count, found, processErr := c.grepReader(
				hc.Stdout, r, re, displayName,
				*invertMatch, *showLineNumbers, *countOnly, *filesWithMatches, showFilename,
			)
			if processErr != nil {
				return processErr
			}

			if found {
				matchFound = true
			}

			if *countOnly {
				if showFilename && displayName != "" {
					fmt.Fprintf(hc.Stdout, "%s:%d\n", displayName, count)
				} else {
					fmt.Fprintln(hc.Stdout, count)
				}
			}

			if *filesWithMatches && found && displayName != "" {
				fmt.Fprintln(hc.Stdout, displayName)
			}

			return nil
		})
	if err != nil {
		return err
	}

	// grep returns exit status 1 when no matches found
	if !matchFound {
		return wrapError(c.name, fmt.Errorf("no matches found"))
	}

	return nil
}

// grepReader searches for matches in a reader using streaming I/O.
// Returns the match count, whether any matches were found, and any error.
func (c *grepCommand) grepReader(out io.Writer, in io.Reader, re *regexp.Regexp, filename string, invertMatch, showLineNumbers, countOnly, filesWithMatches, showFilename bool) (matchCount int, found bool, err error) {
	scanner := bufio.NewScanner(in)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		matches := re.MatchString(line)

		if invertMatch {
			matches = !matches
		}

		if matches {
			matchCount++

			// Skip output if only counting or listing files
			if countOnly || filesWithMatches {
				continue
			}

			// Build output line
			var prefix string
			if showFilename && filename != "" {
				prefix = filename + ":"
			}
			if showLineNumbers {
				prefix += fmt.Sprintf("%d:", lineNum)
			}

			fmt.Fprintln(out, prefix+line)
		}
	}

	if err := scanner.Err(); err != nil {
		return matchCount, matchCount > 0, fmt.Errorf("reading input: %w", err)
	}
	return matchCount, matchCount > 0, nil
}
