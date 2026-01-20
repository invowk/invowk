// SPDX-License-Identifier: EPL-2.0

// Package sshserver provides an SSH server using the Wish library for container callback.
// This allows container-executed commands to SSH back into the host system.
// The server only accepts connections from commands that invowk itself has spawned,
// using a token-based authentication mechanism.
package sshserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
)

const (
	// StateCreated indicates the server has been created but not started.
	StateCreated ServerState = iota
	// StateStarting indicates the server is in the process of starting.
	StateStarting
	// StateRunning indicates the server is running and accepting connections.
	StateRunning
	// StateStopping indicates the server is shutting down.
	StateStopping
	// StateStopped indicates the server has stopped (terminal state).
	StateStopped
	// StateFailed indicates the server failed to start or encountered a fatal error (terminal state).
	StateFailed
)

type (
	// ServerState represents the lifecycle state of the server.
	ServerState int32

	// Token represents an authentication token for container callbacks.
	Token struct {
		Value     string
		CreatedAt time.Time
		ExpiresAt time.Time
		CommandID string
		Used      bool
	}

	// Server represents the SSH server for container callbacks.
	// A Server instance is single-use: once stopped or failed, create a new instance.
	Server struct {
		// Immutable configuration (set at creation, never modified)
		cfg Config

		// State management (atomic for lock-free reads)
		state atomic.Int32

		// Initialized during Start() - protected by stateMu for writes
		stateMu  sync.Mutex
		srv      *ssh.Server
		listener net.Listener
		addr     string // Actual bound address (including resolved port)

		// Lifecycle management
		ctx       context.Context
		cancel    context.CancelFunc
		wg        sync.WaitGroup
		startedCh chan struct{} // Closed when server is ready to accept connections
		errCh     chan error    // Receives fatal errors from background goroutines
		lastErr   error         // Stores the last error for State() == StateFailed

		// Token management
		tokens  map[string]*Token
		tokenMu sync.RWMutex

		// Logger
		logger *log.Logger
	}

	// Config holds immutable configuration for the SSH server.
	Config struct {
		// Host is the address to bind to (default: 127.0.0.1)
		Host string
		// Port is the port to listen on (0 = auto-select)
		Port int
		// TokenTTL is how long tokens are valid (default: 1 hour)
		TokenTTL time.Duration
		// ShutdownTimeout is the timeout for graceful shutdown (default: 10s)
		ShutdownTimeout time.Duration
		// DefaultShell is the shell to use (default: /bin/sh)
		DefaultShell string
		// StartupTimeout is the max time to wait for server to be ready (default: 5s)
		StartupTimeout time.Duration
	}

	// ConnectionInfo contains information needed to connect to the SSH server.
	ConnectionInfo struct {
		Host     string
		Port     int
		Token    string
		User     string
		ExpireAt time.Time
	}
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

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		Host:            "127.0.0.1",
		Port:            0,
		TokenTTL:        time.Hour,
		ShutdownTimeout: 10 * time.Second,
		DefaultShell:    "/bin/sh",
		StartupTimeout:  5 * time.Second,
	}
}

// New creates a new SSH server instance.
// The server is not started; call Start() to begin accepting connections.
func New(cfg Config) *Server {
	// Apply defaults
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = time.Hour
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.DefaultShell == "" {
		cfg.DefaultShell = "/bin/sh"
	}
	if cfg.StartupTimeout == 0 {
		cfg.StartupTimeout = 5 * time.Second
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		Prefix: "ssh-server",
	})

	s := &Server{
		cfg:       cfg,
		tokens:    make(map[string]*Token),
		startedCh: make(chan struct{}),
		errCh:     make(chan error, 1), // Buffered so goroutines don't block
		logger:    logger,
	}
	s.state.Store(int32(StateCreated))

	return s
}

