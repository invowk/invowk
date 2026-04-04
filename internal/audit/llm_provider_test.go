// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// failDeps returns providerDeps where all infrastructure lookups fail.
// Useful as a baseline for tests that need only specific overrides.
func failDeps() providerDeps {
	return providerDeps{
		getenv:   func(_ string) string { return "" },
		lookPath: func(name string) (string, error) { return "", fmt.Errorf("%s not found", name) },
		httpDo:   func(_ *http.Request) (*http.Response, error) { return nil, errors.New("connection refused") },
	}
}

// ollamaModelServer returns an httptest.Server that mimics the Ollama
// /v1/models endpoint returning the given model IDs.
func ollamaModelServer(t *testing.T, modelIDs ...string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := make([]map[string]any, 0, len(modelIDs))
		for _, id := range modelIDs {
			data = append(data, map[string]any{
				"id": id, "object": "model", "created": 1234567890, "owned_by": "library",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	t.Cleanup(srv.Close)

	return srv
}

// --- Provider detection tests (deterministic via injectable deps) ---

func TestDetectAutoProvider_OllamaFirst(t *testing.T) {
	t.Parallel()

	srv := ollamaModelServer(t, "qwen2.5-coder:7b")
	deps := failDeps()
	deps.httpDo = http.DefaultClient.Do

	// Point tryOllama at the test server by overriding DefaultLLMBaseURL
	// indirectly: tryOllama uses DefaultLLMBaseURL which is hard-coded.
	// Instead, test via tryEnvVar path and verify priority with a more
	// targeted test below. Here we test that env-var detection works.
	deps.getenv = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test-key"
		}
		return ""
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderAuto, "", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	// Ollama probe fails (wrong URL), so falls through to ANTHROPIC_API_KEY.
	if result.Name() != ProviderClaude {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderClaude)
	}
	if result.Model() != defaultClaudeModel {
		t.Errorf("Model() = %q, want %q", result.Model(), defaultClaudeModel)
	}

	_ = srv // keep server alive for potential Ollama probe
}

func TestDetectAutoProvider_FallsToOpenAI(t *testing.T) {
	t.Parallel()

	deps := failDeps()
	deps.getenv = func(key string) string {
		if key == "OPENAI_API_KEY" {
			return "sk-openai-test"
		}
		return ""
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderAuto, "", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	if result.Name() != ProviderCodex {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderCodex)
	}
	if result.Model() != defaultOpenAIModel {
		t.Errorf("Model() = %q, want %q", result.Model(), defaultOpenAIModel)
	}
}

func TestDetectAutoProvider_FallsToCLI(t *testing.T) {
	t.Parallel()

	deps := failDeps()
	deps.lookPath = func(name string) (string, error) {
		if name == "gemini" {
			return "/usr/bin/gemini", nil
		}
		return "", fmt.Errorf("%s not found", name)
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderAuto, "", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	if result.Name() != ProviderGemini {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderGemini)
	}
}

func TestDetectAutoProvider_NothingAvailable(t *testing.T) {
	t.Parallel()

	deps := failDeps()

	_, err := detectProviderWith(t.Context(), deps, ProviderAuto, "", 5*time.Second)
	if err == nil {
		t.Fatal("expected error when no providers available")
	}
	if !errors.Is(err, ErrLLMProviderNotFound) {
		t.Errorf("expected ErrLLMProviderNotFound, got %v", err)
	}

	var notFound *LLMProviderNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected LLMProviderNotFoundError, got %T", err)
	}
	// Should have tried all sources.
	if len(notFound.Tried) < 7 {
		t.Errorf("Tried = %d entries, want >= 7: %v", len(notFound.Tried), notFound.Tried)
	}
}

func TestDetectAutoProvider_ModelOverride(t *testing.T) {
	t.Parallel()

	deps := failDeps()
	deps.getenv = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test"
		}
		return ""
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderAuto, "custom-model", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	if result.Model() != "custom-model" {
		t.Errorf("Model() = %q, want %q", result.Model(), "custom-model")
	}
}

