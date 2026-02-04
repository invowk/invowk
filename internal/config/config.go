// SPDX-License-Identifier: MPL-2.0

// Package config handles application configuration using Viper with CUE as the file format.
package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"invowk-cli/internal/cueutil"
	"invowk-cli/internal/issue"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/spf13/viper"
)

const (
	// ContainerEnginePodman uses Podman as the container runtime.
	ContainerEnginePodman ContainerEngine = "podman"
	// ContainerEngineDocker uses Docker as the container runtime.
	ContainerEngineDocker ContainerEngine = "docker"

	// AppName is the application name.
	AppName = "invowk"
	// ConfigFileName is the name of the config file (without extension).
	ConfigFileName = "config"
	// ConfigFileExt is the config file extension.
	ConfigFileExt = "cue"
)

var (
	//go:embed config_schema.cue
	configSchema string

	// globalConfig holds the loaded configuration.
	globalConfig *Config
	// configPath stores the path where config was loaded from.
	configPath string
	// configDirOverride allows tests to override the config directory.
	// This is necessary because os.UserHomeDir() doesn't reliably respect
	// the HOME environment variable on all platforms (e.g., macOS in CI).
	configDirOverride string
	// configFilePathOverride allows specifying a custom config file path.
	// This is used by the --config CLI flag to load config from a specific file.
	configFilePathOverride string
	// errLastLoad stores the last error from Load() for later retrieval.
	// This allows Get() to return defaults while preserving error information.
	errLastLoad error
)

type (
	// ContainerEngine specifies which container runtime to use.
	ContainerEngine string

	// Config holds the application configuration.
	Config struct {
		// ContainerEngine specifies whether to use "podman" or "docker"
		ContainerEngine ContainerEngine `json:"container_engine" mapstructure:"container_engine"`
		// SearchPaths contains additional directories to search for invkfiles
		SearchPaths []string `json:"search_paths" mapstructure:"search_paths"`
		// DefaultRuntime sets the global default runtime mode
		DefaultRuntime string `json:"default_runtime" mapstructure:"default_runtime"`
		// VirtualShell configures the virtual shell behavior
		VirtualShell VirtualShellConfig `json:"virtual_shell" mapstructure:"virtual_shell"`
		// UI configures the user interface
		UI UIConfig `json:"ui" mapstructure:"ui"`
		// Container configures container runtime behavior
		Container ContainerConfig `json:"container" mapstructure:"container"`
		// ModuleAliases maps module paths to alias names for collision disambiguation
		ModuleAliases map[string]string `json:"module_aliases" mapstructure:"module_aliases"`
	}

	// ContainerConfig configures container runtime behavior.
	ContainerConfig struct {
		// AutoProvision configures automatic provisioning of invowk resources
		AutoProvision AutoProvisionConfig `json:"auto_provision" mapstructure:"auto_provision"`
	}

	// AutoProvisionConfig controls auto-provisioning of invowk resources into containers.
	AutoProvisionConfig struct {
		// Enabled enables/disables auto-provisioning (default: true)
		Enabled bool `json:"enabled" mapstructure:"enabled"`
		// BinaryPath overrides the path to the invowk binary to provision
		BinaryPath string `json:"binary_path" mapstructure:"binary_path"`
		// ModulesPaths specifies additional directories to search for modules
		ModulesPaths []string `json:"modules_paths" mapstructure:"modules_paths"`
		// CacheDir specifies where to store cached provisioned images metadata
		CacheDir string `json:"cache_dir" mapstructure:"cache_dir"`
	}

	// VirtualShellConfig configures the virtual shell runtime.
	VirtualShellConfig struct {
		// EnableUrootUtils enables u-root utilities in virtual shell
		EnableUrootUtils bool `json:"enable_uroot_utils" mapstructure:"enable_uroot_utils"`
	}

	// UIConfig configures the user interface.
	UIConfig struct {
		// ColorScheme sets the color scheme ("auto", "dark", "light")
		ColorScheme string `json:"color_scheme" mapstructure:"color_scheme"`
		// Verbose enables verbose output
		Verbose bool `json:"verbose" mapstructure:"verbose"`
		// Interactive enables alternate screen buffer mode for command execution
		Interactive bool `json:"interactive" mapstructure:"interactive"`
	}
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
			Interactive: false,
		},
		Container: ContainerConfig{
			AutoProvision: AutoProvisionConfig{
				Enabled:      true,
				BinaryPath:   "", // Will use os.Executable() if empty
				ModulesPaths: []string{},
				CacheDir:     "", // Will use default cache dir if empty
			},
		},
	}
}

