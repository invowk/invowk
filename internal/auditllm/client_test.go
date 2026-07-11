// SPDX-License-Identifier: MPL-2.0

package auditllm

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/llm"
)

func TestLLMClientConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     LLMClientConfig
		wantErr bool
	}{
		{
			name:    "valid config with defaults applied",
			cfg:     LLMClientConfig{BaseURL: "http://localhost:11434/v1", Model: "test"},
			wantErr: false,
		},
		{
			name:    "missing base URL",
			cfg:     LLMClientConfig{Model: "test"},
			wantErr: true,
		},
		{
			name:    "missing model",
			cfg:     LLMClientConfig{BaseURL: "http://localhost:11434/v1"},
			wantErr: true,
		},
		{
			name:    "negative timeout",
			cfg:     LLMClientConfig{BaseURL: "http://x", Model: "m", Timeout: -1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrLLMClientConfigInvalid) {
				t.Errorf("expected ErrLLMClientConfigInvalid sentinel, got %v", err)
			}
		})
	}
}

func TestLLMClientConfig_withDefaults(t *testing.T) {
	t.Parallel()

	cfg := LLMClientConfig{}.withDefaults()
	if cfg.BaseURL != DefaultLLMBaseURL {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, DefaultLLMBaseURL)
	}
	if cfg.Model != DefaultLLMModel {
		t.Errorf("Model = %q, want %q", cfg.Model, DefaultLLMModel)
	}
	if cfg.Timeout != DefaultLLMTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, DefaultLLMTimeout)
	}
}

func TestNewLLMClient_InvalidConfig(t *testing.T) {
	t.Parallel()

	// Force validation failure: empty config with negative timeout defeats defaults.
	_, err := NewLLMClient(LLMClientConfig{Timeout: -1})
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !errors.Is(err, ErrLLMClientConfigInvalid) {
		t.Errorf("expected ErrLLMClientConfigInvalid, got %v", err)
	}
}

func TestLLMClient_VerifyModel_Found(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "llama3:8b", "object": "model", "created": 1234567890, "owned_by": "library"},
				{"id": "qwen2.5-coder:7b", "object": "model", "created": 1234567890, "owned_by": "library"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "qwen2.5-coder:7b",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	if err := client.VerifyModel(t.Context()); err != nil {
		t.Fatalf("VerifyModel should succeed for available model: %v", err)
	}
}

func TestLLMClient_VerifyModel_NotFound(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "llama3:8b", "object": "model", "created": 1234567890, "owned_by": "library"},
				{"id": "mistral:7b", "object": "model", "created": 1234567890, "owned_by": "library"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "qwen2.5-coder:7b",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	verifyErr := client.VerifyModel(t.Context())
	if verifyErr == nil {
		t.Fatal("VerifyModel should fail for unavailable model")
	}
	if !errors.Is(verifyErr, ErrLLMModelNotFound) {
		t.Errorf("expected ErrLLMModelNotFound, got %v", verifyErr)
	}

	var modelErr *LLMModelNotFoundError
	if !errors.As(verifyErr, &modelErr) {
		t.Fatalf("expected LLMModelNotFoundError, got %T", verifyErr)
	}
	if modelErr.Model != "qwen2.5-coder:7b" {
		t.Errorf("Model = %q, want %q", modelErr.Model, "qwen2.5-coder:7b")
	}
	if len(modelErr.Available) != 2 {
		t.Errorf("Available = %d models, want 2", len(modelErr.Available))
	}
}

func TestLLMClient_VerifyModel_NotFoundWithSuggestion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "llama3:8b", "object": "model", "created": 1234567890, "owned_by": "library"},
				{"id": "qwen2.5-coder:14b", "object": "model", "created": 1234567890, "owned_by": "library"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "nonexistent-model",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	verifyErr := client.VerifyModel(t.Context())
	if verifyErr == nil {
		t.Fatal("VerifyModel should fail")
	}

	var modelErr *LLMModelNotFoundError
	if !errors.As(verifyErr, &modelErr) {
		t.Fatalf("expected LLMModelNotFoundError, got %T", verifyErr)
	}
	// Should suggest qwen2.5-coder:14b since it's available and code-focused.
	if modelErr.Suggestion != "qwen2.5-coder:14b" {
		t.Errorf("Suggestion = %q, want %q", modelErr.Suggestion, "qwen2.5-coder:14b")
	}
}

