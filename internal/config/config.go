// SPDX-License-Identifier: MPL-2.0

package config

import (
	"context"
	_ "embed" // required for go:embed config_schema.cue
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/cueutil"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// AppName is the application name.
	AppName = "invowk"
	// ConfigFileName is the name of the config file (without extension).
	ConfigFileName = "config"
	// ConfigFileExt is the config file extension.
	ConfigFileExt = "cue"

	errMsgHomeDir = "failed to get home directory: %w"
)

//go:embed config_schema.cue
var configSchema string

type (
	configPatch struct {
		ContainerEngine *ContainerEngine         `json:"container_engine"`
		Includes        *[]IncludeEntry          `json:"includes"`
		DefaultRuntime  *RuntimeMode             `json:"default_runtime"`
		VirtualShell    *virtualShellConfigPatch `json:"virtual_shell"`
		UI              *uiConfigPatch           `json:"ui"`
		Container       *containerConfigPatch    `json:"container"`
	}

	virtualShellConfigPatch struct {
		EnableUrootUtils *bool `json:"enable_uroot_utils"`
	}

	uiConfigPatch struct {
		ColorScheme *ColorScheme `json:"color_scheme"`
		Verbose     *bool        `json:"verbose"`
		Interactive *bool        `json:"interactive"`
	}

	containerConfigPatch struct {
		AutoProvision *autoProvisionConfigPatch `json:"auto_provision"`
	}

	autoProvisionConfigPatch struct {
		Enabled         *bool           `json:"enabled"`
		Strict          *bool           `json:"strict"`
		BinaryPath      *BinaryFilePath `json:"binary_path"`
		Includes        *[]IncludeEntry `json:"includes"`
		InheritIncludes *bool           `json:"inherit_includes"`
		CacheDir        *CacheDirPath   `json:"cache_dir"`
	}
)

func (p configPatch) Validate() error {
	var errs []error
	if p.ContainerEngine != nil {
		errs = append(errs, p.ContainerEngine.Validate())
	}
	if p.Includes != nil {
		for i := range *p.Includes {
			errs = append(errs, (*p.Includes)[i].Validate())
		}
	}
	if p.DefaultRuntime != nil {
		errs = append(errs, p.DefaultRuntime.Validate())
	}
	if p.VirtualShell != nil {
		errs = append(errs, p.VirtualShell.Validate())
	}
	if p.UI != nil {
		errs = append(errs, p.UI.Validate())
	}
	if p.Container != nil {
		errs = append(errs, p.Container.Validate())
	}
	return errors.Join(errs...)
}

func (p virtualShellConfigPatch) Validate() error {
	return nil
}

func (p uiConfigPatch) Validate() error {
	if p.ColorScheme != nil {
		return p.ColorScheme.Validate()
	}
	return nil
}

func (p containerConfigPatch) Validate() error {
	if p.AutoProvision != nil {
		return p.AutoProvision.Validate()
	}
	return nil
}

func (p autoProvisionConfigPatch) Validate() error {
	var errs []error
	if p.BinaryPath != nil {
		errs = append(errs, p.BinaryPath.Validate())
	}
	if p.Includes != nil {
		for i := range *p.Includes {
			errs = append(errs, (*p.Includes)[i].Validate())
		}
	}
	if p.CacheDir != nil {
		errs = append(errs, p.CacheDir.Validate())
	}
	return errors.Join(errs...)
}

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
				return "", errors.New("neither APPDATA nor USERPROFILE environment variable is set")
			}
			configDir = filepath.Join(userProfile, "AppData", "Roaming")
		}
	case "darwin":
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf(errMsgHomeDir, err)
		}
		configDir = filepath.Join(home, "Library", "Application Support")
	default: // Linux and others
		configDir = getenv("XDG_CONFIG_HOME")
		if configDir == "" {
			home, err := userHomeDir()
			if err != nil {
				return "", fmt.Errorf(errMsgHomeDir, err)
			}
			configDir = filepath.Join(home, ".config")
		}
	}

	configPath := types.FilesystemPath(filepath.Join(configDir, AppName))
	if err := configPath.Validate(); err != nil {
		return "", fmt.Errorf("config directory: %w", err)
	}
	return configPath, nil
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
		return "", fmt.Errorf(errMsgHomeDir, err)
	}
	cmdsDir := types.FilesystemPath(filepath.Join(home, ".invowk", "cmds"))
	if err := cmdsDir.Validate(); err != nil {
		return "", fmt.Errorf("commands directory: %w", err)
	}
	return cmdsDir, nil
}

