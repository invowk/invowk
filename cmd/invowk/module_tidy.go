// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

// newModuleTidyCommand creates the `invowk module tidy` command.
func newModuleTidyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tidy",
		Short: "Add missing transitive dependencies to invowkmod.cue",
		Long: `Scan all resolved modules' dependencies and add any missing transitive
requirements to your invowkmod.cue.

This implements the Go-style explicit-only dependency model: every module in
the dependency tree must appear in YOUR invowkmod.cue. If a module you depend
on requires another module, you must declare it too.

After running tidy, run 'invowk module sync' to update the lock file.

Examples:
  invowk module tidy`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runModuleTidy(cmd.Context())
		},
	}
}

func runModuleTidy(ctx context.Context) error {
	fmt.Println(moduleTitleStyle.Render("Tidy Module Dependencies"))

	invowkmodulePath := filepath.Join(".", invowkmodCueFileName)
	requirements, missing, err := modulesync.TidyModule(ctx, types.FilesystemPath(invowkmodulePath)) //goplint:ignore -- relative path from current dir
	if err != nil {
		return err
	}

	if len(requirements) == 0 {
		fmt.Printf("%s No requires field found in invowkmod.cue — nothing to tidy\n", moduleInfoIcon)
		return nil
	}

	fmt.Printf("%s Found %d requirement(s) in invowkmod.cue\n", moduleInfoIcon, len(requirements))
	fmt.Printf("%s Resolving dependencies to find missing transitive requirements...\n", moduleInfoIcon)

	if len(missing) == 0 {
		fmt.Printf("%s All transitive dependencies are already declared — nothing to tidy\n", moduleSuccessIcon)
		return nil
	}

	for _, req := range missing {
		fmt.Printf("%s Added %s (%s)\n", moduleSuccessIcon,
			modulePathStyle.Render(string(req.GitURL)),
			CmdStyle.Render(string(req.Version)))
	}

	fmt.Println()
	fmt.Printf("%s Added %d missing transitive dependency(ies) to invowkmod.cue\n", moduleSuccessIcon, len(missing))
	fmt.Printf("%s Run %s to update the lock file\n", moduleInfoIcon, CmdStyle.Render("invowk module sync"))

	return nil
}
