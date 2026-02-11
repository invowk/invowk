// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"invowk-cli/internal/core/serverbase"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
)

type (
	// Clock abstracts time operations for deterministic testing.
	// Production code uses realClock; tests inject a fake clock.
	Clock interface {
		Now() time.Time
	}

	// realClock implements Clock using actual system time.
	realClock struct{}

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
	//
	// Server embeds serverbase.Base for lifecycle management and state machine.
	Server struct {
		// Embed serverbase.Base for common server lifecycle management
		*serverbase.Base

		// Immutable configuration (set at creation, never modified)
		cfg Config

		// Clock for time operations (enables deterministic testing)
		clock Clock

		// Initialized during Start() - protected by srvMu for writes
		srvMu    sync.Mutex
		srv      *ssh.Server
		listener serverListener
		addr     string // Actual bound address (including resolved port)

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

// Now returns the current system time.
func (realClock) Now() time.Time {
	return time.Now()
}

// New creates a new SSH server instance with real system time.
// The server is not started; call Start() to begin accepting connections.
func New(cfg Config) *Server {
	return NewWithClock(cfg, realClock{})
}

// NewWithClock creates a new SSH server instance with a custom clock.
// This is primarily used for testing with FakeClock for deterministic time control.
// The server is not started; call Start() to begin accepting connections.
func NewWithClock(cfg Config, clock Clock) *Server {
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
		Base:   serverbase.NewBase(),
		cfg:    cfg,
		clock:  clock,
		tokens: make(map[string]*Token),
		logger: logger,
	}

	return s
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
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
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
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
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
	if opErr, ok := errors.AsType[*netOpError](err); ok {
		return opErr.Err.Error() == "use of closed network connection"
	}
	return false
}
