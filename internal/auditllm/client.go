// SPDX-License-Identifier: MPL-2.0

package auditllm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"

	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/internal/llm"
)

const (
	llmClientConfigInvalidErrMsg = "invalid LLM client config"
	llmServerUnavailableErrMsg   = "LLM server unavailable"
	llmModelNotFoundErrMsg       = "LLM model not found"

	// DefaultLLMBaseURL is the default Ollama OpenAI-compatible endpoint.
	DefaultLLMBaseURL = "http://localhost:11434/v1"
	// DefaultLLMModel is a good balance of quality and resource usage.
	DefaultLLMModel = "qwen2.5-coder:7b"
	// DefaultLLMTimeout is generous to accommodate slow inference on CPU.
	DefaultLLMTimeout = 2 * time.Minute
	// DefaultLLMConcurrency limits parallel LLM requests.
	DefaultLLMConcurrency = audit.DefaultLLMConcurrency
)

var (
	_ llm.Completer = (*LLMClient)(nil) // compile-time interface assertion
	_ ModelVerifier = (*LLMClient)(nil) // compile-time interface assertion

	// ErrLLMClientConfigInvalid is the sentinel for invalid client configuration.
	ErrLLMClientConfigInvalid = errors.New(llmClientConfigInvalidErrMsg)
	// ErrLLMServerUnavailable is the sentinel for unreachable LLM servers.
	ErrLLMServerUnavailable = errors.New(llmServerUnavailableErrMsg)
	// ErrLLMModelNotFound is the sentinel for when the configured model is not available.
	ErrLLMModelNotFound = errors.New(llmModelNotFoundErrMsg)

	// codeModelPatterns are substrings that identify code-focused models,
	// ordered by preference. Uses pattern matching rather than exact names
	// so new model versions are automatically detected.
	codeModelPatterns = []string{
		"qwen2.5-coder",
		"deepseek-coder",
		"codellama",
		"codegemma",
		"starcoder",
		"codestral",
	}
)

type (
	// ModelVerifier is an optional capability for completers that support
	// pre-flight model validation via API model listing. HTTP-based
	// completers (LLMClient) implement this; CLI-based completers do not
	// because CLI tools have no model-listing endpoint.
	ModelVerifier interface {
		VerifyModel(ctx context.Context) error
	}

	// LLMClientConfig holds the configuration for creating an LLMClient.
	// BaseURL and Model are required; APIKey is optional (empty means no auth,
	// which is the common case for local servers like Ollama).
	//nolint:recvcheck // Validate() uses pointer, withDefaults() uses value (copy semantics)
	LLMClientConfig struct {
		BaseURL     string
		Model       string
		APIKey      string
		Timeout     time.Duration
		Concurrency int
	}

	// LLMClient sends chat completion requests to an OpenAI-compatible API.
	// Safe for concurrent use — each request creates its own context.
	LLMClient struct {
		client *openai.Client
		model  string
		url    string
	}

	// LLMClientConfigInvalidError is returned when LLMClientConfig validation fails.
	LLMClientConfigInvalidError struct {
		Reason string
	}

	// LLMRequestError is returned when the LLM API returns a non-success response.
	LLMRequestError struct {
		StatusCode int
		Body       string
	}

	// LLMServerUnavailableError is returned when the LLM server cannot be reached.
	LLMServerUnavailableError struct {
		URL string
		Err error
	}

	// LLMModelNotFoundError is returned when the configured model is not
	// available on the server. It includes available models and suggestions
	// for code-focused alternatives.
	LLMModelNotFoundError struct {
		Model      string
		Available  []string
		Suggestion string
	}
)

// Error implements the error interface.
func (e *LLMClientConfigInvalidError) Error() string {
	return fmt.Sprintf("%s: %s", llmClientConfigInvalidErrMsg, e.Reason)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMClientConfigInvalidError) Unwrap() error { return ErrLLMClientConfigInvalid }

// Error implements the error interface.
func (e *LLMRequestError) Error() string {
	return fmt.Sprintf("%s (status %d): %s", audit.ErrLLMRequestFailed, e.StatusCode, e.Body)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMRequestError) Unwrap() error { return audit.ErrLLMRequestFailed }

// Error implements the error interface.
func (e *LLMServerUnavailableError) Error() string {
	return fmt.Sprintf("%s at %q: %v", llmServerUnavailableErrMsg, e.URL, e.Err)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMServerUnavailableError) Unwrap() error { return ErrLLMServerUnavailable }

// Error implements the error interface.
func (e *LLMModelNotFoundError) Error() string {
	msg := fmt.Sprintf("%s: %q is not available on the server", llmModelNotFoundErrMsg, e.Model)
	if e.Suggestion != "" {
		msg += "; try: " + e.Suggestion
	}
	if len(e.Available) > 0 {
		// Show up to 10 available models to avoid flooding the terminal.
		shown := e.Available
		suffix := ""
		if len(shown) > 10 {
			shown = shown[:10]
			suffix = fmt.Sprintf(" (and %d more)", len(e.Available)-10)
		}
		msg += fmt.Sprintf("\navailable models: %s%s", strings.Join(shown, ", "), suffix)
	}
	return msg
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMModelNotFoundError) Unwrap() error { return ErrLLMModelNotFound }

