// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/chmod"
)

// chmodCommand wraps the u-root chmod implementation.
type chmodCommand struct {
	baseWrapper
}

func init() {
	RegisterDefault(newChmodCommand())
}

// newChmodCommand creates a new chmod command wrapper.
func newChmodCommand() *chmodCommand {
	return &chmodCommand{
		baseWrapper: baseWrapper{
			name: "chmod",
			flags: []FlagInfo{
				{Name: "recursive", Description: "change files and directories recursively"},
				{Name: "reference", Description: "use RFILE's mode instead of MODE values", TakesValue: true},
			},
		},
	}
}

// Run executes the chmod command.
func (c *chmodCommand) Run(ctx context.Context, args []string) error {
	cmd := chmod.New()
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
