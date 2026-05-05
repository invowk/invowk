// SPDX-License-Identifier: MPL-2.0

package llmconfig

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/types"
)

type testLoader struct {
	cfg      *config.Config
	loadErr  error
	lastOpts config.LoadOptions
}

func (l *testLoader) Load(_ context.Context, opts config.LoadOptions) (*config.Config, error) {
	l.lastOpts = opts
	if l.loadErr != nil {
		return nil, l.loadErr
	}
	return l.cfg, nil
}

func TestResolveUsesConfiguredProviderDefault(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderCodex
	cfg.LLM.Model = "gpt-5.1-codex"
	loader := &testLoader{cfg: cfg}

	got, err := Resolve(t.Context(), loader, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Mode != ModeProvider || got.Provider != config.LLMProviderCodex || got.Model != "gpt-5.1-codex" {
		t.Fatalf("Resolve() = %+v, want codex provider with model override", got)
	}
}

func TestResolveSkipsConfiguredDefaultWhenNotRequested(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderCodex
	loader := &testLoader{cfg: cfg}

	got, err := Resolve(t.Context(), loader, ResolveOptions{
		Defaults: testDefaults(),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Mode != ModeNone {
		t.Fatalf("Mode = %v, want none", got.Mode)
	}
}

func TestResolveFlagUsesConfiguredProvider(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderClaude
	loader := &testLoader{cfg: cfg}
	configPath := configPathFixture()

	got, err := Resolve(t.Context(), loader, ResolveOptions{
		ConfigFilePath: &configPath,
		Defaults:       testDefaults(),
		Flags: FlagValues{
			Enable: true,
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Mode != ModeProvider || got.Provider != config.LLMProviderClaude {
		t.Fatalf("Resolve() = %+v, want claude provider", got)
	}
	if loader.lastOpts.ConfigFilePath != "/tmp/invowk-config.cue" {
		t.Fatalf("ConfigFilePath = %q, want /tmp/invowk-config.cue", loader.lastOpts.ConfigFilePath)
	}
}

func TestResolveAPIConfigUsesEnvReference(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.LLM.API = config.LLMAPIConfig{ //nolint:gosec // test uses an environment-variable name, not a credential value.
		BaseURL:       "https://example.invalid/v1",
		Model:         "custom-model",
		CredentialEnv: "CUSTOM_LLM_TOKEN_VAR",
	}
	loader := &testLoader{cfg: cfg}

	got, err := Resolve(t.Context(), loader, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
		Getenv: func(key string) string {
			if key == "CUSTOM_LLM_TOKEN_VAR" {
				return "secret-value"
			}
			return ""
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Mode != ModeAPI {
		t.Fatalf("Mode = %v, want API", got.Mode)
	}
	if got.APIConfig.BaseURL != "https://example.invalid/v1" || got.APIConfig.Model != "custom-model" || got.APIConfig.APIKey != "secret-value" {
		t.Fatalf("APIConfig = %+v, want configured URL/model/API key from env ref", got.APIConfig)
	}
}

func TestResolveRequiresConfigOrFlagWhenRequested(t *testing.T) {
	t.Parallel()

	_, err := Resolve(t.Context(), &testLoader{cfg: config.DefaultConfig()}, ResolveOptions{
		UseConfiguredDefault: true,
		Defaults:             testDefaults(),
	})
	if !errors.Is(err, ErrMissingLLMConfig) {
		t.Fatalf("Resolve() error = %v, want ErrMissingLLMConfig", err)
	}
}

func testDefaults() Defaults {
	return Defaults{
		BaseURL:     "http://localhost:11434/v1",
		Model:       "qwen2.5-coder:7b",
		Timeout:     2 * time.Minute,
		Concurrency: 2,
	}
}

func configPathFixture() types.FilesystemPath {
	return "/tmp/invowk-config.cue"
}