// ConfigDir returns the invowk configuration directory.
//
//nolint:revive // ConfigDir is more descriptive than Dir for external callers
func ConfigDir() (string, error) {
	// Allow tests to override the config directory
	if configDirOverride != "" {
		return configDirOverride, nil
	}

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

// CommandsDir returns the directory for user-defined invkfiles
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

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("container_engine", defaults.ContainerEngine)
	v.SetDefault("search_paths", defaults.SearchPaths)
	v.SetDefault("default_runtime", defaults.DefaultRuntime)
	v.SetDefault("virtual_shell.enable_uroot_utils", defaults.VirtualShell.EnableUrootUtils)
	v.SetDefault("ui.color_scheme", defaults.UI.ColorScheme)
	v.SetDefault("ui.verbose", defaults.UI.Verbose)
	v.SetDefault("ui.interactive", defaults.UI.Interactive)
	v.SetDefault("container.auto_provision.enabled", defaults.Container.AutoProvision.Enabled)
	v.SetDefault("container.auto_provision.binary_path", defaults.Container.AutoProvision.BinaryPath)
	v.SetDefault("container.auto_provision.modules_paths", defaults.Container.AutoProvision.ModulesPaths)
	v.SetDefault("container.auto_provision.cache_dir", defaults.Container.AutoProvision.CacheDir)

	// If a custom config file path is set via --config flag, use it exclusively
	if configFilePathOverride != "" {
		if !fileExists(configFilePathOverride) {
			return nil, issue.NewErrorContext().
				WithOperation("load configuration").
				WithResource(configFilePathOverride).
				WithSuggestion("Verify the file path is correct").
				WithSuggestion("Check that the file exists and is readable").
				WithSuggestion("Use 'invowk config show' to see default configuration").
				Wrap(fmt.Errorf("config file not found: %s", configFilePathOverride)).
				BuildError()
		}
		if err := loadCUEIntoViper(v, configFilePathOverride); err != nil {
			return nil, issue.NewErrorContext().
				WithOperation("load configuration").
				WithResource(configFilePathOverride).
				WithSuggestion("Check that the file contains valid CUE syntax").
				WithSuggestion("Verify the configuration values match the expected schema").
				WithSuggestion("See 'invowk config --help' for configuration options").
				Wrap(err).
				BuildError()
		}
		configPath = configFilePathOverride
	} else {
		// Get config directory
		cfgDir, err := ConfigDir()
		if err != nil {
			return nil, err
		}

		// Try to load CUE config file
		cuePath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)
		if fileExists(cuePath) {
			if err := loadCUEIntoViper(v, cuePath); err != nil {
				return nil, issue.NewErrorContext().
					WithOperation("load configuration").
					WithResource(cuePath).
					WithSuggestion("Check that the file contains valid CUE syntax").
					WithSuggestion("Verify the configuration values match the expected schema").
					WithSuggestion("See 'invowk config --help' for configuration options").
					Wrap(err).
					BuildError()
			}
			configPath = cuePath
		} else {
			// Also check current directory
			localCuePath := ConfigFileName + "." + ConfigFileExt
			if fileExists(localCuePath) {
				if err := loadCUEIntoViper(v, localCuePath); err != nil {
					return nil, issue.NewErrorContext().
						WithOperation("load configuration").
						WithResource(localCuePath).
						WithSuggestion("Check that the file contains valid CUE syntax").
						WithSuggestion("Verify the configuration values match the expected schema").
						WithSuggestion("See 'invowk config --help' for configuration options").
						Wrap(err).
						BuildError()
				}
				configPath = localCuePath
			}
			// If no config file found, use defaults (no error)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	globalConfig = &cfg
	return globalConfig, nil
}

// loadCUEIntoViper parses a CUE file, validates it against the #Config schema,
// and merges its contents into Viper.
//
// Note: This uses manual CUE parsing instead of cueutil.ParseAndDecode because:
// 1. Config decodes to map[string]any (not a struct) for Viper integration
// 2. Uses Concrete(false) because config fields are optional
// 3. Needs to merge into Viper's config map, not return a struct
func loadCUEIntoViper(v *viper.Viper, path string) error {
	// Read CUE file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Check file size using cueutil
	if err := cueutil.CheckFileSize(data, cueutil.DefaultMaxFileSize, path); err != nil {
		return err
	}

	// Parse with CUE
	ctx := cuecontext.New()

	// Compile the schema
	schemaValue := ctx.CompileString(configSchema)
	if schemaValue.Err() != nil {
		return fmt.Errorf("internal error: failed to compile config schema: %w", schemaValue.Err())
	}

	// Compile the user's config file
	userValue := ctx.CompileBytes(data, cue.Filename(path))
	if userValue.Err() != nil {
		return cueutil.FormatError(userValue.Err(), path)
	}

	// Unify with schema to validate against #Config definition
	schema := schemaValue.LookupPath(cue.ParsePath("#Config"))
	unified := schema.Unify(userValue)
	if err := unified.Validate(cue.Concrete(false)); err != nil {
		return cueutil.FormatError(err, path)
	}

	// Decode to Go map
	var configMap map[string]any
	if err := unified.Decode(&configMap); err != nil {
		return cueutil.FormatError(err, path)
	}

	// Merge into Viper (preserves defaults, allows env overrides)
	if err := v.MergeConfigMap(configMap); err != nil {
		return fmt.Errorf("failed to merge config: %w", err)
	}

	return nil
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// Get returns the currently loaded configuration.
// If configuration loading fails, it returns defaults and stores the error
// for retrieval via LastLoadError().
func Get() *Config {
	if globalConfig == nil {
		cfg, err := Load()
		if err != nil {
			errLastLoad = err
			return DefaultConfig()
		}
		return cfg
	}
	return globalConfig
}

// LastLoadError returns the most recent error from configuration loading.
// This is useful for surfacing config errors to users even when defaults are used.
// Returns nil if configuration loaded successfully or was never attempted.
func LastLoadError() error {
	return errLastLoad
}

// ConfigFilePath returns the path to the config file.
//
//nolint:revive // ConfigFilePath is more descriptive than FilePath for external callers
func ConfigFilePath() string {
	return configPath
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	cfgDir, err := ConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(cfgDir, 0o755)
}

// EnsureCommandsDir creates the commands directory if it doesn't exist
func EnsureCommandsDir() error {
	cmdsDir, err := CommandsDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(cmdsDir, 0o755)
}

// CreateDefaultConfig creates a default config file if it doesn't exist
func CreateDefaultConfig() error {
	cfgDir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)

	// Check if file already exists
	if _, err := os.Stat(cfgPath); err == nil {
		return nil // File exists
	}

	defaults := DefaultConfig()
	cueContent := GenerateCUE(defaults)

	if err := os.WriteFile(cfgPath, []byte(cueContent), 0o644); err != nil {
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

	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)

	cueContent := GenerateCUE(cfg)

	if err := os.WriteFile(cfgPath, []byte(cueContent), 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	globalConfig = cfg
	configPath = cfgPath
	return nil
}

// GenerateCUE generates a CUE representation of the configuration
func GenerateCUE(cfg *Config) string {
	var sb strings.Builder

	sb.WriteString("// Invowk Configuration File\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation.\n\n")

	// Container engine
	sb.WriteString(fmt.Sprintf("container_engine: %q\n", cfg.ContainerEngine))

	// Default runtime
	sb.WriteString(fmt.Sprintf("default_runtime: %q\n", cfg.DefaultRuntime))

	// Search paths
	if len(cfg.SearchPaths) > 0 {
		sb.WriteString("\nsearch_paths: [\n")
		for _, p := range cfg.SearchPaths {
			sb.WriteString(fmt.Sprintf("\t%q,\n", p))
		}
		sb.WriteString("]\n")
	}

	// Virtual shell config
	sb.WriteString("\nvirtual_shell: {\n")
	sb.WriteString(fmt.Sprintf("\tenable_uroot_utils: %v\n", cfg.VirtualShell.EnableUrootUtils))
	sb.WriteString("}\n")

	// UI config
	sb.WriteString("\nui: {\n")
	sb.WriteString(fmt.Sprintf("\tcolor_scheme: %q\n", cfg.UI.ColorScheme))
	sb.WriteString(fmt.Sprintf("\tverbose: %v\n", cfg.UI.Verbose))
	sb.WriteString(fmt.Sprintf("\tinteractive: %v\n", cfg.UI.Interactive))
	sb.WriteString("}\n")

	// Container config
	sb.WriteString("\ncontainer: {\n")
	sb.WriteString("\tauto_provision: {\n")
	sb.WriteString(fmt.Sprintf("\t\tenabled: %v\n", cfg.Container.AutoProvision.Enabled))
	if cfg.Container.AutoProvision.BinaryPath != "" {
		sb.WriteString(fmt.Sprintf("\t\tbinary_path: %q\n", cfg.Container.AutoProvision.BinaryPath))
	}
	if len(cfg.Container.AutoProvision.ModulesPaths) > 0 {
		sb.WriteString("\t\tmodules_paths: [\n")
		for _, p := range cfg.Container.AutoProvision.ModulesPaths {
			sb.WriteString(fmt.Sprintf("\t\t\t%q,\n", p))
		}
		sb.WriteString("\t\t]\n")
	}
	if cfg.Container.AutoProvision.CacheDir != "" {
		sb.WriteString(fmt.Sprintf("\t\tcache_dir: %q\n", cfg.Container.AutoProvision.CacheDir))
	}
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return sb.String()
}

// Reset clears all state including cached configuration and test overrides
func Reset() {
	globalConfig = nil
	configPath = ""
	configDirOverride = ""
	configFilePathOverride = ""
	errLastLoad = nil
}

// ResetCache clears only the cached configuration, preserving any test overrides.
// This is useful when testing scenarios that require reloading the config from disk
// without losing the test's config directory override.
func ResetCache() {
	globalConfig = nil
	configPath = ""
	errLastLoad = nil
}

// SetConfigDirOverride sets a custom config directory path.
// This is primarily intended for testing to bypass os.UserHomeDir() which
// doesn't reliably respect the HOME env var on all platforms (e.g., macOS in CI).
func SetConfigDirOverride(dir string) {
	configDirOverride = dir
}

// SetConfigFilePathOverride sets a custom config file path.
// This is used by the --config CLI flag to load config from a specific file.
// When set, the specified file must exist or Load() will return an error.
// This also clears the cached configuration to force reloading from the new path.
func SetConfigFilePathOverride(path string) {
	configFilePathOverride = path
	// Clear cache to force reload from the new path
	globalConfig = nil
	configPath = ""
	errLastLoad = nil
}
