// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/tar"
)

// tarCommand wraps the u-root tar implementation.
type tarCommand struct {
	baseWrapper
}

// newTarCommand creates a new tar command wrapper.
func newTarCommand() *tarCommand {
	return &tarCommand{
		name: "tar",
		flags: []FlagInfo{
			{Name: "create", ShortName: "c", Description: "create a new archive"},
			{Name: "extract", ShortName: "x", Description: "extract files from an archive"},
			{Name: "list", ShortName: "t", Description: "list the contents of an archive"},
			{Name: "file", ShortName: "f", Description: "use archive file", TakesValue: true},
			{Name: "verbose", ShortName: "v", Description: "verbosely list files processed"},
			{Name: "no-recursion", Description: "do not recurse into directories"},
		},
	}
}

// Run executes the tar command.
func (c *tarCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, tar.New(), args)
}
