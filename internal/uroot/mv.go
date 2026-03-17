// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/mv"
)

// mvCommand wraps the u-root mv implementation.
type mvCommand struct {
	baseWrapper
}

// newMvCommand creates a new mv command wrapper.
func newMvCommand() *mvCommand {
	return &mvCommand{
		baseWrapper: baseWrapper{
			name: "mv",
			flags: []FlagInfo{
				{Name: "f", ShortName: "f", Description: "do not prompt before overwriting"},
				{Name: "n", ShortName: "n", Description: "do not overwrite an existing file"},
			},
		},
	}
}

// Run executes the mv command.
func (c *mvCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, mv.New(), args)
}
