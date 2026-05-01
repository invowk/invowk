// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/invowk/invowk/pkg/types"
)

// ErrTUIServerResponse is returned when the TUI server responds with an error message.
var ErrTUIServerResponse = errors.New("TUI server error")

// Client provides methods to communicate with the TUI server.
// It is used by child processes to delegate TUI rendering to the parent.
type Client struct {
	addr   string // Composite URL — intentional string (transport boundary).
	token  AuthToken
	client *http.Client
}

// NewClientFromEnv creates a new Client from environment variables.
// Returns nil if the environment variables are not set.
func NewClientFromEnv() *Client {
	addr := os.Getenv(EnvTUIAddr)
	token := os.Getenv(EnvTUIToken)

	if addr == "" || token == "" {
		return nil
	}

	authToken := AuthToken(token)
	if err := authToken.Validate(); err != nil {
		return nil
	}
	return NewClient(addr, authToken)
}

// NewClient creates a new Client with the given server address and token.
func NewClient(addr string, token AuthToken) *Client {
	return &Client{
		addr:  addr,
		token: token,
		client: &http.Client{
			Timeout: 10 * time.Minute, // Long timeout for user interaction
		},
	}
}

// IsAvailable checks if the TUI server is available.
func (c *Client) IsAvailable() bool {
	return c.IsAvailableContext(context.Background())
}

// IsAvailableContext checks if the TUI server is available with caller cancellation.
func (c *Client) IsAvailableContext(ctx context.Context) bool {
	if c == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.addr+"/health", http.NoBody)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }() // Health check; close error non-critical

	return resp.StatusCode == http.StatusOK
}

// Input sends an input prompt request to the TUI server.
// Returns the entered text or an error.
func (c *Client) Input(opts InputRequest) (string, error) {
	return c.InputContext(context.Background(), opts)
}

// InputContext sends an input prompt request with caller cancellation.
func (c *Client) InputContext(ctx context.Context, opts InputRequest) (string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentInput, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", types.ErrUserCancelled
	}

	var result InputResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse input result: %w", err)
	}

	return result.Value, nil
}

// Confirm sends a confirm prompt request to the TUI server.
// Returns true if confirmed, false if not.
func (c *Client) Confirm(opts ConfirmRequest) (bool, error) {
	return c.ConfirmContext(context.Background(), opts)
}

// ConfirmContext sends a confirm prompt request with caller cancellation.
func (c *Client) ConfirmContext(ctx context.Context, opts ConfirmRequest) (bool, error) {
	resp, err := c.sendRequestContext(ctx, ComponentConfirm, opts)
	if err != nil {
		return false, err
	}

	if resp.Cancelled {
		return false, types.ErrUserCancelled
	}

	var result ConfirmResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return false, fmt.Errorf("failed to parse confirm result: %w", err)
	}

	return result.Confirmed, nil
}

// Choose sends a choose prompt request to the TUI server.
// For single-select (limit <= 1), returns the selected option as a string.
// For multi-select (limit > 1 or no_limit), returns a slice of strings.
func (c *Client) Choose(opts ChooseRequest) (any, error) {
	return c.ChooseContext(context.Background(), opts)
}

// ChooseContext sends a choose prompt request with caller cancellation.
func (c *Client) ChooseContext(ctx context.Context, opts ChooseRequest) (any, error) {
	resp, err := c.sendRequestContext(ctx, ComponentChoose, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, types.ErrUserCancelled
	}

	var result ChooseResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse choose result: %w", err)
	}

	return result.Selected, nil
}

// ChooseSingle is a convenience method for single-select choose.
// Returns the selected option as a string.
func (c *Client) ChooseSingle(opts ChooseRequest) (string, error) {
	return c.ChooseSingleContext(context.Background(), opts)
}

// ChooseSingleContext is a convenience method for single-select choose with caller cancellation.
func (c *Client) ChooseSingleContext(ctx context.Context, opts ChooseRequest) (string, error) {
	opts.Limit = 1
	opts.NoLimit = false

	result, err := c.ChooseContext(ctx, opts)
	if err != nil {
		return "", err
	}

	// Result could be a string or a single-element array
	switch v := result.(type) {
	case string:
		return v, nil
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s, nil
			}
		}
		return "", nil
	default:
		return "", fmt.Errorf("unexpected choose result type: %T", result)
	}
}

// ChooseMultiple is a convenience method for multi-select choose.
// Returns the selected options as a slice of strings.
func (c *Client) ChooseMultiple(opts ChooseRequest) ([]string, error) {
	return c.ChooseMultipleContext(context.Background(), opts)
}

