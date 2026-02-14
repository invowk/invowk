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

// Run executes the gzip command.
// Note: gzip's upstream RunContext expects args[0] to be the program name
// (used for gunzip/gzcat symlink detection), so we pass the full args slice
// rather than stripping the command name like other wrappers. The embedded
// baseWrapper provides the NativePreprocessor marker, so Registry.Run()
// skips centralized flag preprocessing.
func (c *gzipCommand) Run(ctx context.Context, args []string) error {
	cmd := gzip.New()
	configureCommand(ctx, cmd)

	if err := cmd.RunContext(ctx, args...); err != nil {
		return wrapError(c.name, err)
	}
	return nil
}
