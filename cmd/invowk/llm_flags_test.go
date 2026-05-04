// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/config"
)

type testConfigProvider struct {
	cfg      *config.Config
	loadErr  error
	lastOpts config.LoadOptions
}

func (p *testConfigProvider) Load(_ context.Context, opts config.LoadOptions) (*config.Config, error) {
	p.lastOpts = opts
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	return p.cfg, nil
}

func (p *testConfigProvider) LoadWithSource(_ context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	p.lastOpts = opts
	if p.loadErr != nil {
		return config.LoadResult{}, p.loadErr
	}
	return config.LoadResult{Config: p.cfg}, nil
}

func newTestLLMCommand(t *testing.T) (*cobra.Command, *llmFlagValues) {
	t.Helper()

	flags := &llmFlagValues{}
	cmd := &cobra.Command{Use: "test"}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	bindLLMFlags(cmd, flags)
	return cmd, flags
}

func TestResolveLLMForCommand_UsesConfiguredProviderDefault(t *testing.T) {
	t.Parallel()

	cmd, flags := newTestLLMCommand(t)
	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderCodex
	cfg.LLM.Model = "gpt-5.1-codex"
	provider := &testConfigProvider{cfg: cfg}

	got, err := resolveLLMForCommand(t.Context(), cmd, provider, "", *flags, true)
	if err != nil {
		t.Fatalf("resolveLLMForCommand() error = %v", err)
	}
	if got.mode != llmModeProvider || got.provider != "codex" || got.model != "gpt-5.1-codex" {
		t.Fatalf("resolved = %+v, want codex provider with model override", got)
	}
}

func TestResolveLLMForCommand_AuditDoesNotImplicitlyUseConfiguredDefault(t *testing.T) {
	t.Parallel()

	cmd, flags := newTestLLMCommand(t)
	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderCodex
	provider := &testConfigProvider{cfg: cfg}

	got, err := resolveLLMForCommand(t.Context(), cmd, provider, "", *flags, false)
	if err != nil {
		t.Fatalf("resolveLLMForCommand() error = %v", err)
	}
	if got.mode != llmModeNone {
		t.Fatalf("mode = %v, want none", got.mode)
	}
}

func TestResolveLLMForCommand_LLMFlagUsesConfiguredProvider(t *testing.T) {
	t.Parallel()

	cmd, flags := newTestLLMCommand(t)
	if err := cmd.Flags().Set("llm", "true"); err != nil {
		t.Fatalf("set --llm: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.LLM.Provider = config.LLMProviderClaude
	provider := &testConfigProvider{cfg: cfg}

	got, err := resolveLLMForCommand(t.Context(), cmd, provider, "/tmp/invowk-config.cue", *flags, false)
	if err != nil {
		t.Fatalf("resolveLLMForCommand() error = %v", err)
	}
	if got.mode != llmModeProvider || got.provider != "claude" {
		t.Fatalf("resolved = %+v, want claude provider", got)
	}
	if provider.lastOpts.ConfigFilePath != "/tmp/invowk-config.cue" {
		t.Fatalf("ConfigFilePath = %q, want /tmp/invowk-config.cue", provider.lastOpts.ConfigFilePath)
	}
}

func TestResolveLLMForCommand_APIConfigUsesEnvReference(t *testing.T) {
	t.Setenv("CUSTOM_LLM_TOKEN_VAR", "secret-value")

	cmd, flags := newTestLLMCommand(t)
	cfg := config.DefaultConfig()
	cfg.LLM.API = config.LLMAPIConfig{ //nolint:gosec // test uses an environment-variable name, not a credential value.
		BaseURL:   "https://example.invalid/v1",
		Model:     "custom-model",
		APIKeyEnv: "CUSTOM_LLM_TOKEN_VAR",
	}
	provider := &testConfigProvider{cfg: cfg}

	got, err := resolveLLMForCommand(t.Context(), cmd, provider, "", *flags, true)
	if err != nil {
		t.Fatalf("resolveLLMForCommand() error = %v", err)
	}
	if got.mode != llmModeAPI {
		t.Fatalf("mode = %v, want API", got.mode)
	}
	if got.apiConfig.BaseURL != "https://example.invalid/v1" || got.apiConfig.Model != "custom-model" || got.apiConfig.APIKey != "secret-value" {
		t.Fatalf("apiConfig = %+v, want configured URL/model/API key from env ref", got.apiConfig)
	}
}

func TestResolveLLMForCommand_AgentRequiresConfigOrFlag(t *testing.T) {
	t.Parallel()

	cmd, flags := newTestLLMCommand(t)
	provider := &testConfigProvider{cfg: config.DefaultConfig()}

	if _, err := resolveLLMForCommand(t.Context(), cmd, provider, "", *flags, true); err == nil {
		t.Fatal("resolveLLMForCommand() succeeded, want error")
	}
}