// Validate checks that all required fields are present and valid.
func (c *LLMClientConfig) Validate() error {
	if c.BaseURL == "" {
		return &LLMClientConfigInvalidError{Reason: "base URL is required"}
	}
	if c.Model == "" {
		return &LLMClientConfigInvalidError{Reason: "model is required"}
	}
	if c.Timeout < 0 {
		return &LLMClientConfigInvalidError{Reason: "timeout must be non-negative"}
	}
	if c.Concurrency < 0 {
		return &LLMClientConfigInvalidError{Reason: "concurrency must be non-negative"}
	}
	return nil
}

// withDefaults returns a copy with zero-valued fields set to defaults.
func (c LLMClientConfig) withDefaults() LLMClientConfig {
	if c.BaseURL == "" {
		c.BaseURL = DefaultLLMBaseURL
	}
	if c.Model == "" {
		c.Model = DefaultLLMModel
	}
	if c.Timeout == 0 {
		c.Timeout = DefaultLLMTimeout
	}
	if c.Concurrency == 0 {
		c.Concurrency = DefaultLLMConcurrency
	}
	return c
}

// NewLLMClient creates an LLMClient configured for the given endpoint.
// The underlying openai.Client handles HTTP lifecycle and retries.
func NewLLMClient(cfg LLMClientConfig) (*LLMClient, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts := []option.RequestOption{
		option.WithBaseURL(cfg.BaseURL),
		option.WithHTTPClient(&http.Client{Timeout: cfg.Timeout}),
	}

	// Only set API key when provided — local servers like Ollama
	// reject non-empty Authorization headers in some configurations.
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	} else {
		// Suppress the SDK's automatic OPENAI_API_KEY env var lookup.
		opts = append(opts, option.WithAPIKey("ollama"))
	}

	client := openai.NewClient(opts...)

	return &LLMClient{
		client: &client,
		model:  cfg.Model,
		url:    cfg.BaseURL,
	}, nil
}

// Complete sends a chat completion request with the given system and user prompts.
// Returns the assistant's response content. The context controls the request
// lifecycle; callers should set appropriate deadlines for LLM inference.
func (c *LLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: c.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
		Temperature: param.NewOpt(0.0),
	})
	if err != nil {
		return "", c.classifyError(err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("%w: no choices in response", audit.ErrLLMEmptyResponse)
	}

	choice := resp.Choices[0]

	// Check for content filtering.
	if choice.FinishReason == "content_filter" {
		return "", fmt.Errorf("%w: model refused to analyze content", audit.ErrLLMResponseContentFiltered)
	}

	content := strings.TrimSpace(choice.Message.Content)
	if content == "" {
		return "", fmt.Errorf("%w: empty content in response", audit.ErrLLMEmptyResponse)
	}

	return content, nil
}

// VerifyModel checks that the configured model is available on the server by
// querying GET /v1/models. Returns nil if the model exists, or a descriptive
// LLMModelNotFoundError with available models and suggestions if not.
// Server connectivity errors are returned directly for consistent handling.
func (c *LLMClient) VerifyModel(ctx context.Context) error {
	page, err := c.client.Models.List(ctx)
	if err != nil {
		return c.classifyError(err)
	}

	available := make([]string, 0, len(page.Data))
	for i := range page.Data {
		available = append(available, page.Data[i].ID)
		if page.Data[i].ID == c.model {
			return nil
		}
	}

	// Also check for prefix matches — Ollama returns model IDs like
	// "qwen2.5-coder:7b" but users might specify "qwen2.5-coder:7b-q4_0".
	for _, name := range available {
		if strings.HasPrefix(name, c.model) || strings.HasPrefix(c.model, name) {
			return nil
		}
	}

	return &LLMModelNotFoundError{
		Model:      c.model,
		Available:  available,
		Suggestion: suggestCodeModel(available),
	}
}

// suggestCodeModel returns the best code-focused model from the available list,
// or an install hint if none are found. Uses pattern matching on model names
// to dynamically detect code-focused models regardless of version/quantization.
func suggestCodeModel(available []string) string {
	// For each pattern (in preference order), find the largest matching model.
	for _, pattern := range codeModelPatterns {
		var best string
		for _, avail := range available {
			if strings.Contains(strings.ToLower(avail), pattern) {
				// Prefer models with more characters in the name — a rough
				// heuristic that favors larger variants (32b > 14b > 7b)
				// without parsing version numbers.
				if best == "" || len(avail) > len(best) {
					best = avail
				}
			}
		}
		if best != "" {
			return best
		}
	}

	return "ollama pull qwen2.5-coder:7b"
}

// classifyError maps SDK and network errors to typed audit errors.
func (c *LLMClient) classifyError(err error) error {
	// Check for network-level errors (connection refused, DNS failure).
	if netErr, ok := errors.AsType[*net.OpError](err); ok && netErr != nil {
		return &LLMServerUnavailableError{URL: c.url, Err: err}
	}

	// The openai-go SDK wraps API errors; check for HTTP status codes.
	if apiErr, ok := errors.AsType[*openai.Error](err); ok {
		return &LLMRequestError{
			StatusCode: apiErr.StatusCode,
			Body:       apiErr.Message,
		}
	}

	// Context cancellation or deadline exceeded — pass through directly
	// so the scanner's partial-results model handles it naturally.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	return fmt.Errorf("%w: %w", audit.ErrLLMRequestFailed, err)
}
