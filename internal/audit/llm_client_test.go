// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		{
			name:    "negative concurrency",
			cfg:     LLMClientConfig{BaseURL: "http://x", Model: "m", Concurrency: -1},
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
	if cfg.Concurrency != DefaultLLMConcurrency {
		t.Errorf("Concurrency = %d, want %d", cfg.Concurrency, DefaultLLMConcurrency)
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

func TestLLMClient_Complete_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("expected /chat/completions path, got %s", r.URL.Path)
		}

		// Verify request body structure.
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("invalid request JSON: %v", err)
		}
		if req["model"] != "test-model" {
			t.Errorf("expected model 'test-model', got %v", req["model"])
		}

		// Return a valid chat completion response.
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"findings": []}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

	content, err := client.Complete(t.Context(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	want := `{"findings": []}`
	if content != want {
		t.Errorf("content = %q, want %q", content, want)
	}
}

func TestLLMClient_Complete_EmptyChoices(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-empty",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

	_, err = client.Complete(t.Context(), "sys", "user")
	if !errors.Is(err, ErrLLMEmptyResponse) {
		t.Errorf("expected ErrLLMEmptyResponse, got %v", err)
	}
}

func TestLLMClient_Complete_EmptyContent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-blank",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message": map[string]any{
						"role":    "assistant",
						"content": "   ",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

	_, err = client.Complete(t.Context(), "sys", "user")
	if !errors.Is(err, ErrLLMEmptyResponse) {
		t.Errorf("expected ErrLLMEmptyResponse, got %v", err)
	}
}

func TestLLMClient_Complete_ContentFiltered(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-filtered",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "content_filter",
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

	_, err = client.Complete(t.Context(), "sys", "user")
	if !errors.Is(err, ErrLLMResponseContentFiltered) {
		t.Errorf("expected ErrLLMResponseContentFiltered, got %v", err)
	}
}

func TestLLMClient_Complete_ServerError(t *testing.T) {
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

	_, err = client.Complete(t.Context(), "sys", "user")
	if !errors.Is(err, ErrLLMRequestFailed) {
		t.Errorf("expected ErrLLMRequestFailed, got %v", err)
	}

	var reqErr *LLMRequestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected LLMRequestError, got %T", err)
	}
	if reqErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", reqErr.StatusCode, http.StatusInternalServerError)
	}
}

func TestLLMClient_Complete_ContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Simulate slow server that never responds.
		time.Sleep(10 * time.Second)
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: srv.URL,
		Model:   "test-model",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	_, err = client.Complete(ctx, "sys", "user")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestLLMClient_Complete_APIKeyIncluded(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{
			"id":      "chatcmpl-auth",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "ok"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client, err := NewLLMClient(LLMClientConfig{ //nolint:gosec // Test credential, not a real API key.
		BaseURL: srv.URL,
		Model:   "test-model",
		APIKey:  "sk-test-key-123",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	_, err = client.Complete(t.Context(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if gotAuth != "Bearer sk-test-key-123" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer sk-test-key-123")
	}
}

func TestLLMClient_Complete_NoAPIKey(t *testing.T) {
	t.Parallel()

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		resp := map[string]any{
			"id":      "chatcmpl-noauth",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": "ok"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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

	_, err = client.Complete(t.Context(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// When no API key is provided, we set "ollama" as the key to suppress
	// SDK auto-detection from OPENAI_API_KEY env var.
	if gotAuth != "Bearer ollama" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer ollama")
	}
}
