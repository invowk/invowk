// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/shasum"
)

// shasumCommand wraps the u-root shasum implementation.
type shasumCommand struct {
	baseWrapper
}

// newShasumCommand creates a new shasum command wrapper.
func newShasumCommand() *shasumCommand {
	return &shasumCommand{
		baseWrapper: baseWrapper{
			name: "shasum",
			flags: []FlagInfo{
				{Name: "a", Description: "hash algorithm (1, 256, 512)", TakesValue: true},
			},
		},
	}
}

// Run executes the shasum command.
func (c *shasumCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, shasum.New(), args)
}