// Start starts the SSH server and blocks until either:
//   - The server is ready to accept connections (returns nil)
//   - The server fails to start (returns error)
//   - The context is cancelled (returns context error)
//   - The startup timeout is exceeded (returns error)
//
// After Start() returns nil, use Err() to monitor for runtime errors.
func (s *Server) Start(ctx context.Context) error {
	// Check for already-cancelled context BEFORE any setup.
	// This prevents a race condition where the serve goroutine could transition
	// to StateRunning before the cancelled context is detected in the select.
	select {
	case <-ctx.Done():
		s.transitionToFailed(fmt.Errorf("context cancelled before start: %w", ctx.Err()))
		return s.lastErr
	default:
	}

	// Transition: Created -> Starting
	if !s.state.CompareAndSwap(int32(StateCreated), int32(StateStarting)) {
		currentState := ServerState(s.state.Load())
		return fmt.Errorf("cannot start server in state %s", currentState)
	}

	// Create internal context for lifecycle management
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Setup timeout for startup
	startupCtx, startupCancel := context.WithTimeout(ctx, s.cfg.StartupTimeout)
	defer startupCancel()

	// Initialize listener
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	var lc net.ListenConfig
	listener, err := lc.Listen(startupCtx, "tcp", addr)
	if err != nil {
		s.transitionToFailed(fmt.Errorf("failed to listen on %s: %w", addr, err))
		return s.lastErr
	}

	s.stateMu.Lock()
	s.listener = listener
	s.addr = listener.Addr().String()
	s.stateMu.Unlock()

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
		s.transitionToFailed(fmt.Errorf("failed to create SSH server: %w", err))
		return s.lastErr
	}

	s.stateMu.Lock()
	s.srv = srv
	s.stateMu.Unlock()

	// Start the serve goroutine
	s.wg.Add(1)
	go s.serve()

	// Start token cleanup goroutine
	s.wg.Add(1)
	go s.cleanupExpiredTokens()

	// Wait for server to be ready or fail
	select {
	case <-s.startedCh:
		// Server is ready
		s.logger.Info("SSH server started", "address", s.addr)
		return nil

	case err := <-s.errCh:
		// Server failed during startup
		s.transitionToFailed(err)
		return err

	case <-startupCtx.Done():
		// Startup timeout or caller cancelled
		s.cancel() // Stop any background work
		s.transitionToFailed(fmt.Errorf("startup timeout: %w", startupCtx.Err()))
		return s.lastErr
	}
}

// Stop gracefully stops the SSH server.
// It blocks until all connections are closed or the shutdown timeout is reached.
// Safe to call multiple times; subsequent calls are no-ops.
func (s *Server) Stop() error {
	// Only proceed if we're in a stoppable state
	for {
		currentState := ServerState(s.state.Load())
		switch currentState {
		case StateStopped, StateFailed:
			return nil // Already stopped
		case StateCreated:
			if s.state.CompareAndSwap(int32(StateCreated), int32(StateStopped)) {
				return nil // Never started
			}
			continue // State changed, retry
		case StateStopping:
			// Wait for ongoing stop to complete
			s.wg.Wait()
			return nil
		case StateStarting, StateRunning:
			// Transition to Stopping
			if !s.state.CompareAndSwap(int32(currentState), int32(StateStopping)) {
				continue // State changed, retry
			}
			// Proceed with shutdown
			return s.doStop()
		default:
			return fmt.Errorf("unknown server state: %d", currentState)
		}
	}
}

// Err returns a channel that receives fatal server errors.
// Use this to monitor for unexpected failures after Start() returns.
// The channel is closed when the server stops.
func (s *Server) Err() <-chan error {
	return s.errCh
}

// State returns the current server state.
func (s *Server) State() ServerState {
	return ServerState(s.state.Load())
}

// IsRunning returns whether the server is currently running and accepting connections.
// This is a convenience method equivalent to State() == StateRunning.
func (s *Server) IsRunning() bool {
	return s.State() == StateRunning
}

