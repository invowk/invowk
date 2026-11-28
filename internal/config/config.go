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
	"sync"

	"cuelang.org/go/cue"

	"github.com/invowk/invowk/internal/basedir"
	"github.com/invowk/invowk/pkg/cueutil"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
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
	// Compile the embedded config schema lazily once so repeated config loads skip
	// schema and stdlib ("strings") recompilation without adding init-time cost.
	compiledConfigSchema = sync.OnceValues(func() (*cueutil.CompiledSchema, error) {
		return cueutil.CompileSchema([]byte(configSchema), "#Config")
	})

	// ErrConfigFileNotFound marks an explicit config path that does not exist.
	ErrConfigFileNotFound = errors.New("config file not found")
	// ErrConfigLoadFailed marks config CUE read/parse/decode failures.
	ErrConfigLoadFailed = errors.New("config load failed")
	// ErrBaseDirNotAbsolute marks a relative home directory or USERPROFILE.
	// invowk never derives the user config or commands directory from one,
	// because it would resolve under the current directory. It is the shared
	// resolver's sentinel, so errors.Is matches either name.
	ErrBaseDirNotAbsolute = basedir.ErrNotAbsolute
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
//
// Every base directory must be absolute, so the user config file can never
// resolve under the current directory. The shared resolver (internal/basedir)
// ignores a relative XDG_CONFIG_HOME or APPDATA, as the XDG Base Directory spec
// requires, and uses the next base; a relative home directory or USERPROFILE
// is an ErrBaseDirNotAbsolute error.
func configDirFrom(goos string, getenv func(string) string, userHomeDir func() (string, error)) (types.FilesystemPath, error) {
	configHome, err := basedir.New(goos, getenv, userHomeDir).ConfigHome()
	if err != nil {
		return "", err
	}
	return types.FilesystemPath(filepath.Join(string(configHome), AppName)), nil //goplint:ignore -- derived from an absolute OS config directory and fixed app name.
}

// ConfigDir returns the invowk configuration directory using platform-specific
// conventions: Windows uses %APPDATA%, macOS uses ~/Library/Application Support,
// and Linux/others use $XDG_CONFIG_HOME (defaulting to ~/.config).
//
//nolint:revive // ConfigDir is more descriptive than Dir for external callers
func ConfigDir() (types.FilesystemPath, error) {
	// A nil home lookup makes the shared resolver use the real process home.
	return configDirFrom(runtime.GOOS, os.Getenv, nil)
}

// CommandsDir returns the directory for user-defined invowkfiles.
// The path is ~/.invowk/cmds on all platforms; a relative home directory is an
// ErrBaseDirNotAbsolute error.
func CommandsDir() (types.FilesystemPath, error) {
	return commandsDirFrom(nil) // nil: the real process home (internal/basedir)
}

// commandsDirFrom is CommandsDir with an injectable home-directory lookup.
func commandsDirFrom(userHomeDir func() (string, error)) (types.FilesystemPath, error) {
	home, err := basedir.New(runtime.GOOS, os.Getenv, userHomeDir).Home()
	if err != nil {
		return "", err
	}
	return types.FilesystemPath(filepath.Join(string(home), ".invowk", "cmds")), nil //goplint:ignore -- derived from an absolute OS home directory and fixed config path.
}

// HostKeyPath returns the private host key location for the built-in SSH
// host-callback server. The key is generated there on first use.
// The path is <config-dir>/ssh/host_ed25519 on all platforms.
func HostKeyPath() (types.FilesystemPath, error) {
	cfgDir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return types.FilesystemPath(filepath.Join(string(cfgDir), "ssh", "host_ed25519")), nil //goplint:ignore -- derived from OS config directory and fixed app path.
}

