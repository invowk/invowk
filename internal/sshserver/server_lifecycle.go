// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/invowk/invowk/internal/core/serverbase"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
)

type (
	// serverListener is an interface for net.Listener to enable testing.
	serverListener interface {
		Accept() (net.Conn, error)
		Close() error
		Addr() net.Addr
	}

	// netOpError is a type alias for net.OpError to avoid importing net in server.go.
	netOpError = net.OpError
)

// Start starts the SSH server and blocks until either:
//   - The server is ready to accept connections (returns nil)
//   - The server fails to start (returns error)
//   - The context is cancelled (returns context error)
//   - The startup timeout is exceeded (returns error)
//
// After Start() returns nil, use Err() to monitor for runtime errors.
func (s *Server) Start(ctx context.Context) error {
	// Delegate state transition to serverbase.Base
	// This handles the cancelled context check and Created -> Starting transition
	if err := s.TransitionToStarting(ctx); err != nil {
		return err
	}

	// Setup timeout for startup
	startupCtx, startupCancel := context.WithTimeout(ctx, s.cfg.StartupTimeout)
	defer startupCancel()

	// Initialize listener
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	var lc net.ListenConfig
	listener, err := lc.Listen(startupCtx, "tcp", addr)
	if err != nil {
		s.TransitionToFailed(fmt.Errorf("failed to listen on %s: %w", addr, err))
		return s.LastError()
	}

	s.srvMu.Lock()
	s.listener = listener
	s.addr = listener.Addr().String()
	s.srvMu.Unlock()

	// Create SSH server
	srv, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithPublicKeyAuth(s.publicKeyHandler),
		wish.WithPasswordAuth(s.passwordHandler),
		wish.WithMiddleware(
			activeterm.Middleware(),
			s.commandMiddleware(),
		),
	)
	if err != nil {
		_ = listener.Close() // Best-effort cleanup on error
		s.TransitionToFailed(fmt.Errorf("failed to create SSH server: %w", err))
		return s.LastError()
	}

	s.srvMu.Lock()
	s.srv = srv
	s.srvMu.Unlock()

	// Start the serve goroutine
	s.AddGoroutine()
	go s.serve()

	// Start token cleanup goroutine
	s.AddGoroutine()
	go s.cleanupExpiredTokens()

	// Wait for server to be ready or fail
	select {
	case <-s.StartedChannel():
		// Server is ready
		s.logger.Info("SSH server started", "address", s.addr)
		return nil

	case err := <-s.Err():
		// Server failed during startup
		s.TransitionToFailed(err)
		return err

	case <-startupCtx.Done():
		// Startup timeout or caller cancelled
		s.TransitionToFailed(fmt.Errorf("startup timeout: %w", startupCtx.Err()))
		return s.LastError()
	}
}

// Stop gracefully stops the SSH server.
// It blocks until all connections are closed or the shutdown timeout is reached.
// Safe to call multiple times; subsequent calls are no-ops.
func (s *Server) Stop() error {
	// Use serverbase.Base to handle state transition
	if !s.TransitionToStopping() {
		// Already stopped, stopping, created, or failed
		s.WaitForShutdown()
		return nil
	}

	// Proceed with shutdown
	return s.doStop()
}

// doStop performs the actual shutdown logic.
func (s *Server) doStop() error {
	// Shutdown SSH server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer shutdownCancel()

	var shutdownErr error
	s.srvMu.Lock()
	if s.srv != nil {
		shutdownErr = s.srv.Shutdown(shutdownCtx)
		if shutdownErr != nil && !isClosedConnError(shutdownErr) {
			s.logger.Error("shutdown error", "error", shutdownErr)
		} else {
			shutdownErr = nil
		}
	}
	if s.listener != nil {
		_ = s.listener.Close() //nolint:errcheck // Best-effort cleanup during shutdown
	}
	s.srvMu.Unlock()

	// Wait for all goroutines to exit
	s.WaitForShutdown()

	// Transition to Stopped and close error channel
	s.TransitionToStopped()
	s.CloseErrChannel()
	s.logger.Info("SSH server stopped")

	return shutdownErr
}

// serve runs the SSH server and handles errors.
func (s *Server) serve() {
	defer s.DoneGoroutine()

	// Transition: Starting -> Running (signals readiness)
	s.TransitionToRunning()

	// Block serving connections
	s.srvMu.Lock()
	srv := s.srv
	listener := s.listener
	s.srvMu.Unlock()

	if srv == nil || listener == nil {
		return
	}

	err := srv.Serve(listener)
	// Handle serve completion
	if err != nil {
		// Ignore expected shutdown errors
		if errors.Is(err, ssh.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
			return
		}

		// Report unexpected errors
		s.SendError(fmt.Errorf("serve error: %w", err))
	}
}

// State returns the current server state.
// This delegates to the embedded serverbase.Base.
func (s *Server) State() serverbase.State {
	return s.Base.State()
}

// IsRunning returns whether the server is currently running and accepting connections.
// This is a convenience method equivalent to State() == serverbase.StateRunning.
func (s *Server) IsRunning() bool {
	return s.Base.IsRunning()
}

// Address returns the server's bound address (host:port).
// Blocks until the server has started or failed.
// Returns empty string if server never started or failed.
func (s *Server) Address() string {
	select {
	case <-s.StartedChannel():
		s.srvMu.Lock()
		defer s.srvMu.Unlock()
		return s.addr
	default:
		// Server not started yet, check if context exists
		ctx := s.Context()
		if ctx == nil {
			return ""
		}
		select {
		case <-s.StartedChannel():
			s.srvMu.Lock()
			defer s.srvMu.Unlock()
			return s.addr
		case <-ctx.Done():
			return ""
		}
	}
}

// Port returns the server's listening port.
// Blocks until the server has started or failed.
// Returns 0 if server never started or failed.
func (s *Server) Port() int {
	addr := s.Address()
	if addr == "" {
		return 0
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		return 0 // Invalid port string
	}
	return port
}

// Host returns the server's configured host address.
func (s *Server) Host() string {
	return s.cfg.Host
}

// Wait blocks until the server stops (either gracefully or due to error).
// Returns the error if the server failed, nil otherwise.
func (s *Server) Wait() error {
	s.WaitForShutdown()

	state := s.State()
	if state == serverbase.StateFailed {
		return s.LastError()
	}
	return nil
}
