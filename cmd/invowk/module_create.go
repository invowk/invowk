// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"path/filepath"

	"invowk-cli/pkg/invkmod"

	"github.com/spf13/cobra"
)

// newModuleCreateCommand creates the `invowk module create` command.
func newModuleCreateCommand() *cobra.Command {
	var (
		createPath        string
		createScripts     bool
		createModule      string
		createDescription string
	)

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new invowk module",
		Long: `Create a new invowk module with the given name.

The module name must follow naming conventions:
  - Start with a letter
  - Contain only alphanumeric characters
  - Use dots to separate segments (RDNS style recommended)

Examples:
  invowk module create mycommands
  invowk module create com.example.mytools
  invowk module create mytools --scripts
  invowk module create mytools --path /path/to/dir --module-id "com.example.tools"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleCreate(args, createPath, createScripts, createModule, createDescription)
		},
	}

	cmd.Flags().StringVarP(&createPath, "path", "p", "", "parent directory for the module (default: current directory)")
	cmd.Flags().BoolVar(&createScripts, "scripts", false, "create a scripts/ subdirectory")
	cmd.Flags().StringVarP(&createModule, "module-id", "g", "", "module identifier for invkmod.cue (default: module name)")
	cmd.Flags().StringVarP(&createDescription, "description", "d", "", "description for invkmod.cue")

	return cmd
}

func runModuleCreate(args []string, createPath string, createScripts bool, createModule, createDescription string) error {
	moduleName := args[0]

	// Validate module name first
	if err := invkmod.ValidateName(moduleName); err != nil {
		return err
	}

	fmt.Println(moduleTitleStyle.Render("Create Module"))

	// Create the module
	opts := invkmod.CreateOptions{
		Name:             moduleName,
		ParentDir:        createPath,
		Module:           createModule,
		Description:      createDescription,
		CreateScriptsDir: createScripts,
	}

	modulePath, err := invkmod.Create(opts)
	if err != nil {
		return fmt.Errorf("failed to create module: %w", err)
	}

	fmt.Printf("%s Module created successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(modulePath))
	fmt.Printf("%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(moduleName))

	if createScripts {
		fmt.Printf("%s Scripts directory created\n", moduleInfoIcon)
	}

	fmt.Println()
	fmt.Printf("%s Next steps:\n", moduleInfoIcon)
	fmt.Printf("   1. Edit %s to add your commands\n", modulePathStyle.Render(filepath.Join(modulePath, "invkfile.cue")))
	if createScripts {
		fmt.Printf("   2. Add script files to %s\n", modulePathStyle.Render(filepath.Join(modulePath, "scripts")))
	}
	fmt.Printf("   3. Run %s to validate\n", CmdStyle.Render("invowk module validate "+modulePath))

	return nil
}
