// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"github.com/spf13/cobra"
)

// internalVHSCmd is the parent command for VHS-related internal subcommands.
// These are hidden commands used by the VHS test infrastructure.
var internalVHSCmd = &cobra.Command{
	Use:    "vhs",
	Short:  "VHS test utilities (internal use only)",
	Hidden: true,
}

func init() {
	internalCmd.AddCommand(internalVHSCmd)
}
