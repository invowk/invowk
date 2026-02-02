// SPDX-License-Identifier: MPL-2.0

package cmd

import "github.com/spf13/cobra"

var docsCmd = &cobra.Command{
	Use:    "docs",
	Short:  "Documentation tools",
	Long:   "Documentation utilities for audits, checks, and reference workflows.",
	Hidden: true,
}

func init() {
	docsCmd.AddCommand(docsAuditCmd)
	internalCmd.AddCommand(docsCmd)
}
