// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"fmt"
	"path"
)

// dirnameCommand implements the dirname utility.
// It extracts the directory portion from each given path.
type dirnameCommand struct {
	name  string
	flags []FlagInfo
}

// newDirnameCommand creates a new dirname command.
func newDirnameCommand() *dirnameCommand {
	return &dirnameCommand{
		name:  "dirname",
		flags: nil, // No flags, only positional args
	}
}

// Name returns the command name.
func (c *dirnameCommand) Name() string { return c.name }

// SupportedFlags returns the flags supported by this command.
func (c *dirnameCommand) SupportedFlags() []FlagInfo { return c.flags }

// Run executes the dirname command.
// Usage: dirname PATH [PATH...]
func (c *dirnameCommand) Run(ctx context.Context, args []string) error {
	hc := GetHandlerContext(ctx)

	posArgs := args[1:]
	if len(posArgs) == 0 {
		return wrapError(c.name, fmt.Errorf("missing operand"))
	}

	for _, p := range posArgs {
		fmt.Fprintln(hc.Stdout, path.Dir(p))
	}

	return nil
}
