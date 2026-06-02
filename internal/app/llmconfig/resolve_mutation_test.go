// SPDX-License-Identifier: MPL-2.0

package llmconfig

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

const (
	mutationBaseURL       config.LLMBaseURL     = "https://llm.example.invalid/v1"
	mutationModel         config.LLMModelName   = "mutation-model"
	mutationAPIKey                              = "test-api-key"
	mutationTimeout                             = 45 * time.Second
	mutationConcurrency   config.LLMConcurrency = 7
	mutationProviderModel config.LLMModelName   = "provider-model"
)

func TestResolveMutationMainContracts(t *testing.T) {
	t.Parallel()

	t.Run("provider is required before option validation", testResolveRequiresProvider)
	t.Run("invalid options are rejected before loading config", testResolveRejectsOptionsBeforeLoad)
	t.Run("load errors are wrapped with LLM configuration context", testResolveWrapsLoadErrors)
	t.Run("enable flag promotes defaults to API mode", testResolveEnableFlagPromotesDefaults)
	t.Run("provider flag wins over configured API backend", testResolveProviderFlagWins)
}

func TestModeMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode Mode
		want string
	}{
		{mode: ModeNone, want: "none"},
		{mode: ModeProvider, want: "provider"},
		{mode: ModeAPI, want: "api"},
		{mode: Mode(99), want: "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			if got := tt.mode.String(); got != tt.want {
				t.Fatalf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}

	if err := ModeAPI.Validate(); err != nil {
		t.Fatalf("ModeAPI.Validate() error = %v, want nil", err)
	}
	if err := Mode(99).Validate(); err == nil || !strings.Contains(err.Error(), "invalid LLM mode 99") {
		t.Fatalf("Mode(99).Validate() error = %v, want invalid mode", err)
	}
}

func TestValidateMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("defaults join every invalid typed field", testDefaultsValidationJoinsInvalidFields)
	t.Run("flags join every invalid typed field", testFlagValidationJoinsInvalidFields)
	t.Run("resolve options join path flags and defaults", testResolveOptionsValidationJoinsInvalidFields)
	t.Run("api config reports missing required API fields", testAPIConfigValidationRequiredFields)
	t.Run("api config joins invalid typed fields", testAPIConfigValidationJoinsInvalidFields)
	t.Run("resolved validates mode provider model api and concurrency contracts", testResolvedValidationContracts)
	t.Run("resolved provider mode ignores incomplete API config", testResolvedProviderModeIgnoresAPIConfig)
}

func TestApplyConfigDefaultsMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("api backend overrides provider defaults and credential env", func(t *testing.T) {
		t.Parallel()

		result := resolvedWithDefaults()
		llm := config.LLMConfig{
			Provider:    config.LLMProviderClaude,
			Model:       mutationProviderModel,
			Timeout:     "5s",
			Concurrency: mutationConcurrency,
			API: config.LLMAPIConfig{ //nolint:gosec // test uses an environment-variable name, not a credential value.
				BaseURL:       mutationBaseURL,
				Model:         mutationModel,
				CredentialEnv: "MUTATION_LLM_ENV",
			},
		}

		applyConfigDefaults(&result, llm, mutationGetenv(map[string]string{
			"MUTATION_LLM_ENV": mutationAPIKey,
		}))

		if result.Mode != ModeAPI || result.Provider != "" {
			t.Fatalf("resolved mode/provider = %v/%q, want API mode with no provider", result.Mode, result.Provider)
		}
		if result.Model != mutationProviderModel {
			t.Fatalf("resolved model = %q, want common provider model", result.Model)
		}
		if result.APIConfig.BaseURL != mutationBaseURL ||
			result.APIConfig.Model != mutationModel ||
			result.APIConfig.APIKey != mutationAPIKey ||
			result.APIConfig.Timeout != 5*time.Second ||
			result.APIConfig.Concurrency != mutationConcurrency {
			t.Fatalf("APIConfig = %+v, want configured API defaults", result.APIConfig)
		}
	})

	t.Run("invalid configured timeout is ignored", func(t *testing.T) {
		t.Parallel()

		result := resolvedWithDefaults()
		applyConfigDefaults(&result, config.LLMConfig{Timeout: "not-a-duration"}, mutationGetenv(nil))
		if result.APIConfig.Timeout != testDefaults().Timeout {
			t.Fatalf("APIConfig.Timeout = %v, want unchanged default %v", result.APIConfig.Timeout, testDefaults().Timeout)
		}
	})
}

