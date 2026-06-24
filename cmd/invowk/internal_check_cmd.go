// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// newInternalCheckCmdCommand creates `invowk internal check-cmd <name>`.
// Returns exit 0 if the command is discoverable, exit 1 otherwise.
// Used by runtime-level cmds dependency validation inside containers to verify
// that auto-provisioned commands are reachable.
func newInternalCheckCmdCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	return &cobra.Command{
		Use:    "check-cmd [name]",
		Short:  "Check if a command is discoverable",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdName := args[0]
			if app == nil || app.Discovery == nil {
				return &ExitError{Code: 1, Err: errors.New("discovery service unavailable")}
			}

			configPath := ""
			if rootFlags != nil {
				configPath = rootFlags.configPath
			}

			result, err := app.Discovery.DiscoverCommandSet(contextWithConfigPath(cmd.Context(), configPath))
			if err != nil {
				return &ExitError{Code: 1, Err: fmt.Errorf("discovery failed: %w", err)}
			}
			if result.Set == nil {
				return &ExitError{Code: 1, Err: fmt.Errorf("command %q not discoverable", cmdName)}
			}

			for _, c := range result.Set.Commands {
				if string(c.Name) == cmdName {
					return nil // Found — exit 0
				}
			}

			return &ExitError{Code: 1, Err: fmt.Errorf("command %q not discoverable", cmdName)}
		},
	}
}
