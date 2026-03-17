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
	return c.runUpstream(ctx, cat.New(), args)
}