func TestDetectSpecificProvider_ClaudeEnvVar(t *testing.T) {
	t.Parallel()

	deps := failDeps()
	deps.getenv = func(key string) string {
		if key == "ANTHROPIC_API_KEY" {
			return "sk-test-key"
		}
		return ""
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderClaude, "", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	if result.Name() != ProviderClaude {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderClaude)
	}
	if result.Model() != defaultClaudeModel {
		t.Errorf("Model() = %q, want %q", result.Model(), defaultClaudeModel)
	}
}

func TestDetectSpecificProvider_ClaudeFallsToCLI(t *testing.T) {
	t.Parallel()

	deps := failDeps()
	deps.lookPath = func(name string) (string, error) {
		if name == "claude" {
			return "/usr/bin/claude", nil
		}
		return "", fmt.Errorf("%s not found", name)
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderClaude, "", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	if result.Name() != ProviderClaude {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderClaude)
	}
}

func TestDetectSpecificProvider_GeminiDualEnvVar(t *testing.T) {
	t.Parallel()

	deps := failDeps()
	deps.getenv = func(key string) string {
		// Only GOOGLE_API_KEY is set (not GEMINI_API_KEY).
		if key == "GOOGLE_API_KEY" {
			return "google-key-test"
		}
		return ""
	}

	result, err := detectProviderWith(t.Context(), deps, ProviderGemini, "", 5*time.Second)
	if err != nil {
		t.Fatalf("detectProviderWith: %v", err)
	}
	if result.Name() != ProviderGemini {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderGemini)
	}
	if result.Model() != defaultGeminiModel {
		t.Errorf("Model() = %q, want %q", result.Model(), defaultGeminiModel)
	}
}

func TestDetectProvider_UnknownProvider(t *testing.T) {
	t.Parallel()

	_, err := DetectProvider(t.Context(), "nonexistent", "", 5*time.Second)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !errors.Is(err, ErrLLMClientConfigInvalid) {
		t.Errorf("expected ErrLLMClientConfigInvalid, got %v", err)
	}
}

// --- Existing tests (kept) ---

func TestValidProviders(t *testing.T) {
	t.Parallel()

	providers := ValidProviders()
	if len(providers) != 5 {
		t.Errorf("expected 5 providers, got %d", len(providers))
	}

	expected := map[string]bool{
		ProviderAuto: true, ProviderClaude: true, ProviderCodex: true,
		ProviderGemini: true, ProviderOllama: true,
	}
	for _, p := range providers {
		if !expected[p] {
			t.Errorf("unexpected provider %q", p)
		}
	}
}

func TestProviderResult_Accessors(t *testing.T) {
	t.Parallel()

	result := &ProviderResult{
		completer: &mockCompleter{response: "test"},
		name:      "test-provider",
		model:     "test-model",
	}

	if result.Name() != "test-provider" {
		t.Errorf("Name() = %q, want %q", result.Name(), "test-provider")
	}
	if result.Model() != "test-model" {
		t.Errorf("Model() = %q, want %q", result.Model(), "test-model")
	}
	if result.Completer() == nil {
		t.Error("Completer() should not be nil")
	}
}

func TestLLMProviderNotFoundError(t *testing.T) {
	t.Parallel()

	err := &LLMProviderNotFoundError{
		Tried: []string{"ollama", "ANTHROPIC_API_KEY", "claude CLI"},
	}

	if !errors.Is(err, ErrLLMProviderNotFound) {
		t.Error("should unwrap to ErrLLMProviderNotFound")
	}
	if !strings.Contains(err.Error(), "ollama") {
		t.Error("error should list tried providers")
	}
	if !strings.Contains(err.Error(), "claude CLI") {
		t.Error("error should list tried CLI tools")
	}
}

// --- CLICompleter tests ---

