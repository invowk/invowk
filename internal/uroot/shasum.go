// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/shasum"
)

// shasumCommand wraps the u-root shasum implementation.
type shasumCommand struct {
	baseWrapper
}

func init() {
	RegisterDefault(newShasumCommand())
}

// newShasumCommand creates a new shasum command wrapper.
func newShasumCommand() *shasumCommand {
	return &shasumCommand{
		baseWrapper: baseWrapper{
			name: "shasum",
			flags: []FlagInfo{
				{Name: "a", Description: "hash algorithm (1, 256, 512)", TakesValue: true},
			},
		},
	}
}

// Run executes the shasum command.
func (c *shasumCommand) Run(ctx context.Context, args []string) error {
	cmd := shasum.New()
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
