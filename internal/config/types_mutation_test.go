// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigTypesMutationScalarErrorPayloads(t *testing.T) {
	t.Parallel()

	t.Run("LLM base URL preserves invalid values and accepts http", func(t *testing.T) {
		t.Parallel()

		if err := LLMBaseURL("http://localhost:11434/v1").Validate(); err != nil {
			t.Fatalf("LLMBaseURL(http).Validate() error = %v, want nil", err)
		}

		tests := []struct {
			name  string
			value LLMBaseURL
		}{
			{name: "missing host", value: "https://"},
			{name: "missing scheme", value: "api.local/v1"},
			{name: "protocol relative", value: "//localhost/v1"},
			{name: "invalid escape", value: "http://localhost/%zz"},
			{name: "unsupported scheme", value: "ftp://localhost"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				err := tt.value.Validate()
				if !errors.Is(err, ErrInvalidLLMBaseURL) {
					t.Fatalf("LLMBaseURL(%q).Validate() error = %v, want ErrInvalidLLMBaseURL", tt.value, err)
				}
				var baseErr *InvalidLLMBaseURLError
				if !errors.As(err, &baseErr) {
					t.Fatalf("LLMBaseURL(%q).Validate() error type = %T, want *InvalidLLMBaseURLError", tt.value, err)
				}
				if baseErr.Value != tt.value {
					t.Fatalf("InvalidLLMBaseURLError.Value = %q, want %q", baseErr.Value, tt.value)
				}
				want := "invalid LLM base URL \"" + string(tt.value) + "\": must be an absolute http(s) URL"
				if got := err.Error(); got != want {
					t.Fatalf("InvalidLLMBaseURLError.Error() = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("LLM timeout returns zero duration and detailed errors", func(t *testing.T) {
		t.Parallel()

		maxRunesTimeout := LLMTimeout(strings.Repeat("1s", llmTimeoutMaxRunes/2))
		duration, err := maxRunesTimeout.Duration()
		if err != nil {
			t.Fatalf("LLMTimeout(%q).Duration() error = %v, want nil", maxRunesTimeout, err)
		}
		if duration != 32*time.Second {
			t.Fatalf("LLMTimeout(%q).Duration() = %s, want 32s", maxRunesTimeout, duration)
		}

		tests := []struct {
			name       string
			value      LLMTimeout
			wantReason string
		}{
			{
				name:       "over max runes",
				value:      LLMTimeout(strings.Repeat("1s", llmTimeoutMaxRunes/2+1)),
				wantReason: "must be at most 64 runes",
			},
			{
				name:       "malformed duration",
				value:      "soon",
				wantReason: "must be a positive Go duration",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				duration, err := tt.value.Duration()
				if duration != 0 {
					t.Fatalf("LLMTimeout(%q).Duration() duration = %s, want 0s", tt.value, duration)
				}
				if !errors.Is(err, ErrInvalidLLMTimeout) {
					t.Fatalf("LLMTimeout(%q).Duration() error = %v, want ErrInvalidLLMTimeout", tt.value, err)
				}
				var timeoutErr *InvalidLLMTimeoutError
				if !errors.As(err, &timeoutErr) {
					t.Fatalf("LLMTimeout(%q).Duration() error type = %T, want *InvalidLLMTimeoutError", tt.value, err)
				}
				if timeoutErr.Value != tt.value {
					t.Fatalf("InvalidLLMTimeoutError.Value = %q, want %q", timeoutErr.Value, tt.value)
				}
				if timeoutErr.Reason != tt.wantReason {
					t.Fatalf("InvalidLLMTimeoutError.Reason = %q, want %q", timeoutErr.Reason, tt.wantReason)
				}
				want := "invalid LLM timeout \"" + string(tt.value) + "\": " + tt.wantReason
				if got := err.Error(); got != want {
					t.Fatalf("InvalidLLMTimeoutError.Error() = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("path and enum errors preserve rejected values", func(t *testing.T) {
		t.Parallel()

		modulePath := ModuleIncludePath(" \t ")
		moduleErr := modulePath.Validate()
		var invalidModulePath *InvalidModuleIncludePathError
		if !errors.As(moduleErr, &invalidModulePath) || invalidModulePath.Value != modulePath {
			t.Fatalf("ModuleIncludePath.Validate() error = %#v, want value %q", moduleErr, modulePath)
		}
		if got, want := moduleErr.Error(), "invalid module include path \" \\t \": must be an absolute *.invowkmod path"; got != want {
			t.Fatalf("InvalidModuleIncludePathError.Error() = %q, want %q", got, want)
		}

		binaryPath := BinaryFilePath(" \t ")
		binaryErr := binaryPath.Validate()
		var invalidBinaryPath *InvalidBinaryFilePathError
		if !errors.As(binaryErr, &invalidBinaryPath) || invalidBinaryPath.Value != binaryPath {
			t.Fatalf("BinaryFilePath.Validate() error = %#v, want value %q", binaryErr, binaryPath)
		}
		if got, want := binaryErr.Error(), "invalid binary file path \" \\t \": non-empty value must not be whitespace-only"; got != want {
			t.Fatalf("InvalidBinaryFilePathError.Error() = %q, want %q", got, want)
		}

		cacheDir := CacheDirPath(" \n ")
		cacheErr := cacheDir.Validate()
		var invalidCacheDir *InvalidCacheDirPathError
		if !errors.As(cacheErr, &invalidCacheDir) || invalidCacheDir.Value != cacheDir {
			t.Fatalf("CacheDirPath.Validate() error = %#v, want value %q", cacheErr, cacheDir)
		}
		if got, want := cacheErr.Error(), "invalid cache dir path \" \\n \": non-empty value must not be whitespace-only"; got != want {
			t.Fatalf("InvalidCacheDirPathError.Error() = %q, want %q", got, want)
		}

		engine := ContainerEngine("containerd")
		engineErr := engine.Validate()
		var invalidEngine *InvalidContainerEngineError
		if !errors.As(engineErr, &invalidEngine) || invalidEngine.Value != engine {
			t.Fatalf("ContainerEngine.Validate() error = %#v, want value %q", engineErr, engine)
		}
		if got, want := engineErr.Error(), `invalid container engine "containerd" (valid: podman, docker)`; got != want {
			t.Fatalf("InvalidContainerEngineError.Error() = %q, want %q", got, want)
		}

		scheme := ColorScheme("sepia")
		schemeErr := scheme.Validate()
		var invalidScheme *InvalidColorSchemeError
		if !errors.As(schemeErr, &invalidScheme) || invalidScheme.Value != scheme {
			t.Fatalf("ColorScheme.Validate() error = %#v, want value %q", schemeErr, scheme)
		}
		if got, want := schemeErr.Error(), `invalid color scheme "sepia" (valid: auto, dark, light)`; got != want {
			t.Fatalf("InvalidColorSchemeError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("LLM concurrency rejects minus one", func(t *testing.T) {
		t.Parallel()

		err := LLMConcurrency(-1).Validate()
		if !errors.Is(err, ErrInvalidLLMConcurrency) {
			t.Fatalf("LLMConcurrency(-1).Validate() error = %v, want ErrInvalidLLMConcurrency", err)
		}
		var concurrencyErr *InvalidLLMConcurrencyError
		if !errors.As(err, &concurrencyErr) || concurrencyErr.Value != -1 {
			t.Fatalf("LLMConcurrency(-1).Validate() error = %#v, want value -1", err)
		}
		if got, want := err.Error(), "invalid LLM concurrency -1: must be zero or greater"; got != want {
			t.Fatalf("InvalidLLMConcurrencyError.Error() = %q, want %q", got, want)
		}
	})
}

func TestConfigTypesMutationLLMContracts(t *testing.T) {
	t.Parallel()

	t.Run("API config presence treats each field independently", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			cfg  LLMAPIConfig
			want bool
		}{
			{name: "empty", cfg: LLMAPIConfig{}, want: false},
			{name: "base URL only", cfg: LLMAPIConfig{BaseURL: "http://localhost:11434/v1"}, want: true},
			{name: "model only", cfg: LLMAPIConfig{Model: "llama3.2"}, want: true},
			{name: "credential only", cfg: LLMAPIConfig{CredentialEnv: validLLMEnvVar()}, want: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				if got := tt.cfg.HasConfig(); got != tt.want {
					t.Fatalf("LLMAPIConfig.HasConfig() = %v, want %v", got, tt.want)
				}
			})
		}
	})

	t.Run("API config validation exposes model and credential field errors", func(t *testing.T) {
		t.Parallel()

		cfg := LLMAPIConfig{
			Model:         " \t ",
			CredentialEnv: "1BAD",
		}
		err := cfg.Validate()
		if !errors.Is(err, ErrInvalidLLMAPIConfig) {
			t.Fatalf("LLMAPIConfig.Validate() error = %v, want ErrInvalidLLMAPIConfig", err)
		}
		if !errors.Is(err, ErrInvalidLLMModelName) {
			t.Fatalf("LLMAPIConfig.Validate() error = %v, want ErrInvalidLLMModelName", err)
		}
		if !errors.Is(err, ErrInvalidLLMCredentialEnvVar) {
			t.Fatalf("LLMAPIConfig.Validate() error = %v, want ErrInvalidLLMCredentialEnvVar", err)
		}
		var apiErr *InvalidLLMAPIConfigError
		if !errors.As(err, &apiErr) {
			t.Fatalf("LLMAPIConfig.Validate() error type = %T, want *InvalidLLMAPIConfigError", err)
		}
		if len(apiErr.FieldErrors) != 2 {
			t.Fatalf("InvalidLLMAPIConfigError.FieldErrors length = %d, want 2", len(apiErr.FieldErrors))
		}
		if got, want := err.Error(), "invalid LLM API config: 2 field error(s)"; got != want {
			t.Fatalf("InvalidLLMAPIConfigError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("LLM backend presence survives API-only config and explicit clearing", func(t *testing.T) {
		t.Parallel()

		apiOnly := LLMConfig{API: LLMAPIConfig{Model: "llama3.2"}}
		if !apiOnly.HasAPIBackend() {
			t.Fatal("LLMConfig.HasAPIBackend() = false, want true for API model")
		}
		if !apiOnly.HasConfig() {
			t.Fatal("LLMConfig.HasConfig() = false, want true for API model")
		}

		explicitEmpty := LLMConfig{}.WithAPIBackendPresent()
		if !explicitEmpty.HasAPIBackend() {
			t.Fatal("LLMConfig.HasAPIBackend() = false, want true for explicit empty API block")
		}
		if !explicitEmpty.HasConfig() {
			t.Fatal("LLMConfig.HasConfig() = false, want true for explicit empty API block")
		}

		cleared := explicitEmpty.WithoutAPIBackend()
		if cleared.HasAPIBackend() {
			t.Fatal("WithoutAPIBackend().HasAPIBackend() = true, want false")
		}
		if cleared.HasConfig() {
			t.Fatal("WithoutAPIBackend().HasConfig() = true, want false")
		}
	})

	t.Run("LLM config validation preserves direct field failures", func(t *testing.T) {
		t.Parallel()

		cfg := LLMConfig{
			Provider:    "bad-provider",
			Timeout:     "soon",
			Concurrency: -1,
		}
		err := cfg.Validate()
		if !errors.Is(err, ErrInvalidLLMConfig) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMConfig", err)
		}
		if !errors.Is(err, ErrInvalidLLMProvider) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMProvider", err)
		}
		if !errors.Is(err, ErrInvalidLLMTimeout) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMTimeout", err)
		}
		if !errors.Is(err, ErrInvalidLLMConcurrency) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMConcurrency", err)
		}
		var llmErr *InvalidLLMConfigError
		if !errors.As(err, &llmErr) {
			t.Fatalf("LLMConfig.Validate() error type = %T, want *InvalidLLMConfigError", err)
		}
		if len(llmErr.FieldErrors) != 3 {
			t.Fatalf("InvalidLLMConfigError.FieldErrors length = %d, want 3", len(llmErr.FieldErrors))
		}
		if got, want := err.Error(), "invalid LLM config: 3 field error(s)"; got != want {
			t.Fatalf("InvalidLLMConfigError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("LLM config rejects explicit empty API backend", func(t *testing.T) {
		t.Parallel()

		err := LLMConfig{}.WithAPIBackendPresent().Validate()
		if !errors.Is(err, ErrInvalidLLMConfig) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMConfig", err)
		}
		if !errors.Is(err, ErrInvalidLLMAPIConfig) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMAPIConfig", err)
		}
		var llmErr *InvalidLLMConfigError
		if !errors.As(err, &llmErr) {
			t.Fatalf("LLMConfig.Validate() error type = %T, want *InvalidLLMConfigError", err)
		}
		if len(llmErr.FieldErrors) != 1 {
			t.Fatalf("InvalidLLMConfigError.FieldErrors length = %d, want 1", len(llmErr.FieldErrors))
		}
		var apiErr *InvalidLLMAPIConfigError
		if !errors.As(llmErr.FieldErrors[0], &apiErr) {
			t.Fatalf("LLMConfig field error type = %T, want *InvalidLLMAPIConfigError", llmErr.FieldErrors[0])
		}
		if len(apiErr.FieldErrors) != 1 {
			t.Fatalf("InvalidLLMAPIConfigError.FieldErrors length = %d, want 1", len(apiErr.FieldErrors))
		}
		if got, want := apiErr.FieldErrors[0].Error(), "llm.api must set at least one of base_url, model, or api_key_env"; got != want {
			t.Fatalf("empty API field error = %q, want %q", got, want)
		}
	})

	t.Run("LLM config rejects provider with API backend", func(t *testing.T) {
		t.Parallel()

		cfg := LLMConfig{
			Provider: LLMProviderCodex,
			API:      LLMAPIConfig{Model: "gpt-5"},
		}
		err := cfg.Validate()
		if !errors.Is(err, ErrInvalidLLMConfig) {
			t.Fatalf("LLMConfig.Validate() error = %v, want ErrInvalidLLMConfig", err)
		}
		var llmErr *InvalidLLMConfigError
		if !errors.As(err, &llmErr) {
			t.Fatalf("LLMConfig.Validate() error type = %T, want *InvalidLLMConfigError", err)
		}
		if len(llmErr.FieldErrors) != 1 {
			t.Fatalf("InvalidLLMConfigError.FieldErrors length = %d, want 1", len(llmErr.FieldErrors))
		}
		if got, want := llmErr.FieldErrors[0].Error(), "llm.provider and llm.api are mutually exclusive"; got != want {
			t.Fatalf("provider/API field error = %q, want %q", got, want)
		}
	})
}

func validLLMEnvVar() LLMCredentialEnvVar {
	return "INVOWK_LLM_ENVVAR"
}

func TestConfigTypesMutationAggregateErrorContracts(t *testing.T) {
	t.Parallel()

	t.Run("include entry and collection errors expose payloads", func(t *testing.T) {
		t.Parallel()

		err := IncludeEntry{Path: "", Alias: "   "}.Validate()
		if !errors.Is(err, ErrInvalidIncludeEntry) {
			t.Fatalf("IncludeEntry.Validate() error = %v, want ErrInvalidIncludeEntry", err)
		}
		var entryErr *InvalidIncludeEntryError
		if !errors.As(err, &entryErr) {
			t.Fatalf("IncludeEntry.Validate() error type = %T, want *InvalidIncludeEntryError", err)
		}
		if len(entryErr.FieldErrors) != 2 {
			t.Fatalf("InvalidIncludeEntryError.FieldErrors length = %d, want 2", len(entryErr.FieldErrors))
		}
		if got, want := err.Error(), "invalid include entry: 2 field error(s)"; got != want {
			t.Fatalf("InvalidIncludeEntryError.Error() = %q, want %q", got, want)
		}

		if got, want := IncludeCollectionAutoProvision.String(), "container.auto_provision.includes"; got != want {
			t.Fatalf("IncludeCollectionAutoProvision.String() = %q, want %q", got, want)
		}

		var nilCollectionErr *InvalidIncludeCollectionError
		if !errors.Is(nilCollectionErr.Unwrap(), ErrInvalidIncludeCollection) {
			t.Fatal("nil InvalidIncludeCollectionError.Unwrap() does not include ErrInvalidIncludeCollection")
		}

		cause := errors.New("duplicate")
		collectionErr := &InvalidIncludeCollectionError{Field: IncludeCollectionRoot, Cause: cause}
		if !errors.Is(collectionErr, ErrInvalidIncludeCollection) {
			t.Fatalf("InvalidIncludeCollectionError does not wrap ErrInvalidIncludeCollection: %v", collectionErr)
		}
		if !errors.Is(collectionErr, cause) {
			t.Fatalf("InvalidIncludeCollectionError does not wrap cause: %v", collectionErr)
		}
		if got, want := collectionErr.Error(), "includes: duplicate"; got != want {
			t.Fatalf("InvalidIncludeCollectionError.Error() = %q, want %q", got, want)
		}
	})

	t.Run("container and config aggregates preserve nested field errors", func(t *testing.T) {
		t.Parallel()

		containerErr := ContainerConfig{
			AutoProvision: AutoProvisionConfig{BinaryPath: "   "},
		}.Validate()
		if !errors.Is(containerErr, ErrInvalidContainerConfig) {
			t.Fatalf("ContainerConfig.Validate() error = %v, want ErrInvalidContainerConfig", containerErr)
		}
		if !errors.Is(containerErr, ErrInvalidAutoProvisionConfig) {
			t.Fatalf("ContainerConfig.Validate() error = %v, want ErrInvalidAutoProvisionConfig", containerErr)
		}
		var invalidContainer *InvalidContainerConfigError
		if !errors.As(containerErr, &invalidContainer) {
			t.Fatalf("ContainerConfig.Validate() error type = %T, want *InvalidContainerConfigError", containerErr)
		}
		if len(invalidContainer.FieldErrors) != 1 {
			t.Fatalf("InvalidContainerConfigError.FieldErrors length = %d, want 1", len(invalidContainer.FieldErrors))
		}
		if got, want := containerErr.Error(), "invalid container config: 1 field error(s)"; got != want {
			t.Fatalf("InvalidContainerConfigError.Error() = %q, want %q", got, want)
		}

		cfg := *DefaultConfig()
		cfg.ContainerEngine = "bad-engine"
		cfg.DefaultRuntime = "bad-runtime"
		duplicateIncludePath := ModuleIncludePath(filepath.Join(t.TempDir(), "bad.invowkmod"))
		cfg.Includes = []IncludeEntry{{Path: duplicateIncludePath}, {Path: duplicateIncludePath}}
		cfg.UI.ColorScheme = "neon"
		cfg.LLM.Provider = "bad-provider"
		cfg.Container.AutoProvision.CacheDir = " \t "

		configErr := cfg.Validate()
		if !errors.Is(configErr, ErrInvalidConfig) {
			t.Fatalf("Config.Validate() error = %v, want ErrInvalidConfig", configErr)
		}
		for _, want := range []error{
			ErrInvalidContainerEngine,
			ErrInvalidConfigRuntimeMode,
			ErrInvalidIncludeCollection,
			ErrInvalidUIConfig,
			ErrInvalidLLMConfig,
			ErrInvalidContainerConfig,
		} {
			if !errors.Is(configErr, want) {
				t.Fatalf("Config.Validate() error = %v, want wrapped %v", configErr, want)
			}
		}
		var invalidConfig *InvalidConfigError
		if !errors.As(configErr, &invalidConfig) {
			t.Fatalf("Config.Validate() error type = %T, want *InvalidConfigError", configErr)
		}
		if len(invalidConfig.FieldErrors) != 6 {
			t.Fatalf("InvalidConfigError.FieldErrors length = %d, want 6", len(invalidConfig.FieldErrors))
		}
		if got, want := configErr.Error(), "invalid config: 6 field error(s)"; got != want {
			t.Fatalf("InvalidConfigError.Error() = %q, want %q", got, want)
		}
	})
}
