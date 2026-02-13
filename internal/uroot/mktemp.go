// SPDX-License-Identifier: MPL-2.0

package uroot

import (
	"context"

	"github.com/u-root/u-root/pkg/core/mktemp"
)

// mktempCommand wraps the u-root mktemp implementation.
type mktempCommand struct {
	baseWrapper
}

func init() {
	RegisterDefault(newMktempCommand())
}

// newMktempCommand creates a new mktemp command wrapper.
func newMktempCommand() *mktempCommand {
	return &mktempCommand{
		baseWrapper: baseWrapper{
			name: "mktemp",
			flags: []FlagInfo{
				{Name: "d", Description: "create a directory, not a file"},
				{Name: "p", Description: "use DIR as a prefix", TakesValue: true},
				{Name: "q", Description: "suppress diagnostics"},
			},
		},
	}
}

// Run executes the mktemp command.
func (c *mktempCommand) Run(ctx context.Context, args []string) error {
	cmd := mktemp.New()
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
