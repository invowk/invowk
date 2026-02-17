// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
)

// newInternalCheckCmdCommand creates `invowk internal check-cmd <name>`.
// Returns exit 0 if the command is discoverable, exit 1 otherwise.
// Used by runtime-level cmds dependency validation inside containers to verify
// that auto-provisioned commands are reachable.
func newInternalCheckCmdCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "check-cmd [name]",
		Short:  "Check if a command is discoverable",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdName := args[0]

			provider := config.NewProvider()
			cfg, err := provider.Load(cmd.Context(), config.LoadOptions{})
			if err != nil {
				return &ExitError{Code: 1, Err: fmt.Errorf("config load failed: %w", err)}
			}

			disc := discovery.New(cfg)
			result, err := disc.DiscoverCommandSet(cmd.Context())
			if err != nil {
				return &ExitError{Code: 1, Err: fmt.Errorf("discovery failed: %w", err)}
			}

			for _, c := range result.Set.Commands {
				if c.Name == cmdName {
					return nil // Found — exit 0
				}
			}

			return &ExitError{Code: 1, Err: fmt.Errorf("command %q not discoverable", cmdName)}
		},
	}
}
