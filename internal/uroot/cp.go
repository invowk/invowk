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
		baseWrapper: baseWrapper{
			name: "cp",
			flags: []FlagInfo{
				{Name: "r", ShortName: "r", Description: "copy directories recursively"},
				{Name: "R", Description: "same as -r"},
				{Name: "f", ShortName: "f", Description: "force copy by removing destination file if needed"},
				{Name: "n", ShortName: "n", Description: "do not overwrite an existing file"},
				{Name: "P", Description: "never follow symbolic links"},
			},
		},
	}
}

// Run executes the cp command.
// Note: This uses u-root's streaming implementation which uses io.Copy internally,
// ensuring constant memory usage regardless of file size.
func (c *cpCommand) Run(ctx context.Context, args []string) error {
	cmd := cp.New()
	configureCommand(ctx, cmd)

	// args[0] is the command name, args[1:] are the actual arguments
	var cmdArgs []string
	if len(args) > 1 {
		cmdArgs = args[1:]
	}

	if err := cmd.RunContext(ctx, cmdArgs...); err != nil {
		return wrapError(c.name, err)
	}
	return nil
}
