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
	"github.com/invowk/invowk/pkg/types"

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

// configDirFrom computes the config directory from injectable dependencies,
// enabling tests to avoid mutating process-global environment variables.
func configDirFrom(goos string, getenv func(string) string, userHomeDir func() (string, error)) (types.FilesystemPath, error) {
	var configDir string

	switch goos {
	case platform.Windows:
		configDir = getenv("APPDATA")
		if configDir == "" {
			userProfile := getenv("USERPROFILE")
			if userProfile == "" {
				return "", fmt.Errorf("neither APPDATA nor USERPROFILE environment variable is set")
			}
			configDir = filepath.Join(userProfile, "AppData", "Roaming")
		}
	case "darwin":
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		configDir = getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := userHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	return types.FilesystemPath(filepath.Join(configDir, AppName)), nil
}

// ConfigDir returns the invowk configuration directory using platform-specific
// conventions: Windows uses %APPDATA%, macOS uses ~/Library/Application Support,
// and Linux/others use $XDG_CONFIG_HOME (defaulting to ~/.config).
//
//nolint:revive // ConfigDir is more descriptive than Dir for external callers
func ConfigDir() (types.FilesystemPath, error) {
	return configDirFrom(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

// CommandsDir returns the directory for user-defined invowkfiles.
// The path is ~/.invowk/cmds on all platforms.
func CommandsDir() (types.FilesystemPath, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return types.FilesystemPath(filepath.Join(home, ".invowk", "cmds")), nil
}

// loadWithOptions performs option-driven config loading from the filesystem.
// Each call reads and parses configuration from disk with no caching.
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
	configFilePath := string(opts.ConfigFilePath)
	if configFilePath != "" {
		if !fileExists(configFilePath) {
			return nil, "", issue.NewErrorContext().
				WithOperation("load configuration").
				WithResource(configFilePath).
				WithSuggestion("Verify the file path is correct").
				WithSuggestion("Check that the file exists and is readable").
				WithSuggestion("Use 'invowk config show' to see default configuration").
				Wrap(fmt.Errorf("config file not found: %s", configFilePath)).
				BuildError()
		}
		if err := loadCUEIntoViper(v, configFilePath); err != nil {
			return nil, "", cueLoadError(configFilePath, err)
		}
		resolvedPath = configFilePath
	} else {
		// Get config directory
		cfgDir, err := configDirWithOverride(opts.ConfigDirPath)
		if err != nil {
			return nil, "", err
		}

		// Try to load CUE config file
		cuePath := filepath.Join(string(cfgDir), ConfigFileName+"."+ConfigFileExt)
		if fileExists(cuePath) {
			if err := loadCUEIntoViper(v, cuePath); err != nil {
				return nil, "", cueLoadError(cuePath, err)
			}
			resolvedPath = cuePath
		} else {
			// Also check current directory (or BaseDir override)
			localCuePath := ConfigFileName + "." + ConfigFileExt
			if opts.BaseDir != "" {
				localCuePath = filepath.Join(string(opts.BaseDir), localCuePath)
			}
			if fileExists(localCuePath) {
				if err := loadCUEIntoViper(v, localCuePath); err != nil {
					return nil, "", cueLoadError(localCuePath, err)
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

	// Defense-in-depth: validate all typed fields after unmarshalling.
	// CUE schema is the primary validation layer; this catches any gaps
	// or values that bypass CUE (e.g., env var overrides via Viper).
	if err := cfg.Validate(); err != nil {
		return nil, "", fmt.Errorf("invalid config: %w", err)
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
func configDirWithOverride(configDirPath types.FilesystemPath) (types.FilesystemPath, error) {
	if configDirPath != "" {
		return configDirPath, nil
	}

	return ConfigDir()
}

// commandsDirWithOverride resolves the commands directory, honoring
// explicit provider options before platform defaults.
func commandsDirWithOverride(commandsDirPath types.FilesystemPath) (types.FilesystemPath, error) {
	if commandsDirPath != "" {
		return commandsDirPath, nil
	}

	return CommandsDir()
}

// cueLoadError wraps a CUE loading/parsing error with actionable suggestions.
// This is the common error path for all config file locations (explicit path,
// config dir, current dir).
func cueLoadError(path string, err error) error {
	return issue.NewErrorContext().
		WithOperation("load configuration").
		WithResource(path).
		WithSuggestion("Check that the file contains valid CUE syntax").
		WithSuggestion("Verify the configuration values match the expected schema").
		WithSuggestion("See 'invowk config --help' for configuration options").
		Wrap(err).
		BuildError()
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
//   - all paths must be absolute (CUE regex cannot enforce cross-platform absolute paths)
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
		pathStr := string(entry.Path)

		// Check path is absolute (CUE regex cannot enforce cross-platform absolute paths)
		if !filepath.IsAbs(pathStr) {
			return fmt.Errorf("%s[%d]: path %q must be absolute", fieldName, i, entry.Path)
		}

		// Check path uniqueness (normalized to handle trailing slashes and redundant separators)
		cleanPath := filepath.Clean(pathStr)
		if firstIdx, exists := seenPaths[cleanPath]; exists {
			return fmt.Errorf("%s[%d]: duplicate path %q (same as %s[%d])", fieldName, i, entry.Path, fieldName, firstIdx)
		}
		seenPaths[cleanPath] = i

		// Track short name for collision detection
		shortName := strings.TrimSuffix(filepath.Base(pathStr), moduleSuffix)
		shortNames[shortName] = append(shortNames[shortName], i)

		// Check alias uniqueness
		aliasStr := string(entry.Alias)
		if aliasStr != "" {
			if existingPath, exists := seenAliases[aliasStr]; exists {
				return fmt.Errorf("%s: duplicate alias %q used by both %q and %q", fieldName, entry.Alias, existingPath, entry.Path)
			}
			seenAliases[aliasStr] = pathStr
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

// EnsureConfigDir creates the config directory if it doesn't exist.
// When configDirPath is empty, the platform-default directory from ConfigDir() is used.
func EnsureConfigDir(configDirPath types.FilesystemPath) error {
	cfgDir, err := configDirWithOverride(configDirPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(string(cfgDir), 0o755)
}

// EnsureCommandsDir creates the commands directory if it doesn't exist.
// When commandsDirPath is empty, the platform-default directory from CommandsDir() is used.
func EnsureCommandsDir(commandsDirPath types.FilesystemPath) error {
	cmdsDir, err := commandsDirWithOverride(commandsDirPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(string(cmdsDir), 0o755)
}

// CreateDefaultConfig creates a default config file if it doesn't exist.
// When configDirPath is empty, the platform-default directory from ConfigDir() is used.
func CreateDefaultConfig(configDirPath types.FilesystemPath) error {
	cfgDir, err := configDirWithOverride(configDirPath)
	if err != nil {
		return err
	}

	cfgDirStr := string(cfgDir)
	if err := os.MkdirAll(cfgDirStr, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDirStr, ConfigFileName+"."+ConfigFileExt)

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

// Save writes the current configuration to file.
// When configDirPath is empty, the platform-default directory from ConfigDir() is used.
func Save(cfg *Config, configDirPath types.FilesystemPath) error {
	cfgDir, err := configDirWithOverride(configDirPath)
	if err != nil {
		return err
	}

	cfgDirStr := string(cfgDir)
	if err := os.MkdirAll(cfgDirStr, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgPath := filepath.Join(cfgDirStr, ConfigFileName+"."+ConfigFileExt)

	cueContent := GenerateCUE(cfg)

	if err := os.WriteFile(cfgPath, []byte(cueContent), 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GenerateCUE generates a CUE representation of the configuration
//
//plint:render
func GenerateCUE(cfg *Config) string {
	var sb strings.Builder

	sb.WriteString("// Invowk Configuration File\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation.\n\n")

	// Container engine
	fmt.Fprintf(&sb, "container_engine: %q\n", cfg.ContainerEngine)

	// Default runtime
	fmt.Fprintf(&sb, "default_runtime: %q\n", cfg.DefaultRuntime)

	// Includes
	if len(cfg.Includes) > 0 {
		sb.WriteString("\nincludes: [\n")
		for _, entry := range cfg.Includes {
			if entry.Alias != "" {
				fmt.Fprintf(&sb, "\t{path: %q, alias: %q},\n", entry.Path, entry.Alias)
			} else {
				fmt.Fprintf(&sb, "\t{path: %q},\n", entry.Path)
			}
		}
		sb.WriteString("]\n")
	}

	// Virtual shell config
	sb.WriteString("\nvirtual_shell: {\n")
	fmt.Fprintf(&sb, "\tenable_uroot_utils: %v\n", cfg.VirtualShell.EnableUrootUtils)
	sb.WriteString("}\n")

	// UI config
	sb.WriteString("\nui: {\n")
	fmt.Fprintf(&sb, "\tcolor_scheme: %q\n", cfg.UI.ColorScheme)
	fmt.Fprintf(&sb, "\tverbose: %v\n", cfg.UI.Verbose)
	fmt.Fprintf(&sb, "\tinteractive: %v\n", cfg.UI.Interactive)
	sb.WriteString("}\n")

	// Container config
	sb.WriteString("\ncontainer: {\n")
	sb.WriteString("\tauto_provision: {\n")
	fmt.Fprintf(&sb, "\t\tenabled: %v\n", cfg.Container.AutoProvision.Enabled)
	if cfg.Container.AutoProvision.BinaryPath != "" {
		fmt.Fprintf(&sb, "\t\tbinary_path: %q\n", cfg.Container.AutoProvision.BinaryPath)
	}
	if len(cfg.Container.AutoProvision.Includes) > 0 {
		sb.WriteString("\t\tincludes: [\n")
		for _, entry := range cfg.Container.AutoProvision.Includes {
			if entry.Alias != "" {
				fmt.Fprintf(&sb, "\t\t\t{path: %q, alias: %q},\n", entry.Path, entry.Alias)
			} else {
				fmt.Fprintf(&sb, "\t\t\t{path: %q},\n", entry.Path)
			}
		}
		sb.WriteString("\t\t]\n")
	}
	fmt.Fprintf(&sb, "\t\tinherit_includes: %v\n", cfg.Container.AutoProvision.InheritIncludes)
	if cfg.Container.AutoProvision.CacheDir != "" {
		fmt.Fprintf(&sb, "\t\tcache_dir: %q\n", cfg.Container.AutoProvision.CacheDir)
	}
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return sb.String()
}
