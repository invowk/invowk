// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/mkdir"
)

// mkdirCommand wraps the u-root mkdir implementation.
type mkdirCommand struct {
	baseWrapper
}

// newMkdirCommand creates a new mkdir command wrapper.
func newMkdirCommand() *mkdirCommand {
	return &mkdirCommand{
		baseWrapper: baseWrapper{
			name: "mkdir",
			flags: []FlagInfo{
				{Name: "p", Description: "create parent directories as needed"},
				{Name: "m", Description: "set file mode", TakesValue: true},
			},
		},
	}
}

// Run executes the mkdir command.
func (c *mkdirCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, mkdir.New(), args)
}
