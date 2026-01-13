package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
	"invowk-cli/internal/issue"
)

// configCmd is the parent command for configuration operations
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage invowk configuration",
	Long: `Manage invowk configuration.

Configuration is stored in:
  - Linux: ~/.config/invowk/config.toml
  - macOS: ~/Library/Application Support/invowk/config.toml
  - Windows: %APPDATA%\invowk\config.toml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// configShowCmd displays current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showConfig()
	},
}

// configInitCmd creates a default configuration
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
}

// configPathCmd shows the configuration file path
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showConfigPath()
	},
}

// configSetCmd sets a configuration value
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setConfigValue(args[0], args[1])
	},
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configSetCmd)
}

func showConfig() error {
	cfg, err := config.Load()
	if err != nil {
		rendered, _ := issue.Get(issue.ConfigLoadFailedId).Render("dark")
		fmt.Fprint(os.Stderr, rendered)
		return err
	}

	// Style definitions
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))

	fmt.Println(headerStyle.Render("Current Configuration"))
	fmt.Println()

	// Show path
	cfgPath := config.ConfigFilePath()
	if cfgPath != "" {
		fmt.Printf("%s: %s\n", keyStyle.Render("Config file"), cfgPath)
	} else {
		fmt.Printf("%s: %s\n", keyStyle.Render("Config file"), subtitleStyle.Render("(using defaults)"))
	}
	fmt.Println()

	// Show values
	fmt.Printf("%s: %s\n", keyStyle.Render("container_engine"), valueStyle.Render(string(cfg.ContainerEngine)))
	fmt.Printf("%s: %s\n", keyStyle.Render("default_runtime"), valueStyle.Render(cfg.DefaultRuntime))

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("search_paths"))
	if len(cfg.SearchPaths) == 0 {
		fmt.Printf("  %s\n", subtitleStyle.Render("(none configured)"))
	} else {
		for _, path := range cfg.SearchPaths {
			fmt.Printf("  - %s\n", valueStyle.Render(path))
		}
	}

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("virtual_shell"))
	fmt.Printf("  enable_uroot_utils: %s\n", valueStyle.Render(fmt.Sprintf("%v", cfg.VirtualShell.EnableUrootUtils)))

	fmt.Println()
	fmt.Printf("%s:\n", keyStyle.Render("ui"))
	fmt.Printf("  color_scheme: %s\n", valueStyle.Render(cfg.UI.ColorScheme))
	fmt.Printf("  verbose: %s\n", valueStyle.Render(fmt.Sprintf("%v", cfg.UI.Verbose)))

	return nil
}

func initConfig() error {
	// Check if config exists
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	if err := config.CreateDefaultConfig(); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	fmt.Printf("%s Created default configuration at %s/config.toml\n", successStyle.Render("✓"), cfgDir)

	// Also create commands directory
	cmdsDir, err := config.CommandsDir()
	if err == nil {
		if err := config.EnsureCommandsDir(); err == nil {
			fmt.Printf("%s Created commands directory at %s\n", successStyle.Render("✓"), cmdsDir)
		}
	}

	return nil
}

func showConfigPath() error {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	fmt.Printf("Config directory: %s\n", cfgDir)
	fmt.Printf("Config file: %s/config.toml\n", cfgDir)

	cmdsDir, err := config.CommandsDir()
	if err == nil {
		fmt.Printf("Commands directory: %s\n", cmdsDir)
	}

	return nil
}

func setConfigValue(key, value string) error {
	cfg, err := config.Load()
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

	case "ui.color_scheme":
		cfg.UI.ColorScheme = value

	case "virtual_shell.enable_uroot_utils":
		cfg.VirtualShell.EnableUrootUtils = value == "true" || value == "1"

	default:
		return fmt.Errorf("unknown configuration key: %s\nValid keys: container_engine, default_runtime, ui.verbose, ui.color_scheme, virtual_shell.enable_uroot_utils", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("%s Set %s = %s\n", successStyle.Render("✓"), key, value)
	return nil
}

// configDumpCmd outputs raw configuration as TOML
var configDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Output raw configuration as TOML",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		data, err := toml.Marshal(cfg)
		if err != nil {
			return err
		}

		fmt.Print(string(data))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configDumpCmd)
}
