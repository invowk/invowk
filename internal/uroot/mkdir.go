// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/mkdir"
)

// mkdirCommand wraps the u-root mkdir implementation.
type mkdirCommand struct {
	baseWrapper
}

func init() {
	RegisterDefault(newMkdirCommand())
}

// newMkdirCommand creates a new mkdir command wrapper.
func newMkdirCommand() *mkdirCommand {
	return &mkdirCommand{
		baseWrapper: baseWrapper{
			name: "mkdir",
			flags: []FlagInfo{
				{Name: "p", Description: "create parent directories as needed"},
				{Name: "m", Description: "set file mode", TakesValue: true},
			},
		},
	}
}

// Run executes the mkdir command.
func (c *mkdirCommand) Run(ctx context.Context, args []string) error {
	cmd := mkdir.New()
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
