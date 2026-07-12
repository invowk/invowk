// SPDX-License-Identifier: MPL-2.0

package llmconfig

import (
	"context"
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

type mutationContextKey struct{}

func TestResolveMutationMainContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "provider is required before option validation", run: testResolveRequiresProvider},
		{name: "invalid options are rejected before loading config", run: testResolveRejectsOptionsBeforeLoad},
		{name: "load errors are wrapped with LLM configuration context", run: testResolveWrapsLoadErrors},
		{name: "load receives caller context and config file path", run: testResolvePassesContextAndPath},
		{name: "enable flag promotes defaults to API mode", run: testResolveEnableFlagPromotesDefaults},
		{name: "provider flag wins over configured API backend", run: testResolveProviderFlagWins},
		{name: "configured default API preserves model and normalized concurrency", run: testResolveConfiguredAPIDefaultContracts},
		{name: "configured default provider preserves common model", run: testResolveConfiguredProviderModel},
		{name: "invalid environment URL is rejected through resolve", run: testResolveRejectsInvalidEnvURL},
		{name: "invalid configured provider is rejected after load", run: testResolveRejectsInvalidConfiguredProvider},
		{name: "invalid configured API concurrency is reported once", run: testResolveRejectsInvalidConfiguredAPIConcurrencyOnce},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
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

	for _, mode := range []Mode{ModeNone, ModeProvider, ModeAPI} {
		t.Run("valid "+mode.String(), func(t *testing.T) {
			t.Parallel()

			if err := mode.Validate(); err != nil {
				t.Fatalf("%s.Validate() error = %v, want nil", mode, err)
			}
		})
	}
	if err := Mode(99).Validate(); err == nil || !strings.Contains(err.Error(), "invalid LLM mode 99") {
		t.Fatalf("Mode(99).Validate() error = %v, want invalid mode", err)
	}
}

func TestValidateMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "defaults accepts zero timeout and valid typed fields", run: testDefaultsValidationAcceptsZeroTimeout},
		{name: "defaults join every invalid typed field", run: testDefaultsValidationJoinsInvalidFields},
		{name: "flags accepts zero timeout and valid typed fields", run: testFlagValidationAcceptsZeroTimeout},
		{name: "flags join every invalid typed field", run: testFlagValidationJoinsInvalidFields},
		{name: "resolve options join path flags and defaults", run: testResolveOptionsValidationJoinsInvalidFields},
		{name: "api config reports missing required API fields", run: testAPIConfigValidationRequiredFields},
		{name: "api config accepts zero timeout with required API fields", run: testAPIConfigValidationAcceptsZeroTimeout},
		{name: "api config joins invalid typed fields", run: testAPIConfigValidationJoinsInvalidFields},
		{name: "resolved validates mode provider model api and concurrency contracts", run: testResolvedValidationContracts},
		{name: "resolved rejects unknown mode without API validation", run: testResolvedValidationRejectsUnknownMode},
		{name: "resolved provider mode ignores incomplete API config", run: testResolvedProviderModeIgnoresAPIConfig},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestApplyConfigDefaultsMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "api backend overrides provider defaults and credential env", run: func(t *testing.T) {
			t.Helper()

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
		}},

		{name: "invalid configured timeout is ignored", run: func(t *testing.T) {
			t.Helper()

			result := resolvedWithDefaults()
			applyConfigDefaults(&result, config.LLMConfig{Timeout: "not-a-duration"}, mutationGetenv(nil))
			if result.APIConfig.Timeout != testDefaults().Timeout {
				t.Fatalf("APIConfig.Timeout = %v, want unchanged default %v", result.APIConfig.Timeout, testDefaults().Timeout)
			}
		}},

		{name: "common model applies to result and API config before API override", run: func(t *testing.T) {
			t.Helper()

			result := resolvedWithDefaults()
			applyConfigDefaults(&result, config.LLMConfig{Model: mutationModel}, mutationGetenv(nil))
			if result.Model != mutationModel || result.APIConfig.Model != mutationModel {
				t.Fatalf("configured common model result = %+v, want both model fields", result)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func TestApplyEnvOverridesMutationContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "environment fills unchanged API fields", run: testEnvOverridesFillUnchangedAPIFields},
		{name: "changed flags and configured API key block environment overrides", run: testEnvOverridesRespectChangedFlags},
		{name: "changed API key flag blocks environment key when empty", run: testEnvOverridesChangedAPIKeyBlocksEnvWhenEmpty},
		{name: "configured API key still allows other environment overrides", run: testEnvOverridesConfiguredAPIKeyAllowsOtherFields},
		{name: "invalid environment values return only validating errors", run: testEnvOverridesInvalidValues},
		{name: "invalid environment model returns model validation error", run: testEnvOverridesInvalidModel},
		{name: "invalid timeout and nonnumeric concurrency are ignored", run: testEnvOverridesIgnoreParseFailures},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
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
	t.Helper()

	_, err := Resolve(t.Context(), nil, ResolveOptions{})
	if err == nil || !strings.Contains(err.Error(), "config provider is required") {
		t.Fatalf("Resolve() error = %v, want config provider requirement", err)
	}
}

func testResolveRejectsOptionsBeforeLoad(t *testing.T) {
	t.Helper()

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
	t.Helper()

	loadErr := errors.New("boom")
	_, err := Resolve(t.Context(), &testLoader{loadErr: loadErr}, ResolveOptions{
		Defaults: testDefaults(),
	})
	if !errors.Is(err, loadErr) || !strings.Contains(err.Error(), "load LLM configuration") {
		t.Fatalf("Resolve() error = %v, want wrapped load error", err)
	}
}

func testResolvePassesContextAndPath(t *testing.T) {
	t.Helper()

	ctx := context.WithValue(t.Context(), mutationContextKey{}, "marker")
	configPath := configPathFixture()
	loader := &testLoader{cfg: config.DefaultConfig()}
	_, err := Resolve(ctx, loader, ResolveOptions{
		ConfigFilePath: &configPath,
		Defaults:       testDefaults(),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if loader.lastCtx != ctx {
		t.Fatalf("loader context = %v, want caller context", loader.lastCtx)
	}
	if got := loader.lastCtx.Value(mutationContextKey{}); got != "marker" {
		t.Fatalf("loader context marker = %v, want marker", got)
	}
	if loader.lastOpts.ConfigFilePath != configPath {
		t.Fatalf("loader ConfigFilePath = %q, want %q", loader.lastOpts.ConfigFilePath, configPath)
	}
}

func testResolveEnableFlagPromotesDefaults(t *testing.T) {
	t.Helper()

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

func testResolveConfiguredAPIDefaultContracts(t *testing.T) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.LLM.Model = mutationProviderModel
	cfg.LLM.API = config.LLMAPIConfig{
		BaseURL: mutationBaseURL,
	}
	got, err := Resolve(t.Context(), &testLoader{cfg: cfg}, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if got.Mode != ModeAPI || got.Provider != "" {
		t.Fatalf("Resolve() mode/provider = %v/%q, want API with empty provider", got.Mode, got.Provider)
	}
	if got.Model != mutationProviderModel || got.APIConfig.Model != mutationProviderModel {
		t.Fatalf("Resolve() model fields = %q/%q, want configured common model", got.Model, got.APIConfig.Model)
	}
	if got.Concurrency != testDefaults().Concurrency {
		t.Fatalf("Resolve() Concurrency = %d, want normalized default", got.Concurrency)
	}
}

func testResolveConfiguredProviderModel(t *testing.T) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderClaude
	cfg.LLM.Model = mutationProviderModel
	got, err := Resolve(t.Context(), &testLoader{cfg: cfg}, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if got.Mode != ModeProvider || got.Provider != config.LLMProviderClaude || got.Model != mutationProviderModel {
		t.Fatalf("Resolve() = %+v, want provider mode with configured model", got)
	}
}

func testResolveProviderFlagWins(t *testing.T) {
	t.Helper()

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

func testResolveRejectsInvalidConfiguredProvider(t *testing.T) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.LLM.Provider = "bogus"
	_, err := Resolve(t.Context(), &testLoader{cfg: cfg}, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
	})
	if !errors.Is(err, config.ErrInvalidLLMProvider) {
		t.Fatalf("Resolve() error = %v, want ErrInvalidLLMProvider", err)
	}
}

func testResolveRejectsInvalidEnvURL(t *testing.T) {
	t.Helper()

	_, err := Resolve(t.Context(), &testLoader{cfg: config.DefaultConfig()}, ResolveOptions{
		Defaults: testDefaults(),
		Flags: FlagValues{
			Enable: true,
		},
		Getenv: mutationGetenv(map[string]string{
			"INVOWK_LLM_URL": "notaurl",
		}),
	})
	if !errors.Is(err, config.ErrInvalidLLMBaseURL) {
		t.Fatalf("Resolve() error = %v, want ErrInvalidLLMBaseURL", err)
	}
}

func testResolveRejectsInvalidConfiguredAPIConcurrencyOnce(t *testing.T) {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.LLM.API = config.LLMAPIConfig{
		BaseURL: mutationBaseURL,
		Model:   mutationModel,
	}
	cfg.LLM.Concurrency = -1
	_, err := Resolve(t.Context(), &testLoader{cfg: cfg}, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
	})
	if !errors.Is(err, config.ErrInvalidLLMConcurrency) {
		t.Fatalf("Resolve() error = %v, want ErrInvalidLLMConcurrency", err)
	}
	if got := joinedErrorLen(t, err); got != 1 {
		t.Fatalf("Resolve() joined error count = %d, want 1", got)
	}
}

func testDefaultsValidationAcceptsZeroTimeout(t *testing.T) {
	t.Helper()

	err := Defaults{
		BaseURL:     mutationBaseURL,
		Model:       mutationModel,
		Timeout:     0,
		Concurrency: mutationConcurrency,
	}.Validate()
	if err != nil {
		t.Fatalf("Defaults.Validate() error = %v, want nil", err)
	}
}

func testDefaultsValidationJoinsInvalidFields(t *testing.T) {
	t.Helper()

	err := Defaults{
		BaseURL:     "ftp://example.invalid",
		Model:       " \t ",
		Timeout:     -time.Nanosecond,
		Concurrency: -1,
	}.Validate()
	requireJoinedError(t, err, 4, "default LLM timeout must be non-negative",
		config.ErrInvalidLLMBaseURL,
		config.ErrInvalidLLMModelName,
		config.ErrInvalidLLMConcurrency,
	)
}

func testFlagValidationAcceptsZeroTimeout(t *testing.T) {
	t.Helper()

	err := FlagValues{
		Provider:    config.LLMProviderCodex,
		BaseURL:     mutationBaseURL,
		Model:       mutationModel,
		Timeout:     0,
		Concurrency: mutationConcurrency,
	}.Validate()
	if err != nil {
		t.Fatalf("FlagValues.Validate() error = %v, want nil", err)
	}
}

func testFlagValidationJoinsInvalidFields(t *testing.T) {
	t.Helper()

	err := FlagValues{
		Provider:    "bogus",
		BaseURL:     "file:///tmp/llm.sock",
		Model:       "\n",
		Timeout:     -time.Nanosecond,
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
	t.Helper()

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
	t.Helper()

	err := APIConfig{}.Validate()
	requireJoinedError(t, err, 2, "LLM API base URL is required")
	if !strings.Contains(err.Error(), "LLM API model is required") {
		t.Fatalf("APIConfig.Validate() error = %v, want missing model", err)
	}
}

func testAPIConfigValidationAcceptsZeroTimeout(t *testing.T) {
	t.Helper()

	err := APIConfig{
		BaseURL:     mutationBaseURL,
		Model:       mutationModel,
		Timeout:     0,
		Concurrency: mutationConcurrency,
	}.Validate()
	if err != nil {
		t.Fatalf("APIConfig.Validate() error = %v, want nil", err)
	}
}

func testAPIConfigValidationJoinsInvalidFields(t *testing.T) {
	t.Helper()

	err := APIConfig{
		BaseURL:     "ftp://example.invalid",
		Model:       " ",
		Timeout:     -time.Nanosecond,
		Concurrency: -1,
	}.Validate()
	requireJoinedError(t, err, 4, "LLM API timeout must be non-negative",
		config.ErrInvalidLLMBaseURL,
		config.ErrInvalidLLMModelName,
		config.ErrInvalidLLMConcurrency,
	)
}

func testResolvedValidationContracts(t *testing.T) {
	t.Helper()

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

func testResolvedValidationRejectsUnknownMode(t *testing.T) {
	t.Helper()

	err := Resolved{
		Mode:        Mode(99),
		Provider:    config.LLMProviderCodex,
		Concurrency: mutationConcurrency,
	}.Validate()
	requireJoinedError(t, err, 1, "invalid LLM mode 99")
}

func testResolvedProviderModeIgnoresAPIConfig(t *testing.T) {
	t.Helper()

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
	t.Helper()

	result := resolvedWithDefaults()
	err := applyEnvOverrides(&result, ChangedFlags{}, mutationGetenv(apiEnvValues()))
	if err != nil {
		t.Fatalf("applyEnvOverrides() error = %v, want nil", err)
	}
	requireResolvedEnvValues(t, result)
}

func testEnvOverridesRespectChangedFlags(t *testing.T) {
	t.Helper()

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

func testEnvOverridesChangedAPIKeyBlocksEnvWhenEmpty(t *testing.T) {
	t.Helper()

	result := resolvedWithDefaults()
	err := applyEnvOverrides(&result, ChangedFlags{APIKey: true}, mutationGetenv(map[string]string{
		"INVOWK_LLM_API_KEY": mutationAPIKey,
	}))
	if err != nil {
		t.Fatalf("applyEnvOverrides() error = %v, want nil", err)
	}
	if result.APIConfig.APIKey != "" {
		t.Fatalf("APIConfig.APIKey = %q, want unchanged empty key", result.APIConfig.APIKey)
	}
}

func testEnvOverridesConfiguredAPIKeyAllowsOtherFields(t *testing.T) {
	t.Helper()

	result := resolvedWithDefaults()
	result.APIConfig.APIKey = "configured-key"
	err := applyEnvOverrides(&result, ChangedFlags{}, mutationGetenv(apiEnvValues()))
	if err != nil {
		t.Fatalf("applyEnvOverrides() error = %v, want nil", err)
	}
	if result.APIConfig.APIKey != "configured-key" {
		t.Fatalf("APIConfig.APIKey = %q, want configured key", result.APIConfig.APIKey)
	}
	if result.APIConfig.BaseURL != mutationBaseURL ||
		result.Model != mutationModel ||
		result.APIConfig.Model != mutationModel ||
		result.APIConfig.Timeout != mutationTimeout ||
		result.APIConfig.Concurrency != mutationConcurrency {
		t.Fatalf("non-key env overrides = %+v, want env values with configured key preserved", result)
	}
}

func testEnvOverridesInvalidValues(t *testing.T) {
	t.Helper()

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

func testEnvOverridesInvalidModel(t *testing.T) {
	t.Helper()

	result := resolvedWithDefaults()
	err := applyEnvOverrides(&result, ChangedFlags{}, mutationGetenv(map[string]string{
		"INVOWK_LLM_MODEL": " \n ",
	}))
	if !errors.Is(err, config.ErrInvalidLLMModelName) {
		t.Fatalf("applyEnvOverrides() error = %v, want ErrInvalidLLMModelName", err)
	}
	if result.Model != "" || result.APIConfig.Model != testDefaults().Model {
		t.Fatalf("invalid env model mutated result = %+v", result)
	}
}

func testEnvOverridesIgnoreParseFailures(t *testing.T) {
	t.Helper()

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
