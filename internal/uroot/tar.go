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
		baseWrapper: baseWrapper{
			name: "tar",
			flags: []FlagInfo{
				{Name: "c", Description: "create a new archive"},
				{Name: "x", Description: "extract files from an archive"},
				{Name: "t", Description: "list the contents of an archive"},
				{Name: "f", Description: "use archive file", TakesValue: true},
				{Name: "v", Description: "verbosely list files processed"},
			},
		},
	}
}

// Run executes the tar command.
func (c *tarCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, tar.New(), args)
}
