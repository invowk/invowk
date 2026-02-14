// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"
)

// teeCommand implements the tee utility.
// It reads from stdin and writes to both stdout and each named file.
type teeCommand struct {
	name  string
	flags []FlagInfo
}

// newTeeCommand creates a new tee command.
func newTeeCommand() *teeCommand {
	return &teeCommand{
		name: "tee",
		flags: []FlagInfo{
			{Name: "a", Description: "append to the given files, do not overwrite"},
		},
	}
}

// Name returns the command name.
func (c *teeCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *teeCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the tee command.
// Usage: tee [-a] [FILE...]
// Reads stdin and writes to stdout and each FILE. With -a, appends instead of overwriting.
func (c *teeCommand) Run(ctx context.Context, args []string) (err error) {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("tee", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	appendMode := fs.Bool("a", false, "append")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	fileArgs := fs.Args()

	// Determine file open flags based on append mode
	openFlags := os.O_CREATE | os.O_WRONLY
	if *appendMode {
		openFlags |= os.O_APPEND
	} else {
		openFlags |= os.O_TRUNC
	}

	// Open all output files, collecting writers for MultiWriter
	writers := []io.Writer{hc.Stdout}
	var files []*os.File

	for _, name := range fileArgs {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(hc.Dir, path)
		}

		f, openErr := os.OpenFile(path, openFlags, 0o644)
		if openErr != nil {
			// Close any files already opened before returning
			for _, opened := range files {
				_ = opened.Close() //nolint:errcheck // Best-effort cleanup on open failure
			}
			return wrapError(c.name, openErr)
		}
		files = append(files, f)
		writers = append(writers, f)
	}

	// Ensure all files are closed, aggregating close errors via named return
	defer func() {
		for _, f := range files {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = wrapError(c.name, closeErr)
			}
		}
	}()

	mw := io.MultiWriter(writers...)
	if _, copyErr := io.Copy(mw, hc.Stdin); copyErr != nil {
		return wrapError(c.name, copyErr)
	}

	return nil
}
