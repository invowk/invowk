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

	"cuelang.org/go/cue"

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

var (
	//go:embed config_schema.cue
	configSchema string

	// ErrConfigFileNotFound marks an explicit config path that does not exist.
	ErrConfigFileNotFound = errors.New("config file not found")
	// ErrConfigLoadFailed marks config CUE read/parse/decode failures.
	ErrConfigLoadFailed = errors.New("config load failed")
)

type (
	// FileNotFoundError identifies a missing explicit config file.
	FileNotFoundError struct {
		Path types.FilesystemPath
	}

	// LoadError wraps a config file read/parse/decode failure.
	LoadError struct {
		Path types.FilesystemPath
		Err  error
	}

	configCUESource struct {
		data     configCUEData
		filename configCUEFilename
	}

	configCUEData     []byte
	configCUEFilename string
)

func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("config file not found: %s", e.Path)
}

func (e *FileNotFoundError) Unwrap() error {
	return errors.Join(ErrConfigFileNotFound, os.ErrNotExist)
}

func (e *LoadError) Error() string {
	return fmt.Sprintf("failed to load config %s: %v", e.Path, e.Err)
}

func (e *LoadError) Unwrap() error {
	return errors.Join(ErrConfigLoadFailed, e.Err)
}

func (s configCUESource) Validate() error { return s.filename.Validate() }

func (f configCUEFilename) String() string { return string(f) }

func (f configCUEFilename) Validate() error { return nil }

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
			return nil, "", &FileNotFoundError{Path: opts.ConfigFilePath}
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
		if includeErr, ok := errors.AsType[*InvalidIncludeCollectionError](err); ok {
			return nil, "", wrapIncludeCollectionError(includeErr)
		}
		return nil, "", err
	}

	return cfg, resolvedPath, nil
}

func wrapIncludeCollectionError(err *InvalidIncludeCollectionError) error {
	return &InvalidConfigError{FieldErrors: []error{err}}
}

// configDirWithOverride resolves the configuration directory, honoring
// explicit provider options before platform defaults.
func configDirWithOverride(configDirPath types.FilesystemPath) (types.FilesystemPath, error) {
	if configDirPath != "" {
		if err := configDirPath.Validate(); err != nil {
			return "", err
		}
		return configDirPath, nil
	}

	return ConfigDir()
}

// commandsDirWithOverride resolves the commands directory, honoring
// explicit provider options before platform defaults.
func commandsDirWithOverride(commandsDirPath types.FilesystemPath) (types.FilesystemPath, error) {
	if commandsDirPath != "" {
		if err := commandsDirPath.Validate(); err != nil {
			return "", err
		}
		return commandsDirPath, nil
	}

	return CommandsDir()
}

// cueLoadError wraps a CUE loading/parsing error with typed config semantics.
// This is the common error path for all config file locations (explicit path,
// config dir, current dir).
func cueLoadError(path string, err error) error {
	return &LoadError{
		Path: types.FilesystemPath(path), //goplint:ignore -- path came from provider/file discovery and is preserved for diagnostics.
		Err:  err,
	}
}

// decodeCUEConfigFile parses a CUE file through the shared schema parser.
func decodeCUEConfigFile(path types.FilesystemPath) (*Config, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return decodeCUEConfigSource(configCUESource{
		data:     configCUEData(data),
		filename: configCUEFilename(path),
	})
}

func decodeCUEConfigSource(source configCUESource) (*Config, error) {
	parsed, err := cueutil.ParseAndDecodeString[Config](
		configSchema,
		[]byte(source.data),
		"#Config",
		cueutil.WithConcrete(true),
		cueutil.WithFilename(string(source.filename)),
	)
	if err != nil {
		return nil, err
	}
	cfg := parsed.Value
	if apiValue := parsed.Unified.LookupPath(cue.ParsePath("llm.api")); apiValue.Exists() {
		cfg.LLM = cfg.LLM.WithAPIBackendPresent()
	}
	return cfg, nil
}

