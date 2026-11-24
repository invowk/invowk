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

// errLnLinkNameIsDirectory refuses a link name that ends in a separator and
// names an existing directory: ln never replaces a directory.
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

	// A link name that still ends in a separator names a directory: the
	// no-follow validator refuses one that resolves to a file. ln never
	// replaces a directory, since -f would remove an empty one and the link
	// cannot be created at a path that ends in a separator, so it refuses
	// before anything is removed.
	if linkName != "" && os.IsPathSeparator(linkName[len(linkName)-1]) {
		if info, statErr := os.Stat(linkName); statErr == nil && info.IsDir() {
			return wrapError(c.name, fmt.Errorf("%s: %w", linkName, errLnLinkNameIsDirectory))
		}
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
		if _, err := os.Lstat(linkName); err == nil {
			if removeErr := os.Remove(linkName); removeErr != nil {
				return wrapError(c.name, fmt.Errorf("cannot remove %q: %w", linkName, removeErr))
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return wrapError(c.name, fmt.Errorf("cannot stat %q: %w", linkName, err))
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
