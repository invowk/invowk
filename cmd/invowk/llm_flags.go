// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/internal/auditllm"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

const (
	llmFlagModel = "llm-model"

	llmModeNone     llmMode = 0
	llmModeProvider llmMode = 1
	llmModeAPI      llmMode = 2
)

type (
	llmMode int

	llmFlagValues struct {
		enable      bool
		provider    string
		url         string
		model       string
		apiKey      string
		timeout     time.Duration
		concurrency int
	}

	llmCompleterResult struct {
		completer   audit.LLMCompleter
		concurrency int
	}

	llmResolvedConfig struct {
		mode        llmMode
		provider    config.LLMProvider
		model       config.LLMModelName
		apiConfig   auditllm.LLMClientConfig
		concurrency config.LLMConcurrency
	}
)

func bindLLMFlags(cmd *cobra.Command, flags *llmFlagValues) {
	cmd.Flags().StringVar(&flags.provider, "llm-provider", "", "auto-detect or use specific provider: auto, claude, codex, gemini, ollama")
	cmd.Flags().BoolVar(&flags.enable, "llm", false, "enable LLM using configured or OpenAI-compatible API settings")
	cmd.Flags().StringVar(&flags.url, "llm-url", auditllm.DefaultLLMBaseURL, "OpenAI-compatible API base URL")
	cmd.Flags().StringVar(&flags.model, llmFlagModel, auditllm.DefaultLLMModel, "LLM model name; required for cloud API providers and optional for CLI providers")
	cmd.Flags().StringVar(&flags.apiKey, "llm-api-key", "", "API key (empty for local servers)")
	cmd.Flags().DurationVar(&flags.timeout, "llm-timeout", auditllm.DefaultLLMTimeout, "per-request timeout")
	cmd.Flags().IntVar(&flags.concurrency, "llm-concurrency", audit.DefaultLLMConcurrency, "max parallel LLM requests")
}

func validateLLMFlagSelection(flags llmFlagValues) error {
	if flags.enable && flags.provider != "" {
		return errors.New("--llm and --llm-provider are mutually exclusive")
	}
	if flags.provider == "" {
		return nil
	}
	validProviders := auditllm.ValidProviders()
	if !slices.Contains(validProviders, flags.provider) {
		return fmt.Errorf("invalid --llm-provider %q; valid: %s", flags.provider, strings.Join(validProviders, ", "))
	}
	return nil
}