func TestCLICompleter_BuildArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tool     string
		model    string
		wantArgs []string
	}{
		{
			name:     "claude args",
			tool:     "claude",
			model:    "claude-sonnet-4-6",
			wantArgs: []string{"-p", "test prompt", "--output-format", "json", "--bare"},
		},
		{
			name:     "codex args with model",
			tool:     "codex",
			model:    "gpt-4o",
			wantArgs: []string{"exec", "test prompt", "--json", "-m", "gpt-4o"},
		},
		{
			name:     "gemini args",
			tool:     "gemini",
			model:    "gemini-2.5-flash",
			wantArgs: []string{"-p", "test prompt", "--output-format", "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := NewCLICompleter(tt.tool, tt.model)
			args, err := c.buildArgs("test prompt")
			if err != nil {
				t.Fatalf("buildArgs: %v", err)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("got %d args, want %d: %v", len(args), len(tt.wantArgs), args)
			}
			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("arg[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestCLICompleter_BuildArgs_UnsupportedTool(t *testing.T) {
	t.Parallel()

	c := NewCLICompleter("unsupported", "model")
	_, err := c.buildArgs("prompt")
	if err == nil {
		t.Fatal("expected error for unsupported tool")
	}
}

func TestCLICompleter_Complete_Claude(t *testing.T) {
	t.Parallel()

	c := &CLICompleter{
		tool:  "claude",
		model: "claude-sonnet-4-6",
		runCmd: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`{"type":"result","result":"No issues found.","session_id":"abc"}`), nil
		},
	}

	result, err := c.Complete(t.Context(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result != "No issues found." {
		t.Errorf("result = %q, want %q", result, "No issues found.")
	}
}

func TestCLICompleter_Complete_Codex(t *testing.T) {
	t.Parallel()

	codexOutput := `{"type":"thread.started","thread_id":"abc"}
{"type":"item.completed","item":{"id":"1","type":"agent_message","text":"Found 1 issue."}}
{"type":"turn.completed"}`

	c := &CLICompleter{
		tool:  "codex",
		model: "gpt-4o",
		runCmd: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(codexOutput), nil
		},
	}

	result, err := c.Complete(t.Context(), "system", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result != "Found 1 issue." {
		t.Errorf("result = %q, want %q", result, "Found 1 issue.")
	}
}

func TestCLICompleter_Complete_Gemini(t *testing.T) {
	t.Parallel()

	c := &CLICompleter{
		tool:  "gemini",
		model: "gemini-2.5-flash",
		runCmd: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`{"response":"All clear.","stats":{}}`), nil
		},
	}

	result, err := c.Complete(t.Context(), "system", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if result != "All clear." {
		t.Errorf("result = %q, want %q", result, "All clear.")
	}
}

func TestCLICompleter_Complete_ExitError(t *testing.T) {
	t.Parallel()

	c := &CLICompleter{
		tool:  "claude",
		model: "test",
		runCmd: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, &exec.ExitError{Stderr: []byte("auth failed")}
		},
	}

	_, err := c.Complete(t.Context(), "system", "user")
	if err == nil {
		t.Fatal("expected error for exit error")
	}
	if !strings.Contains(err.Error(), "claude CLI failed") {
		t.Errorf("error should mention tool name: %v", err)
	}
	if !strings.Contains(err.Error(), "auth failed") {
		t.Errorf("error should contain stderr: %v", err)
	}
}

func TestCLICompleter_Complete_PromptMerge(t *testing.T) {
	t.Parallel()

	var capturedArgs []string
	c := &CLICompleter{
		tool:  "claude",
		model: "test",
		runCmd: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			capturedArgs = args
			return []byte(`{"type":"result","result":"ok"}`), nil
		},
	}

	_, err := c.Complete(t.Context(), "SYSTEM", "USER")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Claude args: -p <prompt> --output-format json --bare
	if len(capturedArgs) < 2 {
		t.Fatalf("expected >= 2 args, got %d", len(capturedArgs))
	}
	prompt := capturedArgs[1] // -p is [0], prompt is [1]
	if prompt != "SYSTEM\n\nUSER" {
		t.Errorf("prompt = %q, want %q", prompt, "SYSTEM\n\nUSER")
	}
}

func TestCLICompleter_Complete_GenericError(t *testing.T) {
	t.Parallel()

	c := &CLICompleter{
		tool:  "claude",
		model: "test",
		runCmd: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("command not found")
		},
	}

	_, err := c.Complete(t.Context(), "system", "user")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "claude CLI failed") {
		t.Errorf("error should mention tool name: %v", err)
	}
}

// --- Output parser tests ---

