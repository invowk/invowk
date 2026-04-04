// SPDX-License-Identifier: MPL-2.0

package audit

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
)

const (
	llmClientConfigInvalidErrMsg     = "invalid LLM client config"
	llmRequestFailedErrMsg           = "LLM request failed"
	llmServerUnavailableErrMsg       = "LLM server unavailable"
	llmMalformedResponseErrMsg       = "LLM returned malformed response"
	llmEmptyResponseErrMsg           = "LLM returned empty response"
	llmResponseContentFilteredErrMsg = "LLM response was filtered"

	// DefaultLLMBaseURL is the default Ollama OpenAI-compatible endpoint.
	DefaultLLMBaseURL = "http://localhost:11434/v1"
	// DefaultLLMModel is a good balance of quality and resource usage.
	DefaultLLMModel = "qwen2.5-coder:7b"
	// DefaultLLMTimeout is generous to accommodate slow inference on CPU.
	DefaultLLMTimeout = 2 * time.Minute
	// DefaultLLMConcurrency limits parallel LLM requests.
	DefaultLLMConcurrency = 2

	// maxErrorResponseLen is the truncation limit for raw LLM responses in error messages.
	maxErrorResponseLen = 200
)

// Sentinel errors for LLM operations.
var (
	// ErrLLMClientConfigInvalid is the sentinel for invalid client configuration.
	ErrLLMClientConfigInvalid = errors.New(llmClientConfigInvalidErrMsg)
	// ErrLLMRequestFailed is the sentinel for general LLM request failures.
	ErrLLMRequestFailed = errors.New(llmRequestFailedErrMsg)
	// ErrLLMServerUnavailable is the sentinel for unreachable LLM servers.
	ErrLLMServerUnavailable = errors.New(llmServerUnavailableErrMsg)
	// ErrLLMMalformedResponse is the sentinel for unparseable LLM responses.
	ErrLLMMalformedResponse = errors.New(llmMalformedResponseErrMsg)
	// ErrLLMEmptyResponse is the sentinel for empty LLM responses.
	ErrLLMEmptyResponse = errors.New(llmEmptyResponseErrMsg)
	// ErrLLMResponseContentFiltered is the sentinel for content-filtered responses.
	ErrLLMResponseContentFiltered = errors.New(llmResponseContentFilteredErrMsg)
)

type (
	// llmCompleter abstracts the LLM chat completion call for testability.
	// Production code uses *LLMClient; tests inject a mock.
	llmCompleter interface {
		Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error)
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

	// LLMMalformedResponseError is returned when the LLM response cannot be parsed.
	LLMMalformedResponseError struct {
		RawResponse string
		Err         error
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
	return fmt.Sprintf("%s (status %d): %s", llmRequestFailedErrMsg, e.StatusCode, e.Body)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMRequestError) Unwrap() error { return ErrLLMRequestFailed }

// Error implements the error interface.
func (e *LLMServerUnavailableError) Error() string {
	return fmt.Sprintf("%s at %q: %v", llmServerUnavailableErrMsg, e.URL, e.Err)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMServerUnavailableError) Unwrap() error { return ErrLLMServerUnavailable }

// Error implements the error interface.
func (e *LLMMalformedResponseError) Error() string {
	raw := e.RawResponse
	if len(raw) > maxErrorResponseLen {
		raw = raw[:maxErrorResponseLen] + "..."
	}
	return fmt.Sprintf("%s: %v (response: %q)", llmMalformedResponseErrMsg, e.Err, raw)
}

// Unwrap returns the sentinel for errors.Is chains.
func (e *LLMMalformedResponseError) Unwrap() error { return ErrLLMMalformedResponse }

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
		return "", fmt.Errorf("%w: no choices in response", ErrLLMEmptyResponse)
	}

	choice := resp.Choices[0]

	// Check for content filtering.
	if choice.FinishReason == "content_filter" {
		return "", fmt.Errorf("%w: model refused to analyze content", ErrLLMResponseContentFiltered)
	}

	content := strings.TrimSpace(choice.Message.Content)
	if content == "" {
		return "", fmt.Errorf("%w: empty content in response", ErrLLMEmptyResponse)
	}

	return content, nil
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

	return fmt.Errorf("%w: %w", ErrLLMRequestFailed, err)
}
