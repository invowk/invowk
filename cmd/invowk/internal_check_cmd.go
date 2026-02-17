// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"

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
			cfg, err := provider.Load(context.Background(), config.LoadOptions{})
			if err != nil {
				// If config load fails, commands can't be discovered
				fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
				os.Exit(1)
			}

			disc := discovery.New(cfg)
			result, err := disc.DiscoverCommandSet(context.Background())
			if err != nil {
				fmt.Fprintf(os.Stderr, "discovery failed: %v\n", err)
				os.Exit(1)
			}

			for _, c := range result.Set.Commands {
				if c.Name == cmdName {
					return nil // Found — exit 0
				}
			}

			os.Exit(1)
			return nil // unreachable
		},
	}
}
