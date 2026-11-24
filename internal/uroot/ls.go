// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"
	"runtime"

	"github.com/u-root/u-root/pkg/core/ls"
)

// lsCommand wraps the u-root ls implementation.
type lsCommand struct {
	baseWrapper
}

// newLsCommand creates a new ls command wrapper.
func newLsCommand() *lsCommand {
	flags := []FlagInfo{
		{Name: "l", Description: "use a long listing format"},
		{Name: "a", Description: "include entries starting with ."},
		{Name: "R", Description: "list subdirectories recursively"},
		{Name: "h", Description: "print sizes in human readable format"},
		{Name: "Q", Description: "enclose entry names in double quotes"},
		{Name: "d", Description: "list directories themselves, not their contents"},
		{Name: "F", Description: "append an indicator to entries"},
		{Name: "S", Description: "sort by size"},
	}
	// u-root defines -p on Windows and Plan 9 only.
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		flags = append(flags, FlagInfo{Name: "p", Description: "print only the final path element of each file name"})
	}
	return &lsCommand{
		name:  "ls",
		flags: flags,
	}
}

// Run executes the ls command.
func (c *lsCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, ls.New(), args)
}
