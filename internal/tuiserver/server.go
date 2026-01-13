// SPDX-License-Identifier: EPL-2.0

package tuiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// TUIRequest represents a request for a TUI component to be rendered.
// The HTTP handler sends these to the parent Bubbletea program via a channel.
type TUIRequest struct {
	// Component is the type of TUI component to render.
	Component Component
	// Options contains the component-specific options as raw JSON.
	Options json.RawMessage
	// ResponseCh is where the result should be sent.
	ResponseCh chan<- Response
}

// Server is an HTTP server that handles TUI rendering requests from child processes.
// It listens on localhost and requires token-based authentication.
//
// Instead of rendering TUI components directly, the server sends requests
// to a channel that the parent Bubbletea program reads from. This allows
// TUI components to be rendered as overlays within the parent's alt-screen.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
	addr       string
	token      string

	// mu protects concurrent access during TUI rendering.
	// Only one TUI component can be rendered at a time.
	mu sync.Mutex

	// running indicates whether the server is currently running.
	running bool

	// shutdownCh is closed when the server is shutting down.
	shutdownCh chan struct{}

	// requestCh receives TUI component requests from HTTP handlers.
	// The parent Bubbletea program should read from this channel.
	requestCh chan TUIRequest
}

// New creates a new TUI server listening on a random localhost port.
// The server is not started until Start() is called.
func New() (*Server, error) {
	token, err := generateToken(32)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	s := &Server{
		listener:   listener,
		addr:       listener.Addr().String(),
		token:      token,
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

// Start begins accepting connections. This is non-blocking.
func (s *Server) Start() error {
	s.running = true
	go func() {
		if err := s.httpServer.Serve(s.listener); err != http.ErrServerClosed {
			// Log error but don't panic - server might be shutting down
		}
	}()
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	close(s.shutdownCh)
	s.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// Address returns the server's address (e.g., "127.0.0.1:54321").
func (s *Server) Address() string {
	return s.addr
}

// URL returns the full server URL (e.g., "http://127.0.0.1:54321").
func (s *Server) URL() string {
	return "http://" + s.addr
}

// Token returns the authentication token.
func (s *Server) Token() string {
	return s.token
}

// IsRunning returns true if the server is currently running.
func (s *Server) IsRunning() bool {
	return s.running
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
	expectedAuth := "Bearer " + s.token
	if authHeader != expectedAuth {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendError(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

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