// loadWithOptions loads the source chosen by SelectSource. Each call reads
// and parses configuration from disk with no caching.
//
// Precedence and isolation: an explicit selection (flag or INVOWK_CONFIG) is
// loaded on its own and the user config directory is never resolved or
// stat'ed. Only an implicit selection looks at <ConfigDir>/config.cue, and
// when that file is absent the built-in defaults apply. The current working
// directory is never a config source.
func loadWithOptions(ctx context.Context, opts LoadOptions, selection SourceSelection) (*Config, types.FilesystemPath, ConfigSourceKind, error) {
	select {
	case <-ctx.Done():
		return nil, "", "", fmt.Errorf("load config canceled: %w", ctx.Err())
	default:
	}

	cfg := DefaultConfig()
	var resolvedPath types.FilesystemPath
	source := ConfigSourceDefault

	switch selection.Kind {
	case ConfigSourceFlag, ConfigSourceEnv:
		configFilePath := string(selection.Path)
		if !fileExists(configFilePath) {
			return nil, "", "", &FileNotFoundError{Path: selection.Path}
		}
		loaded, err := decodeCUEConfigFile(selection.Path)
		if err != nil {
			return nil, "", "", cueLoadError(configFilePath, err)
		}
		cfg = loaded
		resolvedPath = selection.Path
		source = selection.Kind
	case ConfigSourceImplicit, ConfigSourceUser, ConfigSourceDefault:
		cfgDir, err := configDirWithOverride(opts.ConfigDirPath)
		if err != nil {
			return nil, "", "", err
		}

		cuePath := filepath.Join(string(cfgDir), ConfigFileName+"."+ConfigFileExt)
		if fileExists(cuePath) {
			resolved := types.FilesystemPath(cuePath)
			if err := resolved.Validate(); err != nil {
				return nil, "", "", fmt.Errorf("resolved config path: %w", err)
			}
			loaded, err := decodeCUEConfigFile(resolved)
			if err != nil {
				return nil, "", "", cueLoadError(cuePath, err)
			}
			cfg = loaded
			resolvedPath = resolved
			source = ConfigSourceUser
		}
		// No user config file: built-in defaults apply (no error).
	default:
		return nil, "", "", selection.Kind.Validate()
	}

	// Defense-in-depth: validate all typed fields after unmarshalling.
	// CUE schema is the primary validation layer; this catches any gaps
	// or values that bypass CUE.
	if err := cfg.Validate(); err != nil {
		if includeErr, ok := errors.AsType[*InvalidIncludeCollectionError](err); ok {
			return nil, "", "", wrapIncludeCollectionError(includeErr)
		}
		return nil, "", "", err
	}

	return cfg, resolvedPath, source, nil
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
// This is the common error path for both config file locations (explicit
// path and user config dir).
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
	schema, err := compiledConfigSchema()
	if err != nil {
		return nil, err
	}

	parsed, err := cueutil.ParseAndDecodeCompiled[Config](
		schema,
		[]byte(source.data),
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

		// Track short name for collision detection. Path validation already
		// checked that the basename is a structurally valid module directory.
		shortName := strings.TrimSuffix(filepath.Base(pathStr), invowkmod.ModuleSuffix)
		shortNames[shortName] = append(shortNames[shortName], i)

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
		writeIncludeEntryCUE(sb, "\t", entry)
	}
	sb.WriteString("]\n")
}

//goplint:ignore -- CUE rendering helper taking indentation string.
func writeIncludeEntryCUE(sb *strings.Builder, indent string, entry IncludeEntry) {
	sb.WriteString(indent + "{path: ")
	fmt.Fprintf(sb, "%q", entry.Path)
	if entry.Alias != "" {
		fmt.Fprintf(sb, ", alias: %q", entry.Alias)
	}
	if entry.WorkspaceRoot != "" {
		fmt.Fprintf(sb, ", workspace_root: %q", entry.WorkspaceRoot)
	}
	sb.WriteString("},\n")
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
		writeIncludeEntryCUE(sb, "\t\t\t", entry)
	}
	sb.WriteString("\t\t]\n")
	fmt.Fprintf(sb, "\t\tinherit_includes: %v\n", container.AutoProvision.InheritIncludes)
	fmt.Fprintf(sb, "\t\tcache_dir: %q\n", container.AutoProvision.CacheDir)
	sb.WriteString("\t}\n")
	if container.TUIServerBind != "" {
		fmt.Fprintf(sb, "\ttui_server_bind: %q\n", container.TUIServerBind)
	}
	if container.SELinuxLabeling != "" {
		fmt.Fprintf(sb, "\tselinux_labeling: %q\n", container.SELinuxLabeling)
	}
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
