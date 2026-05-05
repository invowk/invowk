// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/app/llmconfig"
	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/internal/auditllm"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/llm"
	"github.com/invowk/invowk/pkg/types"
)

const (
	invalidLLMConfigurationFmt = "invalid LLM configuration: %v\n"
	llmFlagModel               = "llm-model"
)

type (
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
		completer   llm.Completer
		concurrency int
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

func resolveLLMForCommand(
	ctx context.Context,
	cmd *cobra.Command,
	provider config.Provider,
	configPath types.FilesystemPath,
	flags llmFlagValues,
	useConfiguredDefault bool,
) (*llmconfig.Resolved, *ExitError) {
	flagValues, flagErr := llmConfigFlagValues(cmd, flags)
	if flagErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), invalidLLMConfigurationFmt, flagErr)
		return nil, &ExitError{Code: auditExitError, Err: flagErr}
	}
	resolved, err := llmconfig.Resolve(ctx, provider, llmconfig.ResolveOptions{
		ConfigFilePath:       optionalConfigFilePath(configPath),
		Flags:                flagValues,
		UseConfiguredDefault: useConfiguredDefault,
		Defaults: llmconfig.Defaults{
			BaseURL:     auditllm.DefaultLLMBaseURL,
			Model:       auditllm.DefaultLLMModel,
			Timeout:     auditllm.DefaultLLMTimeout,
			Concurrency: audit.DefaultLLMConcurrency,
		},
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), invalidLLMConfigurationFmt, err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}
	return resolved, nil
}

func llmConfigFlagValues(cmd *cobra.Command, flags llmFlagValues) (llmconfig.FlagValues, error) {
	provider := config.LLMProvider(flags.provider)
	if err := provider.Validate(); err != nil {
		return llmconfig.FlagValues{}, err
	}
	baseURL := config.LLMBaseURL(flags.url)
	if err := baseURL.Validate(); err != nil {
		return llmconfig.FlagValues{}, err
	}
	model := config.LLMModelName(flags.model)
	if err := model.Validate(); err != nil {
		return llmconfig.FlagValues{}, err
	}
	concurrency := config.LLMConcurrency(flags.concurrency)
	if err := concurrency.Validate(); err != nil {
		return llmconfig.FlagValues{}, err
	}
	return llmconfig.FlagValues{
		Enable:      flags.enable,
		Provider:    provider,
		BaseURL:     baseURL,
		Model:       model,
		APIKey:      flags.apiKey,
		Timeout:     flags.timeout,
		Concurrency: concurrency,
		Changed: llmconfig.ChangedFlags{
			BaseURL:     cmd.Flags().Changed("llm-url"),
			Model:       cmd.Flags().Changed(llmFlagModel),
			APIKey:      cmd.Flags().Changed("llm-api-key"),
			Timeout:     cmd.Flags().Changed("llm-timeout"),
			Concurrency: cmd.Flags().Changed("llm-concurrency"),
		},
	}, nil
}

func optionalConfigFilePath(path types.FilesystemPath) *types.FilesystemPath {
	if path == "" {
		return nil
	}
	return &path
}

func buildLLMCompleter(ctx context.Context, cmd *cobra.Command, resolved *llmconfig.Resolved) (*llmCompleterResult, *ExitError) {
	if resolved == nil || resolved.Mode == llmconfig.ModeNone {
		err := errors.New("LLM is not enabled")
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	apiConfig := toLLMClientConfig(resolved.APIConfig)
	if resolved.Mode == llmconfig.ModeAPI {
		return buildManualCompleter(ctx, cmd, apiConfig)
	}

	if err := apiConfig.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), invalidLLMConfigurationFmt, err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	return buildProviderCompleter(ctx, cmd, resolved.Provider, resolved.Model, apiConfig)
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
		concurrency: llmOpts.Concurrency,
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
		concurrency: llmOpts.Concurrency,
	}, nil
}

func toLLMClientConfig(cfg llmconfig.APIConfig) auditllm.LLMClientConfig {
	return auditllm.LLMClientConfig{
		BaseURL:     cfg.BaseURL.String(),
		Model:       cfg.Model.String(),
		APIKey:      cfg.APIKey,
		Timeout:     cfg.Timeout,
		Concurrency: int(cfg.Concurrency),
	}
}
