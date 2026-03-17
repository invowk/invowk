// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/ls"
)

// lsCommand wraps the u-root ls implementation.
type lsCommand struct {
	baseWrapper
}

// newLsCommand creates a new ls command wrapper.
func newLsCommand() *lsCommand {
	return &lsCommand{
		baseWrapper: baseWrapper{
			name: "ls",
			flags: []FlagInfo{
				{Name: "l", ShortName: "l", Description: "use a long listing format"},
				{Name: "a", ShortName: "a", Description: "include entries starting with ."},
				{Name: "R", Description: "list subdirectories recursively"},
				{Name: "h", ShortName: "h", Description: "print sizes in human readable format"},
				{Name: "Q", Description: "enclose entry names in double quotes"},
			},
		},
	}
}

// Run executes the ls command.
func (c *lsCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, ls.New(), args)
}
