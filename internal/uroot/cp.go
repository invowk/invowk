// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/cp"
)

// cpCommand wraps the u-root cp implementation.
type cpCommand struct {
	baseWrapper
}

// newCpCommand creates a new cp command wrapper.
func newCpCommand() *cpCommand {
	return &cpCommand{
		name: "cp",
		flags: []FlagInfo{
			{Name: "recursive", ShortName: "r", Description: "copy directories recursively"},
			{Name: "RECURSIVE", ShortName: "R", Description: "same as -r"},
			{Name: "interactive", ShortName: "i", Description: "prompt before overwriting a file"},
			{Name: "force", ShortName: "f", Description: "force copy by removing destination file if needed"},
			{Name: "verbose", ShortName: "v", Description: "print each copied file"},
			{Name: "no-dereference", ShortName: "P", Description: "never follow symbolic links"},
		},
	}
}

// Run executes the cp command.
// Note: This uses u-root's streaming implementation which uses io.Copy internally,
// ensuring constant memory usage regardless of file size.
func (c *cpCommand) Run(ctx context.Context, args []string) error {
	return c.runUpstream(ctx, cp.New(), args)
}
