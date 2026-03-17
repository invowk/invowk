// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/gzip"
)

// gzipCommand wraps the u-root gzip implementation.
type gzipCommand struct {
	baseWrapper
}

// newGzipCommand creates a new gzip command wrapper.
func newGzipCommand() *gzipCommand {
	return &gzipCommand{
		baseWrapper: baseWrapper{
			name: "gzip",
			flags: []FlagInfo{
				{Name: "d", Description: "decompress"},
				{Name: "c", Description: "write to stdout"},
				{Name: "f", Description: "force overwrite"},
				{Name: "v", Description: "verbose"},
				{Name: "q", Description: "suppress warnings"},
			},
		},
	}
}

// Run executes the gzip command. gzip.New takes the program name for
// gunzip/gzcat symlink detection (since u-root v0.16.0).
func (c *gzipCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, gzip.New(c.name), args)
}
