// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/mv"
)

// mvCommand wraps the u-root mv implementation.
type mvCommand struct {
	baseWrapper
}

func init() {
	RegisterDefault(newMvCommand())
}

// newMvCommand creates a new mv command wrapper.
func newMvCommand() *mvCommand {
	return &mvCommand{
		baseWrapper: baseWrapper{
			name: "mv",
			flags: []FlagInfo{
				{Name: "f", ShortName: "f", Description: "do not prompt before overwriting"},
				{Name: "n", ShortName: "n", Description: "do not overwrite an existing file"},
			},
		},
	}
}

// Run executes the mv command.
func (c *mvCommand) Run(ctx context.Context, args []string) error {
	cmd := mv.New()
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
