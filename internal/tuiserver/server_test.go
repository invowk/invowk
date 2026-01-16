// SPDX-License-Identifier: EPL-2.0

package tuiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestServerStartStop(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initial state should be Created
	if server.State() != StateCreated {
		t.Errorf("State should be Created, got %s", server.State())
	}

	if server.IsRunning() {
		t.Error("Server should not be running before Start()")
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// State should be Running
	if server.State() != StateRunning {
		t.Errorf("State should be Running, got %s", server.State())
	}

	if !server.IsRunning() {
		t.Error("Server should be running after Start()")
	}

	// Check that URL and Token are set
	if server.URL() == "" {
		t.Error("Server URL should not be empty")
	}

	if server.Token() == "" {
		t.Error("Server Token should not be empty")
	}

	// Test health endpoint
	resp, err := http.Get(server.URL() + "/health")
	if err != nil {
		t.Fatalf("Failed to check health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health check returned %d, expected %d", resp.StatusCode, http.StatusOK)
	}

	// Stop the server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// State should be Stopped
	if server.State() != StateStopped {
		t.Errorf("State should be Stopped, got %s", server.State())
	}

	if server.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}
}

func TestServerDoubleStart(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Second Start() should fail
	err = server.Start(ctx)
	if err == nil {
		t.Error("Second Start() should return error")
	}
}

func TestServerDoubleStop(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// First Stop() should succeed
	if err := server.Stop(); err != nil {
		t.Fatalf("First Stop() failed: %v", err)
	}

	// Second Stop() should be no-op (not error)
	if err := server.Stop(); err != nil {
		t.Errorf("Second Stop() should not error, got: %v", err)
	}
}

func TestStopWithoutStart(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Stop without Start should be safe
	if err := server.Stop(); err != nil {
		t.Errorf("Stop without Start should not error, got: %v", err)
	}

	// State should be Stopped
	if server.State() != StateStopped {
		t.Errorf("State should be Stopped, got %s", server.State())
	}
}

func TestServerStartWithCancelledContext(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = server.Start(ctx)
	if err == nil {
		t.Error("Start with cancelled context should return error")
		server.Stop()
	}

	// State should be Failed
	if server.State() != StateFailed {
		t.Errorf("State should be Failed, got %s", server.State())
	}
}

func TestServerStateString(t *testing.T) {
	tests := []struct {
		state    ServerState
		expected string
	}{
		{StateCreated, "created"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateStopped, "stopped"},
		{StateFailed, "failed"},
		{ServerState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("ServerState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestServerAuthentication(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Create a request without authentication
	req := Request{
		Component: ComponentConfirm,
		Options:   json.RawMessage(`{"title": "Test"}`),
	}
	reqBody, _ := json.Marshal(req)

	// Test without auth header
	httpReq, _ := http.NewRequest(http.MethodPost, server.URL()+"/tui", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	// Test with wrong token
	httpReq, _ = http.NewRequest(http.MethodPost, server.URL()+"/tui", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer wrong-token")

	resp, err = client.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestServerMethodNotAllowed(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Test GET request (should be method not allowed)
	resp, err := http.Get(server.URL() + "/tui")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, resp.StatusCode)
	}
}

func TestServerUnknownComponent(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Start a goroutine to consume requests from the channel and respond with error
	go func() {
		for req := range server.RequestChannel() {
			// Simulate the parent program receiving and responding to the request
			// For unknown components, the parent would send an error response
			req.ResponseCh <- Response{
				Error: "unknown component type: unknown",
			}
		}
	}()

	// Create a request with unknown component
	req := Request{
		Component: "unknown",
		Options:   json.RawMessage(`{}`),
	}
	reqBody, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest(http.MethodPost, server.URL()+"/tui", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+server.Token())

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// With the new architecture, the server returns 200 with an error in the response body
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Check the response contains an error
	var tuiResp Response
	if err := json.NewDecoder(resp.Body).Decode(&tuiResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if tuiResp.Error == "" {
		t.Error("Expected error in response for unknown component")
	}
}

func TestServerInvalidJSON(t *testing.T) {
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Send invalid JSON
	httpReq, _ := http.NewRequest(http.MethodPost, server.URL()+"/tui", bytes.NewReader([]byte("not json")))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+server.Token())

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestClientFromEnv(t *testing.T) {
	// Test when env vars are not set
	os.Unsetenv(EnvTUIAddr)
	os.Unsetenv(EnvTUIToken)

	client := NewClientFromEnv()
	if client != nil {
		t.Error("Expected nil client when env vars are not set")
	}

	// Test when only addr is set
	os.Setenv(EnvTUIAddr, "http://127.0.0.1:12345")
	os.Unsetenv(EnvTUIToken)

	client = NewClientFromEnv()
	if client != nil {
		t.Error("Expected nil client when token is not set")
	}

	// Test when both are set
	os.Setenv(EnvTUIAddr, "http://127.0.0.1:12345")
	os.Setenv(EnvTUIToken, "test-token")

	client = NewClientFromEnv()
	if client == nil {
		t.Error("Expected client when both env vars are set")
	}

	// Cleanup
	os.Unsetenv(EnvTUIAddr)
	os.Unsetenv(EnvTUIToken)
}

func TestClientIsAvailable(t *testing.T) {
	// Test with nil client
	var nilClient *Client
	if nilClient.IsAvailable() {
		t.Error("Nil client should not be available")
	}

	// Test with client pointing to non-existent server
	client := NewClient("http://127.0.0.1:59999", "token")
	if client.IsAvailable() {
		t.Error("Client to non-existent server should not be available")
	}

	// Test with running server
	server, err := New()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	client = NewClient(server.URL(), server.Token())
	if !client.IsAvailable() {
		t.Error("Client to running server should be available")
	}
}

func TestIsUserCancelledError(t *testing.T) {
	tests := []struct {
		input    error
		expected bool
	}{
		{nil, false},
		{&simpleError{msg: "user aborted"}, true},
		{&simpleError{msg: "interrupted"}, true},
		{&simpleError{msg: "user quit"}, true},
		{&simpleError{msg: "other error"}, false},
		{&simpleError{msg: "user aborted something"}, false}, // partial match should not work
	}

	for _, tt := range tests {
		result := isUserCancelledError(tt.input)
		if result != tt.expected {
			t.Errorf("isUserCancelledError(%v) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

type simpleError struct {
	msg string
}

func (e *simpleError) Error() string {
	return e.msg
}

// isUserCancelledError checks if the error indicates user cancellation.
// Used in tests to verify error handling behavior.
func isUserCancelledError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "user aborted" || msg == "interrupted" || msg == "user quit"
}
