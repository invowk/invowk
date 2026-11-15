// SPDX-License-Identifier: MPL-2.0

package tuiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/invowk/invowk/internal/tuiwire"
)

// Environment variable names for TUI server communication.
const (
	// EnvTUIAddr is the environment variable containing the TUI server address.
	EnvTUIAddr = tuiwire.EnvTUIAddr
	// EnvTUIToken is the environment variable containing the authentication token.
	EnvTUIToken = tuiwire.EnvTUIToken
	// ComponentInput is the TUI component type constant for input prompts.
	ComponentInput Component = tuiwire.ComponentInput
	// ComponentConfirm is the TUI component type constant for confirm prompts.
	ComponentConfirm Component = tuiwire.ComponentConfirm
	// ComponentChoose is the TUI component type constant for choose prompts.
	ComponentChoose Component = tuiwire.ComponentChoose
	// ComponentFilter is the TUI component type constant for filter prompts.
	ComponentFilter Component = tuiwire.ComponentFilter
	// ComponentFile is the TUI component type constant for file pickers.
	ComponentFile Component = tuiwire.ComponentFile
	// ComponentWrite is the TUI component type constant for styled text output.
	ComponentWrite Component = tuiwire.ComponentWrite
	// ComponentTextArea is the TUI component type constant for text areas.
	ComponentTextArea = tuiwire.ComponentTextArea
	// ComponentSpin is the TUI component type constant for spinners.
	ComponentSpin Component = tuiwire.ComponentSpin
	// ComponentPager is the TUI component type constant for pagers.
	ComponentPager Component = tuiwire.ComponentPager
	// ComponentTable is the TUI component type constant for tables.
	ComponentTable Component = tuiwire.ComponentTable

	// reachTimeout bounds the connect and the authenticated /health
	// preflight that precedes every component request. The component request
	// itself keeps the long overall timeout, because it waits for the user.
	reachTimeout = 5 * time.Second
	// healthBodyLimit caps how much of a /health answer is drained so the
	// connection can be reused for the component request.
	healthBodyLimit = 512
)

var (
	// ErrTUIServerResponse is returned when the TUI server responds with an error message.
	ErrTUIServerResponse = errors.New("TUI server error")
	// ErrUserCancelled is returned when a delegated TUI component is cancelled.
	ErrUserCancelled = tuiwire.ErrUserCancelled
	// ErrTUIServerUnreachable is returned when the TUI server named by
	// INVOWK_TUI_ADDR does not answer an authenticated health check within
	// the reach timeout: a refused or dropped connect, a peer that resets,
	// closes or stays silent (for example an engine proxy that accepted the
	// connection but cannot reach the host), or a non-TUI responder.
	ErrTUIServerUnreachable = errors.New("TUI server unreachable")
)

type (
	// URL represents the URL of a TUI server for interactive mode.
	URL = tuiwire.TUIServerURL
	// AuthToken authenticates delegated child requests to the parent TUI server.
	AuthToken = tuiwire.AuthToken
	// Component represents a TUI component type.
	Component = tuiwire.Component
	// Request is the common wrapper for all TUI requests.
	Request = tuiwire.Request
	// Response is the common wrapper for all TUI responses.
	Response = tuiwire.Response
	// InputRequest contains options for the input component.
	InputRequest = tuiwire.InputRequest
	// InputResult contains the result of an input prompt.
	InputResult = tuiwire.InputResult
	// ConfirmRequest contains options for the confirm component.
	ConfirmRequest = tuiwire.ConfirmRequest
	// ConfirmResult contains the result of a confirm prompt.
	ConfirmResult = tuiwire.ConfirmResult
	// ChooseRequest contains options for the choose component.
	ChooseRequest = tuiwire.ChooseRequest
	// ChooseResult contains the result of a choose prompt.
	ChooseResult = tuiwire.ChooseResult
	// FilterRequest contains options for the filter component.
	FilterRequest = tuiwire.FilterRequest
	// FilterResult contains the result of a filter prompt.
	FilterResult = tuiwire.FilterResult
	// FileRequest contains options for the file picker component.
	FileRequest = tuiwire.FileRequest
	// FileResult contains the result of a file picker.
	FileResult = tuiwire.FileResult
	// WriteRequest contains options for the write component.
	WriteRequest = tuiwire.WriteRequest
	// WriteResult is empty because write does not return a value.
	WriteResult = tuiwire.WriteResult
	// TextAreaRequest contains options for the textarea component.
	TextAreaRequest = tuiwire.TextAreaRequest
	// TextAreaResult contains the result of a textarea prompt.
	TextAreaResult = tuiwire.TextAreaResult
	// SpinRequest contains options for the spin component.
	SpinRequest = tuiwire.SpinRequest
	// SpinResult contains the result of a spin operation.
	SpinResult = tuiwire.SpinResult
	// PagerRequest contains options for the pager component.
	PagerRequest = tuiwire.PagerRequest
	// PagerResult is empty because pager does not return a value.
	PagerResult = tuiwire.PagerResult
	// TableRequest contains options for the table component.
	TableRequest = tuiwire.TableRequest
	// TableResult contains the result of a table selection.
	TableResult = tuiwire.TableResult

	// Client provides methods to communicate with the TUI server.
	// It is used by child processes to delegate TUI rendering to the parent.
	Client struct {
		addr   URL
		token  AuthToken
		client *http.Client
		// reachTimeout bounds checkReachable.
		reachTimeout time.Duration
	}

	// UnreachableError reports a TUI server that did not answer its health check.
	// It names the address and the container.tui_server_bind opt-in, because
	// the usual cause is a container engine that does not route to the host.
	UnreachableError struct {
		Addr URL
		Err  error
	}
)

