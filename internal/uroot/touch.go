// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/touch"
)

// touchCommand wraps the u-root touch implementation.
type touchCommand struct {
	baseWrapper
}

func init() {
	RegisterDefault(newTouchCommand())
}

// newTouchCommand creates a new touch command wrapper.
func newTouchCommand() *touchCommand {
	return &touchCommand{
		baseWrapper: baseWrapper{
			name: "touch",
			flags: []FlagInfo{
				{Name: "c", ShortName: "c", Description: "do not create any files"},
				{Name: "a", Description: "change only access time"},
				{Name: "m", Description: "change only modification time"},
				{Name: "t", Description: "use specified time", TakesValue: true},
				{Name: "r", Description: "use reference file's time", TakesValue: true},
			},
		},
	}
}

// Run executes the touch command.
func (c *touchCommand) Run(ctx context.Context, args []string) error {
	cmd := touch.New()
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
