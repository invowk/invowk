// SPDX-License-Identifier: EPL-2.0

package tuiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client provides methods to communicate with the TUI server.
// It is used by child processes to delegate TUI rendering to the parent.
type Client struct {
	addr   string
	token  string
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

	return NewClient(addr, token)
}

// NewClient creates a new Client with the given server address and token.
func NewClient(addr, token string) *Client {
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
	if c == nil {
		return false
	}

	resp, err := c.client.Get(c.addr + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// sendRequest sends a TUI request to the server and returns the response.
func (c *Client) sendRequest(component Component, options interface{}) (*Response, error) {
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

	httpReq, err := http.NewRequest(http.MethodPost, c.addr+"/tui", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer httpResp.Body.Close()

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
		return nil, fmt.Errorf("TUI error: %s", resp.Error)
	}

	return &resp, nil
}

// Input sends an input prompt request to the TUI server.
// Returns the entered text or an error.
func (c *Client) Input(opts InputRequest) (string, error) {
	resp, err := c.sendRequest(ComponentInput, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", fmt.Errorf("user aborted")
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
	resp, err := c.sendRequest(ComponentConfirm, opts)
	if err != nil {
		return false, err
	}

	if resp.Cancelled {
		return false, fmt.Errorf("user aborted")
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
func (c *Client) Choose(opts ChooseRequest) (interface{}, error) {
	resp, err := c.sendRequest(ComponentChoose, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, fmt.Errorf("user aborted")
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
	opts.Limit = 1
	opts.NoLimit = false

	result, err := c.Choose(opts)
	if err != nil {
		return "", err
	}

	// Result could be a string or a single-element array
	switch v := result.(type) {
	case string:
		return v, nil
	case []interface{}:
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
	result, err := c.Choose(opts)
	if err != nil {
		return nil, err
	}

	// Result should be an array
	switch v := result.(type) {
	case []interface{}:
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
	resp, err := c.sendRequest(ComponentFilter, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, fmt.Errorf("user aborted")
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
	resp, err := c.sendRequest(ComponentFile, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", fmt.Errorf("user aborted")
	}

	var result FileResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse file result: %w", err)
	}

	return result.Path, nil
}

// Write sends a styled text output request to the TUI server.
func (c *Client) Write(opts WriteRequest) error {
	_, err := c.sendRequest(ComponentWrite, opts)
	return err
}

// TextArea sends a multi-line text input request to the TUI server.
// Returns the entered text or an error.
func (c *Client) TextArea(opts TextAreaRequest) (string, error) {
	resp, err := c.sendRequest(ComponentTextArea, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", fmt.Errorf("user aborted")
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
	resp, err := c.sendRequest(ComponentSpin, opts)
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
	_, err := c.sendRequest(ComponentPager, opts)
	return err
}

// Table sends a table request to the TUI server.
// Returns the selected row and index.
func (c *Client) Table(opts TableRequest) (*TableResult, error) {
	resp, err := c.sendRequest(ComponentTable, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, fmt.Errorf("user aborted")
	}

	var result TableResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse table result: %w", err)
	}

	return &result, nil
}