// NewClientFromEnv creates a new Client from environment variables.
// Returns nil if the environment variables are not set.
func NewClientFromEnv() *Client {
	addr := os.Getenv(EnvTUIAddr)
	token := os.Getenv(EnvTUIToken)

	if addr == "" || token == "" {
		return nil
	}

	serverURL := URL(addr)
	if err := serverURL.Validate(); err != nil {
		return nil
	}
	authToken := AuthToken(token)
	if err := authToken.Validate(); err != nil {
		return nil
	}
	return NewClient(serverURL, authToken)
}

// NewClient creates a new Client with the given server address and token.
func NewClient(addr URL, token AuthToken) *Client {
	return newClientWithReachTimeout(addr, token, reachTimeout)
}

func newClientWithReachTimeout(addr URL, token AuthToken, timeout time.Duration) *Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{}
	}
	dialer := &net.Dialer{Timeout: timeout}
	transport.DialContext = dialer.DialContext
	// The TUI server is always a direct loopback or bridge peer. An
	// HTTP_PROXY injected into a container (for example by the Docker CLI's
	// "proxies" setting) would otherwise receive the bearer token.
	transport.Proxy = nil
	return &Client{
		addr:  addr,
		token: token,
		client: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Minute, // Long timeout for user interaction
		},
		reachTimeout: timeout,
	}
}

// Error implements error.
func (e *UnreachableError) Error() string {
	return fmt.Sprintf("%s at %s: %v; if this runs in a container, its engine may not route to the host: "+
		"set container.tui_server_bind: \"all-interfaces\" in the invowk config to restore the wide bind", ErrTUIServerUnreachable, e.Addr, e.Err)
}

// Unwrap returns ErrTUIServerUnreachable and the underlying error.
func (e *UnreachableError) Unwrap() []error { return []error{ErrTUIServerUnreachable, e.Err} }

// IsAvailable checks if the TUI server is available.
func (c *Client) IsAvailable() bool {
	return c.IsAvailableContext(context.Background())
}

// IsAvailableContext checks if the TUI server is available with caller cancellation.
//
//nolint:contextcheck // nil context is accepted for availability-probe compatibility.
func (c *Client) IsAvailableContext(ctx context.Context) bool {
	if c == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}

	return c.checkReachable(ctx) == nil
}

// Input sends an input prompt request to the TUI server.
// Returns the entered text or an error.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) Input(opts InputRequest) (string, error) {
	return c.InputContext(context.Background(), opts)
}

// InputContext sends an input prompt request with caller cancellation.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) InputContext(ctx context.Context, opts InputRequest) (string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentInput, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", ErrUserCancelled
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
		return false, ErrUserCancelled
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
		return nil, ErrUserCancelled
	}

	var result ChooseResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse choose result: %w", err)
	}

	return result.Selected, nil
}

// ChooseSingle is a convenience method for single-select choose.
// Returns the selected option as a string.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) ChooseSingle(opts ChooseRequest) (string, error) {
	return c.ChooseSingleContext(context.Background(), opts)
}

