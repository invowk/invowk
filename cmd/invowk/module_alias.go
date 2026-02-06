// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"invowk-cli/internal/config"

	"github.com/spf13/cobra"
)

// newModuleAliasCommand creates the `invowk module alias` command tree.
// Alias operations load/save config and capture the App via closure.
func newModuleAliasCommand(app *App) *cobra.Command {
	aliasCmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage module aliases",
		Long: `Manage module aliases for collision disambiguation.

When two modules have the same 'module' identifier, you can use aliases to
give them different names. Aliases are stored in your invowk configuration.

Examples:
  invowk module alias set /path/to/module my-alias
  invowk module alias list
  invowk module alias remove /path/to/module`,
	}

	aliasCmd.AddCommand(&cobra.Command{
		Use:   "set <module-path> <alias>",
		Short: "Set an alias for a module",
		Long: `Set an alias for a module to resolve naming collisions.

The alias will be used as the module identifier instead of the module's
declared 'module' field when discovering commands.

Examples:
  invowk module alias set ./mymodule.invkmod my-tools
  invowk module alias set /absolute/path/mymodule.invkmod custom-name`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleAliasSet(cmd.Context(), app, args)
		},
	})

	aliasCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all module aliases",
		Long: `List all configured module aliases.

Shows a table of module paths and their assigned aliases.

Examples:
  invowk module alias list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleAliasList(cmd.Context(), app)
		},
	})

	aliasCmd.AddCommand(&cobra.Command{
		Use:   "remove <module-path>",
		Short: "Remove an alias for a module",
		Long: `Remove a previously configured alias for a module.

The module will revert to using its declared 'module' identifier.

Examples:
  invowk module alias remove ./mymodule.invkmod
  invowk module alias remove /absolute/path/mymodule.invkmod`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleAliasRemove(cmd.Context(), app, args)
		},
	})

	return aliasCmd
}

func runModuleAliasSet(ctx context.Context, app *App, args []string) error {
	modulePath := args[0]
	alias := args[1]

	fmt.Println(moduleTitleStyle.Render("Set Module Alias"))

	// Convert to absolute path
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Verify the path exists and is a valid module or invkfile
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Load current config via provider
	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize ModuleAliases map if nil
	if cfg.ModuleAliases == nil {
		cfg.ModuleAliases = make(map[string]string)
	}

	// Set the alias
	cfg.ModuleAliases[absPath] = alias

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Alias set successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path:  %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))
	fmt.Printf("%s Alias: %s\n", moduleInfoIcon, CmdStyle.Render(alias))

	return nil
}

func runModuleAliasList(ctx context.Context, app *App) error {
	fmt.Println(moduleTitleStyle.Render("Module Aliases"))

	// Load config via provider
	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.ModuleAliases) == 0 {
		fmt.Printf("%s No aliases configured\n", moduleWarningIcon)
		fmt.Println()
		fmt.Printf("%s To set an alias: %s\n", moduleInfoIcon, CmdStyle.Render("invowk module alias set <path> <alias>"))
		return nil
	}

	fmt.Printf("%s Found %d alias(es)\n", moduleInfoIcon, len(cfg.ModuleAliases))
	fmt.Println()

	for path, alias := range cfg.ModuleAliases {
		fmt.Printf("%s %s\n", moduleSuccessIcon, CmdStyle.Render(alias))
		fmt.Printf("   %s\n", moduleDetailStyle.Render(path))
	}

	return nil
}

func runModuleAliasRemove(ctx context.Context, app *App, args []string) error {
	modulePath := args[0]

	fmt.Println(moduleTitleStyle.Render("Remove Module Alias"))

	// Convert to absolute path
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load config via provider
	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.ModuleAliases == nil {
		return fmt.Errorf("no alias found for: %s", absPath)
	}

	// Check if alias exists
	alias, exists := cfg.ModuleAliases[absPath]
	if !exists {
		return fmt.Errorf("no alias found for: %s", absPath)
	}

	// Remove the alias
	delete(cfg.ModuleAliases, absPath)

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Alias removed successfully\n", moduleSuccessIcon)
	fmt.Println()
	fmt.Printf("%s Path:  %s\n", moduleInfoIcon, modulePathStyle.Render(absPath))
	fmt.Printf("%s Alias: %s (removed)\n", moduleInfoIcon, CmdStyle.Render(alias))

	return nil
}
