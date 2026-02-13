// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// lnCommand implements the ln utility.
// It creates hard or symbolic links between files.
type lnCommand struct {
	name  string
	flags []FlagInfo
}

func init() {
	RegisterDefault(newLnCommand())
}

// newLnCommand creates a new ln command.
func newLnCommand() *lnCommand {
	return &lnCommand{
		name: "ln",
		flags: []FlagInfo{
			{Name: "s", Description: "make symbolic links instead of hard links"},
			{Name: "f", Description: "remove existing destination files"},
		},
	}
}

// Name returns the command name.
func (c *lnCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *lnCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the ln command.
// Usage: ln [-sf] TARGET LINK_NAME
func (c *lnCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	fs := flag.NewFlagSet("ln", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence unknown flag errors
	symbolic := fs.Bool("s", false, "symbolic link")
	force := fs.Bool("f", false, "force overwrite")

	// Parse known flags, ignore errors for unsupported flags
	_ = fs.Parse(args[1:]) //nolint:errcheck // Intentionally ignoring unsupported flags

	posArgs := fs.Args()
	if len(posArgs) < 2 {
		return wrapError(c.name, fmt.Errorf("missing file operand"))
	}

	target := posArgs[0]
	linkName := posArgs[1]

	// Resolve relative paths using the working directory
	if !filepath.IsAbs(target) {
		target = filepath.Join(hc.Dir, target)
	}
	if !filepath.IsAbs(linkName) {
		linkName = filepath.Join(hc.Dir, linkName)
	}

	// Remove existing link destination if -f is specified
	if *force {
		// Only remove if the path exists (ignore "not exist" errors)
		if _, err := os.Lstat(linkName); err == nil {
			if err := os.Remove(linkName); err != nil {
				return wrapError(c.name, fmt.Errorf("cannot remove %q: %w", linkName, err))
			}
		}
	}

	if *symbolic {
		if err := os.Symlink(target, linkName); err != nil {
			return wrapError(c.name, err)
		}
	} else {
		if err := os.Link(target, linkName); err != nil {
			return wrapError(c.name, err)
		}
	}

	return nil
}
