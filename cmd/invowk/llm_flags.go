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
)

const llmFlagModel = "llm-model"

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
		completer   audit.LLMCompleter
		concurrency int
	}
)

func bindLLMFlags(cmd *cobra.Command, flags *llmFlagValues) {
	cmd.Flags().StringVar(&flags.provider, "llm-provider", "", "auto-detect or use specific provider: auto, claude, codex, gemini, ollama")
	cmd.Flags().BoolVar(&flags.enable, "llm", false, "enable LLM with manual --llm-url/--llm-api-key config")
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

func resolveLLMFlags(cmd *cobra.Command, cfg auditllm.LLMClientConfig) auditllm.LLMClientConfig {
	if !cmd.Flags().Changed("llm-url") {
		if envURL := os.Getenv("INVOWK_LLM_URL"); envURL != "" {
			cfg.BaseURL = envURL
		}
	}
	if !cmd.Flags().Changed(llmFlagModel) {
		if envModel := os.Getenv("INVOWK_LLM_MODEL"); envModel != "" {
			cfg.Model = envModel
		}
	}
	if !cmd.Flags().Changed("llm-api-key") {
		if envKey := os.Getenv("INVOWK_LLM_API_KEY"); envKey != "" {
			cfg.APIKey = envKey
		}
	}
	if !cmd.Flags().Changed("llm-timeout") {
		if envTimeout := os.Getenv("INVOWK_LLM_TIMEOUT"); envTimeout != "" {
			if d, err := time.ParseDuration(envTimeout); err == nil {
				cfg.Timeout = d
			}
		}
	}
	if !cmd.Flags().Changed("llm-concurrency") {
		if envConc := os.Getenv("INVOWK_LLM_CONCURRENCY"); envConc != "" {
			if n, err := strconv.Atoi(envConc); err == nil {
				cfg.Concurrency = n
			}
		}
	}

	return cfg
}

func llmClientConfigFromFlags(cmd *cobra.Command, flags llmFlagValues) auditllm.LLMClientConfig {
	return resolveLLMFlags(cmd, auditllm.LLMClientConfig{
		BaseURL:     flags.url,
		Model:       flags.model,
		APIKey:      flags.apiKey,
		Timeout:     flags.timeout,
		Concurrency: flags.concurrency,
	})
}

func buildLLMCompleter(ctx context.Context, cmd *cobra.Command, flags llmFlagValues, llmOpts auditllm.LLMClientConfig) (*llmCompleterResult, *ExitError) {
	if err := llmOpts.Validate(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "invalid LLM configuration: %v\n", err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}

	switch {
	case flags.provider != "":
		return buildProviderCompleter(ctx, cmd, flags.provider, llmOpts)
	case flags.enable:
		return buildManualCompleter(ctx, cmd, llmOpts)
	default:
		err := errors.New("specify --llm-provider or --llm")
		fmt.Fprintln(cmd.ErrOrStderr(), err)
		return nil, &ExitError{Code: auditExitError, Err: err}
	}
}

func buildProviderCompleter(ctx context.Context, cmd *cobra.Command, provider string, llmOpts auditllm.LLMClientConfig) (*llmCompleterResult, *ExitError) {
	modelOverride := ""
	if cmd.Flags().Changed(llmFlagModel) {
		modelOverride = llmOpts.Model
	}

	timeout := llmOpts.Timeout
	if timeout == 0 {
		timeout = auditllm.DefaultLLMTimeout
	}

	result, err := auditllm.DetectProvider(ctx, provider, modelOverride, timeout)
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
