// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/find"
)

// findCommand wraps the u-root find implementation.
type findCommand struct {
	baseWrapper
}

// newFindCommand creates a new find command wrapper.
func newFindCommand() *findCommand {
	return &findCommand{
		baseWrapper: baseWrapper{
			name: "find",
			flags: []FlagInfo{
				{Name: "name", Description: "match file name pattern", TakesValue: true},
				{Name: "type", Description: "match file type (f, d, l)", TakesValue: true},
				{Name: "mode", Description: "match file mode", TakesValue: true},
				{Name: "l", Description: "long listing format"},
			},
		},
	}
}

// Run executes the find command.
func (c *findCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, find.New(), args)
}
