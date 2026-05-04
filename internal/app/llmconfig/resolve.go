// SPDX-License-Identifier: MPL-2.0

package llmconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

const (
	missingLLMConfigErrMsg = "configure llm.provider or llm.api.*, or specify --llm-provider/--llm"

	// ModeNone means no LLM backend should be used for this request.
	ModeNone Mode = 0
	// ModeProvider means a named provider harness should be detected.
	ModeProvider Mode = 1
	// ModeAPI means an OpenAI-compatible API backend should be used directly.
	ModeAPI Mode = 2
)

// ErrMissingLLMConfig is returned when a use case requires an LLM but neither
// config nor request flags selected a backend.
var ErrMissingLLMConfig = errors.New(missingLLMConfigErrMsg)

type (
	// Mode identifies which LLM backend selection path is active.
	Mode int

	// Defaults contains built-in LLM defaults supplied by the concrete adapter.
	Defaults struct {
		BaseURL     config.LLMBaseURL
		Model       config.LLMModelName
		Timeout     time.Duration
		Concurrency config.LLMConcurrency
	}

	// ChangedFlags records which optional request flags were explicitly set.
	ChangedFlags struct {
		BaseURL     bool
		Model       bool
		APIKey      bool
		Timeout     bool
		Concurrency bool
	}

	// FlagValues contains LLM-related request flags after adapter parsing.
	FlagValues struct {
		Enable      bool
		Provider    config.LLMProvider
		BaseURL     config.LLMBaseURL
		Model       config.LLMModelName
		APIKey      string //goplint:ignore -- transient secret value supplied by CLI/env, never persisted.
		Timeout     time.Duration
		Concurrency config.LLMConcurrency
		Changed     ChangedFlags
	}

	// ResolveOptions configures an LLM resolution request.
	ResolveOptions struct {
		ConfigFilePath       *types.FilesystemPath
		Flags                FlagValues
		UseConfiguredDefault bool
		Defaults             Defaults
		Getenv               func(string) string
	}

	// APIConfig is the resolved OpenAI-compatible API client configuration.
	APIConfig struct {
		BaseURL     config.LLMBaseURL
		Model       config.LLMModelName
		APIKey      string //goplint:ignore -- transient secret value supplied by CLI/env, never persisted.
		Timeout     time.Duration
		Concurrency config.LLMConcurrency
	}

	// Resolved is the typed LLM backend selection for one use case invocation.
	Resolved struct {
		Mode        Mode
		Provider    config.LLMProvider
		Model       config.LLMModelName
		APIConfig   APIConfig
		Concurrency config.LLMConcurrency
	}
)

