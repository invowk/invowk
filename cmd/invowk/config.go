// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/types"
)

const (
	configCommandName = "config"
	configFieldFmt    = "%s: %s\n"
	configFileLabel   = "Config file"
	validConfigKeys   = "container_engine, default_runtime, ui.verbose, ui.interactive, ui.color_scheme, virtual.utilities.enabled, llm.provider, llm.model, llm.timeout, llm.concurrency, llm.api.base_url, llm.api.model, llm.api.api_key_env"
)

var configSetters = map[string]configSetter{
	"container_engine":          setContainerEngineConfig,
	"default_runtime":           setDefaultRuntimeConfig,
	"ui.verbose":                setUIVerboseConfig,
	"ui.interactive":            setUIInteractiveConfig,
	"ui.color_scheme":           setUIColorSchemeConfig,
	"virtual.utilities.enabled": setVirtualUtilitiesEnabledConfig,
	"llm.provider":              setLLMProviderConfig,
	"llm.model":                 setLLMModelConfig,
	"llm.timeout":               setLLMTimeoutConfig,
	"llm.concurrency":           setLLMConcurrencyConfig,
	"llm.api.base_url":          setLLMAPIBaseURLConfig,
	"llm.api.model":             setLLMAPIModelConfig,
	"llm.api.api_key_env":       setLLMAPIKeyEnvConfig,
}

type (
	configSetter func(*config.Config, configValue) error
	configValue  string
)

