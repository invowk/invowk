// Package config handles application configuration using Viper.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

// ContainerEngine specifies which container runtime to use
type ContainerEngine string

const (
	// ContainerEnginePodman uses Podman as the container runtime
	ContainerEnginePodman ContainerEngine = "podman"
	// ContainerEngineDocker uses Docker as the container runtime
	ContainerEngineDocker ContainerEngine = "docker"
)

// Config holds the application configuration
type Config struct {
	// ContainerEngine specifies whether to use "podman" or "docker"
	ContainerEngine ContainerEngine `toml:"container_engine" mapstructure:"container_engine"`
	// SearchPaths contains additional directories to search for invowkfiles
	SearchPaths []string `toml:"search_paths" mapstructure:"search_paths"`
	// DefaultRuntime sets the global default runtime mode
	DefaultRuntime string `toml:"default_runtime" mapstructure:"default_runtime"`
	// VirtualShell configures the virtual shell behavior
	VirtualShell VirtualShellConfig `toml:"virtual_shell" mapstructure:"virtual_shell"`
	// UI configures the user interface
	UI UIConfig `toml:"ui" mapstructure:"ui"`
}

// VirtualShellConfig configures the virtual shell runtime
type VirtualShellConfig struct {
	// EnableUrootUtils enables u-root utilities in virtual shell
	EnableUrootUtils bool `toml:"enable_uroot_utils" mapstructure:"enable_uroot_utils"`
}

// UIConfig configures the user interface
type UIConfig struct {
	// ColorScheme sets the color scheme ("auto", "dark", "light")
	ColorScheme string `toml:"color_scheme" mapstructure:"color_scheme"`
	// Verbose enables verbose output
	Verbose bool `toml:"verbose" mapstructure:"verbose"`
}

const (
	// AppName is the application name
	AppName = "invowk"
	// ConfigFileName is the name of the config file (without extension)
	ConfigFileName = "config"
	// ConfigFileExt is the config file extension
	ConfigFileExt = "toml"
)

var (
	// globalConfig holds the loaded configuration
	globalConfig *Config
	// configPath stores the path where config was loaded from
	configPath string
)

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ContainerEngine: ContainerEnginePodman,
		SearchPaths:     []string{},
		DefaultRuntime:  "native",
		VirtualShell: VirtualShellConfig{
			EnableUrootUtils: true,
		},
		UI: UIConfig{
			ColorScheme: "auto",
			Verbose:     false,
		},
	}
}

// ConfigDir returns the invowk configuration directory
func ConfigDir() (string, error) {
	var configDir string

	switch runtime.GOOS {
	case "windows":
		configDir = os.Getenv("APPDATA")
		if configDir == "" {
			configDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	return filepath.Join(configDir, AppName), nil
}

// CommandsDir returns the directory for user-defined invowkfiles
func CommandsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, ".invowk", "cmds"), nil
	default:
		return filepath.Join(home, ".invowk", "cmds"), nil
	}
}

// Load reads and parses the configuration file
func Load() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	v := viper.New()
	v.SetConfigName(ConfigFileName)
	v.SetConfigType(ConfigFileExt)

	// Get config directory
	cfgDir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	v.AddConfigPath(cfgDir)

	// Also search in current directory
	v.AddConfigPath(".")

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("container_engine", defaults.ContainerEngine)
	v.SetDefault("search_paths", defaults.SearchPaths)
	v.SetDefault("default_runtime", defaults.DefaultRuntime)
	v.SetDefault("virtual_shell.enable_uroot_utils", defaults.VirtualShell.EnableUrootUtils)
	v.SetDefault("ui.color_scheme", defaults.UI.ColorScheme)
	v.SetDefault("ui.verbose", defaults.UI.Verbose)

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
			globalConfig = defaults
			return globalConfig, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	configPath = v.ConfigFileUsed()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	globalConfig = &cfg
	return globalConfig, nil
}

// Get returns the currently loaded configuration
func Get() *Config {
	if globalConfig == nil {
		cfg, err := Load()
		if err != nil {
			return DefaultConfig()
		}
		return cfg
	}
	return globalConfig
}

// ConfigFilePath returns the path to the config file
func ConfigFilePath() string {
	return configPath
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	cfgDir, err := ConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(cfgDir, 0755)
}

// EnsureCommandsDir creates the commands directory if it doesn't exist
func EnsureCommandsDir() error {
	cmdsDir, err := CommandsDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(cmdsDir, 0755)
}

// CreateDefaultConfig creates a default config file if it doesn't exist
func CreateDefaultConfig() error {
	cfgDir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)

	// Check if file already exists
	if _, err := os.Stat(cfgPath); err == nil {
		return nil // File exists
	}

	defaults := DefaultConfig()
	data, err := toml.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}

	header := []byte(`# Invowk Configuration File
# This file configures the invowk command runner.
# See https://github.com/invowk/invowk for documentation.

`)

	if err := os.WriteFile(cfgPath, append(header, data...), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Save writes the current configuration to file
func Save(cfg *Config) error {
	cfgDir, err := ConfigDir()
	if err != nil {
		return err
	}

	cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)

	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	globalConfig = cfg
	return nil
}

// Reset clears the cached configuration
func Reset() {
	globalConfig = nil
	configPath = ""
}
