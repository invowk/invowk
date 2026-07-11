// SPDX-License-Identifier: MPL-2.0

package auditllm

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

	"github.com/invowk/invowk/internal/llm"
)

func TestLLMClient_Complete_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := readChatCompletionRequest(t, r)
		assertChatCompletionCoreRequest(t, req, "test-model", "system prompt", "user prompt")

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
	if !errors.Is(err, llm.ErrLLMEmptyResponse) {
		t.Errorf("expected llm.ErrLLMEmptyResponse, got %v", err)
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
	if !errors.Is(err, llm.ErrLLMEmptyResponse) {
		t.Errorf("expected llm.ErrLLMEmptyResponse, got %v", err)
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
	if !errors.Is(err, llm.ErrLLMResponseContentFiltered) {
		t.Errorf("expected llm.ErrLLMResponseContentFiltered, got %v", err)
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
	if !errors.Is(err, llm.ErrLLMRequestFailed) {
		t.Errorf("expected llm.ErrLLMRequestFailed, got %v", err)
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

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// Simulate a slow server without sleeping; cancellation closes the request context.
		<-r.Context().Done()
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
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestLLMClient_Complete_NetworkError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := srv.URL
	srv.Close()

	client, err := NewLLMClient(LLMClientConfig{
		BaseURL: baseURL,
		Model:   "test-model",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}

	_, err = client.Complete(t.Context(), "sys", "user")
	if !errors.Is(err, ErrLLMServerUnavailable) {
		t.Errorf("expected ErrLLMServerUnavailable, got %v", err)
	}

	var unavailableErr *LLMServerUnavailableError
	if !errors.As(err, &unavailableErr) {
		t.Fatalf("expected LLMServerUnavailableError, got %T", err)
	}
	if unavailableErr.URL != baseURL {
		t.Errorf("URL = %q, want %q", unavailableErr.URL, baseURL)
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
	t.Setenv("OPENAI_API_KEY", "sk-poison-ambient-key")
	t.Setenv("OPENAI_BASE_URL", "http://127.0.0.1:1/v1")

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
	if strings.Contains(gotAuth, "sk-poison-ambient-key") {
		t.Errorf("Authorization should not contain ambient OPENAI_API_KEY, got %q", gotAuth)
	}
}

func TestLLMClient_CompleteJSONSchema_OpenAIHostRequest(t *testing.T) {
	t.Parallel()

	var gotReq map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = readChatCompletionRequest(t, r)
		assertChatCompletionCoreRequest(t, gotReq, "test-model", "system prompt", "user prompt")

		resp := map[string]any{
			"id":      "chatcmpl-structured",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "test-model",
			"choices": []map[string]any{
				{
					"index":         0,
					"finish_reason": "stop",
					"message":       map[string]any{"role": "assistant", "content": `{"findings":[]}`},
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
	client.url = "https://api.openai.com/v1"

	format := llm.JSONSchemaFormat{
		Name:        "audit_findings",
		Description: "Audit findings response",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"findings": map[string]any{"type": "array"},
			},
			"required": []any{"findings"},
		},
		Strict: true,
	}

	content, err := client.CompleteJSONSchema(t.Context(), "system prompt", "user prompt", format)
	if err != nil {
		t.Fatalf("CompleteJSONSchema: %v", err)
	}
	if content != `{"findings":[]}` {
		t.Errorf("content = %q, want structured JSON", content)
	}

	responseFormat := requireMap(t, gotReq, "response_format")
	if responseFormat["type"] != "json_schema" {
		t.Errorf("response_format.type = %v, want json_schema", responseFormat["type"])
	}
	jsonSchema := requireMap(t, responseFormat, "json_schema")
	if jsonSchema["name"] != format.Name {
		t.Errorf("json_schema.name = %v, want %q", jsonSchema["name"], format.Name)
	}
	if jsonSchema["description"] != format.Description {
		t.Errorf("json_schema.description = %v, want %q", jsonSchema["description"], format.Description)
	}
	if jsonSchema["strict"] != true {
		t.Errorf("json_schema.strict = %v, want true", jsonSchema["strict"])
	}
	schema := requireMap(t, jsonSchema, "schema")
	if schema["type"] != "object" {
		t.Errorf("json_schema.schema.type = %v, want object", schema["type"])
	}
}

func TestLLMClient_CompleteJSONSchema_NonOpenAIHostNoRequest(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
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

	_, err = client.CompleteJSONSchema(t.Context(), "sys", "user", llm.JSONSchemaFormat{Name: "audit"})
	if !errors.Is(err, llm.ErrStructuredOutputUnsupported) {
		t.Errorf("expected ErrStructuredOutputUnsupported, got %v", err)
	}
	if called {
		t.Error("non-OpenAI structured-output path should not make an HTTP request")
	}
}

func readChatCompletionRequest(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	if r.URL.Path == "/responses" {
		t.Fatal("LLM client must preserve Chat Completions and must not use Responses API")
	}
	if r.Method != http.MethodPost {
		t.Fatalf("expected POST, got %s", r.Method)
	}
	if r.URL.Path != "/chat/completions" {
		t.Fatalf("expected /chat/completions path, got %s", r.URL.Path)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("invalid request JSON: %v", err)
	}
	return req
}

func assertChatCompletionCoreRequest(t *testing.T, req map[string]any, model, systemPrompt, userPrompt string) {
	t.Helper()

	if req["model"] != model {
		t.Errorf("model = %v, want %q", req["model"], model)
	}
	if req["temperature"] != 0.0 {
		t.Errorf("temperature = %v, want 0", req["temperature"])
	}

	messages, ok := req["messages"].([]any)
	if !ok {
		t.Fatalf("messages has type %T, want []any", req["messages"])
	}
	if len(messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(messages))
	}

	assertMessage(t, messages[0], "system", systemPrompt)
	assertMessage(t, messages[1], "user", userPrompt)
}

func assertMessage(t *testing.T, message any, role, content string) {
	t.Helper()

	msg, ok := message.(map[string]any)
	if !ok {
		t.Fatalf("message has type %T, want map[string]any", message)
	}
	if msg["role"] != role {
		t.Errorf("role = %v, want %q", msg["role"], role)
	}
	if msg["content"] != content {
		t.Errorf("content = %v, want %q", msg["content"], content)
	}
}

func requireMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s has type %T, want map[string]any", key, parent[key])
	}
	return value
}