// Address returns the server's bound address (host:port).
// Blocks until the server has started or failed.
// Returns empty string if server never started or failed.
func (s *Server) Address() string {
	select {
	case <-s.startedCh:
		s.stateMu.Lock()
		defer s.stateMu.Unlock()
		return s.addr
	case <-s.ctx.Done():
		return ""
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
	s.wg.Wait()

	state := s.State()
	if state == StateFailed {
		return s.lastErr
	}
	return nil
}

// GenerateToken creates a new authentication token for a command.
func (s *Server) GenerateToken(commandID string) (*Token, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	tokenValue := hex.EncodeToString(tokenBytes)

	token := &Token{
		Value:     tokenValue,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(s.cfg.TokenTTL),
		CommandID: commandID,
		Used:      false,
	}

	s.tokenMu.Lock()
	s.tokens[tokenValue] = token
	s.tokenMu.Unlock()

	s.logger.Debug("Generated token", "commandID", commandID)

	return token, nil
}

// ValidateToken checks if a token is valid.
func (s *Server) ValidateToken(tokenValue string) (*Token, bool) {
	s.tokenMu.RLock()
	token, exists := s.tokens[tokenValue]
	s.tokenMu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().After(token.ExpiresAt) {
		s.RevokeToken(tokenValue)
		return nil, false
	}

	return token, true
}

// RevokeToken invalidates a token.
func (s *Server) RevokeToken(tokenValue string) {
	s.tokenMu.Lock()
	delete(s.tokens, tokenValue)
	s.tokenMu.Unlock()
}

// RevokeTokensForCommand revokes all tokens for a specific command.
func (s *Server) RevokeTokensForCommand(commandID string) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	for tokenValue, token := range s.tokens {
		if token.CommandID == commandID {
			delete(s.tokens, tokenValue)
		}
	}
}

// GetConnectionInfo returns connection information for a command.
// Returns an error if the server is not running.
func (s *Server) GetConnectionInfo(commandID string) (*ConnectionInfo, error) {
	if !s.IsRunning() {
		return nil, fmt.Errorf("SSH server is not running (state: %s)", s.State())
	}

	token, err := s.GenerateToken(commandID)
	if err != nil {
		return nil, err
	}

	return &ConnectionInfo{
		Host:     s.cfg.Host,
		Port:     s.Port(),
		Token:    token.Value,
		User:     "invowk",
		ExpireAt: token.ExpiresAt,
	}, nil
}

// serve runs the SSH server and handles errors.
func (s *Server) serve() {
	defer s.wg.Done()

	// Transition: Starting -> Running (signals readiness)
	if s.state.CompareAndSwap(int32(StateStarting), int32(StateRunning)) {
		close(s.startedCh)
	}

	// Block serving connections
	s.stateMu.Lock()
	srv := s.srv
	listener := s.listener
	s.stateMu.Unlock()

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
		select {
		case s.errCh <- fmt.Errorf("serve error: %w", err):
		default:
			// Error channel full, log instead
			s.logger.Error("SSH server error (channel full)", "error", err)
		}
	}
}

// doStop performs the actual shutdown logic.
func (s *Server) doStop() error {
	// Signal all goroutines to stop
	if s.cancel != nil {
		s.cancel()
	}

	// Shutdown SSH server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer shutdownCancel()

	var shutdownErr error
	s.stateMu.Lock()
	if s.srv != nil {
		shutdownErr = s.srv.Shutdown(shutdownCtx)
		if shutdownErr != nil && !isClosedConnError(shutdownErr) {
			s.logger.Error("shutdown error", "error", shutdownErr)
		} else {
			shutdownErr = nil
		}
	}
	if s.listener != nil {
		_ = s.listener.Close() // Best-effort cleanup during shutdown
	}
	s.stateMu.Unlock()

	// Wait for all goroutines to exit
	s.wg.Wait()

	// Transition to Stopped
	s.state.Store(int32(StateStopped))
	s.logger.Info("SSH server stopped")

	// Close error channel to signal consumers
	close(s.errCh)

	return shutdownErr
}

