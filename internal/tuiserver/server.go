// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/invowk/invowk/internal/core/serverbase"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// TUIRequest represents a request for a TUI component to be rendered.
	// The HTTP handler sends these to the parent Bubbletea program via a channel.
	TUIRequest struct {
		// Component is the type of TUI component to render.
		Component Component
		// Options contains the component-specific options as raw JSON.
		Options json.RawMessage
		// ResponseCh is where the result should be sent.
		ResponseCh chan<- Response
	}

	// Server is an HTTP server that handles TUI rendering requests from child processes.
	// It listens on all interfaces (0.0.0.0) and requires token-based authentication.
	//
	// Instead of rendering TUI components directly, the server sends requests
	// to a channel that the parent Bubbletea program reads from. This allows
	// TUI components to be rendered as overlays within the parent's alt-screen.
	//
	// A Server instance is single-use: once stopped or failed, create a new instance.
	Server struct {
		// Embedded base provides state machine and lifecycle management
		*serverbase.Base

		// Immutable configuration (set at creation, never modified)
		listener   net.Listener
		httpServer *http.Server
		port       types.ListenPort
		token      AuthToken

		// Shutdown coordination (TUI-specific: used to signal HTTP handlers)
		shutdownCh   chan struct{}
		shutdownOnce sync.Once

		// Request handling - mu protects concurrent access during TUI rendering.
		// Only one TUI component can be rendered at a time.
		mu sync.Mutex

		// requestCh receives TUI component requests from HTTP handlers.
		// The parent Bubbletea program should read from this channel.
		requestCh chan TUIRequest
	}
)

// New creates a new TUI server listening on a random port on all interfaces.
// The server uses token-based authentication for security.
// The server is not started until Start() is called.
func New() (*Server, error) {
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	//nolint:gosec // G102: Binding to 0.0.0.0 required for container runtime access to TUI server
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Extract the port from the listener address
	tcpAddr := listener.Addr().(*net.TCPAddr)

	s := &Server{
		Base:       serverbase.NewBase(),
		listener:   listener,
		port:       types.ListenPort(tcpAddr.Port),
		token:      AuthToken(token),
		shutdownCh: make(chan struct{}),
		requestCh:  make(chan TUIRequest),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tui", s.handleTUI)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Long timeout for user interaction
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Start begins accepting connections. Blocks until server is ready or context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	if err := s.TransitionToStarting(ctx); err != nil {
		return err
	}

	// Start serving in background
	s.AddGoroutine()
	go func() {
		defer s.DoneGoroutine()
		// Signal that we're ready to accept connections
		s.TransitionToRunning()
		if err := s.httpServer.Serve(s.listener); !errors.Is(err, http.ErrServerClosed) {
			s.SendError(err)
		}
	}()

	// Wait for ready signal or context cancellation
	if err := s.WaitForReady(ctx); err != nil {
		s.TransitionToFailed(err)
		_ = s.httpServer.Close() // Best-effort cleanup on error
		return err
	}

	return nil
}

// Stop gracefully shuts down the server. Safe to call multiple times.
func (s *Server) Stop() error {
	// Signal shutdown to handlers (do this before state transition)
	s.shutdownOnce.Do(func() { close(s.shutdownCh) })

	if !s.TransitionToStopping() {
		// Already stopped/stopping, or never started - clean up listener
		_ = s.listener.Close() // Best-effort cleanup; server already stopping
		return nil
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.httpServer.Shutdown(shutdownCtx)

	// Wait for serve goroutine to finish
	s.WaitForShutdown()
	s.TransitionToStopped()

	return err
}

// Port returns the port number the server is listening on.
func (s *Server) Port() types.ListenPort {
	return s.port
}

// URL returns the full server URL for localhost access (e.g., "http://127.0.0.1:54321").
// For container access, use URLWithHost() with the appropriate host address.
func (s *Server) URL() string {
	return s.URLWithHost("127.0.0.1")
}

// URLWithHost returns the full server URL with a custom host (e.g., "http://host.docker.internal:54321").
// This is useful for containers that need to access the server via a different hostname.
func (s *Server) URLWithHost(host string) string {
	return fmt.Sprintf("http://%s:%d", host, s.port)
}

// Token returns the authentication token.
func (s *Server) Token() AuthToken {
	return s.token
}

// RequestChannel returns the channel that receives TUI rendering requests.
// The parent Bubbletea program should read from this channel and render
// the requested components as overlays.
func (s *Server) RequestChannel() <-chan TUIRequest {
	return s.requestCh
}

// handleHealth responds with 200 OK for health checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleTUI is the main handler for TUI component requests.
// Instead of rendering components directly, it sends requests to the parent
// Bubbletea program via a channel and waits for the response.
func (s *Server) handleTUI(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify authentication token
	authHeader := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + string(s.token)
	if authHeader != expectedAuth {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	defer func() { _ = r.Body.Close() }() // HTTP handler; close error non-critical

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Parse the request
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		s.sendError(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Acquire lock - only one TUI component at a time
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a response channel for this request
	responseCh := make(chan Response, 1)

	// Send the request to the parent Bubbletea program
	tuiReq := TUIRequest{
		Component:  req.Component,
		Options:    req.Options,
		ResponseCh: responseCh,
	}

	// Send request or handle shutdown
	select {
	case s.requestCh <- tuiReq:
		// Request sent successfully
	case <-s.shutdownCh:
		s.sendError(w, "server shutting down", http.StatusServiceUnavailable)
		return
	case <-r.Context().Done():
		s.sendError(w, "request cancelled", http.StatusRequestTimeout)
		return
	}

	// Wait for the response from the parent Bubbletea program
	select {
	case resp := <-responseCh:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	case <-s.shutdownCh:
		s.sendError(w, "server shutting down", http.StatusServiceUnavailable)
	case <-r.Context().Done():
		s.sendError(w, "request cancelled", http.StatusRequestTimeout)
	}
}

// sendError sends an error response.
func (s *Server) sendError(w http.ResponseWriter, msg string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(Response{Error: msg})
}

// generateToken generates a random hex-encoded token of the specified byte length.
func generateToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