func TestApplyEnvOverridesMutationContracts(t *testing.T) {
	t.Parallel()

	t.Run("environment fills unchanged API fields", testEnvOverridesFillUnchangedAPIFields)
	t.Run("changed flags and configured API key block environment overrides", testEnvOverridesRespectChangedFlags)
	t.Run("invalid environment values return only validating errors", testEnvOverridesInvalidValues)
	t.Run("invalid timeout and nonnumeric concurrency are ignored", testEnvOverridesIgnoreParseFailures)
}

func TestApplyFlagsAndNormalizationMutationContracts(t *testing.T) {
	t.Parallel()

	result := resolvedWithDefaults()
	applyFlagOverrides(&result, FlagValues{
		BaseURL:     mutationBaseURL,
		Model:       mutationModel,
		APIKey:      mutationAPIKey,
		Timeout:     mutationTimeout,
		Concurrency: mutationConcurrency,
		Changed: ChangedFlags{
			BaseURL:     true,
			Model:       true,
			APIKey:      true,
			Timeout:     true,
			Concurrency: true,
		},
	})

	if result.Model != mutationModel ||
		result.APIConfig.BaseURL != mutationBaseURL ||
		result.APIConfig.Model != mutationModel ||
		result.APIConfig.APIKey != mutationAPIKey ||
		result.APIConfig.Timeout != mutationTimeout ||
		result.APIConfig.Concurrency != mutationConcurrency {
		t.Fatalf("flag overrides = %+v, want all flag values", result)
	}

	if got := normalizedConcurrency(0, DefaultConcurrency); got != DefaultConcurrency {
		t.Fatalf("normalizedConcurrency(0, %d) = %d, want fallback", DefaultConcurrency, got)
	}
	if got := normalizedConcurrency(mutationConcurrency, DefaultConcurrency); got != mutationConcurrency {
		t.Fatalf("normalizedConcurrency(%d, %d) = %d, want explicit", mutationConcurrency, DefaultConcurrency, got)
	}
}

func TestResolveOptionsAccessorsMutationContracts(t *testing.T) {
	t.Parallel()

	path := configPathFixture()
	opts := ResolveOptions{
		ConfigFilePath: &path,
		Getenv: mutationGetenv(map[string]string{
			"MUTATION": "value",
		}),
	}
	if got := opts.configFilePath(); got != path {
		t.Fatalf("configFilePath() = %q, want %q", got, path)
	}
	if got := opts.getenv()("MUTATION"); got != "value" {
		t.Fatalf("getenv()(MUTATION) = %q, want value", got)
	}

	empty := ResolveOptions{}
	if got := empty.configFilePath(); got != "" {
		t.Fatalf("empty configFilePath() = %q, want empty", got)
	}
	if empty.getenv() == nil {
		t.Fatal("default getenv() returned nil")
	}
}

func testResolveRequiresProvider(t *testing.T) {
	t.Parallel()

	_, err := Resolve(t.Context(), nil, ResolveOptions{})
	if err == nil || !strings.Contains(err.Error(), "config provider is required") {
		t.Fatalf("Resolve() error = %v, want config provider requirement", err)
	}
}

