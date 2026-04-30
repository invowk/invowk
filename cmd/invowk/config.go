// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/types"
)

const (
	configFieldFmt  = "%s: %s\n"
	configFileLabel = "Config file"
)

// newConfigCommand creates the `invowk config` command tree.
// Subcommands that read configuration use the App's config.Provider.
func newConfigCommand(app *App) *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage invowk configuration",
		Long: `Manage invowk configuration.

Configuration is stored in:
  - Linux: ~/.config/invowk/config.cue
  - macOS: ~/Library/Application Support/invowk/config.cue
  - Windows: %APPDATA%\invowk\config.cue`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return showConfig(cmd.Context(), app)
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration file",
		RunE: func(_ *cobra.Command, _ []string) error {
			return initConfig()
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		RunE: func(_ *cobra.Command, _ []string) error {
			return showConfigPath()
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setConfigValue(cmd.Context(), app, args[0], args[1])
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "dump",
		Short: "Output raw configuration as CUE",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load(cmd.Context(), config.LoadOptions{})
			if err != nil {
				return err
			}

			cueContent := config.GenerateCUE(cfg)
			fmt.Print(cueContent)
			return nil
		},
	})

	return cfgCmd
}

func showConfig(ctx context.Context, app *App) error {
	result, err := app.Config.LoadWithSource(ctx, config.LoadOptions{})
	if err != nil {
		return renderConfigLoadFailure(err)
	}
	cfg := result.Config

	// Style definitions using shared color palette
	headerStyle := TitleStyle
	keyStyle := CmdStyle
	valueStyle := SuccessStyle

	fmt.Println(headerStyle.Render("Current Configuration"))
	fmt.Println()

	printConfigFileLine(keyStyle, result.SourcePath)
	fmt.Println()

	// Show values
	fmt.Printf(configFieldFmt, keyStyle.Render("container_engine"), valueStyle.Render(string(cfg.ContainerEngine)))
	fmt.Printf(configFieldFmt, keyStyle.Render("default_runtime"), valueStyle.Render(string(cfg.DefaultRuntime)))

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("includes"))
	printIncludes(cfg.Includes, valueStyle)

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("virtual_shell"))
	fmt.Printf("  enable_uroot_utils: %s\n", valueStyle.Render(strconv.FormatBool(cfg.VirtualShell.EnableUrootUtils)))

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("ui"))
	fmt.Printf("  color_scheme: %s\n", valueStyle.Render(string(cfg.UI.ColorScheme)))
	fmt.Printf("  interactive: %s\n", valueStyle.Render(strconv.FormatBool(cfg.UI.Interactive)))
	fmt.Printf("  verbose: %s\n", valueStyle.Render(strconv.FormatBool(cfg.UI.Verbose)))

	return nil
}

func renderConfigLoadFailure(err error) error {
	rendered, renderErr := renderIssueCatalogEntry(issue.Get(issue.ConfigLoadFailedId), "dark")
	if renderErr != nil {
		slog.Warn("failed to render issue catalog entry", "issue_id", issue.ConfigLoadFailedId, "error", renderErr)
	}
	fmt.Fprint(os.Stderr, rendered)
	return err
}

func printConfigFileLine(keyStyle lipgloss.Style, sourcePath types.FilesystemPath) {
	if sourcePath != "" {
		fmt.Printf(configFieldFmt, keyStyle.Render(configFileLabel), sourcePath)
		return
	}
	fmt.Printf(configFieldFmt, keyStyle.Render(configFileLabel), SubtitleStyle.Render("(using defaults)"))
}

func printIncludes(includes []config.IncludeEntry, valueStyle lipgloss.Style) {
	if len(includes) == 0 {
		fmt.Printf("  %s\n", SubtitleStyle.Render("(none configured)"))
		return
	}
	for _, inc := range includes {
		if inc.Alias != "" {
			fmt.Printf("  - %s (alias: %s)\n", valueStyle.Render(string(inc.Path)), valueStyle.Render(string(inc.Alias)))
			continue
		}
		fmt.Printf("  - %s\n", valueStyle.Render(string(inc.Path)))
	}
}

func initConfig() error {
	// Check if config exists
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	if err = config.CreateDefaultConfig(""); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	fmt.Printf("%s Created default configuration at %s/config.cue\n", SuccessStyle.Render("✓"), cfgDir)

	// Also create commands directory
	cmdsDir, err := config.CommandsDir()
	if err == nil {
		if mkdirErr := config.EnsureCommandsDir(""); mkdirErr != nil {
			slog.Warn("failed to create commands directory", "path", cmdsDir, "error", mkdirErr)
		} else {
			fmt.Printf("%s Created commands directory at %s\n", SuccessStyle.Render("✓"), cmdsDir)
		}
	} else {
		slog.Warn("failed to determine commands directory", "error", err)
	}

	return nil
}

func showConfigPath() error {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	fmt.Printf("Config directory: %s\n", cfgDir)
	fmt.Printf("Config file: %s/config.cue\n", cfgDir)

	cmdsDir, err := config.CommandsDir()
	if err == nil {
		fmt.Printf("Commands directory: %s\n", cmdsDir)
	}

	return nil
}

func setConfigValue(ctx context.Context, app *App, key, value string) error {
	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return err
	}

	switch key {
	case "container_engine":
		ce := config.ContainerEngine(value)
		if err := ce.Validate(); err != nil {
			return err
		}
		cfg.ContainerEngine = ce

	case "default_runtime":
		rm := config.RuntimeMode(value)
		if err := rm.Validate(); err != nil {
			return err
		}
		cfg.DefaultRuntime = rm

	case "ui.verbose":
		cfg.UI.Verbose = value == "true" || value == "1"

	case "ui.interactive":
		cfg.UI.Interactive = value == "true" || value == "1"

	case "ui.color_scheme":
		cs := config.ColorScheme(value)
		if err := cs.Validate(); err != nil {
			return err
		}
		cfg.UI.ColorScheme = cs

	case "virtual_shell.enable_uroot_utils":
		cfg.VirtualShell.EnableUrootUtils = value == "true" || value == "1"

	default:
		return fmt.Errorf("unknown configuration key: %s\nValid keys: container_engine, default_runtime, ui.verbose, ui.interactive, ui.color_scheme, virtual_shell.enable_uroot_utils", key)
	}

	if err := config.Save(cfg, ""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Set %s = %s\n", SuccessStyle.Render("✓"), key, value)
	return nil
}
