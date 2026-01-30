// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	// Style definitions for module validation output
	moduleSuccessIcon = successStyle.Render("✓")
	moduleErrorIcon   = errorStyle.Render("✗")
	moduleWarningIcon = warningStyle.Render("!")
	moduleInfoIcon    = subtitleStyle.Render("•")

	moduleTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7C3AED")).
				MarginBottom(1)

	moduleIssueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#EF4444")).
				PaddingLeft(2)

	moduleIssueTypeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				Italic(true)

	modulePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))

	moduleDetailStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6B7280")).
				PaddingLeft(2)

	// moduleCmd represents the module command group
	moduleCmd = &cobra.Command{
		Use:     "module",
		Aliases: []string{"mod"},
		Short:   "Manage invowk modules",
		Long: `Manage invowk modules - self-contained folders containing invkfiles and scripts.

A module is a folder with the ` + cmdStyle.Render(".invkmod") + ` suffix that contains:
  - ` + cmdStyle.Render("invkmod.cue") + ` (required): Module metadata (name, version, dependencies)
  - ` + cmdStyle.Render("invkfile.cue") + ` (optional): Command definitions
  - Optional script files referenced by command implementations

Module names follow these rules:
  - Must start with a letter
  - Can contain alphanumeric characters with dot-separated segments
  - Compatible with RDNS naming (e.g., ` + cmdStyle.Render("com.example.mycommands.invkmod") + `)
  - The folder prefix must match the 'module' field in invkmod.cue

Examples:
  invowk module validate ./mycommands.invkmod
  invowk module validate ./com.example.tools.invkmod --deep`,
	}

	// moduleListCmd lists all discovered modules
	moduleListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all discovered modules",
		Long: `List all invowk modules discovered in:
  - Current directory
  - User commands directory (~/.invowk/cmds)
  - Configured search paths

Examples:
  invowk module list`,
		RunE: runModuleList,
	}
)

func init() {
	// Core module commands
	moduleCmd.AddCommand(moduleValidateCmd)
	moduleCmd.AddCommand(moduleCreateCmd)
	moduleCmd.AddCommand(moduleListCmd)
	moduleCmd.AddCommand(moduleArchiveCmd)
	moduleCmd.AddCommand(moduleImportCmd)
	moduleCmd.AddCommand(moduleAliasCmd)
	moduleCmd.AddCommand(moduleVendorCmd)

	// Dependency management commands
	moduleCmd.AddCommand(moduleAddCmd)
	moduleCmd.AddCommand(moduleRemoveCmd)
	moduleCmd.AddCommand(moduleSyncCmd)
	moduleCmd.AddCommand(moduleUpdateCmd)
	moduleCmd.AddCommand(moduleDepsCmd)

	// Initialize subcommand flags
	initModuleValidateCmd()
	initModuleCreateCmd()
	initModuleAliasCmd()
	initModulePackageCmd()
	initModuleDepsCmd()
}

func runModuleList(cmd *cobra.Command, args []string) error {
	fmt.Println(moduleTitleStyle.Render("Discovered Modules"))

	// Load config
	cfg, err := config.Load()
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
		fmt.Printf("   - Configured search paths\n")
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
		discovery.SourceUserDir,
		discovery.SourceConfigPath,
		discovery.SourceModule,
	}

	for _, source := range sources {
		sourceModules := bySource[source]
		if len(sourceModules) == 0 {
			continue
		}

		fmt.Printf("%s %s:\n", moduleInfoIcon, source.String())
		for _, p := range sourceModules {
			fmt.Printf("   %s %s\n", moduleSuccessIcon, cmdStyle.Render(p.Module.Name()))
			fmt.Printf("      %s\n", moduleDetailStyle.Render(p.Module.Path))
		}
		fmt.Println()
	}

	return nil
}
