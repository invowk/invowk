// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/cat"
)

// catCommand wraps the u-root cat implementation.
type catCommand struct {
	baseWrapper
}

// newCatCommand creates a new cat command wrapper.
func newCatCommand() *catCommand {
	return &catCommand{
		baseWrapper: baseWrapper{
			name: "cat",
			flags: []FlagInfo{
				{Name: "u", Description: "ignored (for compatibility)"},
			},
		},
	}
}

// Run executes the cat command.
func (c *catCommand) Run(ctx context.Context, args []string) error {
	cmd := cat.New()
	configureCommand(ctx, cmd)

	// args[0] is the command name, args[1:] are the actual arguments
	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}

	if err := cmd.RunContext(ctx, cmdArgs...); err != nil {
		return wrapError(c.name, err)
	}
	return nil
}