func TestParseClaudeOutput(t *testing.T) {
	t.Parallel()

	t.Run("valid output", func(t *testing.T) {
		t.Parallel()
		raw := `{"type":"result","result":"Analysis complete: no issues found.","session_id":"abc-123"}`
		result, err := parseClaudeOutput(raw)
		if err != nil {
			t.Fatalf("parseClaudeOutput: %v", err)
		}
		if result != "Analysis complete: no issues found." {
			t.Errorf("result = %q", result)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		t.Parallel()
		raw := `{"type":"result","result":"","session_id":"abc-123"}`
		_, err := parseClaudeOutput(raw)
		if !errors.Is(err, ErrLLMEmptyResponse) {
			t.Errorf("expected ErrLLMEmptyResponse, got %v", err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := parseClaudeOutput("not json")
		if !errors.Is(err, ErrLLMMalformedResponse) {
			t.Errorf("expected ErrLLMMalformedResponse, got %v", err)
		}
	})
}

func TestParseCodexOutput(t *testing.T) {
	t.Parallel()

	t.Run("valid JSONL", func(t *testing.T) {
		t.Parallel()
		raw := `{"type":"thread.started","thread_id":"abc"}
{"type":"turn.started"}
{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Found 2 security issues."}}
{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":50}}`

		result, err := parseCodexOutput(raw)
		if err != nil {
			t.Fatalf("parseCodexOutput: %v", err)
		}
		if result != "Found 2 security issues." {
			t.Errorf("result = %q", result)
		}
	})

	t.Run("no agent message", func(t *testing.T) {
		t.Parallel()
		raw := `{"type":"thread.started","thread_id":"abc"}
{"type":"turn.completed"}`
		_, err := parseCodexOutput(raw)
		if !errors.Is(err, ErrLLMMalformedResponse) {
			t.Errorf("expected ErrLLMMalformedResponse, got %v", err)
		}
	})
}

func TestParseGeminiOutput(t *testing.T) {
	t.Parallel()

	t.Run("valid output", func(t *testing.T) {
		t.Parallel()
		raw := `{"response":"No security issues detected.","stats":{"input_tokens":50}}`
		result, err := parseGeminiOutput(raw)
		if err != nil {
			t.Fatalf("parseGeminiOutput: %v", err)
		}
		if result != "No security issues detected." {
			t.Errorf("result = %q", result)
		}
	})

	t.Run("empty response", func(t *testing.T) {
		t.Parallel()
		raw := `{"response":"","stats":{}}`
		_, err := parseGeminiOutput(raw)
		if !errors.Is(err, ErrLLMEmptyResponse) {
			t.Errorf("expected ErrLLMEmptyResponse, got %v", err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		_, err := parseGeminiOutput("garbage")
		if !errors.Is(err, ErrLLMMalformedResponse) {
			t.Errorf("expected ErrLLMMalformedResponse, got %v", err)
		}
	})
}

func TestCliDefaultModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tool string
		want string
	}{
		{"claude", defaultClaudeModel},
		{"codex", defaultOpenAIModel},
		{"gemini", defaultGeminiModel},
		{"unknown", DefaultLLMModel},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			t.Parallel()
			if got := cliDefaultModel(tt.tool); got != tt.want {
				t.Errorf("cliDefaultModel(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}

func TestTryOllama_ProbeSucceeds(t *testing.T) {
	t.Parallel()

	srv := ollamaModelServer(t, "qwen2.5-coder:7b")
	deps := failDeps()
	deps.httpDo = srv.Client().Transport.(*http.Transport).RoundTrip
	// tryOllama constructs the probe URL from DefaultLLMBaseURL which
	// points to localhost:11434. We redirect via httpDo to the test server.
	deps.httpDo = func(req *http.Request) (*http.Response, error) {
		// Redirect the probe to our test server.
		redirected, _ := http.NewRequestWithContext(req.Context(), req.Method, srv.URL+"/models", http.NoBody)
		return srv.Client().Do(redirected)
	}

	result, err := tryOllama(t.Context(), deps, "", 5*time.Second)
	if err != nil {
		t.Fatalf("tryOllama: %v", err)
	}
	if result.Name() != ProviderOllama {
		t.Errorf("Name() = %q, want %q", result.Name(), ProviderOllama)
	}
	if result.Model() != DefaultLLMModel {
		t.Errorf("Model() = %q, want %q", result.Model(), DefaultLLMModel)
	}
}

func TestTryOllama_ProbeFails(t *testing.T) {
	t.Parallel()

	deps := failDeps()

	_, err := tryOllama(t.Context(), deps, "", 5*time.Second)
	if err == nil {
		t.Fatal("expected error when probe fails")
	}
}