// validateIncludes checks include collection constraints:
//   - all paths must be unique (normalized via filepath.Clean)
//   - all non-empty aliases must be globally unique across entries
//   - when two or more entries share the same filesystem short name (e.g., "foo.invowkmod"),
//     ALL entries with that short name must have a non-empty alias for disambiguation
//
// The fieldName parameter is used in error messages to identify which includes
// section failed validation (e.g., "includes" vs "container.auto_provision.includes").
func validateIncludes(fieldName IncludeCollectionField, includes []IncludeEntry) error {
	fieldLabel := fieldName.String()
	seenAliases := make(map[string]string) // alias -> path of first occurrence
	seenPaths := make(map[string]int)      // cleaned path -> index of first occurrence
	shortNames := make(map[string][]int)   // short name -> indices of entries with that name

	for i, entry := range includes {
		if err := entry.Validate(); err != nil {
			return &InvalidIncludeCollectionError{
				Field: fieldName,
				Cause: fmt.Errorf("%s[%d]: %w", fieldLabel, i, err),
			}
		}
		pathStr := string(entry.Path)

		// Check path uniqueness (normalized to handle trailing slashes and redundant separators)
		cleanPath := filepath.Clean(pathStr)
		if firstIdx, exists := seenPaths[cleanPath]; exists {
			return &InvalidIncludeCollectionError{
				Field: fieldName,
				Cause: fmt.Errorf("%s[%d]: duplicate path %q (same as %s[%d])", fieldLabel, i, entry.Path, fieldLabel, firstIdx),
			}
		}
		seenPaths[cleanPath] = i

		// Track short name for collision detection using the module domain's
		// structural folder-name parser. Path validation already checked this.
		shortName, parseErr := invowkmod.ParseModuleName(filepath.Base(pathStr))
		if parseErr != nil {
			return &InvalidIncludeCollectionError{
				Field: fieldName,
				Cause: fmt.Errorf("%s[%d]: %w", fieldLabel, i, parseErr),
			}
		}
		shortNames[string(shortName)] = append(shortNames[string(shortName)], i)

		// Check alias uniqueness
		aliasStr := string(entry.Alias)
		if aliasStr != "" {
			if existingPath, exists := seenAliases[aliasStr]; exists {
				return &InvalidIncludeCollectionError{
					Field: fieldName,
					Cause: fmt.Errorf("%s: duplicate alias %q used by both %q and %q", fieldLabel, entry.Alias, existingPath, entry.Path),
				}
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
				return &InvalidIncludeCollectionError{
					Field: fieldName,
					Cause: fmt.Errorf(
						"%s[%d]: module %q shares short name %q with %d other entry(ies); all entries with this short name must have unique aliases",
						fieldLabel, idx, includes[idx].Path, shortName, len(indices)-1,
					),
				}
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
	if cfg == nil {
		return errors.New("config is nil")
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

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
	return generateCUE(cfg, false)
}

//goplint:ignore -- private renderer returns generated CUE text.
func generateCUE(cfg *Config, forceLLMAPIBlock bool) string {
	var sb strings.Builder

	sb.WriteString("// Invowk Configuration File\n")
	sb.WriteString("// See https://github.com/invowk/invowk for documentation.\n\n")

	// Container engine
	fmt.Fprintf(&sb, "container_engine: %q\n", cfg.ContainerEngine)

	// Default runtime
	fmt.Fprintf(&sb, "default_runtime: %q\n", cfg.DefaultRuntime)

	writeConfigCUEIncludes(&sb, cfg.Includes)
	writeConfigCUEVirtual(&sb, cfg.Virtual)
	writeConfigCUEUI(&sb, cfg.UI)
	writeConfigCUELLM(&sb, cfg.LLM, forceLLMAPIBlock)
	writeConfigCUEContainer(&sb, cfg.Container)

	return sb.String()
}

func writeConfigCUEIncludes(sb *strings.Builder, includes []IncludeEntry) {
	sb.WriteString("\nincludes: [\n")
	for _, entry := range includes {
		if entry.Alias != "" {
			fmt.Fprintf(sb, "\t{path: %q, alias: %q},\n", entry.Path, entry.Alias)
		} else {
			fmt.Fprintf(sb, "\t{path: %q},\n", entry.Path)
		}
	}
	sb.WriteString("]\n")
}

func writeConfigCUEVirtual(sb *strings.Builder, virtual VirtualConfig) {
	sb.WriteString("\nvirtual: {\n")
	sb.WriteString("\tutilities: {\n")
	fmt.Fprintf(sb, "\t\tenabled: %v\n", virtual.Utilities.Enabled)
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")
}

func writeConfigCUEUI(sb *strings.Builder, ui UIConfig) {
	sb.WriteString("\nui: {\n")
	fmt.Fprintf(sb, "\tcolor_scheme: %q\n", ui.ColorScheme)
	fmt.Fprintf(sb, "\tverbose: %v\n", ui.Verbose)
	fmt.Fprintf(sb, "\tinteractive: %v\n", ui.Interactive)
	sb.WriteString("}\n")
}

func writeConfigCUELLM(sb *strings.Builder, llm LLMConfig, forceAPIBlock bool) {
	sb.WriteString("\nllm: {\n")
	if llm.Provider != "" {
		fmt.Fprintf(sb, "\tprovider: %q\n", llm.Provider)
	}
	if llm.Model != "" {
		fmt.Fprintf(sb, "\tmodel: %q\n", llm.Model)
	}
	if llm.Timeout != "" {
		fmt.Fprintf(sb, "\ttimeout: %q\n", llm.Timeout)
	}
	if llm.Concurrency != 0 {
		fmt.Fprintf(sb, "\tconcurrency: %d\n", llm.Concurrency)
	}
	if llm.API.HasConfig() || forceAPIBlock {
		sb.WriteString("\tapi: {\n")
		writeConfigCUELLMAPI(sb, llm.API)
		sb.WriteString("\t}\n")
	}
	sb.WriteString("}\n")
}

func writeConfigCUELLMAPI(sb *strings.Builder, api LLMAPIConfig) {
	if api.BaseURL != "" {
		fmt.Fprintf(sb, "\t\tbase_url: %q\n", api.BaseURL)
	}
	if api.Model != "" {
		fmt.Fprintf(sb, "\t\tmodel: %q\n", api.Model)
	}
	if api.CredentialEnv != "" {
		fmt.Fprintf(sb, "\t\tapi_key_env: %q\n", api.CredentialEnv)
	}
}

func writeConfigCUEContainer(sb *strings.Builder, container ContainerConfig) {
	sb.WriteString("\ncontainer: {\n")
	sb.WriteString("\tauto_provision: {\n")
	fmt.Fprintf(sb, "\t\tenabled: %v\n", container.AutoProvision.Enabled)
	fmt.Fprintf(sb, "\t\tstrict: %v\n", container.AutoProvision.Strict)
	fmt.Fprintf(sb, "\t\tbinary_path: %q\n", container.AutoProvision.BinaryPath)
	sb.WriteString("\t\tincludes: [\n")
	for _, entry := range container.AutoProvision.Includes {
		if entry.Alias != "" {
			fmt.Fprintf(sb, "\t\t\t{path: %q, alias: %q},\n", entry.Path, entry.Alias)
		} else {
			fmt.Fprintf(sb, "\t\t\t{path: %q},\n", entry.Path)
		}
	}
	sb.WriteString("\t\t]\n")
	fmt.Fprintf(sb, "\t\tinherit_includes: %v\n", container.AutoProvision.InheritIncludes)
	fmt.Fprintf(sb, "\t\tcache_dir: %q\n", container.AutoProvision.CacheDir)
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")
}

// GenerateDisplayCUE generates configuration output for terminal display.
//
// It intentionally omits credential-related references that are safe to keep in
// config.cue but should not be echoed by diagnostic commands.
func GenerateDisplayCUE(cfg *Config) string {
	display := *cfg
	forceAPIBlock := display.LLM.API.CredentialEnv != ""
	display.LLM.API.CredentialEnv = ""
	return generateCUE(&display, forceAPIBlock)
}
