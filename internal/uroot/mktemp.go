// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// mktempCommand implements the mktemp utility for creating temporary files
// or directories. This is a custom implementation that uses os.TempDir() for
// cross-platform correctness, replacing the upstream u-root wrapper which
// hardcodes "/tmp".
type mktempCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newMktempCommand())
}

// newMktempCommand creates a new mktemp command.
func newMktempCommand() *mktempCommand {
	return &mktempCommand{
		name: "mktemp",
		flags: []FlagInfo{
			{Name: "d", Description: "create a directory, not a file"},
			{Name: "p", Description: "use DIR as a prefix", TakesValue: true},
			{Name: "q", Description: "suppress diagnostics"},
		},
	}
}

// Name returns the command name.
func (c *mktempCommand) Name() string {
	return c.name
}

// SupportedFlags returns the flags supported by this command.
func (c *mktempCommand) SupportedFlags() []FlagInfo {
	return c.flags
}

// Run executes the mktemp command, creating a temporary file or directory
// and printing its path to stdout. Supports -d (directory), -p DIR (parent
// directory), and -q (quiet errors). The default parent directory is
// os.TempDir(), which returns the platform-correct temp location (/tmp on
// Unix, %TEMP% on Windows). An optional template argument specifies the
// filename prefix (trailing X characters are stripped).
func (c *mktempCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("mktemp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	isDir := fs.Bool("d", false, "create directory")
	parentDir := fs.String("p", "", "parent directory")
	quiet := fs.Bool("q", false, "suppress errors")
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	dir := *parentDir
	if dir == "" {
		// Respect TMPDIR from the virtual shell environment, falling back
		// to the OS default temp directory for cross-platform correctness.
		if tmpdir, ok := hc.LookupEnv("TMPDIR"); ok && tmpdir != "" {
			dir = tmpdir
		} else {
			dir = os.TempDir()
		}
	}

	// Resolve relative parent dir against the working directory.
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(hc.Dir, dir)
	}

	// Extract prefix from template argument. Trailing X characters are stripped
	// because Go's os.CreateTemp appends its own random suffix. This follows the
	// POSIX convention where X's mark the random portion (e.g., "tmp.XXXXXX" → prefix "tmp.").
	prefix := "tmp."
	if fs.NArg() > 0 {
		tmpl := fs.Arg(0)
		prefix = strings.TrimRight(tmpl, "X")
		if prefix == "" {
			prefix = "tmp."
		}
	}

	var path string
	var err error
	if *isDir {
		path, err = os.MkdirTemp(dir, prefix)
	} else {
		var f *os.File
		f, err = os.CreateTemp(dir, prefix)
		if err == nil {
			path = f.Name()
			_ = f.Close() // Nothing written; we only need the path, so close error is benign
		}
	}

	if err != nil {
		if *quiet {
			// POSIX mktemp -q: suppress diagnostics AND return success on failure.
			// The caller detects failure by checking for empty stdout output.
			return nil
		}
		return wrapError(c.name, err)
	}

	fmt.Fprintln(hc.Stdout, path)
	return nil
}
