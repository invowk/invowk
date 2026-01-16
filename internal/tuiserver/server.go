// SPDX-License-Identifier: EPL-2.0

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
	"sync/atomic"
	"time"
)

// ServerState represents the lifecycle state of the TUI server.
type ServerState int32

const (
	// StateCreated indicates the server has been created but not started.
	StateCreated ServerState = iota
	// StateStarting indicates the server is in the process of starting.
	StateStarting
	// StateRunning indicates the server is running and accepting requests.
	StateRunning
	// StateStopping indicates the server is shutting down.
	StateStopping
	// StateStopped indicates the server has stopped (terminal state).
	StateStopped
	// StateFailed indicates the server failed to start or encountered a fatal error (terminal state).
	StateFailed
)

// String returns a human-readable representation of the server state.
func (s ServerState) String() string {
	switch s {
	case StateCreated:
		return "created"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateStopped:
		return "stopped"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

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
// It listens on all interfaces (0.0.0.0) and requires token-based authentication.
//
// Instead of rendering TUI components directly, the server sends requests
// to a channel that the parent Bubbletea program reads from. This allows
// TUI components to be rendered as overlays within the parent's alt-screen.
//
// A Server instance is single-use: once stopped or failed, create a new instance.
type Server struct {
	// Immutable configuration (set at creation, never modified)
	listener   net.Listener
	httpServer *http.Server
	port       int
	token      string

	// State management (atomic for lock-free reads)
	state atomic.Int32

	// State transition protection
	stateMu sync.Mutex

	// Lifecycle management
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startedCh chan struct{}
	errCh     chan error
	lastErr   error

	// Shutdown coordination
	shutdownCh   chan struct{}
	shutdownOnce sync.Once

	// Request handling - mu protects concurrent access during TUI rendering.
	// Only one TUI component can be rendered at a time.
	mu sync.Mutex

	// requestCh receives TUI component requests from HTTP handlers.
	// The parent Bubbletea program should read from this channel.
	requestCh chan TUIRequest
}

// New creates a new TUI server listening on a random port on all interfaces.
// The server uses token-based authentication for security.
// The server is not started until Start() is called.
func New() (*Server, error) {
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Extract the port from the listener address
	tcpAddr := listener.Addr().(*net.TCPAddr)

	s := &Server{
		listener:   listener,
		port:       tcpAddr.Port,
		token:      token,
		shutdownCh: make(chan struct{}),
		requestCh:  make(chan TUIRequest),
		startedCh:  make(chan struct{}),
		errCh:      make(chan error, 1),
	}
	s.state.Store(int32(StateCreated))

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
	s.stateMu.Lock()
	currentState := s.State()
	if currentState != StateCreated {
		s.stateMu.Unlock()
		return fmt.Errorf("server cannot be started (state: %s)", currentState)
	}
	s.state.Store(int32(StateStarting))
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.stateMu.Unlock()

	// Check for already-cancelled context
	if err := ctx.Err(); err != nil {
		s.stateMu.Lock()
		s.state.Store(int32(StateFailed))
		s.lastErr = err
		s.stateMu.Unlock()
		return fmt.Errorf("context already cancelled: %w", err)
	}

	// Start serving in background
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Signal that we're ready to accept connections
		close(s.startedCh)
		if err := s.httpServer.Serve(s.listener); !errors.Is(err, http.ErrServerClosed) {
			// Send error to channel (non-blocking)
			select {
			case s.errCh <- err:
			default:
			}
		}
	}()

	// Wait for ready signal or context cancellation
	select {
	case <-s.startedCh:
		s.stateMu.Lock()
		s.state.Store(int32(StateRunning))
		s.stateMu.Unlock()
		return nil
	case <-ctx.Done():
		s.stateMu.Lock()
		s.state.Store(int32(StateFailed))
		s.lastErr = ctx.Err()
		s.stateMu.Unlock()
		_ = s.httpServer.Close()
		return fmt.Errorf("startup cancelled: %w", ctx.Err())
	}
}

// Stop gracefully shuts down the server. Safe to call multiple times.
func (s *Server) Stop() error {
	s.stateMu.Lock()
	currentState := s.State()

	// Already stopped or stopping - no-op
	if currentState == StateStopped || currentState == StateStopping {
		s.stateMu.Unlock()
		return nil
	}

	// Never started or failed - just mark as stopped and clean up
	if currentState == StateCreated || currentState == StateFailed {
		s.state.Store(int32(StateStopped))
		s.shutdownOnce.Do(func() { close(s.shutdownCh) })
		_ = s.listener.Close()
		s.stateMu.Unlock()
		return nil
	}

	s.state.Store(int32(StateStopping))
	s.stateMu.Unlock()

	// Signal shutdown to handlers
	s.shutdownOnce.Do(func() { close(s.shutdownCh) })

	// Cancel context to signal background goroutines
	if s.cancel != nil {
		s.cancel()
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.httpServer.Shutdown(shutdownCtx)

	// Wait for serve goroutine to finish
	s.wg.Wait()

	s.stateMu.Lock()
	s.state.Store(int32(StateStopped))
	s.stateMu.Unlock()

	return err
}

// State returns the current server state (lock-free read).
func (s *Server) State() ServerState {
	return ServerState(s.state.Load())
}

// IsRunning returns true if the server is in Running state.
func (s *Server) IsRunning() bool {
	return s.State() == StateRunning
}

// Wait blocks until the server stops and returns any error that caused shutdown.
func (s *Server) Wait() error {
	s.wg.Wait()
	return s.lastErr
}

// Err returns a channel that receives fatal errors from background goroutines.
func (s *Server) Err() <-chan error {
	return s.errCh
}

// Port returns the port number the server is listening on.
func (s *Server) Port() int {
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
func (s *Server) Token() string {
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