// ChooseSingleContext is a convenience method for single-select choose with caller cancellation.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
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
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) ChooseMultiple(opts ChooseRequest) ([]string, error) {
	return c.ChooseMultipleContext(context.Background(), opts)
}

// ChooseMultipleContext is a convenience method for multi-select choose with caller cancellation.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
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
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) Filter(opts FilterRequest) ([]string, error) {
	return c.FilterContext(context.Background(), opts)
}

// FilterContext sends a filter prompt request with caller cancellation.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) FilterContext(ctx context.Context, opts FilterRequest) ([]string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentFilter, opts)
	if err != nil {
		return nil, err
	}

	if resp.Cancelled {
		return nil, ErrUserCancelled
	}

	var result FilterResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse filter result: %w", err)
	}

	return result.Selected, nil
}

// File sends a file picker request to the TUI server.
// Returns the selected file path.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) File(opts FileRequest) (string, error) {
	return c.FileContext(context.Background(), opts)
}

// FileContext sends a file picker request with caller cancellation.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) FileContext(ctx context.Context, opts FileRequest) (string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentFile, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", ErrUserCancelled
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
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) TextArea(opts TextAreaRequest) (string, error) {
	return c.TextAreaContext(context.Background(), opts)
}

// TextAreaContext sends a multi-line text input request with caller cancellation.
//
//goplint:ignore -- TUI client exposes widget protocol result primitives to CLI adapters.
func (c *Client) TextAreaContext(ctx context.Context, opts TextAreaRequest) (string, error) {
	resp, err := c.sendRequestContext(ctx, ComponentTextArea, opts)
	if err != nil {
		return "", err
	}

	if resp.Cancelled {
		return "", ErrUserCancelled
	}

	var result TextAreaResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("failed to parse textarea result: %w", err)
	}

	return result.Value, nil
}

// Spin sends a presentation-only spinner request to the TUI server.
func (c *Client) Spin(opts SpinRequest) error {
	return c.SpinContext(context.Background(), opts)
}

// SpinContext sends a presentation-only spinner request with caller cancellation.
func (c *Client) SpinContext(ctx context.Context, opts SpinRequest) error {
	_, err := c.sendRequestContext(ctx, ComponentSpin, opts)
	if err != nil {
		return err
	}
	return nil
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
		return nil, ErrUserCancelled
	}

	var result TableResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse table result: %w", err)
	}

	return &result, nil
}

// checkReachable sends an authenticated GET /health bounded by the reach
// timeout. The server answers it at once, so a transport error or a timeout
// here means the address does not lead to a working server: it becomes an
// UnreachableError. A 401 stays a plain server error, and the caller's own
// cancellation is returned as such. Callers pass a non-nil ctx.
func (c *Client) checkReachable(ctx context.Context) error {
	reachCtx, cancel := context.WithTimeout(ctx, c.reachTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reachCtx, http.MethodGet, c.addr.String()+"/health", http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+string(c.token))
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("TUI server health check: %w", err)
		}
		return &UnreachableError{Addr: c.addr, Err: err}
	}
	defer func() { _ = resp.Body.Close() }() // Health check; close error non-critical
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, healthBodyLimit))
	if readErr != nil && resp.StatusCode == http.StatusOK {
		return &UnreachableError{Addr: c.addr, Err: fmt.Errorf("read health answer: %w", readErr)}
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(body))
	default:
		return &UnreachableError{Addr: c.addr, Err: fmt.Errorf("health check answered %d, not 200 from a TUI server", resp.StatusCode)}
	}
}

// sendRequestContext sends a TUI request to the server and returns the response.
//
//nolint:contextcheck // nil context is accepted for request helper compatibility.
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

	// The component request waits for the user, so only a preflight can
	// bound how long a dead or silent route takes to fail.
	if reachErr := c.checkReachable(ctx); reachErr != nil {
		return nil, reachErr
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr.String()+"/tui", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+string(c.token))

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", c.classifyDialError(err))
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

// classifyDialError wraps connect failures in UnreachableError and returns
// other errors unchanged.
func (c *Client) classifyDialError(err error) error {
	if opErr, ok := errors.AsType[*net.OpError](err); ok && opErr.Op == "dial" {
		return &UnreachableError{Addr: c.addr, Err: err}
	}
	return err
}
