// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// Style definitions for module validation output
	moduleSuccessIcon = SuccessStyle.Render("✓")
	moduleErrorIcon   = ErrorStyle.Render("✗")
	moduleWarningIcon = WarningStyle.Render("!")
	moduleInfoIcon    = SubtitleStyle.Render("•")

	moduleTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	moduleIssueStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				PaddingLeft(2)

	moduleIssueTypeStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

	modulePathStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight)

	moduleDetailStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				PaddingLeft(2)
)

// newModuleCommand creates the `invowk module` command tree.
// Subcommands that need config access capture the App via closure.
func newModuleCommand(app *App) *cobra.Command {
	modCmd := &cobra.Command{
		Use:     "module",
		Aliases: []string{"mod"},
		Short:   "Manage invowk modules",
		Long: `Manage invowk modules - self-contained folders containing invowkfiles and scripts.

A module is a folder with the ` + CmdStyle.Render(".invowkmod") + ` suffix that contains:
  - ` + CmdStyle.Render("invowkmod.cue") + ` (required): Module metadata (name, version, dependencies)
  - ` + CmdStyle.Render("invowkfile.cue") + ` (optional): Command definitions
  - Optional script files referenced by command implementations

Module names follow these rules:
  - Must start with a letter
  - Can contain alphanumeric characters with dot-separated segments
  - Compatible with RDNS naming (e.g., ` + CmdStyle.Render("com.example.mycommands.invowkmod") + `)
  - The folder prefix must match the 'module' field in invowkmod.cue

Examples:
  invowk module validate ./mycommands.invowkmod
  invowk module validate ./com.example.tools.invowkmod --deep`,
	}

	// Core module commands
	modCmd.AddCommand(newModuleValidateCommand())
	modCmd.AddCommand(newModuleCreateCommand())
	modCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all discovered modules",
		Long: `List all invowk modules discovered in:
  - Current directory
  - User commands directory (~/.invowk/cmds)
  - Configured includes

Examples:
  invowk module list`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleList(cmd.Context(), app)
		},
	})
	modCmd.AddCommand(newModuleArchiveCommand())
	modCmd.AddCommand(newModuleImportCommand())
	modCmd.AddCommand(newModuleVendorCommand())

	// Dependency management commands
	modCmd.AddCommand(newModuleAddCommand())
	modCmd.AddCommand(newModuleRemoveCommand())
	modCmd.AddCommand(newModuleSyncCommand())
	modCmd.AddCommand(newModuleUpdateCommand())
	modCmd.AddCommand(newModuleDepsCommand())

	return modCmd
}

func runModuleList(ctx context.Context, app *App) error {
	fmt.Println(moduleTitleStyle.Render("Discovered Modules"))

	// Load config via provider
	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create discovery instance
	disc := discovery.New(cfg)

	// Discover all files
	files, err := disc.DiscoverAll()
	if err != nil {
		return fmt.Errorf("failed to discover files: %w", err)
	}

	// Filter for modules only
	var modules []*discovery.DiscoveredFile
	for _, f := range files {
		if f.Module != nil {
			modules = append(modules, f)
		}
	}

	if len(modules) == 0 {
		fmt.Printf("%s No modules found\n", moduleWarningIcon)
		fmt.Println()
		fmt.Printf("%s Modules are discovered in:\n", moduleInfoIcon)
		fmt.Printf("   - Current directory\n")
		fmt.Printf("   - User commands directory (~/.invowk/cmds)\n")
		fmt.Printf("   - Configured includes\n")
		return nil
	}

	fmt.Printf("%s Found %d module(s)\n", moduleInfoIcon, len(modules))
	fmt.Println()

	// Group by source
	bySource := make(map[discovery.Source][]*discovery.DiscoveredFile)
	for _, b := range modules {
		bySource[b.Source] = append(bySource[b.Source], b)
	}

	// Display modules by source
	sources := []discovery.Source{
		discovery.SourceCurrentDir,
		discovery.SourceModule,
	}

	for _, source := range sources {
		sourceModules := bySource[source]
		if len(sourceModules) == 0 {
			continue
		}

		fmt.Printf("%s %s:\n", moduleInfoIcon, source.String())
		for _, p := range sourceModules {
			fmt.Printf("   %s %s\n", moduleSuccessIcon, CmdStyle.Render(p.Module.Name()))
			fmt.Printf("      %s\n", moduleDetailStyle.Render(p.Module.Path))
		}
		fmt.Println()
	}

	return nil
}
