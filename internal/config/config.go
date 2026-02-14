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

	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/cueutil"
	"github.com/invowk/invowk/pkg/platform"

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
	case platform.Windows:
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

// CommandsDir returns the directory for user-defined invowkfiles.
// The path is ~/.invowk/cmds on all platforms.
func CommandsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".invowk", "cmds"), nil
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
	v.SetDefault("container.auto_provision.includes", defaults.Container.AutoProvision.Includes)
	v.SetDefault("container.auto_provision.inherit_includes", defaults.Container.AutoProvision.InheritIncludes)
	v.SetDefault("container.auto_provision.cache_dir", defaults.Container.AutoProvision.CacheDir)

	resolvedPath := ""

	// If a custom config file path is set via --ivk-config flag, use it exclusively.
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
	// path uniqueness, alias uniqueness, and short-name collision disambiguation.
	if err := validateIncludes("includes", cfg.Includes); err != nil {
		return nil, "", issue.NewErrorContext().
			WithOperation("validate configuration").
			WithSuggestion("Ensure each alias is unique across all includes entries").
			WithSuggestion("When two modules share the same short name, all must have unique aliases").
			Wrap(err).
			BuildError()
	}

	// Validate auto_provision includes with the same rules.
	if err := validateIncludes("container.auto_provision.includes", cfg.Container.AutoProvision.Includes); err != nil {
		return nil, "", issue.NewErrorContext().
			WithOperation("validate configuration").
			WithSuggestion("Ensure each alias is unique across all auto_provision includes entries").
			WithSuggestion("When two modules share the same short name, all must have unique aliases").
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
//   - all paths must be unique (normalized via filepath.Clean)
//   - all non-empty aliases must be globally unique across entries
//   - when two or more entries share the same filesystem short name (e.g., "foo.invowkmod"),
//     ALL entries with that short name must have a non-empty alias for disambiguation
//
// The fieldName parameter is used in error messages to identify which includes
// section failed validation (e.g., "includes" vs "container.auto_provision.includes").
func validateIncludes(fieldName string, includes []IncludeEntry) error {
	seenAliases := make(map[string]string) // alias -> path of first occurrence
	seenPaths := make(map[string]int)      // cleaned path -> index of first occurrence
	shortNames := make(map[string][]int)   // short name -> indices of entries with that name

	for i, entry := range includes {
		// Check path uniqueness (normalized to handle trailing slashes and redundant separators)
		cleanPath := filepath.Clean(entry.Path)
		if firstIdx, exists := seenPaths[cleanPath]; exists {
			return fmt.Errorf("%s[%d]: duplicate path %q (same as %s[%d])", fieldName, i, entry.Path, fieldName, firstIdx)
		}
		seenPaths[cleanPath] = i

		// Track short name for collision detection
		shortName := strings.TrimSuffix(filepath.Base(entry.Path), moduleSuffix)
		shortNames[shortName] = append(shortNames[shortName], i)

		// Check alias uniqueness
		if entry.Alias != "" {
			if existingPath, exists := seenAliases[entry.Alias]; exists {
				return fmt.Errorf("%s: duplicate alias %q used by both %q and %q", fieldName, entry.Alias, existingPath, entry.Path)
			}
			seenAliases[entry.Alias] = entry.Path
		}
	}

	// Enforce short-name collision rule: if 2+ entries share the same short name,
	// ALL of those entries must have non-empty aliases for disambiguation.
	for shortName, indices := range shortNames {
		if len(indices) < 2 {
			continue
		}
		for _, idx := range indices {
			if includes[idx].Alias == "" {
				return fmt.Errorf(
					"%s[%d]: module %q shares short name %q with %d other entry(ies); all entries with this short name must have unique aliases",
					fieldName, idx, includes[idx].Path, shortName, len(indices)-1,
				)
			}
		}
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
	if len(cfg.Container.AutoProvision.Includes) > 0 {
		sb.WriteString("\t\tincludes: [\n")
		for _, entry := range cfg.Container.AutoProvision.Includes {
			if entry.Alias != "" {
				sb.WriteString(fmt.Sprintf("\t\t\t{path: %q, alias: %q},\n", entry.Path, entry.Alias))
			} else {
				sb.WriteString(fmt.Sprintf("\t\t\t{path: %q},\n", entry.Path))
			}
		}
		sb.WriteString("\t\t]\n")
	}
	sb.WriteString(fmt.Sprintf("\t\tinherit_includes: %v\n", cfg.Container.AutoProvision.InheritIncludes))
	if cfg.Container.AutoProvision.CacheDir != "" {
		sb.WriteString(fmt.Sprintf("\t\tcache_dir: %q\n", cfg.Container.AutoProvision.CacheDir))
	}
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return sb.String()
}
