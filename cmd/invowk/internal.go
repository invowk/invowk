// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// internalCmd is the parent command for internal subcommands.
// These are hidden commands used for inter-process communication
// and subprocess execution patterns.
var internalCmd = &cobra.Command{
	Use:    "internal",
	Short:  "Internal commands (not for direct use)",
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(internalCmd)
}