func TestLLMClient_VerifyModel_EmptyModelList(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"object": "list",
			"data":   []map[string]any{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "qwen2.5-coder:7b",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	verifyErr := client.VerifyModel(t.Context())
	if verifyErr == nil {
		t.Fatal("VerifyModel should fail for empty model list")
	}
	if !errors.Is(verifyErr, ErrLLMModelNotFound) {
		t.Errorf("expected ErrLLMModelNotFound, got %v", verifyErr)
	}

	var modelErr *LLMModelNotFoundError
	if !errors.As(verifyErr, &modelErr) {
		t.Fatalf("expected LLMModelNotFoundError, got %T", verifyErr)
	}
	// With no code models available, should suggest installing one.
	if modelErr.Suggestion != "ollama pull qwen2.5-coder:7b" {
		t.Errorf("Suggestion = %q, want install hint", modelErr.Suggestion)
	}
}

func TestLLMClient_VerifyModel_PrefixMatch(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "qwen2.5-coder:7b-q4_K_M", "object": "model", "created": 1234567890, "owned_by": "library"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// User specifies base model name, server has quantized variant.
	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "qwen2.5-coder:7b",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	if err := client.VerifyModel(t.Context()); err != nil {
		t.Fatalf("VerifyModel should succeed for prefix match: %v", err)
	}
}

func TestLLMClient_VerifyModel_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "internal server error",
				"type":    "server_error",
				"code":    "server_error",
			},
		})
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	verifyErr := client.VerifyModel(t.Context())
	if verifyErr == nil {
		t.Fatal("VerifyModel should fail for server error")
	}
	if !errors.Is(verifyErr, llm.ErrLLMRequestFailed) {
		t.Errorf("expected llm.ErrLLMRequestFailed, got %v", verifyErr)
	}
}

func TestSuggestCodeModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		available []string
		want      string
	}{
		{
			name:      "prefers largest qwen coder variant",
			available: []string{"llama3:8b", "qwen2.5-coder:7b", "qwen2.5-coder:32b"},
			want:      "qwen2.5-coder:32b",
		},
		{
			name:      "matches qwen coder any version",
			available: []string{"llama3:8b", "qwen2.5-coder:7b"},
			want:      "qwen2.5-coder:7b",
		},
		{
			name:      "falls back to codellama when no qwen",
			available: []string{"llama3:8b", "codellama:13b"},
			want:      "codellama:13b",
		},
		{
			name:      "detects deepseek-coder",
			available: []string{"llama3:8b", "deepseek-coder:33b"},
			want:      "deepseek-coder:33b",
		},
		{
			name:      "no code models suggests install",
			available: []string{"llama3:8b", "mistral:7b"},
			want:      "ollama pull qwen2.5-coder:7b",
		},
		{
			name:      "empty list suggests install",
			available: nil,
			want:      "ollama pull qwen2.5-coder:7b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := suggestCodeModel(tt.available)
			if got != tt.want {
				t.Errorf("suggestCodeModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLLMModelNotFoundError_Format(t *testing.T) {
	t.Parallel()

	err := &LLMModelNotFoundError{
		Model:      "missing-model",
		Available:  []string{"llama3:8b", "mistral:7b"},
		Suggestion: "ollama pull qwen2.5-coder:7b",
	}

	msg := err.Error()
	if !strings.Contains(msg, "missing-model") {
		t.Error("error should contain model name")
	}
	if !strings.Contains(msg, "ollama pull") {
		t.Error("error should contain suggestion")
	}
	if !strings.Contains(msg, "llama3:8b") {
		t.Error("error should list available models")
	}
	if !errors.Is(err, ErrLLMModelNotFound) {
		t.Error("should unwrap to ErrLLMModelNotFound")
	}
}