func (m llmMode) String() string {
	switch m {
	case llmModeNone:
		return "none"
	case llmModeProvider:
		return "provider"
	case llmModeAPI:
		return "api"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

func (m llmMode) Validate() error {
	switch m {
	case llmModeNone, llmModeProvider, llmModeAPI:
		return nil
	default:
		return fmt.Errorf("invalid LLM mode %d", m)
	}
}

func (c llmResolvedConfig) Validate() error {
	var errs []error
	if err := c.mode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.provider.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.model.Validate(); err != nil {
		errs = append(errs, err)
	}
	if c.mode == llmModeAPI {
		if err := c.apiConfig.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.concurrency.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func resolveLLMForCommand(
	ctx context.Context,
	cmd *cobra.Command,
	provider config.Provider,
	configPath types.FilesystemPath,
	flags llmFlagValues,
	useConfiguredDefault bool,
) (*llmResolvedConfig, *ExitError) {
	cfg, err := provider.Load(ctx, config.LoadOptions{
		ConfigFilePath: configPath,
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "failed to load LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	result := llmResolvedConfig{
		apiConfig: auditllm.LLMClientConfig{
			BaseURL:     auditllm.DefaultLLMBaseURL,
			Model:       auditllm.DefaultLLMModel,
			Timeout:     auditllm.DefaultLLMTimeout,
			Concurrency: auditllm.DefaultLLMConcurrency,
		},
	}
	applyLLMConfigDefaults(&result, cfg.LLM)

	switch {
	case flags.provider != "":
		providerValue := config.LLMProvider(flags.provider)
		if err := providerValue.Validate(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
			return nil, &ExitError{Code: auditExitError, Err: err}
		}
		result.mode = llmModeProvider
		result.provider = providerValue
	case flags.enable:
		if result.mode == llmModeNone {
			result.mode = llmModeAPI
		}
	case useConfiguredDefault && result.mode != llmModeNone:
		// Use configured default mode as-is.
	case useConfiguredDefault:
		err := errors.New("configure llm.provider or llm.api.*, or specify --llm-provider/--llm")
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	default:
		return &llmResolvedConfig{mode: llmModeNone}, nil
	}

	if err := applyLLMEnvOverrides(cmd, &result); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}
	if err := applyLLMFlagOverrides(cmd, flags, &result); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}
	concurrency := config.LLMConcurrency(normalizedLLMConcurrency(result.apiConfig.Concurrency))
	if err := concurrency.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}
	result.concurrency = concurrency
	if err := result.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}
	return &result, nil
}

func applyLLMConfigDefaults(result *llmResolvedConfig, llm config.LLMConfig) {
	if llm.Provider != "" {
		result.mode = llmModeProvider
		result.provider = llm.Provider
	}
	if llm.Model != "" {
		result.model = llm.Model
		result.apiConfig.Model = llm.Model.String()
	}
	if llm.Timeout != "" {
		if timeout, err := llm.Timeout.Duration(); err == nil {
			result.apiConfig.Timeout = timeout
		}
	}
	if llm.Concurrency != 0 {
		result.apiConfig.Concurrency = int(llm.Concurrency)
	}
	if llm.API.HasConfig() {
		result.mode = llmModeAPI
		result.provider = ""
		if llm.API.BaseURL != "" {
			result.apiConfig.BaseURL = llm.API.BaseURL.String()
		}
		if llm.API.Model != "" {
			result.apiConfig.Model = llm.API.Model.String()
		}
		if llm.API.APIKeyEnv != "" {
			result.apiConfig.APIKey = os.Getenv(llm.API.APIKeyEnv.String())
		}
	}
}

func applyLLMEnvOverrides(cmd *cobra.Command, result *llmResolvedConfig) error {
	if !cmd.Flags().Changed("llm-url") {
		if envURL := os.Getenv("INVOWK_LLM_URL"); envURL != "" {
			result.apiConfig.BaseURL = envURL
		}
	}
	if !cmd.Flags().Changed(llmFlagModel) {
		if envModel := os.Getenv("INVOWK_LLM_MODEL"); envModel != "" {
			model := config.LLMModelName(envModel)
			if err := model.Validate(); err != nil {
				return err
			}
			result.model = model
			result.apiConfig.Model = envModel
		}
	}
	if !cmd.Flags().Changed("llm-api-key") && result.apiConfig.APIKey == "" {
		if envKey := os.Getenv("INVOWK_LLM_API_KEY"); envKey != "" {
			result.apiConfig.APIKey = envKey
		}
	}
	if !cmd.Flags().Changed("llm-timeout") {
		if envTimeout := os.Getenv("INVOWK_LLM_TIMEOUT"); envTimeout != "" {
			if d, err := time.ParseDuration(envTimeout); err == nil {
				result.apiConfig.Timeout = d
			}
		}
	}
	if !cmd.Flags().Changed("llm-concurrency") {
		if envConc := os.Getenv("INVOWK_LLM_CONCURRENCY"); envConc != "" {
			if n, err := strconv.Atoi(envConc); err == nil {
				result.apiConfig.Concurrency = n
			}
		}
	}
	return nil
}

func applyLLMFlagOverrides(cmd *cobra.Command, flags llmFlagValues, result *llmResolvedConfig) error {
	if cmd.Flags().Changed("llm-url") {
		result.apiConfig.BaseURL = flags.url
	}
	if cmd.Flags().Changed(llmFlagModel) {
		model := config.LLMModelName(flags.model)
		if err := model.Validate(); err != nil {
			return err
		}
		result.model = model
		result.apiConfig.Model = flags.model
	}
	if cmd.Flags().Changed("llm-api-key") {
		result.apiConfig.APIKey = flags.apiKey
	}
	if cmd.Flags().Changed("llm-timeout") {
		result.apiConfig.Timeout = flags.timeout
	}
	if cmd.Flags().Changed("llm-concurrency") {
		result.apiConfig.Concurrency = flags.concurrency
	}
	return nil
}

func buildLLMCompleter(ctx context.Context, cmd *cobra.Command, llm *llmResolvedConfig) (*llmCompleterResult, *ExitError) {
	if llm == nil || llm.mode == llmModeNone {
		err := errors.New("LLM is not enabled")
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	if llm.mode == llmModeAPI {
		return buildManualCompleter(ctx, cmd, llm.apiConfig)
	}

	if err := llm.apiConfig.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	return buildProviderCompleter(ctx, cmd, llm.provider, llm.model, llm.apiConfig)
}

func buildProviderCompleter(ctx context.Context, cmd *cobra.Command, provider config.LLMProvider, modelOverride config.LLMModelName, llmOpts auditllm.LLMClientConfig) (*llmCompleterResult, *ExitError) {
	timeout := llmOpts.Timeout
	if timeout == 0 {
		timeout = auditllm.DefaultLLMTimeout
	}

	result, err := auditllm.DetectProvider(ctx, provider.String(), modelOverride.String(), timeout)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	if verifier, ok := result.Completer().(auditllm.ModelVerifier); ok {
		if verifyErr := verifier.VerifyModel(ctx); verifyErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", verifyErr)
			return nil, &ExitError{Code: auditExitError, Err: verifyErr}
		}
	}

	modelDisplay := result.Model()
	if modelDisplay == "" {
		modelDisplay = result.Name() + " CLI default"
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Using LLM provider: %s (model: %s)\n", result.Name(), modelDisplay)

	return &llmCompleterResult{
		completer:   result.Completer(),
		concurrency: normalizedLLMConcurrency(llmOpts.Concurrency),
	}, nil
}

func buildManualCompleter(ctx context.Context, cmd *cobra.Command, llmOpts auditllm.LLMClientConfig) (*llmCompleterResult, *ExitError) {
	llmClient, llmErr := auditllm.NewLLMClient(llmOpts)
	if llmErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "LLM configuration error: %v\n", llmErr)
		return nil, &ExitError{Code: auditExitError, Err: llmErr}
	}

	if verifyErr := llmClient.VerifyModel(ctx); verifyErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", verifyErr)
		return nil, &ExitError{Code: auditExitError, Err: verifyErr}
	}

	return &llmCompleterResult{
		completer:   llmClient,
		concurrency: normalizedLLMConcurrency(llmOpts.Concurrency),
	}, nil
}

func normalizedLLMConcurrency(concurrency int) int {
	if concurrency == 0 {
		return audit.DefaultLLMConcurrency
	}
	return concurrency
}
