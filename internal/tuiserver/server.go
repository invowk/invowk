// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/invowk/invowk/internal/core/serverbase"
	"github.com/invowk/invowk/pkg/types"
)

// maxBindAttempts bounds how often Start retries binding every plan address
// at one shared port.
const maxBindAttempts = 5

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
	// It listens on the addresses of its BindPlan (127.0.0.1 by default; never a
	// wildcard address unless the plan is the explicit wildcard opt-in), all at
	// one port, and every request must carry the session bearer token.
	//
	// Instead of rendering TUI components directly, the server sends requests
	// to a channel that the parent Bubbletea program reads from. This allows
	// TUI components to be rendered as overlays within the parent's alt-screen.
	//
	// A Server instance is single-use: once stopped or failed, create a new instance.
	Server struct {
		// base provides private state machine and lifecycle management.
		base *serverbase.Base

		// Immutable configuration (set at creation, never modified)
		plan       BindPlan
		listeners  []net.Listener
		httpServer *http.Server
		port       types.ListenPort
		token      AuthToken

		// Shutdown coordination (TUI-specific: used to signal HTTP handlers)
		shutdownCh       chan struct{}
		shutdownOnce     sync.Once
		requestCloseOnce sync.Once

		// Request handling - mu protects concurrent access during TUI rendering.
		// Only one TUI component can be rendered at a time.
		mu sync.Mutex

		// requestCh receives TUI component requests from HTTP handlers.
		// The parent Bubbletea program should read from this channel.
		requestCh chan TUIRequest
	}
)

// New creates a new TUI server that listens on 127.0.0.1 only.
// The server uses token-based authentication for security.
// The server is not started until Start() is called.
func New() (*Server, error) {
	return NewWithBindPlan(LoopbackBindPlan())
}

// NewWithBindPlan creates a new TUI server that listens on the plan's
// addresses. The plan is validated, so an unspecified address is accepted only
// through WildcardBindPlan.
func NewWithBindPlan(plan BindPlan) (*Server, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	authToken := AuthToken(token)
	if err := authToken.Validate(); err != nil {
		return nil, fmt.Errorf("validate generated token: %w", err)
	}

	s := &Server{
		base:       serverbase.NewBase(),
		plan:       plan,
		token:      authToken,
		shutdownCh: make(chan struct{}),
		requestCh:  make(chan TUIRequest),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tui", s.handleTUI)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Handler:      s.requireToken(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Long timeout for user interaction
		IdleTimeout:  60 * time.Second,
	}

	return s, nil
}

// Start begins accepting connections. Blocks until server is ready or context is cancelled.
//
// Every plan address is bound before any serve goroutine starts. For a
// restricted plan the server binds 127.0.0.1 on an ephemeral port first and
// then every other address at that port; if a later bind fails, all listeners
// are closed and the sequence starts over, at most maxBindAttempts times. The
// retry covers every error, not only EADDRINUSE, because Windows reports
// WSAEADDRINUSE, which errors.Is does not map to syscall.EADDRINUSE.
func (s *Server) Start(ctx context.Context) error {
	if err := s.base.TransitionToStarting(ctx); err != nil {
		return err
	}

	listeners, err := s.bindAll(ctx)
	if err != nil {
		return s.base.TransitionToFailed(err)
	}
	s.listeners = listeners
	port, err := listenerPort(listeners[0])
	if err != nil {
		closeListeners(listeners)
		return s.base.TransitionToFailed(err)
	}
	s.port = port

	// One http.Server serves every listener. A Serve error on any listener
	// fails the server and closes the others.
	for _, listener := range listeners {
		s.base.AddGoroutine()
		go func(l net.Listener) {
			defer s.base.DoneGoroutine()
			if serveErr := s.httpServer.Serve(l); !errors.Is(serveErr, http.ErrServerClosed) {
				s.shutdownOnce.Do(func() { close(s.shutdownCh) })
				s.closeRequestChannel()
				if failedErr := s.base.TransitionToFailed(fmt.Errorf("serve error: %w", serveErr)); failedErr != nil {
					_ = s.httpServer.Close() // Stop the remaining listeners.
				}
			}
		}(listener)
	}
	s.base.TransitionToRunning()

	// Wait for ready signal or context cancellation
	if err := s.base.WaitForReady(ctx); err != nil {
		failedErr := s.base.TransitionToFailed(err)
		_ = s.httpServer.Close() // Best-effort cleanup on error
		closeListeners(s.listeners)
		return failedErr
	}
	if !s.base.IsRunning() {
		return s.base.LastError()
	}

	return nil
}

