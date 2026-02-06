// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// newInternalCommand creates the `invowk internal` command tree.
// These are hidden commands used for inter-process communication
// and subprocess execution patterns.
func newInternalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "internal",
		Short:  "Internal commands (not for direct use)",
		Hidden: true,
	}

	cmd.AddCommand(newInternalExecVirtualCommand())

	return cmd
}
