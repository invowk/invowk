// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"github.com/charmbracelet/keygen"

	"github.com/invowk/invowk/internal/core/serverbase"
)

// serverListener is an interface for net.Listener to enable testing.
type serverListener interface {
	Accept() (net.Conn, error)
	Close() error
	Addr() net.Addr
}

// Start starts the SSH server and blocks until either:
//   - The server is ready to accept connections (returns nil)
//   - The server fails to start (returns error)
//   - The context is cancelled (returns context error)
//   - The startup timeout is exceeded (returns error)
//
// After Start() returns nil, use Err() to monitor for runtime errors.
func (s *Server) Start(ctx context.Context) error {
	// This handles the cancelled context check and Created -> Starting transition.
	if err := s.base.TransitionToStarting(ctx); err != nil {
		return err
	}

	// Setup timeout for startup
	startupCtx, startupCancel := context.WithTimeout(ctx, s.cfg.StartupTimeout)
	defer startupCancel()

	// A host key is always configured explicitly: without one, wish.NewServer
	// generates a key and writes it to ./id_ed25519 in the process working
	// directory, leaking private keys into arbitrary directories (and, once,
	// into this repository). Resolved before the listener so failures bind
	// no socket.
	hostKeyOption, err := s.hostKeyOption()
	if err != nil {
		return s.base.TransitionToFailed(fmt.Errorf("failed to configure SSH host key: %w", err))
	}

	// Initialize listener
	addr := net.JoinHostPort(string(s.cfg.Host), s.cfg.Port.String())
	var lc net.ListenConfig
	listener, err := lc.Listen(startupCtx, "tcp", addr)
	if err != nil {
		return s.base.TransitionToFailed(fmt.Errorf("failed to listen on %s: %w", addr, err))
	}

	s.srvMu.Lock()
	s.listener = listener
	s.addr = listener.Addr().String()
	s.srvMu.Unlock()

	// Create SSH server
	wishOptions := []ssh.Option{
		wish.WithAddress(addr),
		hostKeyOption,
		wish.WithPublicKeyAuth(s.publicKeyHandler),
		wish.WithPasswordAuth(s.passwordHandler),
		wish.WithMiddleware(
			activeterm.Middleware(),
			s.commandMiddleware(), //nolint:contextcheck // Wish injects request context into sessions handled by middleware.
		),
	}
	srv, err := wish.NewServer(wishOptions...)
	if err != nil {
		_ = listener.Close() // Best-effort cleanup on error
		return s.base.TransitionToFailed(fmt.Errorf("failed to create SSH server: %w", err))
	}

	s.srvMu.Lock()
	s.srv = srv
	s.srvMu.Unlock()

	// Start the serve goroutine
	s.base.AddGoroutine()
	go s.serve()

	// Start token cleanup goroutine
	s.base.AddGoroutine()
	go s.cleanupExpiredTokens()

	// Wait for server to be ready or fail
	select {
	case <-s.base.StartedChannel():
		// Server is ready
		s.logger.Info("SSH server started", "address", s.addr)
		return nil

	case err := <-s.Err():
		// Server failed during startup
		return s.base.TransitionToFailed(err)

	case <-startupCtx.Done():
		// Startup timeout or caller cancelled
		return s.base.TransitionToFailed(fmt.Errorf("startup timeout: %w", startupCtx.Err()))
	}
}