// transitionToFailed sets the server state to Failed and stores the error.
func (s *Server) transitionToFailed(err error) {
	s.lastErr = err
	s.state.Store(int32(StateFailed))
	if s.cancel != nil {
		s.cancel()
	}
	// Send error to channel for Err() consumers (non-blocking)
	select {
	case s.errCh <- err:
	default:
	}
}

// cleanupExpiredTokens periodically removes expired tokens.
func (s *Server) cleanupExpiredTokens() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.tokenMu.Lock()
			now := time.Now()
			for tokenValue, token := range s.tokens {
				if now.After(token.ExpiresAt) {
					delete(s.tokens, tokenValue)
				}
			}
			s.tokenMu.Unlock()
		}
	}
}

// passwordHandler handles password authentication using tokens.
func (s *Server) passwordHandler(ctx ssh.Context, password string) bool {
	token, valid := s.ValidateToken(password)
	if !valid {
		s.logger.Warn("Invalid token authentication attempt", "user", ctx.User())
		return false
	}

	// Store the token info in the context for later use
	ctx.SetValue("token", token)
	ctx.SetValue("commandID", token.CommandID)

	s.logger.Debug("Token authentication successful", "commandID", token.CommandID)
	return true
}

// publicKeyHandler rejects all public key authentication.
// We only want token-based authentication.
func (s *Server) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	// Reject public key auth - we only accept token-based password auth
	return false
}

// commandMiddleware handles command execution.
func (s *Server) commandMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			cmd := sess.Command()

			if len(cmd) == 0 {
				// Interactive shell session
				s.runInteractiveShell(sess)
			} else {
				// Execute command directly
				s.runCommand(sess, cmd)
			}
		}
	}
}

// runInteractiveShell starts an interactive shell session.
func (s *Server) runInteractiveShell(sess ssh.Session) {
	shell := s.cfg.DefaultShell

	cmd := exec.CommandContext(sess.Context(), shell)
	cmd.Env = append(os.Environ(), sess.Environ()...)

	ptyReq, winCh, isPty := sess.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	}

	// Start the command with a pseudo-terminal
	f, err := startPty(cmd)
	if err != nil {
		_, _ = fmt.Fprintf(sess.Stderr(), "Error starting shell: %v\n", err)
		_ = sess.Exit(1) //nolint:errcheck // Terminal operation; error non-critical
		return
	}
	defer func() { _ = f.Close() }() // PTY cleanup; error non-critical

	// Handle window size changes
	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
		}
	}()

	// Copy I/O
	go func() {
		_, _ = copyBuffer(f, sess) //nolint:errcheck // I/O copy; errors are non-recoverable
	}()
	_, _ = copyBuffer(sess, f) //nolint:errcheck // I/O copy; errors are non-recoverable

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			_ = sess.Exit(exitErr.ExitCode()) //nolint:errcheck // Terminal operation; error non-critical
			return
		}
	}
	_ = sess.Exit(0) //nolint:errcheck // Terminal operation; error non-critical
}

// runCommand executes a single command.
func (s *Server) runCommand(sess ssh.Session, args []string) {
	var cmd *exec.Cmd
	if len(args) == 1 {
		cmd = exec.CommandContext(sess.Context(), s.cfg.DefaultShell, "-c", args[0])
	} else {
		cmd = exec.CommandContext(sess.Context(), args[0], args[1:]...)
	}

	cmd.Env = append(os.Environ(), sess.Environ()...)
	cmd.Stdin = sess
	cmd.Stdout = sess
	cmd.Stderr = sess.Stderr()

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			_ = sess.Exit(exitErr.ExitCode()) //nolint:errcheck // Terminal operation; error non-critical
			return
		}
		_, _ = fmt.Fprintf(sess.Stderr(), "Error: %v\n", err)
		_ = sess.Exit(1) //nolint:errcheck // Terminal operation; error non-critical
		return
	}
	_ = sess.Exit(0) //nolint:errcheck // Terminal operation; error non-critical
}

// isClosedConnError checks if the error is a "use of closed network connection" error.
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Err.Error() == "use of closed network connection"
	}
	return false
}