// newConfigCommand creates the `invowk config` command tree.
// Subcommands that read configuration use the App's config.Provider.
func newConfigCommand(app *App, rootFlags *rootFlagValues) *cobra.Command {
	cfgCmd := &cobra.Command{
		Use:   configCommandName,
		Short: "Manage invowk configuration",
		Long: `Manage invowk configuration.

invowk reads exactly one config source, in this order:
  1. --ivk-config <path>        (may be relative to the current directory)
  2. INVOWK_CONFIG=<path>       (must be an absolute path; empty means unset)
  3. The user config file:
     - Linux: $XDG_CONFIG_HOME/invowk/config.cue, or ~/.config/invowk/config.cue
     - macOS: ~/Library/Application Support/invowk/config.cue
     - Windows: %APPDATA%\invowk\config.cue
  4. Built-in defaults

The current directory is never a config source. A missing or invalid file
named by --ivk-config or INVOWK_CONFIG is an error (exit 1); an invalid user
config file falls back to defaults with a diagnostic.

'config show', 'config dump' and 'config path' honour --ivk-config and
INVOWK_CONFIG. 'config set' and 'config init' only touch the user config file,
and 'config set' refuses while --ivk-config or INVOWK_CONFIG is active.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return showConfig(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), app, rootFlags)
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration file",
		RunE: func(_ *cobra.Command, _ []string) error {
			return initConfig()
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Show configuration file path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return showConfigPath(cmd.OutOrStdout(), app, rootFlags)
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return setConfigValue(cmd.Context(), cmd.OutOrStdout(), app, rootFlags, args[0], args[1])
		},
	})

	cfgCmd.AddCommand(&cobra.Command{
		Use:   "dump",
		Short: "Output raw configuration as CUE",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := app.Config.Load(cmd.Context(), explicitLoadOptions(rootFlags))
			if err != nil {
				return renderExplicitConfigError(cmd.ErrOrStderr(), err)
			}

			cueContent := config.GenerateDisplayCUE(cfg)
			fmt.Fprint(cmd.OutOrStdout(), cueContent)
			return nil
		},
	})

	return cfgCmd
}

// explicitLoadOptions builds the standard-precedence load options for config
// readers: --ivk-config when given, otherwise the provider applies
// INVOWK_CONFIG, then the user file, then defaults.
func explicitLoadOptions(rootFlags *rootFlagValues) config.LoadOptions {
	return config.LoadOptions{ConfigFilePath: types.FilesystemPath(rootFlags.configPath)} //goplint:ignore -- CLI flag value, may be empty; validated by the provider.
}

func showConfig(ctx context.Context, w, stderr io.Writer, app *App, rootFlags *rootFlagValues) error {
	result, err := app.Config.LoadWithSource(ctx, explicitLoadOptions(rootFlags))
	if err != nil {
		if errors.Is(err, config.ErrExplicitConfig) {
			return renderExplicitConfigError(stderr, err)
		}
		return renderConfigLoadFailure(stderr, err)
	}
	cfg := result.Config

	// Style definitions using shared color palette
	headerStyle := TitleStyle
	keyStyle := CmdStyle
	valueStyle := SuccessStyle

	fmt.Fprintln(w, headerStyle.Render("Current Configuration"))
	fmt.Fprintln(w)

	printConfigFileLine(w, keyStyle, result)
	fmt.Fprintln(w)

	// Show values
	fmt.Fprintf(w, configFieldFmt, keyStyle.Render("container_engine"), valueStyle.Render(string(cfg.ContainerEngine)))
	fmt.Fprintf(w, configFieldFmt, keyStyle.Render("default_runtime"), valueStyle.Render(string(cfg.DefaultRuntime)))

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s:\n", keyStyle.Render("includes"))
	printIncludes(w, cfg.Includes, valueStyle)

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s:\n", keyStyle.Render("virtual"))
	fmt.Fprintln(w, "  utilities:")
	fmt.Fprintf(w, "    enabled: %s\n", valueStyle.Render(strconv.FormatBool(cfg.Virtual.Utilities.Enabled)))

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s:\n", keyStyle.Render("ui"))
	fmt.Fprintf(w, "  color_scheme: %s\n", valueStyle.Render(string(cfg.UI.ColorScheme)))
	fmt.Fprintf(w, "  interactive: %s\n", valueStyle.Render(strconv.FormatBool(cfg.UI.Interactive)))
	fmt.Fprintf(w, "  verbose: %s\n", valueStyle.Render(strconv.FormatBool(cfg.UI.Verbose)))

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s:\n", keyStyle.Render("llm"))
	printLLMConfig(w, cfg.LLM, valueStyle)

	return nil
}

func renderConfigLoadFailure(w io.Writer, err error) error {
	rendered, renderErr := renderIssueCatalogEntry(issue.Get(issue.ConfigLoadFailedId), "dark")
	if renderErr != nil {
		slog.Warn("failed to render issue catalog entry", "issue_id", issue.ConfigLoadFailedId, "error", renderErr)
	}
	fmt.Fprint(w, rendered)
	return err
}

// printConfigFileLine names the effective config file and its source:
// "(from --ivk-config)", "(from INVOWK_CONFIG)", "(user config)", or defaults.
func printConfigFileLine(w io.Writer, keyStyle lipgloss.Style, result config.LoadResult) {
	if result.SourcePath == "" {
		fmt.Fprintf(w, configFieldFmt, keyStyle.Render(configFileLabel), SubtitleStyle.Render("(using defaults)"))
		return
	}
	fmt.Fprintf(w, configFieldFmt, keyStyle.Render(configFileLabel), fmt.Sprintf("%s %s", result.SourcePath, SubtitleStyle.Render(configSourceSuffix(result.Source))))
}

//plint:render
func configSourceSuffix(source config.ConfigSourceKind) string {
	switch source {
	case config.ConfigSourceFlag, config.ConfigSourceEnv:
		return "(from " + source.Label() + ")"
	case config.ConfigSourceUser:
		return "(user config)"
	case config.ConfigSourceDefault, config.ConfigSourceImplicit:
		return "(defaults)"
	default:
		return ""
	}
}

func printIncludes(w io.Writer, includes []config.IncludeEntry, valueStyle lipgloss.Style) {
	if len(includes) == 0 {
		fmt.Fprintf(w, "  %s\n", SubtitleStyle.Render("(none configured)"))
		return
	}
	for _, inc := range includes {
		if inc.Alias != "" {
			fmt.Fprintf(w, "  - %s (alias: %s)\n", valueStyle.Render(string(inc.Path)), valueStyle.Render(string(inc.Alias)))
			continue
		}
		fmt.Fprintf(w, "  - %s\n", valueStyle.Render(string(inc.Path)))
	}
}

func printLLMConfig(w io.Writer, llm config.LLMConfig, valueStyle lipgloss.Style) {
	if !llm.HasConfig() {
		fmt.Fprintf(w, "  %s\n", SubtitleStyle.Render("(none configured)"))
		return
	}
	if llm.Provider != "" {
		fmt.Fprintf(w, "  provider: %s\n", valueStyle.Render(string(llm.Provider)))
	}
	if llm.Model != "" {
		fmt.Fprintf(w, "  model: %s\n", valueStyle.Render(string(llm.Model)))
	}
	if llm.Timeout != "" {
		fmt.Fprintf(w, "  timeout: %s\n", valueStyle.Render(string(llm.Timeout)))
	}
	if llm.Concurrency != 0 {
		fmt.Fprintf(w, "  concurrency: %s\n", valueStyle.Render(llm.Concurrency.String()))
	}
	if llm.API.HasConfig() {
		fmt.Fprintln(w, "  api:")
		if llm.API.BaseURL != "" {
			fmt.Fprintf(w, "    base_url: %s\n", valueStyle.Render(string(llm.API.BaseURL)))
		}
		if llm.API.Model != "" {
			fmt.Fprintf(w, "    model: %s\n", valueStyle.Render(string(llm.API.Model)))
		}
		if llm.API.CredentialEnv != "" {
			fmt.Fprintf(w, "    api_key_env: %s\n", valueStyle.Render("(configured)"))
		}
	}
}

func initConfig() error {
	w := os.Stdout
	// Check if config exists
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	if err = config.CreateDefaultConfig(""); err != nil {
		return fmt.Errorf("failed to create config: %w", err)
	}

	fmt.Fprintf(w, "%s Created default configuration at %s/config.cue\n", SuccessStyle.Render("✓"), cfgDir)

	// Also create commands directory
	cmdsDir, err := config.CommandsDir()
	if err == nil {
		if mkdirErr := config.EnsureCommandsDir(""); mkdirErr != nil {
			slog.Warn("failed to create commands directory", "path", cmdsDir, "error", mkdirErr)
		} else {
			fmt.Fprintf(w, "%s Created commands directory at %s\n", SuccessStyle.Render("✓"), cmdsDir)
		}
	} else {
		slog.Warn("failed to determine commands directory", "error", err)
	}

	return nil
}

// showConfigPath prints the config directory and the user config file, plus
// the effective explicit file when --ivk-config or INVOWK_CONFIG selects one.
// It never loads or validates that file: a missing file or a relative
// INVOWK_CONFIG is marked in the output and the command still exits 0.
func showConfigPath(w io.Writer, app *App, rootFlags *rootFlagValues) error {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Config directory: %s\n", cfgDir)
	fmt.Fprintf(w, "Config file: %s/config.cue\n", cfgDir)

	if selection := config.SelectSource(explicitLoadOptions(rootFlags), app.Getenv); selection.IsExplicit() {
		fmt.Fprintf(w, "Effective config file: %s (from %s)%s\n", selection.Path, selection.Kind.Label(), effectiveConfigMarker(selection))
	}

	cmdsDir, err := config.CommandsDir()
	if err == nil {
		fmt.Fprintf(w, "Commands directory: %s\n", cmdsDir)
	}

	hostKeyPath, err := config.HostKeyPath()
	if err == nil {
		fmt.Fprintf(w, "SSH host key: %s\n", hostKeyPath)
	}

	return nil
}

// setConfigValue edits the user config file only. While --ivk-config or
// INVOWK_CONFIG is active it refuses before reading or writing any file;
// otherwise loading the user config and saving it back would copy the explicit
// source's values into the user file.
func setConfigValue(ctx context.Context, w io.Writer, app *App, rootFlags *rootFlagValues, key, value string) error {
	if selection := config.SelectSource(explicitLoadOptions(rootFlags), app.Getenv); selection.IsExplicit() {
		return configSetRefusal(selection)
	}

	cfg, err := app.Config.Load(ctx, config.LoadOptions{})
	if err != nil {
		return err
	}

	setter, ok := configSetters[key]
	if !ok {
		return fmt.Errorf("unknown configuration key: %s\nValid keys: %s", key, validConfigKeys)
	}
	rawValue := configValue(value)
	if err := rawValue.Validate(); err != nil {
		return err
	}
	if err := setter(cfg, rawValue); err != nil {
		return err
	}

	if err := config.Save(cfg, ""); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Fprintf(w, "%s Set %s = %s\n", SuccessStyle.Render("✓"), key, value)
	return nil
}

// effectiveConfigMarker flags an explicit selection that would fail to load,
// using only a stat: " [invalid: must be absolute]" or " [missing]".
//
//plint:render
func effectiveConfigMarker(selection config.SourceSelection) string {
	if selection.Problem != nil {
		return " [invalid: must be absolute]"
	}
	if info, err := os.Stat(string(selection.Path)); err != nil || info.IsDir() {
		return " [missing]"
	}
	return ""
}

func configSetRefusal(selection config.SourceSelection) error {
	userFile := "the user config file"
	if cfgDir, err := config.ConfigDir(); err == nil {
		userFile = filepath.Join(string(cfgDir), config.ConfigFileName+"."+config.ConfigFileExt)
	}
	remedy := "drop " + config.ConfigFlagName
	if selection.Kind == config.ConfigSourceEnv {
		remedy = "unset " + config.EnvConfigPath
	}
	return fmt.Errorf("config set edits the user config file %s, but %s selects %s; %s or edit that file directly",
		userFile, selection.Kind.Label(), selection.Path, remedy)
}

func (v configValue) String() string { return string(v) }

func (v configValue) Validate() error { return nil }

func setContainerEngineConfig(cfg *config.Config, value configValue) error {
	ce := config.ContainerEngine(value)
	if err := ce.Validate(); err != nil {
		return err
	}
	cfg.ContainerEngine = ce
	return nil
}

func setDefaultRuntimeConfig(cfg *config.Config, value configValue) error {
	rm := config.RuntimeMode(value)
	if err := rm.Validate(); err != nil {
		return err
	}
	cfg.DefaultRuntime = rm
	return nil
}

func setUIVerboseConfig(cfg *config.Config, value configValue) error {
	cfg.UI.Verbose = value == "true" || value == "1"
	return nil
}

func setUIInteractiveConfig(cfg *config.Config, value configValue) error {
	cfg.UI.Interactive = value == "true" || value == "1"
	return nil
}

func setUIColorSchemeConfig(cfg *config.Config, value configValue) error {
	cs := config.ColorScheme(value)
	if err := cs.Validate(); err != nil {
		return err
	}
	cfg.UI.ColorScheme = cs
	return nil
}

func setVirtualUtilitiesEnabledConfig(cfg *config.Config, value configValue) error {
	cfg.Virtual.Utilities.Enabled = value == "true" || value == "1"
	return nil
}

func setLLMProviderConfig(cfg *config.Config, value configValue) error {
	provider := config.LLMProvider(value)
	if err := provider.Validate(); err != nil {
		return err
	}
	cfg.LLM.Provider = provider
	cfg.LLM = cfg.LLM.WithoutAPIBackend()
	return nil
}

func setLLMModelConfig(cfg *config.Config, value configValue) error {
	model := config.LLMModelName(value)
	if err := model.Validate(); err != nil {
		return err
	}
	cfg.LLM.Model = model
	return nil
}

func setLLMTimeoutConfig(cfg *config.Config, value configValue) error {
	timeout := config.LLMTimeout(value)
	if err := timeout.Validate(); err != nil {
		return err
	}
	cfg.LLM.Timeout = timeout
	return nil
}

func setLLMConcurrencyConfig(cfg *config.Config, value configValue) error {
	parsed, parseErr := strconv.Atoi(string(value))
	if parseErr != nil {
		return fmt.Errorf("invalid llm.concurrency %q: %w", value, parseErr)
	}
	concurrency := config.LLMConcurrency(parsed)
	if err := concurrency.Validate(); err != nil {
		return err
	}
	cfg.LLM.Concurrency = concurrency
	return nil
}

func setLLMAPIBaseURLConfig(cfg *config.Config, value configValue) error {
	baseURL := config.LLMBaseURL(value)
	if err := baseURL.Validate(); err != nil {
		return err
	}
	cfg.LLM.Provider = ""
	cfg.LLM = cfg.LLM.WithAPIBackendPresent()
	cfg.LLM.API.BaseURL = baseURL
	return nil
}

func setLLMAPIModelConfig(cfg *config.Config, value configValue) error {
	model := config.LLMModelName(value)
	if err := model.Validate(); err != nil {
		return err
	}
	cfg.LLM.Provider = ""
	cfg.LLM = cfg.LLM.WithAPIBackendPresent()
	cfg.LLM.API.Model = model
	return nil
}

func setLLMAPIKeyEnvConfig(cfg *config.Config, value configValue) error {
	keyEnv := config.LLMCredentialEnvVar(value)
	if err := keyEnv.Validate(); err != nil {
		return err
	}
	cfg.LLM.Provider = ""
	cfg.LLM = cfg.LLM.WithAPIBackendPresent()
	cfg.LLM.API.CredentialEnv = keyEnv
	return nil
}