// Stop gracefully stops the SSH server.
// It blocks until all connections are closed or the shutdown timeout is reached.
// Safe to call multiple times; subsequent calls are no-ops.
func (s *Server) Stop() error {
	if !s.base.TransitionToStopping() {
		// Already stopped, stopping, created, or failed
		s.base.WaitForShutdown()
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

	s.srvMu.Lock()
	srv := s.srv
	listener := s.listener
	s.srv = nil
	s.listener = nil
	s.srvMu.Unlock()

	// Close the listener before graceful SSH shutdown so a blocked Accept
	// exits promptly on every platform, including Windows race-test runners.
	if listener != nil {
		_ = listener.Close() //nolint:errcheck // Best-effort cleanup during shutdown.
	}

	var shutdownErr error
	if srv != nil {
		shutdownErr = srv.Shutdown(shutdownCtx)
		if shutdownErr != nil && !isClosedConnError(shutdownErr) {
			s.logger.Error("shutdown error", "error", shutdownErr)
		} else {
			shutdownErr = nil
		}
	}

	// Wait for all goroutines to exit
	s.base.WaitForShutdown()

	// Transition to Stopped and close error channel
	s.base.TransitionToStopped()
	s.logger.Info("SSH server stopped")

	return shutdownErr
}

// serve runs the SSH server and handles errors.
func (s *Server) serve() {
	defer s.base.DoneGoroutine()

	// Transition: Starting -> Running (signals readiness)
	s.base.TransitionToRunning()

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

		if s.base.TransitionToFailed(fmt.Errorf("serve error: %w", err)) != nil {
			return
		}
	}
}

// State returns the current server state.
func (s *Server) State() serverbase.State {
	return s.base.State()
}

// IsRunning returns whether the server is currently running and accepting connections.
// This is a convenience method equivalent to State() == serverbase.StateRunning.
func (s *Server) IsRunning() bool {
	return s.base.IsRunning()
}

// Err returns a channel for receiving async errors.
func (s *Server) Err() <-chan error {
	return s.base.Err()
}

// LastError returns the error that caused the Failed state, or nil.
func (s *Server) LastError() error {
	return s.base.LastError()
}

// Address returns the server's bound address (host:port).
// Blocks until the server has started or failed.
// Returns empty string if server never started or failed.
func (s *Server) Address() string {
	select {
	case <-s.base.StartedChannel():
		s.srvMu.Lock()
		defer s.srvMu.Unlock()
		return s.addr
	default:
		// Server not started yet, check if context exists
		ctx := s.base.Context()
		if ctx == nil {
			return ""
		}
		select {
		case <-s.base.StartedChannel():
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
func (s *Server) Port() ListenPort {
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
	lp := ListenPort(port)
	if lp.Validate() != nil {
		return 0 // Port out of valid range
	}
	return lp
}

// Host returns the server's configured host address.
func (s *Server) Host() HostAddress {
	return s.cfg.Host
}

// Wait blocks until the server stops (either gracefully or due to error).
// Returns the error if the server failed, nil otherwise.
func (s *Server) Wait() error {
	s.base.WaitForShutdown()

	state := s.State()
	if state == serverbase.StateFailed {
		return s.LastError()
	}
	return nil
}

// hostKeyOption returns the ssh.Option that installs the server's host key.
// With a configured HostKeyPath the key is generated on first use and reused
// from then on (stable host identity). Without one, a fresh in-memory key is
// generated per start: clients authenticate through short-lived tokens rather
// than host-key pinning, and an ephemeral key must never be written to disk.
func (s *Server) hostKeyOption() (ssh.Option, error) {
	if s.cfg.HostKeyPath == nil {
		kp, err := keygen.New("", keygen.WithKeyType(keygen.Ed25519))
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral host key: %w", err)
		}
		return wish.WithHostKeyPEM(kp.RawPrivateKey()), nil
	}

	keyPath := string(*s.cfg.HostKeyPath)
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if _, genErr := keygen.New(keyPath, keygen.WithKeyType(keygen.Ed25519), keygen.WithWrite()); genErr != nil {
			// keygen refuses to overwrite an existing key, so a concurrent
			// first run may have won the race between the stat above and the
			// write; in that case reuse the winner's key instead of failing.
			if _, statErr := os.Stat(keyPath); statErr != nil {
				return nil, fmt.Errorf("generate host key: %w", genErr)
			}
		}
	}
	return ssh.HostKeyFile(keyPath), nil
}