// Resolve loads configured LLM defaults and applies environment/request
// overrides for one LLM-aware use case.
func Resolve(ctx context.Context, provider config.Loader, opts ResolveOptions) (*Resolved, error) {
	if provider == nil {
		return nil, errors.New("config provider is required")
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	cfg, err := provider.Load(ctx, config.LoadOptions{
		ConfigFilePath: opts.configFilePath(),
	})
	if err != nil {
		return nil, fmt.Errorf("load LLM configuration: %w", err)
	}

	resolved := Resolved{
		APIConfig: APIConfig{
			BaseURL:     opts.Defaults.BaseURL,
			Model:       opts.Defaults.Model,
			Timeout:     opts.Defaults.Timeout,
			Concurrency: opts.Defaults.Concurrency,
		},
	}
	applyConfigDefaults(&resolved, cfg.LLM, opts.getenv())

	switch {
	case opts.Flags.Provider != "":
		resolved.Mode = ModeProvider
		resolved.Provider = opts.Flags.Provider
	case opts.Flags.Enable:
		if resolved.Mode == ModeNone {
			resolved.Mode = ModeAPI
		}
	case opts.UseConfiguredDefault && resolved.Mode != ModeNone:
		// Use configured default mode as-is.
	case opts.UseConfiguredDefault:
		return nil, ErrMissingLLMConfig
	default:
		return &Resolved{Mode: ModeNone}, nil
	}

	if err := applyEnvOverrides(&resolved, opts.Flags.Changed, opts.getenv()); err != nil {
		return nil, err
	}
	applyFlagOverrides(&resolved, opts.Flags)

	concurrency := normalizedConcurrency(resolved.APIConfig.Concurrency, opts.Defaults.Concurrency)
	if err := concurrency.Validate(); err != nil {
		return nil, err
	}
	resolved.Concurrency = concurrency
	if err := resolved.Validate(); err != nil {
		return nil, err
	}
	return &resolved, nil
}

// Validate returns nil when the default adapter configuration is valid.
func (d Defaults) Validate() error {
	var errs []error
	errs = append(errs,
		d.BaseURL.Validate(),
		d.Model.Validate(),
		d.Concurrency.Validate(),
	)
	if d.Timeout < 0 {
		errs = append(errs, errors.New("default LLM timeout must be non-negative"))
	}
	return errors.Join(errs...)
}

// Validate returns nil when explicitly typed flag values are valid.
func (f FlagValues) Validate() error {
	var errs []error
	errs = append(errs,
		f.Provider.Validate(),
		f.BaseURL.Validate(),
		f.Model.Validate(),
		f.Concurrency.Validate(),
	)
	if f.Timeout < 0 {
		errs = append(errs, errors.New("LLM flag timeout must be non-negative"))
	}
	return errors.Join(errs...)
}

// Validate returns nil when LLM resolution options are valid.
func (o ResolveOptions) Validate() error {
	var errs []error
	if o.ConfigFilePath != nil {
		errs = append(errs, o.ConfigFilePath.Validate())
	}
	errs = append(errs, o.Flags.Validate(), o.Defaults.Validate())
	return errors.Join(errs...)
}

// String returns a stable display label for the mode.
func (m Mode) String() string {
	switch m {
	case ModeNone:
		return "none"
	case ModeProvider:
		return "provider"
	case ModeAPI:
		return "api"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

// Validate returns nil for known LLM resolution modes.
func (m Mode) Validate() error {
	switch m {
	case ModeNone, ModeProvider, ModeAPI:
		return nil
	default:
		return fmt.Errorf("invalid LLM mode %d", m)
	}
}

// Validate returns nil when the resolved selection is internally consistent.
func (r Resolved) Validate() error {
	var errs []error
	if err := r.Mode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.Provider.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.Model.Validate(); err != nil {
		errs = append(errs, err)
	}
	if r.Mode == ModeAPI {
		if err := r.APIConfig.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := r.Concurrency.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Validate returns nil when the resolved API config can construct a client.
func (c APIConfig) Validate() error {
	var errs []error
	if err := c.BaseURL.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Model.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Concurrency.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.BaseURL == "" {
		errs = append(errs, errors.New("LLM API base URL is required"))
	}
	if c.Model == "" {
		errs = append(errs, errors.New("LLM API model is required"))
	}
	if c.Timeout < 0 {
		errs = append(errs, errors.New("LLM API timeout must be non-negative"))
	}
	return errors.Join(errs...)
}

func (o ResolveOptions) getenv() func(string) string {
	if o.Getenv != nil {
		return o.Getenv
	}
	return os.Getenv
}

func (o ResolveOptions) configFilePath() types.FilesystemPath {
	if o.ConfigFilePath == nil {
		return ""
	}
	return *o.ConfigFilePath
}

func applyConfigDefaults(result *Resolved, llm config.LLMConfig, getenv func(string) string) {
	if llm.Provider != "" {
		result.Mode = ModeProvider
		result.Provider = llm.Provider
	}
	if llm.Model != "" {
		result.Model = llm.Model
		result.APIConfig.Model = llm.Model
	}
	if llm.Timeout != "" {
		if timeout, err := llm.Timeout.Duration(); err == nil {
			result.APIConfig.Timeout = timeout
		}
	}
	if llm.Concurrency != 0 {
		result.APIConfig.Concurrency = llm.Concurrency
	}
	if llm.API.HasConfig() {
		result.Mode = ModeAPI
		result.Provider = ""
		if llm.API.BaseURL != "" {
			result.APIConfig.BaseURL = llm.API.BaseURL
		}
		if llm.API.Model != "" {
			result.APIConfig.Model = llm.API.Model
		}
		if llm.API.APIKeyEnv != "" {
			result.APIConfig.APIKey = getenv(llm.API.APIKeyEnv.String())
		}
	}
}

func applyEnvOverrides(result *Resolved, changed ChangedFlags, getenv func(string) string) error {
	if !changed.BaseURL {
		if envURL := getenv("INVOWK_LLM_URL"); envURL != "" {
			baseURL := config.LLMBaseURL(envURL)
			if err := baseURL.Validate(); err != nil {
				return err
			}
			result.APIConfig.BaseURL = baseURL
		}
	}
	if !changed.Model {
		if envModel := getenv("INVOWK_LLM_MODEL"); envModel != "" {
			model := config.LLMModelName(envModel)
			if err := model.Validate(); err != nil {
				return err
			}
			result.Model = model
			result.APIConfig.Model = model
		}
	}
	if !changed.APIKey && result.APIConfig.APIKey == "" {
		if envKey := getenv("INVOWK_LLM_API_KEY"); envKey != "" {
			result.APIConfig.APIKey = envKey
		}
	}
	if !changed.Timeout {
		if envTimeout := getenv("INVOWK_LLM_TIMEOUT"); envTimeout != "" {
			if d, err := time.ParseDuration(envTimeout); err == nil {
				result.APIConfig.Timeout = d
			}
		}
	}
	if !changed.Concurrency {
		if envConc := getenv("INVOWK_LLM_CONCURRENCY"); envConc != "" {
			if n, err := strconv.Atoi(envConc); err == nil {
				concurrency := config.LLMConcurrency(n)
				if validateErr := concurrency.Validate(); validateErr != nil {
					return validateErr
				}
				result.APIConfig.Concurrency = concurrency
			}
		}
	}
	return nil
}

func applyFlagOverrides(result *Resolved, flags FlagValues) {
	if flags.Changed.BaseURL {
		result.APIConfig.BaseURL = flags.BaseURL
	}
	if flags.Changed.Model {
		result.Model = flags.Model
		result.APIConfig.Model = flags.Model
	}
	if flags.Changed.APIKey {
		result.APIConfig.APIKey = flags.APIKey
	}
	if flags.Changed.Timeout {
		result.APIConfig.Timeout = flags.Timeout
	}
	if flags.Changed.Concurrency {
		result.APIConfig.Concurrency = flags.Concurrency
	}
}

func normalizedConcurrency(concurrency, fallback config.LLMConcurrency) config.LLMConcurrency {
	if concurrency == 0 {
		return fallback
	}
	return concurrency
}
