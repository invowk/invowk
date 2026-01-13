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
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
)

// Token represents an authentication token for container callbacks
type Token struct {
	Value     string
	CreatedAt time.Time
	ExpiresAt time.Time
	CommandID string
	Used      bool
}

// Server represents the SSH server for container callbacks
type Server struct {
	// srv is the underlying SSH server
	srv *ssh.Server
	// listener is the TCP listener
	listener net.Listener
	// tokens holds valid authentication tokens
	tokens map[string]*Token
	// tokenMu protects the tokens map
	tokenMu sync.RWMutex
	// port is the port the server is listening on
	port int
	// host is the host address
	host string
	// running indicates if the server is running
	running bool
	// runMu protects the running state
	runMu sync.RWMutex
	// tokenTTL is how long tokens are valid
	tokenTTL time.Duration
	// logger is the server logger
	logger *log.Logger
	// defaultShell is the shell to use for commands
	defaultShell string
}

// Config holds configuration for the SSH server
type Config struct {
	// Host is the address to bind to (default: 127.0.0.1)
	Host string
	// Port is the port to listen on (0 = auto-select)
	Port int
	// TokenTTL is how long tokens are valid (default: 1 hour)
	TokenTTL time.Duration
	// DefaultShell is the shell to use (default: /bin/sh)
	DefaultShell string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Host:         "127.0.0.1",
		Port:         0, // Auto-select port
		TokenTTL:     time.Hour,
		DefaultShell: "/bin/sh",
	}
}

// New creates a new SSH server instance
func New(cfg *Config) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = time.Hour
	}
	if cfg.DefaultShell == "" {
		cfg.DefaultShell = "/bin/sh"
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		Prefix: "ssh-server",
	})

	s := &Server{
		tokens:       make(map[string]*Token),
		host:         cfg.Host,
		port:         cfg.Port,
		tokenTTL:     cfg.TokenTTL,
		logger:       logger,
		defaultShell: cfg.DefaultShell,
	}

	return s, nil
}

// Start starts the SSH server
func (s *Server) Start() error {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	if s.running {
		return fmt.Errorf("server already running")
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	// Get the actual port if auto-selected
	s.port = listener.Addr().(*net.TCPAddr).Port

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
		listener.Close()
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	s.srv = srv
	s.running = true

	// Start serving in background
	go func() {
		if err := srv.Serve(listener); err != nil && err != ssh.ErrServerClosed {
			s.logger.Error("SSH server error", "error", err)
		}
	}()

	s.logger.Info("SSH server started", "address", s.Address())

	// Start token cleanup goroutine
	go s.cleanupExpiredTokens()

	return nil
}

// Stop stops the SSH server
func (s *Server) Stop() error {
	s.runMu.Lock()
	defer s.runMu.Unlock()

	if !s.running {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s.running = false

	if s.srv != nil {
		if err := s.srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown SSH server: %w", err)
		}
	}

	s.logger.Info("SSH server stopped")
	return nil
}

// Address returns the server's listening address
func (s *Server) Address() string {
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

// Port returns the server's listening port
func (s *Server) Port() int {
	return s.port
}

// Host returns the server's host address
func (s *Server) Host() string {
	return s.host
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	s.runMu.RLock()
	defer s.runMu.RUnlock()
	return s.running
}

// GenerateToken creates a new authentication token for a command
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
		ExpiresAt: time.Now().Add(s.tokenTTL),
		CommandID: commandID,
		Used:      false,
	}

	s.tokenMu.Lock()
	s.tokens[tokenValue] = token
	s.tokenMu.Unlock()

	s.logger.Debug("Generated token", "commandID", commandID)

	return token, nil
}

// ValidateToken checks if a token is valid
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

// RevokeToken invalidates a token
func (s *Server) RevokeToken(tokenValue string) {
	s.tokenMu.Lock()
	delete(s.tokens, tokenValue)
	s.tokenMu.Unlock()
}

// RevokeTokensForCommand revokes all tokens for a specific command
func (s *Server) RevokeTokensForCommand(commandID string) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	for tokenValue, token := range s.tokens {
		if token.CommandID == commandID {
			delete(s.tokens, tokenValue)
		}
	}
}

// cleanupExpiredTokens periodically removes expired tokens
func (s *Server) cleanupExpiredTokens() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		if !s.IsRunning() {
			return
		}

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

// passwordHandler handles password authentication using tokens
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

// publicKeyHandler rejects all public key authentication
// We only want token-based authentication
func (s *Server) publicKeyHandler(ctx ssh.Context, key ssh.PublicKey) bool {
	// Reject public key auth - we only accept token-based password auth
	return false
}

// commandMiddleware handles command execution
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

// runInteractiveShell starts an interactive shell session
func (s *Server) runInteractiveShell(sess ssh.Session) {
	shell := s.defaultShell

	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), sess.Environ()...)

	ptyReq, winCh, isPty := sess.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	}

	// Start the command with a pseudo-terminal
	f, err := startPty(cmd)
	if err != nil {
		fmt.Fprintf(sess.Stderr(), "Error starting shell: %v\n", err)
		sess.Exit(1)
		return
	}
	defer f.Close()

	// Handle window size changes
	go func() {
		for win := range winCh {
			setWinsize(f, win.Width, win.Height)
		}
	}()

	// Copy I/O
	go func() {
		_, _ = copyBuffer(f, sess)
	}()
	_, _ = copyBuffer(sess, f)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			sess.Exit(exitErr.ExitCode())
			return
		}
	}
	sess.Exit(0)
}

// runCommand executes a single command
func (s *Server) runCommand(sess ssh.Session, args []string) {
	var cmd *exec.Cmd
	if len(args) == 1 {
		cmd = exec.Command(s.defaultShell, "-c", args[0])
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	cmd.Env = append(os.Environ(), sess.Environ()...)
	cmd.Stdin = sess
	cmd.Stdout = sess
	cmd.Stderr = sess.Stderr()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			sess.Exit(exitErr.ExitCode())
			return
		}
		fmt.Fprintf(sess.Stderr(), "Error: %v\n", err)
		sess.Exit(1)
		return
	}
	sess.Exit(0)
}

// ConnectionInfo contains information needed to connect to the SSH server
type ConnectionInfo struct {
	Host     string
	Port     int
	Token    string
	User     string
	ExpireAt time.Time
}

// GetConnectionInfo returns connection information for a command
func (s *Server) GetConnectionInfo(commandID string) (*ConnectionInfo, error) {
	if !s.IsRunning() {
		return nil, fmt.Errorf("SSH server is not running")
	}

	token, err := s.GenerateToken(commandID)
	if err != nil {
		return nil, err
	}

	return &ConnectionInfo{
		Host:     s.host,
		Port:     s.port,
		Token:    token.Value,
		User:     "invowk",
		ExpireAt: token.ExpiresAt,
	}, nil
}
