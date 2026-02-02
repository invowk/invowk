// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"path/filepath"

	"invowk-cli/pkg/invkmod"

	"github.com/spf13/cobra"
)

var (
	// moduleCreatePath is the parent directory for module creation
	moduleCreatePath string
	// moduleCreateScripts creates a scripts directory in the module
	moduleCreateScripts bool
	// moduleCreateModule is the module identifier for the invkfile
	moduleCreateModule string
	// moduleCreateDescription is the description for the module
	moduleCreateDescription string

	// moduleCreateCmd creates a new module
	moduleCreateCmd = &cobra.Command{
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
		RunE: runModuleCreate,
	}
)

func initModuleCreateCmd() {
	moduleCreateCmd.Flags().StringVarP(&moduleCreatePath, "path", "p", "", "parent directory for the module (default: current directory)")
	moduleCreateCmd.Flags().BoolVar(&moduleCreateScripts, "scripts", false, "create a scripts/ subdirectory")
	moduleCreateCmd.Flags().StringVarP(&moduleCreateModule, "module-id", "g", "", "module identifier for invkmod.cue (default: module name)")
	moduleCreateCmd.Flags().StringVarP(&moduleCreateDescription, "description", "d", "", "description for invkmod.cue")
}

func runModuleCreate(cmd *cobra.Command, args []string) error {
	moduleName := args[0]

	// Validate module name first
	if err := invkmod.ValidateName(moduleName); err != nil {
		return err
	}

	fmt.Println(moduleTitleStyle.Render("Create Module"))

	// Create the module
	opts := invkmod.CreateOptions{
		Name:             moduleName,
		ParentDir:        moduleCreatePath,
		Module:           moduleCreateModule,
		Description:      moduleCreateDescription,
		CreateScriptsDir: moduleCreateScripts,
	}

	modulePath, err := invkmod.Create(opts)
	if err != nil {
		return fmt.Errorf("failed to create module: %w", err)
	}

	fmt.Printf("%s Module created successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path: %s\n", moduleInfoIcon, modulePathStyle.Render(modulePath))
	fmt.Printf("%s Name: %s\n", moduleInfoIcon, CmdStyle.Render(moduleName))

	if moduleCreateScripts {
		fmt.Printf("%s Scripts directory created\n", moduleInfoIcon)
	}

	fmt.Println()
	fmt.Printf("%s Next steps:\n", moduleInfoIcon)
	fmt.Printf("   1. Edit %s to add your commands\n", modulePathStyle.Render(filepath.Join(modulePath, "invkfile.cue")))
	if moduleCreateScripts {
		fmt.Printf("   2. Add script files to %s\n", modulePathStyle.Render(filepath.Join(modulePath, "scripts")))
	}
	fmt.Printf("   3. Run %s to validate\n", CmdStyle.Render("invowk module validate "+modulePath))

	return nil
}
