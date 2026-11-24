// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// errLnLinkNameIsDirectory refuses a link name that names an existing
// directory, in any form (`dir`, `dir/`, `dir/.`, `.`): ln never replaces a
// directory.
var errLnLinkNameIsDirectory = errors.New("cannot overwrite directory")

// lnCommand implements the ln utility.
// It creates hard or symbolic links between files.
type lnCommand struct {
	name  string
	flags []FlagInfo
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
		return wrapError(c.name, errors.New("missing file operand"))
	}

	target := posArgs[0]
	linkName := posArgs[1]

	// The link name is the entry ln creates or, with -f, replaces: it is
	// resolved without following its last component, so an existing link
	// there is replaced and never its target.
	resolvedLinkName, err := hc.ResolveNoFollowPath(linkName)
	if err != nil {
		return wrapError(c.name, err)
	}
	linkName = resolvedLinkName

	// ln never replaces a directory: -f would remove an empty one. The check
	// looks at the resolved entry itself (Lstat, not Stat), so it covers every
	// form that names a directory, including `dir/.` and `.`, whose resolved
	// form has no trailing separator, while a link to a directory named
	// without a separator is still replaced as a link. It runs before
	// anything is removed, and the -f removal below reuses its result.
	existing, lstatErr := os.Lstat(linkName)
	if lstatErr == nil && existing.IsDir() {
		return wrapError(c.name, fmt.Errorf("%s: %w", linkName, errLnLinkNameIsDirectory))
	}

	// For hard links, resolve the target to an absolute path. For symlinks,
	// preserve the user-provided target as-is because os.Symlink stores the
	// literal string — relative symlinks remain portable across directory
	// relocations and container mount points.
	if !*symbolic {
		resolvedTarget, err := hc.ResolvePath(target)
		if err != nil {
			return wrapError(c.name, err)
		}
		target = resolvedTarget
	}

	// Remove existing link destination if -f is specified
	if *force {
		if lstatErr == nil {
			if removeErr := os.Remove(linkName); removeErr != nil {
				return wrapError(c.name, fmt.Errorf("cannot remove %q: %w", linkName, removeErr))
			}
		} else if !errors.Is(lstatErr, os.ErrNotExist) {
			return wrapError(c.name, fmt.Errorf("cannot stat %q: %w", linkName, lstatErr))
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