// ChooseMultipleContext is a convenience method for multi-select choose with caller cancellation.
func (c *Client) ChooseMultipleContext(ctx context.Context, opts ChooseRequest) ([]string, error) {
	result, err := c.ChooseContext(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Result should be an array
	switch v := result.(type) {
	case []any:
		strs := make([]string, len(v))
		for i, item := range v {
			if s, ok := item.(string); ok {
				strs[i] = s
			}
		}
		return strs, nil
	case string:
		return []string{v}, nil
	default:
		return nil, fmt.Errorf("unexpected choose result type: %T", result)
	}
}

// Filter sends a filter prompt request to the TUI server.
// Returns the selected options.
func (c *Client) Filter(opts FilterRequest) ([]string, error) {
	return c.FilterContext(context.Background(), opts)
}

// FilterContext sends a filter prompt request with caller cancellation.
func (c *Client) FilterContext(ctx context.Context, opts FilterRequest) ([]string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentFilter, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, types.ErrUserCancelled
	}

	var result FilterResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse filter result: %w", err)
	}

	return result.Selected, nil
}

// File sends a file picker request to the TUI server.
// Returns the selected file path.
func (c *Client) File(opts FileRequest) (string, error) {
	return c.FileContext(context.Background(), opts)
}

// FileContext sends a file picker request with caller cancellation.
func (c *Client) FileContext(ctx context.Context, opts FileRequest) (string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentFile, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", types.ErrUserCancelled
	}

	var result FileResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse file result: %w", err)
	}

	return result.Path, nil
}

// Write sends a styled text output request to the TUI server.
func (c *Client) Write(opts WriteRequest) error {
	return c.WriteContext(context.Background(), opts)
}

// WriteContext sends a styled text output request with caller cancellation.
func (c *Client) WriteContext(ctx context.Context, opts WriteRequest) error {
	_, err := c.sendRequestContext(ctx, ComponentWrite, opts)
	return err
}

// TextArea sends a multi-line text input request to the TUI server.
// Returns the entered text or an error.
func (c *Client) TextArea(opts TextAreaRequest) (string, error) {
	return c.TextAreaContext(context.Background(), opts)
}

// TextAreaContext sends a multi-line text input request with caller cancellation.
func (c *Client) TextAreaContext(ctx context.Context, opts TextAreaRequest) (string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentTextArea, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", types.ErrUserCancelled
	}

	var result TextAreaResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse textarea result: %w", err)
	}

	return result.Value, nil
}

// Spin sends a spinner request to the TUI server.
// Returns the command output and exit code.
func (c *Client) Spin(opts SpinRequest) (*SpinResult, error) {
	return c.SpinContext(context.Background(), opts)
}

// SpinContext sends a spinner request with caller cancellation.
func (c *Client) SpinContext(ctx context.Context, opts SpinRequest) (*SpinResult, error) {
	resp, err := c.sendRequestContext(ctx, ComponentSpin, opts)
	if err != nil {
		return nil, err
	}

	var result SpinResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse spin result: %w", err)
	}

	return &result, nil
}

// Pager sends a pager request to the TUI server.
func (c *Client) Pager(opts PagerRequest) error {
	return c.PagerContext(context.Background(), opts)
}

// PagerContext sends a pager request with caller cancellation.
func (c *Client) PagerContext(ctx context.Context, opts PagerRequest) error {
	_, err := c.sendRequestContext(ctx, ComponentPager, opts)
	return err
}

// Table sends a table request to the TUI server.
// Returns the selected row and index.
func (c *Client) Table(opts TableRequest) (*TableResult, error) {
	return c.TableContext(context.Background(), opts)
}

// TableContext sends a table request with caller cancellation.
func (c *Client) TableContext(ctx context.Context, opts TableRequest) (*TableResult, error) {
	resp, err := c.sendRequestContext(ctx, ComponentTable, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, types.ErrUserCancelled
	}

	var result TableResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse table result: %w", err)
	}

	return &result, nil
}

// sendRequestContext sends a TUI request to the server and returns the response.
func (c *Client) sendRequestContext(ctx context.Context, component Component, options any) (result *Response, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}

	req := Request{
		Component: component,
		Options:   optionsJSON,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr+"/tui", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+string(c.token))

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error (%d): %s", httpResp.StatusCode, string(body))
	}

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrTUIServerResponse, resp.Error)
	}

	return &resp, nil
}