// Stop gracefully shuts down the server. Safe to call multiple times.
func (s *Server) Stop() error {
	// Signal shutdown to handlers (do this before state transition)
	s.shutdownOnce.Do(func() { close(s.shutdownCh) })

	if !s.base.TransitionToStopping() {
		// Already stopped/stopping, or never started - clean up listeners
		closeListeners(s.listeners)
		s.closeRequestChannel()
		return nil
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.httpServer.Shutdown(shutdownCtx)

	// Wait for serve goroutine to finish
	s.base.WaitForShutdown()
	s.base.TransitionToStopped()
	s.closeRequestChannel()

	return err
}

// ListenAddrs returns the addresses the server is listening on, in plan
// order. It is empty before Start.
func (s *Server) ListenAddrs() []net.Addr {
	addrs := make([]net.Addr, 0, len(s.listeners))
	for _, listener := range s.listeners {
		addrs = append(addrs, listener.Addr())
	}
	return addrs
}

// Port returns the port number the server is listening on.
func (s *Server) Port() types.ListenPort {
	return s.port
}

// URL returns the full server URL for localhost access (e.g., "http://127.0.0.1:54321").
// For container access, use URLWithHost() with the appropriate host address.
func (s *Server) URL() URL {
	return s.URLWithHost(types.HostServiceAddress("127.0.0.1")) //goplint:ignore -- localhost literal for loopback server URL
}

// URLWithHost returns the full server URL with a custom host (e.g., "http://host.docker.internal:54321").
// This is useful for containers that need to access the server via a different hostname.
func (s *Server) URLWithHost(host types.HostServiceAddress) URL {
	if err := host.Validate(); err != nil {
		return ""
	}
	return URL("http://" + net.JoinHostPort(host.String(), s.port.String()))
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

// State returns the current server state.
func (s *Server) State() serverbase.State {
	return s.base.State()
}

// IsRunning returns whether the server is currently running and accepting connections.
func (s *Server) IsRunning() bool {
	return s.base.IsRunning()
}

// LastError returns the error that caused the Failed state, or nil.
func (s *Server) LastError() error {
	return s.base.LastError()
}

// Err returns a channel for receiving async errors.
func (s *Server) Err() <-chan error {
	return s.base.Err()
}

func (s *Server) closeRequestChannel() {
	s.requestCloseOnce.Do(func() {
		close(s.requestCh)
	})
}

// requireToken wraps every route, so /tui, /health and unknown paths all
// require the session token before routing, the method check or the body.
// The comparison runs in constant time; the only length it can leak is the
// fixed token length.
func (s *Server) requireToken(next http.Handler) http.Handler {
	expected := []byte("Bearer " + string(s.token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), expected) != 1 {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleHealth responds with 200 OK for authenticated health checks.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
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
	if err := req.Component.Validate(); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
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

// bindAll opens one listener per plan address, all at the same port.
func (s *Server) bindAll(ctx context.Context) ([]net.Listener, error) {
	var lc net.ListenConfig
	if s.plan.IsWildcard() {
		listener, err := lc.Listen(ctx, "tcp", "0.0.0.0:0")
		if err != nil {
			return nil, fmt.Errorf("failed to create listener: %w", err)
		}
		return []net.Listener{listener}, nil
	}

	addrs := s.plan.Addresses()
	var lastErr error
	for range maxBindAttempts {
		listeners, err := bindAtSharedPort(ctx, &lc, addrs)
		if err == nil {
			return listeners, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("failed to create listeners after %d attempts: %w", maxBindAttempts, lastErr)
}

// bindAtSharedPort binds addrs[0] on an ephemeral port and every other address
// at that port. On any failure it closes what it opened.
func bindAtSharedPort(ctx context.Context, lc *net.ListenConfig, addrs []netip.Addr) ([]net.Listener, error) {
	first, err := lc.Listen(ctx, "tcp", netip.AddrPortFrom(addrs[0], 0).String())
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addrs[0], err)
	}
	listeners := []net.Listener{first}
	tcpAddr, ok := first.Addr().(*net.TCPAddr)
	if !ok {
		closeListeners(listeners)
		return nil, fmt.Errorf("listener address has unexpected type %T", first.Addr())
	}
	port := strconv.Itoa(tcpAddr.Port)
	for _, addr := range addrs[1:] {
		listener, listenErr := lc.Listen(ctx, "tcp", net.JoinHostPort(addr.String(), port))
		if listenErr != nil {
			closeListeners(listeners)
			return nil, fmt.Errorf("listen on %s:%s: %w", addr, port, listenErr)
		}
		listeners = append(listeners, listener)
	}
	return listeners, nil
}

// listenerPort returns the validated TCP port of listener.
func listenerPort(listener net.Listener) (types.ListenPort, error) {
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("listener address has unexpected type %T", listener.Addr())
	}
	rawPort := tcpAddr.Port
	port := types.ListenPort(rawPort)
	if err := port.Validate(); err != nil {
		return 0, fmt.Errorf("listener port: %w", err)
	}
	return port, nil
}

func closeListeners(listeners []net.Listener) {
	for _, listener := range listeners {
		_ = listener.Close() // Best-effort cleanup.
	}
}
