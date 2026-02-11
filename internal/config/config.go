// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
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
	// AppName is the application name.
	AppName = "invowk"
	// ConfigFileName is the name of the config file (without extension).
	ConfigFileName = "config"
	// ConfigFileExt is the config file extension.
	ConfigFileExt = "cue"

	osWindows = "windows"
)

//go:embed config_schema.cue
var configSchema string

// ConfigDir returns the invowk configuration directory using platform-specific
// conventions: Windows uses %APPDATA%, macOS uses ~/Library/Application Support,
// and Linux/others use $XDG_CONFIG_HOME (defaulting to ~/.config).
//
//nolint:revive // ConfigDir is more descriptive than Dir for external callers
func ConfigDir() (string, error) {
	// Allow tests to override the config directory
	if configDirOverride != "" {
		return configDirOverride, nil
	}

	var configDir string

	switch runtime.GOOS {
	case osWindows:
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
	case osWindows:
		return filepath.Join(home, ".invowk", "cmds"), nil
	default:
		return filepath.Join(home, ".invowk", "cmds"), nil
	}
}

// loadWithOptions performs option-driven config loading without mutating
// package-level cache state. Callers that want caching can wrap this function.
func loadWithOptions(ctx context.Context, opts LoadOptions) (*Config, string, error) {
	select {
	case <-ctx.Done():
		return nil, "", fmt.Errorf("load config canceled: %w", ctx.Err())
	default:
	}

	v := viper.New()

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("container_engine", defaults.ContainerEngine)
	v.SetDefault("includes", defaults.Includes)
	v.SetDefault("default_runtime", defaults.DefaultRuntime)
	v.SetDefault("virtual_shell.enable_uroot_utils", defaults.VirtualShell.EnableUrootUtils)
	v.SetDefault("ui.color_scheme", defaults.UI.ColorScheme)
	v.SetDefault("ui.verbose", defaults.UI.Verbose)
	v.SetDefault("ui.interactive", defaults.UI.Interactive)
	v.SetDefault("container.auto_provision.enabled", defaults.Container.AutoProvision.Enabled)
	v.SetDefault("container.auto_provision.binary_path", defaults.Container.AutoProvision.BinaryPath)
	v.SetDefault("container.auto_provision.modules_paths", defaults.Container.AutoProvision.ModulesPaths)
	v.SetDefault("container.auto_provision.cache_dir", defaults.Container.AutoProvision.CacheDir)

	resolvedPath := ""

	// If a custom config file path is set via --config flag, use it exclusively.
	if opts.ConfigFilePath != "" {
		if !fileExists(opts.ConfigFilePath) {
			return nil, "", issue.NewErrorContext().
				WithOperation("load configuration").
				WithResource(opts.ConfigFilePath).
				WithSuggestion("Verify the file path is correct").
				WithSuggestion("Check that the file exists and is readable").
				WithSuggestion("Use 'invowk config show' to see default configuration").
				Wrap(fmt.Errorf("config file not found: %s", opts.ConfigFilePath)).
				BuildError()
		}
		if err := loadCUEIntoViper(v, opts.ConfigFilePath); err != nil {
			return nil, "", issue.NewErrorContext().
				WithOperation("load configuration").
				WithResource(opts.ConfigFilePath).
				WithSuggestion("Check that the file contains valid CUE syntax").
				WithSuggestion("Verify the configuration values match the expected schema").
				WithSuggestion("See 'invowk config --help' for configuration options").
				Wrap(err).
				BuildError()
		}
		resolvedPath = opts.ConfigFilePath
	} else {
		// Get config directory
		cfgDir, err := configDirWithOverride(opts.ConfigDirPath)
		if err != nil {
			return nil, "", err
		}

		// Try to load CUE config file
		cuePath := filepath.Join(cfgDir, ConfigFileName+"."+ConfigFileExt)
		if fileExists(cuePath) {
			if err := loadCUEIntoViper(v, cuePath); err != nil {
				return nil, "", issue.NewErrorContext().
					WithOperation("load configuration").
					WithResource(cuePath).
					WithSuggestion("Check that the file contains valid CUE syntax").
					WithSuggestion("Verify the configuration values match the expected schema").
					WithSuggestion("See 'invowk config --help' for configuration options").
					Wrap(err).
					BuildError()
			}
			resolvedPath = cuePath
		} else {
			// Also check current directory
			localCuePath := ConfigFileName + "." + ConfigFileExt
			if fileExists(localCuePath) {
				if err := loadCUEIntoViper(v, localCuePath); err != nil {
					return nil, "", issue.NewErrorContext().
						WithOperation("load configuration").
						WithResource(localCuePath).
						WithSuggestion("Check that the file contains valid CUE syntax").
						WithSuggestion("Verify the configuration values match the expected schema").
						WithSuggestion("See 'invowk config --help' for configuration options").
						Wrap(err).
						BuildError()
				}
				resolvedPath = localCuePath
			}
			// If no config file found, use defaults (no error)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, "", fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate includes constraints that CUE cannot express:
	// alias uniqueness across all entries and alias-only-for-modules.
	if err := validateIncludes(cfg.Includes); err != nil {
		return nil, "", issue.NewErrorContext().
			WithOperation("validate configuration").
			WithSuggestion("Ensure each alias is unique across all includes entries").
			WithSuggestion("Aliases are only valid for module paths ending in .invkmod").
			Wrap(err).
			BuildError()
	}

	return &cfg, resolvedPath, nil
}

// configDirWithOverride resolves the configuration directory, honoring
// explicit provider options before platform defaults.
func configDirWithOverride(configDirPath string) (string, error) {
	if configDirPath != "" {
		return configDirPath, nil
	}

	return ConfigDir()
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

// validateIncludes checks include entries for constraints that CUE cannot express:
//   - alias is only valid when path ends with .invkmod
//   - all non-empty aliases must be globally unique across entries
func validateIncludes(includes []IncludeEntry) error {
	seenAliases := make(map[string]string) // alias -> path of first occurrence
	seenPaths := make(map[string]int)      // cleaned path -> index of first occurrence
	for i, entry := range includes {
		// Check path uniqueness (normalized to handle trailing slashes and redundant separators)
		cleanPath := filepath.Clean(entry.Path)
		if firstIdx, exists := seenPaths[cleanPath]; exists {
			return fmt.Errorf("includes[%d]: duplicate path %q (same as includes[%d])", i, entry.Path, firstIdx)
		}
		seenPaths[cleanPath] = i

		if entry.Alias == "" {
			continue
		}
		// Alias is only meaningful for module paths
		if !entry.IsModule() {
			return fmt.Errorf("includes[%d]: alias %q is only valid for module paths (.invkmod), but path is %q", i, entry.Alias, entry.Path)
		}
		// Check alias uniqueness
		if existingPath, exists := seenAliases[entry.Alias]; exists {
			return fmt.Errorf("includes: duplicate alias %q used by both %q and %q", entry.Alias, existingPath, entry.Path)
		}
		seenAliases[entry.Alias] = entry.Path
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

	// Includes
	if len(cfg.Includes) > 0 {
		sb.WriteString("\nincludes: [\n")
		for _, entry := range cfg.Includes {
			if entry.Alias != "" {
				sb.WriteString(fmt.Sprintf("\t{path: %q, alias: %q},\n", entry.Path, entry.Alias))
			} else {
				sb.WriteString(fmt.Sprintf("\t{path: %q},\n", entry.Path))
			}
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