func testResolveRejectsOptionsBeforeLoad(t *testing.T) {
	t.Parallel()

	loader := &testLoader{cfg: config.DefaultConfig()}
	invalidPath := types.FilesystemPath(" \t ")
	_, err := Resolve(t.Context(), loader, ResolveOptions{
		ConfigFilePath: &invalidPath,
		Defaults:       testDefaults(),
	})
	if !errors.Is(err, types.ErrInvalidFilesystemPath) {
		t.Fatalf("Resolve() error = %v, want ErrInvalidFilesystemPath", err)
	}
	if loader.lastOpts.ConfigFilePath != "" {
		t.Fatalf("loader should not run for invalid options, got opts %+v", loader.lastOpts)
	}
}

func testResolveWrapsLoadErrors(t *testing.T) {
	t.Parallel()

	loadErr := errors.New("boom")
	_, err := Resolve(t.Context(), &testLoader{loadErr: loadErr}, ResolveOptions{
		Defaults: testDefaults(),
	})
	if !errors.Is(err, loadErr) || !strings.Contains(err.Error(), "load LLM configuration") {
		t.Fatalf("Resolve() error = %v, want wrapped load error", err)
	}
}

func testResolveEnableFlagPromotesDefaults(t *testing.T) {
	t.Parallel()

	got, err := Resolve(t.Context(), &testLoader{cfg: config.DefaultConfig()}, ResolveOptions{
		Defaults: testDefaults(),
		Flags: FlagValues{
			Enable: true,
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	assertResolvedAPIConfig(t, got, testDefaults())
}

func testResolveProviderFlagWins(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.LLM.API = config.LLMAPIConfig{BaseURL: mutationBaseURL, Model: mutationModel}
	got, err := Resolve(t.Context(), &testLoader{cfg: cfg}, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
		Flags: FlagValues{
			Provider: config.LLMProviderGemini,
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if got.Mode != ModeProvider || got.Provider != config.LLMProviderGemini {
		t.Fatalf("Resolve() = %+v, want gemini provider mode", got)
	}
}

func testDefaultsValidationJoinsInvalidFields(t *testing.T) {
	t.Parallel()

	err := Defaults{
		BaseURL:     "ftp://example.invalid",
		Model:       " \t ",
		Timeout:     -time.Second,
		Concurrency: -1,
	}.Validate()
	requireJoinedError(t, err, 4, "default LLM timeout must be non-negative",
		config.ErrInvalidLLMBaseURL,
		config.ErrInvalidLLMModelName,
		config.ErrInvalidLLMConcurrency,
	)
}

func testFlagValidationJoinsInvalidFields(t *testing.T) {
	t.Parallel()

	err := FlagValues{
		Provider:    "bogus",
		BaseURL:     "file:///tmp/llm.sock",
		Model:       "\n",
		Timeout:     -time.Millisecond,
		Concurrency: -2,
	}.Validate()
	requireJoinedError(t, err, 5, "LLM flag timeout must be non-negative",
		config.ErrInvalidLLMProvider,
		config.ErrInvalidLLMBaseURL,
		config.ErrInvalidLLMModelName,
		config.ErrInvalidLLMConcurrency,
	)
}

func testResolveOptionsValidationJoinsInvalidFields(t *testing.T) {
	t.Parallel()

	invalidPath := types.FilesystemPath(" \t ")
	err := ResolveOptions{
		ConfigFilePath: &invalidPath,
		Flags:          FlagValues{BaseURL: "notaurl"},
		Defaults:       Defaults{Model: " "},
	}.Validate()
	requireJoinedError(t, err, 3, "",
		types.ErrInvalidFilesystemPath,
		config.ErrInvalidLLMBaseURL,
		config.ErrInvalidLLMModelName,
	)
}

func testAPIConfigValidationRequiredFields(t *testing.T) {
	t.Parallel()

	err := APIConfig{}.Validate()
	requireJoinedError(t, err, 2, "LLM API base URL is required")
	if !strings.Contains(err.Error(), "LLM API model is required") {
		t.Fatalf("APIConfig.Validate() error = %v, want missing model", err)
	}
}

func testAPIConfigValidationJoinsInvalidFields(t *testing.T) {
	t.Parallel()

	err := APIConfig{
		BaseURL:     "ftp://example.invalid",
		Model:       " ",
		Timeout:     -time.Second,
		Concurrency: -1,
	}.Validate()
	requireJoinedError(t, err, 4, "LLM API timeout must be non-negative",
		config.ErrInvalidLLMBaseURL,
		config.ErrInvalidLLMModelName,
		config.ErrInvalidLLMConcurrency,
	)
}

func testResolvedValidationContracts(t *testing.T) {
	t.Parallel()

	err := Resolved{
		Mode:        ModeAPI,
		Provider:    "bogus",
		Model:       " ",
		Concurrency: -1,
	}.Validate()
	requireJoinedError(t, err, 4, "LLM API base URL is required",
		config.ErrInvalidLLMProvider,
		config.ErrInvalidLLMModelName,
		config.ErrInvalidLLMConcurrency,
	)
	if !strings.Contains(err.Error(), "LLM API model is required") {
		t.Fatalf("Resolved.Validate() error = %v, want API model requirement", err)
	}
}

func testResolvedProviderModeIgnoresAPIConfig(t *testing.T) {
	t.Parallel()

	err := Resolved{
		Mode:        ModeProvider,
		Provider:    config.LLMProviderCodex,
		Concurrency: DefaultConcurrency,
	}.Validate()
	if err != nil {
		t.Fatalf("Resolved.Validate() error = %v, want nil", err)
	}
}

func testEnvOverridesFillUnchangedAPIFields(t *testing.T) {
	t.Parallel()

	result := resolvedWithDefaults()
	err := applyEnvOverrides(&result, ChangedFlags{}, mutationGetenv(apiEnvValues()))
	if err != nil {
		t.Fatalf("applyEnvOverrides() error = %v, want nil", err)
	}
	requireResolvedEnvValues(t, result)
}

func testEnvOverridesRespectChangedFlags(t *testing.T) {
	t.Parallel()

	result := resolvedWithDefaults()
	result.APIConfig.APIKey = "configured-key"
	err := applyEnvOverrides(&result, ChangedFlags{
		BaseURL:     true,
		Model:       true,
		APIKey:      true,
		Timeout:     true,
		Concurrency: true,
	}, mutationGetenv(apiEnvValues()))
	if err != nil {
		t.Fatalf("applyEnvOverrides() error = %v, want nil", err)
	}
	defaults := testDefaults()
	if result.APIConfig.BaseURL != defaults.BaseURL || result.Model != "" || result.APIConfig.Model != defaults.Model ||
		result.APIConfig.APIKey != "configured-key" || result.APIConfig.Timeout != defaults.Timeout ||
		result.APIConfig.Concurrency != defaults.Concurrency {
		t.Fatalf("resolved env-blocked values = %+v, want defaults/configured key", result)
	}
}

func testEnvOverridesInvalidValues(t *testing.T) {
	t.Parallel()

	result := resolvedWithDefaults()
	err := applyEnvOverrides(&result, ChangedFlags{}, mutationGetenv(map[string]string{
		"INVOWK_LLM_URL": "notaurl",
	}))
	if !errors.Is(err, config.ErrInvalidLLMBaseURL) {
		t.Fatalf("applyEnvOverrides() error = %v, want ErrInvalidLLMBaseURL", err)
	}

	err = applyEnvConcurrency(&result, ChangedFlags{}, mutationGetenv(map[string]string{
		"INVOWK_LLM_CONCURRENCY": "-1",
	}))
	if !errors.Is(err, config.ErrInvalidLLMConcurrency) {
		t.Fatalf("applyEnvConcurrency() error = %v, want ErrInvalidLLMConcurrency", err)
	}
}

func testEnvOverridesIgnoreParseFailures(t *testing.T) {
	t.Parallel()

	result := resolvedWithDefaults()
	applyEnvTimeout(&result, ChangedFlags{}, mutationGetenv(map[string]string{
		"INVOWK_LLM_TIMEOUT": "not-a-duration",
	}))
	if result.APIConfig.Timeout != testDefaults().Timeout {
		t.Fatalf("APIConfig.Timeout = %v, want unchanged default", result.APIConfig.Timeout)
	}

	err := applyEnvConcurrency(&result, ChangedFlags{}, mutationGetenv(map[string]string{
		"INVOWK_LLM_CONCURRENCY": "abc",
	}))
	if err != nil {
		t.Fatalf("applyEnvConcurrency() error = %v, want nil for nonnumeric env", err)
	}
	if result.APIConfig.Concurrency != testDefaults().Concurrency {
		t.Fatalf("APIConfig.Concurrency = %d, want unchanged default", result.APIConfig.Concurrency)
	}
}

func apiEnvValues() map[string]string {
	return map[string]string{
		"INVOWK_LLM_URL":         string(mutationBaseURL),
		"INVOWK_LLM_MODEL":       string(mutationModel),
		"INVOWK_LLM_API_KEY":     mutationAPIKey,
		"INVOWK_LLM_TIMEOUT":     mutationTimeout.String(),
		"INVOWK_LLM_CONCURRENCY": mutationConcurrency.String(),
	}
}

func requireJoinedError(t *testing.T, err error, wantLen int, wantMessage string, targets ...error) {
	t.Helper()

	if got := joinedErrorLen(t, err); got != wantLen {
		t.Fatalf("joined error count = %d, want %d", got, wantLen)
	}
	assertErrorWraps(t, err, targets...)
	if wantMessage != "" && !strings.Contains(err.Error(), wantMessage) {
		t.Fatalf("error = %v, want message %q", err, wantMessage)
	}
}

func requireResolvedEnvValues(t *testing.T, result Resolved) {
	t.Helper()

	if result.Model != mutationModel ||
		result.APIConfig.BaseURL != mutationBaseURL ||
		result.APIConfig.Model != mutationModel ||
		result.APIConfig.APIKey != mutationAPIKey ||
		result.APIConfig.Timeout != mutationTimeout ||
		result.APIConfig.Concurrency != mutationConcurrency {
		t.Fatalf("resolved env overrides = %+v, want all env values", result)
	}
}

func resolvedWithDefaults() Resolved {
	defaults := testDefaults()
	return Resolved{
		APIConfig: APIConfig{
			BaseURL:     defaults.BaseURL,
			Model:       defaults.Model,
			Timeout:     defaults.Timeout,
			Concurrency: defaults.Concurrency,
		},
	}
}

func assertResolvedAPIConfig(t *testing.T, got *Resolved, defaults Defaults) {
	t.Helper()

	if got.Mode != ModeAPI {
		t.Fatalf("Mode = %v, want API", got.Mode)
	}
	if got.Concurrency != defaults.Concurrency {
		t.Fatalf("Concurrency = %d, want %d", got.Concurrency, defaults.Concurrency)
	}
	if got.APIConfig.BaseURL != defaults.BaseURL ||
		got.APIConfig.Model != defaults.Model ||
		got.APIConfig.Timeout != defaults.Timeout ||
		got.APIConfig.Concurrency != defaults.Concurrency {
		t.Fatalf("APIConfig = %+v, want defaults %+v", got.APIConfig, defaults)
	}
}

func assertErrorWraps(t *testing.T, err error, targets ...error) {
	t.Helper()

	for _, target := range targets {
		if !errors.Is(err, target) {
			t.Fatalf("error = %v, want wrapped %v", err, target)
		}
	}
}

func joinedErrorLen(t *testing.T, err error) int {
	t.Helper()

	if err == nil {
		t.Fatal("joined error is nil")
	}
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return 1
	}
	return len(joined.Unwrap())
}

func mutationGetenv(values map[string]string) func(string) string {
	return func(key string) string {
		if values == nil {
			return ""
		}
		return values[key]
	}
}
