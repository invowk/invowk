// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

// realpathCommand implements the realpath utility.
// It resolves each path argument to an absolute path, resolving symlinks.
type realpathCommand struct {
	name  string
	flags []FlagInfo
}

// newRealpathCommand creates a new realpath command.
func newRealpathCommand() *realpathCommand {
	return &realpathCommand{
		name:  urootCommandRealpath,
		flags: nil, // No flags, only positional args
	}
}

// Name returns the command name.
func (c *realpathCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *realpathCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the realpath command.
// Usage: realpath PATH [PATH...]
// Resolves symlinks and produces absolute paths.
func (c *realpathCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	posArgs := args[1:]
	if len(posArgs) == 0 {
		return wrapError(c.name, errors.New("missing operand"))
	}

	for _, p := range posArgs {
		path, err := hc.ResolvePath(p)
		if err != nil {
			return wrapError(c.name, err)
		}

		// Evaluate symlinks to resolve to the real path
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return wrapError(c.name, err)
		}

		// Ensure the result is absolute
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return wrapError(c.name, err)
		}

		// Convert to forward slashes for POSIX-consistent output in the virtual shell
		fmt.Fprintln(hc.Stdout, filepath.ToSlash(resolved))
	}

	return nil
}
