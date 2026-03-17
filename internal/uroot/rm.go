// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/rm"
)

// rmCommand wraps the u-root rm implementation.
type rmCommand struct {
	baseWrapper
}

// newRmCommand creates a new rm command wrapper.
func newRmCommand() *rmCommand {
	return &rmCommand{
		baseWrapper: baseWrapper{
			name: "rm",
			flags: []FlagInfo{
				{Name: "r", ShortName: "r", Description: "remove directories and their contents recursively"},
				{Name: "R", Description: "same as -r"},
				{Name: "f", ShortName: "f", Description: "ignore nonexistent files, never prompt"},
			},
		},
	}
}

// Run executes the rm command.
func (c *rmCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, rm.New(), args)
}
