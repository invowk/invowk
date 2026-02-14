// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/issue"

	"github.com/spf13/cobra"
)

// newConfigCommand creates the `invowk config` command tree.
// Subcommands that read configuration use the App's ConfigProvider.
func newConfigCommand(app *App) *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage invowk configuration",
		Long: `Manage invowk configuration.

Configuration is stored in:
  - Linux: ~/.config/invowk/config.cue
  - macOS: ~/Library/Application Support/invowk/config.cue
  - Windows: %APPDATA%\invowk\config.cue`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return showConfig(cmd.Context(), app)
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		RunE: func(cmd *cobra.Command, args []string) error {
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
		RunE: func(cmd *cobra.Command, args []string) error {
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
	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		rendered, _ := issue.Get(issue.ConfigLoadFailedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return err
	}

	// Style definitions using shared color palette
	headerStyle := TitleStyle
	keyStyle := CmdStyle
	valueStyle := SuccessStyle

	fmt.Println(headerStyle.Render("Current Configuration"))
	fmt.Println()

	// Derive config file path from the standard config directory since the provider
	// does not cache resolved paths; each call derives from the standard config directory.
	cfgDir, dirErr := config.ConfigDir()
	if dirErr == nil {
		cfgPath := cfgDir + "/config.cue"
		if fileExistsCheck(cfgPath) {
			fmt.Printf("%s: %s\n", keyStyle.Render("Config file"), cfgPath)
		} else {
			fmt.Printf("%s: %s\n", keyStyle.Render("Config file"), SubtitleStyle.Render("(using defaults)"))
		}
	} else {
		fmt.Printf("%s: %s\n", keyStyle.Render("Config file"), SubtitleStyle.Render("(using defaults)"))
	}
	fmt.Println()

	// Show values
	fmt.Printf("%s: %s\n", keyStyle.Render("container_engine"), valueStyle.Render(string(cfg.ContainerEngine)))
	fmt.Printf("%s: %s\n", keyStyle.Render("default_runtime"), valueStyle.Render(cfg.DefaultRuntime))

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("includes"))
	if len(cfg.Includes) == 0 {
		fmt.Printf("  %s\n", SubtitleStyle.Render("(none configured)"))
	} else {
		for _, inc := range cfg.Includes {
			if inc.Alias != "" {
				fmt.Printf("  - %s (alias: %s)\n", valueStyle.Render(inc.Path), valueStyle.Render(inc.Alias))
			} else {
				fmt.Printf("  - %s\n", valueStyle.Render(inc.Path))
			}
		}
	}

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("virtual_shell"))
	fmt.Printf("  enable_uroot_utils: %s\n", valueStyle.Render(fmt.Sprintf("%v", cfg.VirtualShell.EnableUrootUtils)))

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("ui"))
	fmt.Printf("  color_scheme: %s\n", valueStyle.Render(cfg.UI.ColorScheme))
	fmt.Printf("  interactive: %s\n", valueStyle.Render(fmt.Sprintf("%v", cfg.UI.Interactive)))
	fmt.Printf("  verbose: %s\n", valueStyle.Render(fmt.Sprintf("%v", cfg.UI.Verbose)))

	return nil
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
		if value != "podman" && value != "docker" {
			return fmt.Errorf("invalid container_engine: must be 'podman' or 'docker'")
		}
		cfg.ContainerEngine = config.ContainerEngine(value)

	case "default_runtime":
		if value != "native" && value != "virtual" && value != "container" {
			return fmt.Errorf("invalid default_runtime: must be 'native', 'virtual', or 'container'")
		}
		cfg.DefaultRuntime = value

	case "ui.verbose":
		cfg.UI.Verbose = value == "true" || value == "1"

	case "ui.interactive":
		cfg.UI.Interactive = value == "true" || value == "1"

	case "ui.color_scheme":
		cfg.UI.ColorScheme = value

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

// fileExistsCheck checks if a file exists and is not a directory.
func fileExistsCheck(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}
