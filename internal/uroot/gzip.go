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
		name: "gzip",
		flags: []FlagInfo{
			{Name: "b", Description: "compression block size in KiB", TakesValue: true},
			{Name: "d", Description: "decompress"},
			{Name: "f", Description: "force overwrite"},
			{Name: "h", Description: "display help"},
			{Name: "k", Description: "keep the input file"},
			{Name: "p", Description: "number of compression threads", TakesValue: true},
			{Name: "q", Description: "suppress warnings"},
			{Name: "c", Description: "write to stdout"},
			{Name: "S", Description: "suffix of compressed files", TakesValue: true},
			{Name: "t", Description: "test the compressed input"},
			{Name: "v", Description: "verbose"},
			{Name: "1", Description: "compression level 1"},
			{Name: "2", Description: "compression level 2"},
			{Name: "3", Description: "compression level 3"},
			{Name: "4", Description: "compression level 4"},
			{Name: "5", Description: "compression level 5"},
			{Name: "6", Description: "compression level 6"},
			{Name: "7", Description: "compression level 7"},
			{Name: "8", Description: "compression level 8"},
			{Name: "9", Description: "compression level 9"},
		},
	}
}

// Run executes the gzip command. gzip.New takes the program name for
// gunzip/gzcat symlink detection (since u-root v0.16.0).
func (c *gzipCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, gzip.New(c.name), args)
}
