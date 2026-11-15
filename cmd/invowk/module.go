// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

const moduleCommandName = "module"

var (
	// Style definitions for module validation output
	moduleSuccessIcon = SuccessStyle.Render("✓")
	moduleErrorIcon   = ErrorStyle.Render("✗")
	moduleWarningIcon = WarningStyle.Render("!")
	moduleInfoIcon    = SubtitleStyle.Render("•")

	moduleTitleStyle = TitleStyle.MarginBottom(1)

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
func newModuleCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	modCmd := &cobra.Command{
		Use:     moduleCommandName,
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
  invowk module list
  invowk module create ./mycommands.invowkmod`,
	}

	// Core module commands
	modCmd.AddCommand(newModuleCreateCommand())
	modCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all discovered modules",
		Long: `List all invowk modules discovered in:
  - Current directory (skipped under --ivk-no-cwd / INVOWK_NO_CWD)
  - User commands directory (~/.invowk/cmds)
  - Configured includes

Examples:
  invowk module list`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, _, err := scopeDiscovery(cmd, app, rootFlags)
			if err != nil {
				return err
			}
			return runModuleList(ctx, app)
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
	modCmd.AddCommand(newModuleTidyCommand())

	return modCmd
}

func runModuleList(ctx context.Context, app *App) error {
	result, err := app.Discovery.DiscoverModules(ctx)
	if errors.Is(err, config.ErrExplicitConfig) {
		// Fail closed before printing anything: no module list from a broken
		// explicit config source.
		return renderExplicitConfigError(app.stderr, err)
	}
	w := app.stdout
	fmt.Fprintln(w, moduleTitleStyle.Render("Discovered Modules"))
	if err != nil {
		app.Diagnostics.Render(ctx, result.Diagnostics, app.stderr)
		return fmt.Errorf("failed to discover modules: %w", err)
	}
	app.Diagnostics.Render(ctx, result.Diagnostics, app.stderr)

	if len(result.Modules) == 0 {
		fmt.Fprintf(w, "%s No modules found\n", moduleWarningIcon)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "%s Modules are discovered in:\n", moduleInfoIcon)
		fmt.Fprintf(w, "   - Current directory\n")
		fmt.Fprintf(w, "   - User commands directory (~/.invowk/cmds)\n")
		fmt.Fprintf(w, "   - Configured includes\n")
		return nil
	}

	fmt.Fprintf(w, "%s Found %d module(s)\n", moduleInfoIcon, len(result.Modules))
	fmt.Fprintln(w)

	// Group by source
	bySource := make(map[discovery.Source][]*discovery.DiscoveredFile)
	for _, b := range result.Modules {
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

		fmt.Fprintf(w, "%s %s:\n", moduleInfoIcon, source.String())
		for _, p := range sourceModules {
			fmt.Fprintf(w, "   %s %s\n", moduleSuccessIcon, CmdStyle.Render(string(p.Module.Name())))
			fmt.Fprintf(w, "      %s\n", moduleDetailStyle.Render(string(p.Module.Path)))
		}
		fmt.Fprintln(w)
	}

	return nil
}