// loadWithOptions performs option-driven config loading from the filesystem.
// Each call reads and parses configuration from disk with no caching.
func loadWithOptions(ctx context.Context, opts LoadOptions) (*Config, types.FilesystemPath, error) {
	select {
	case <-ctx.Done():
		return nil, "", fmt.Errorf("load config canceled: %w", ctx.Err())
	default:
	}

	cfg := DefaultConfig()
	var resolvedPath types.FilesystemPath

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
		loaded, err := decodeCUEConfigFile(opts.ConfigFilePath)
		if err != nil {
			return nil, "", cueLoadError(configFilePath, err)
		}
		cfg = loaded
		resolvedPath = opts.ConfigFilePath
	} else {
		// Get config directory
		cfgDir, err := configDirWithOverride(opts.ConfigDirPath)
		if err != nil {
			return nil, "", err
		}

		// Try to load CUE config file
		cuePath := filepath.Join(string(cfgDir), ConfigFileName+"."+ConfigFileExt)
		if fileExists(cuePath) {
			resolved := types.FilesystemPath(cuePath)
			if err := resolved.Validate(); err != nil {
				return nil, "", fmt.Errorf("config file path: %w", err)
			}
			loaded, err := decodeCUEConfigFile(resolved)
			if err != nil {
				return nil, "", cueLoadError(cuePath, err)
			}
			cfg = loaded
			resolvedPath = resolved
		} else {
			// Also check current directory (or BaseDir override)
			localCuePath := ConfigFileName + "." + ConfigFileExt
			if opts.BaseDir != "" {
				localCuePath = filepath.Join(string(opts.BaseDir), localCuePath)
			}
			if fileExists(localCuePath) {
				resolved := types.FilesystemPath(localCuePath)
				if err := resolved.Validate(); err != nil {
					return nil, "", fmt.Errorf("config file path: %w", err)
				}
				loaded, err := decodeCUEConfigFile(resolved)
				if err != nil {
					return nil, "", cueLoadError(localCuePath, err)
				}
				cfg = loaded
				resolvedPath = resolved
			}
			// If no config file found, use defaults (no error)
		}
	}

	// Defense-in-depth: validate all typed fields after unmarshalling.
	// CUE schema is the primary validation layer; this catches any gaps
	// or values that bypass CUE.
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

	return cfg, resolvedPath, nil
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

// decodeCUEConfigFile parses a CUE file through the shared schema parser and
// applies the resulting patch over DefaultConfig().
func decodeCUEConfigFile(path types.FilesystemPath) (*Config, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	parsed, err := cueutil.ParseAndDecodeString[configPatch](
		configSchema,
		data,
		"#Config",
		cueutil.WithConcrete(false),
		cueutil.WithFilename(string(path)),
	)
	if err != nil {
		return nil, err
	}
	if err := parsed.Value.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config patch: %w", err)
	}

	return applyConfigPatch(DefaultConfig(), parsed.Value), nil
}

func applyConfigPatch(cfg *Config, patch *configPatch) *Config {
	if patch == nil {
		return cfg
	}
	if patch.ContainerEngine != nil {
		cfg.ContainerEngine = *patch.ContainerEngine
	}
	if patch.Includes != nil {
		cfg.Includes = *patch.Includes
	}
	if patch.DefaultRuntime != nil {
		cfg.DefaultRuntime = *patch.DefaultRuntime
	}
	if patch.VirtualShell != nil && patch.VirtualShell.EnableUrootUtils != nil {
		cfg.VirtualShell.EnableUrootUtils = *patch.VirtualShell.EnableUrootUtils
	}
	if patch.UI != nil {
		if patch.UI.ColorScheme != nil {
			cfg.UI.ColorScheme = *patch.UI.ColorScheme
		}
		if patch.UI.Verbose != nil {
			cfg.UI.Verbose = *patch.UI.Verbose
		}
		if patch.UI.Interactive != nil {
			cfg.UI.Interactive = *patch.UI.Interactive
		}
	}
	if patch.Container != nil && patch.Container.AutoProvision != nil {
		auto := patch.Container.AutoProvision
		if auto.Enabled != nil {
			cfg.Container.AutoProvision.Enabled = *auto.Enabled
		}
		if auto.Strict != nil {
			cfg.Container.AutoProvision.Strict = *auto.Strict
		}
		if auto.BinaryPath != nil {
			cfg.Container.AutoProvision.BinaryPath = *auto.BinaryPath
		}
		if auto.Includes != nil {
			cfg.Container.AutoProvision.Includes = *auto.Includes
		}
		if auto.InheritIncludes != nil {
			cfg.Container.AutoProvision.InheritIncludes = *auto.InheritIncludes
		}
		if auto.CacheDir != nil {
			cfg.Container.AutoProvision.CacheDir = *auto.CacheDir
		}
	}
	return cfg
}

// validateIncludes checks include collection constraints:
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
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("%s[%d]: %w", fieldName, i, err)
		}
		pathStr := string(entry.Path)

		// Check path uniqueness (normalized to handle trailing slashes and redundant separators)
		cleanPath := filepath.Clean(pathStr)
		if firstIdx, exists := seenPaths[cleanPath]; exists {
			return fmt.Errorf("%s[%d]: duplicate path %q (same as %s[%d])", fieldName, i, entry.Path, fieldName, firstIdx)
		}
		seenPaths[cleanPath] = i

		// Track short name for collision detection using the module domain's
		// structural folder-name parser. Path validation already checked this.
		shortName, parseErr := invowkmod.ParseModuleName(filepath.Base(pathStr))
		if parseErr != nil {
			return fmt.Errorf("%s[%d]: %w", fieldName, i, parseErr)
		}
		shortNames[string(shortName)] = append(shortNames[string(shortName)], i)

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
