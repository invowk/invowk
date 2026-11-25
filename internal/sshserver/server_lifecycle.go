// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"github.com/charmbracelet/keygen"

	"github.com/invowk/invowk/internal/core/serverbase"
)

// errStoppedDuringStartup reports a Start that a concurrent Stop overtook
// before the server reached readiness.
var errStoppedDuringStartup = errors.New("ssh server stopped during startup")

type (
	// serverListener is an interface for net.Listener to enable testing.
	serverListener interface {
		Accept() (net.Conn, error)
		Close() error
		Addr() net.Addr
	}

	// serveGate opens once the serve goroutine has reached the SSH library's
	// first Accept, or has returned. teardown waits for it before it calls the
	// library's Shutdown. A nil *serveGate is valid: open and wait are no-ops.
	serveGate struct {
		once sync.Once
		ch   chan struct{}
	}

	// readyListener wraps the listener passed to the SSH library's Serve and
	// runs onReady once, at the first Accept.
	//
	// Workaround for https://github.com/charmbracelet/ssh/issues/61: in
	// charm.land/ssh v0.4.3, a Shutdown that runs before Serve has registered
	// its listener returns at once, and Serve then registers the listener,
	// resets its done channel and serves on, so its listener WaitGroup's Add
	// races Shutdown's Wait. Serve calls trackListener (the Add) before its
	// first Accept in the same goroutine, so readiness signalled from that
	// Accept orders the Add before any Shutdown that teardown runs. Remove
	// the wrapper, the serve gate and teardown's wait for it (back to a plain
	// Shutdown after closing the listener) once upstream fixes the defect;
	// re-check that ordering whenever charm.land/ssh is upgraded.
	readyListener struct {
		serverListener
		once    sync.Once
		onReady func()
	}

	// lifecycleHooks is a test-only seam into Start, serve, teardown and Stop.
	// Every field may be nil, and a nil *lifecycleHooks calls nothing.
	lifecycleHooks struct {
		// beforePublish runs in Start after wish.NewServer has succeeded,
		// immediately before the publish critical section, with the bound address.
		beforePublish func(net.Addr)
		// serveEntered runs at the top of serve, before it reads srv and listener.
		serveEntered func()
		// beforeServe runs in serve after it has read srv and listener, before srv.Serve.
		beforeServe func()
		// beforeShutdown runs in teardown immediately before the library's Shutdown.
		beforeShutdown func()
		// stopEntered runs at the top of Stop, before it takes stopMu.
		stopEntered func()
	}
)

// Start starts the SSH server and blocks until either:
//   - The SSH library serves the listener: it has registered it and is
//     about to accept the first connection (returns nil)
//   - The server fails to start (returns error)
//   - The context is cancelled (returns context error)
//   - The startup timeout is exceeded (returns error)
//   - A concurrent Stop stops the server first (returns an error)
//
// Start returns nil only once it has observed readiness. Every failure after
// the server left the Created state tears the server down, through the same
// routine as Stop, before Start returns: no listener stays open.
//
// After Start() returns nil, use Err() to monitor for runtime errors.
func (s *Server) Start(ctx context.Context) error {
	// This handles the cancelled context check and Created -> Starting transition.
	// Its errors mean this call bound nothing and owns nothing to tear down.
	if err := s.base.TransitionToStarting(ctx); err != nil {
		return err
	}
	// With an already-cancelled ctx, TransitionToStarting returns nil without
	// a transition on a server a Stop already moved Created -> Stopped. Such a
	// server must not bind a listener or launch goroutines.
	if s.base.State().IsTerminal() {
		return s.failStart(ctx, errStoppedDuringStartup)
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
		return s.failStart(ctx, fmt.Errorf("failed to configure SSH host key: %w", err))
	}

	// Initialize listener
	addr := net.JoinHostPort(string(s.cfg.Host), s.cfg.Port.String())
	var lc net.ListenConfig
	listener, err := lc.Listen(startupCtx, "tcp", addr)
	if err != nil {
		return s.failStart(ctx, fmt.Errorf("failed to listen on %s: %w", addr, err))
	}

	// Create SSH server
	wishOptions := []ssh.Option{
		wish.WithAddress(addr),
		hostKeyOption,
		wish.WithPublicKeyAuth(s.publicKeyHandler),
		wish.WithPasswordAuth(s.passwordHandler),
		s.trackConnections(),
		wish.WithMiddleware(
			activeterm.Middleware(),
			s.commandMiddleware(), //nolint:contextcheck // Wish injects request context into sessions handled by middleware.
		),
	}
	srv, err := wish.NewServer(wishOptions...)
	if err != nil {
		_ = listener.Close() // Best-effort cleanup on error
		return s.failStart(ctx, fmt.Errorf("failed to create SSH server: %w", err))
	}

	s.hooks.callBeforePublish(listener.Addr())
	if !s.publish(listener, srv, newServeGate()) {
		// A Stop began its teardown during setup: launch nothing.
		_ = listener.Close() // Best-effort cleanup of an unpublished listener.
		return s.failStart(ctx, errStoppedDuringStartup)
	}

	// Wait for server to be ready or fail
	select {
	case <-s.base.StartedChannel():
		// Server is ready
		s.logger.Info("SSH server started", "address", listener.Addr().String())
		return nil

	case err := <-s.Err():
		// Server failed during startup, or a concurrent Stop closed the
		// channel (err is nil then).
		return s.failStart(ctx, err)

	case <-startupCtx.Done():
		// Startup timeout or caller cancelled
		return s.failStart(ctx, fmt.Errorf("startup timeout: %w", startupCtx.Err()))
	}
}

// Stop gracefully stops the SSH server.
// It blocks until all connections are closed or the shutdown timeout is reached.
// Safe to call multiple times and concurrently: a Stop that does not perform
// the shutdown itself returns once the server's goroutines have exited.
func (s *Server) Stop() error {
	s.hooks.callStopEntered()

	// Held for the whole call: a losing Stop reaches WaitForShutdown only
	// after the winning Stop or a failing Start has finished its teardown, so
	// it never waits while Start can still add a goroutine.
	s.stopMu.Lock()
	defer s.stopMu.Unlock()

	if !s.base.TransitionToStopping() {
		// Already stopped, stopping, created, or failed
		s.base.WaitForShutdown()
		return nil
	}

	// Proceed with shutdown
	return s.doStop()
}

// publish stores the listener, the SSH server and the serve gate and launches
// the server goroutines, all in one srvMu critical section. It refuses, and
// launches nothing, once a teardown has begun: every AddGoroutine therefore
// precedes the WaitForShutdown of any Stop that follows that teardown.
func (s *Server) publish(listener serverListener, srv *ssh.Server, gate *serveGate) bool {
	s.srvMu.Lock()
	defer s.srvMu.Unlock()

	if s.tornDown {
		return false
	}
	s.listener = listener
	s.addr = listener.Addr().String()
	s.srv = srv
	s.gate = gate

	// Start the serve goroutine; it receives its gate as an argument, so a
	// teardown that clears the fields cannot leave it without one.
	s.base.AddGoroutine()
	go s.serve(gate)

	// Start token cleanup goroutine
	s.base.AddGoroutine()
	go s.cleanupExpiredTokens()
	return true
}

// failStart ends a Start that will not reach readiness; ctx is Start's context. It holds stopMu, so
// it never runs inside a concurrent Stop, marks the server Failed and tears
// it down. It never returns nil: it returns the recorded failure or, when a
// concurrent Stop already stopped the server without one, an error matching
// errStoppedDuringStartup.
func (s *Server) failStart(ctx context.Context, cause error) error {
	if cause == nil {
		cause = errStoppedDuringStartup
	}

	s.stopMu.Lock()
	defer s.stopMu.Unlock()

	recorded := s.base.TransitionToFailed(cause)
	// The teardown outlives a cancelled caller context: it has its own
	// ShutdownTimeout and must close the listener whatever ctx says.
	if err := s.teardown(context.WithoutCancel(ctx)); err != nil {
		s.logger.Error("teardown after failed start", "error", err)
	}

	if recorded != nil {
		return recorded
	}
	if errors.Is(cause, errStoppedDuringStartup) {
		return cause
	}
	return fmt.Errorf("%w: %w", errStoppedDuringStartup, cause)
}

// doStop performs the actual shutdown logic.
func (s *Server) doStop() error {
	// Stop has no caller context; the teardown bounds itself by ShutdownTimeout.
	shutdownErr := s.teardown(context.Background())

	// Wait for all goroutines to exit
	s.base.WaitForShutdown()

	// Transition to Stopped and close error channel
	s.base.TransitionToStopped()
	s.logger.Info("SSH server stopped")

	return shutdownErr
}

// teardown closes the published listener and shuts the SSH server down. It is
// the only code that does either, it runs at most once, and its caller holds
// stopMu. It calls the library's Shutdown only after the serve goroutine has
// reached its first Accept or returned, waiting at most ShutdownTimeout; on a
// timeout it skips Shutdown (so the library's listener WaitGroup Add cannot
// race its Wait) and returns an error wrapping context.DeadlineExceeded. The
// closed listener still ends the serve loop at its first Accept. It neither
// waits for the server goroutines nor changes the state. parent must not be
// cancelled by the caller: only its values are used, with ShutdownTimeout.
func (s *Server) teardown(parent context.Context) error {
	s.srvMu.Lock()
	if s.tornDown {
		s.srvMu.Unlock()
		return nil
	}
	s.tornDown = true
	srv := s.srv
	listener := s.listener
	gate := s.gate
	s.srv = nil
	s.listener = nil
	s.gate = nil
	s.srvMu.Unlock()

	// Close the listener before graceful SSH shutdown so a blocked Accept
	// exits promptly on every platform, including Windows race-test runners.
	if listener != nil {
		_ = listener.Close() //nolint:errcheck // Best-effort cleanup during shutdown.
	}
	if srv == nil {
		return nil
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(parent, s.cfg.ShutdownTimeout)
	defer shutdownCancel()

	if !gate.wait(shutdownCtx) {
		s.logger.Error("serve loop did not start before the shutdown timeout; skipping graceful shutdown")
		return fmt.Errorf("waiting for the SSH serve loop: %w", shutdownCtx.Err())
	}

	s.hooks.callBeforeShutdown()
	shutdownErr := srv.Shutdown(shutdownCtx)
	if shutdownErr != nil && !isClosedConnError(shutdownErr) {
		s.logger.Error("shutdown error", "error", shutdownErr)
		return shutdownErr
	}
	return nil
}

// serve runs the SSH server and handles errors. Readiness (Starting ->
// Running) is signalled from the first Accept of the listener the SSH library
// serves, never earlier. gate opens on every return, so teardown never waits
// for the shutdown timeout on an early exit.
func (s *Server) serve(gate *serveGate) {
	defer s.base.DoneGoroutine()
	defer gate.open()

	s.hooks.callServeEntered()

	s.srvMu.Lock()
	srv := s.srv
	listener := s.listener
	s.srvMu.Unlock()

	if srv == nil || listener == nil {
		return
	}

	s.hooks.callBeforeServe()

	// Block serving connections
	err := srv.Serve(&readyListener{
		serverListener: listener,
		onReady: func() {
			// Transition: Starting -> Running (signals readiness)
			s.base.TransitionToRunning()
			gate.open()
		},
	})
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
//
// Call Wait only after Start has returned, or without any Start: it waits for
// the goroutines Start launches, and a Wait concurrent with Start could return
// before Start has launched them.
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

// newServeGate returns a closed-once gate for one serve goroutine.
func newServeGate() *serveGate {
	return &serveGate{ch: make(chan struct{})}
}

// open opens the gate; later calls do nothing. A nil gate does nothing.
func (g *serveGate) open() {
	if g == nil {
		return
	}
	g.once.Do(func() { close(g.ch) })
}

// wait blocks until the gate opens or ctx ends, and reports whether the gate
// opened. An open gate wins over an ended ctx. A nil gate returns true at once.
func (g *serveGate) wait(ctx context.Context) bool {
	if g == nil {
		return true
	}
	select {
	case <-g.ch:
		return true
	default:
	}
	select {
	case <-g.ch:
		return true
	case <-ctx.Done():
		return false
	}
}

// Accept runs onReady before the first Accept, then delegates.
func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(l.onReady)
	return l.serverListener.Accept()
}

func (h *lifecycleHooks) callBeforePublish(addr net.Addr) {
	if h != nil && h.beforePublish != nil {
		h.beforePublish(addr)
	}
}

func (h *lifecycleHooks) callServeEntered() {
	if h != nil && h.serveEntered != nil {
		h.serveEntered()
	}
}

func (h *lifecycleHooks) callBeforeServe() {
	if h != nil && h.beforeServe != nil {
		h.beforeServe()
	}
}

func (h *lifecycleHooks) callBeforeShutdown() {
	if h != nil && h.beforeShutdown != nil {
		h.beforeShutdown()
	}
}

func (h *lifecycleHooks) callStopEntered() {
	if h != nil && h.stopEntered != nil {
		h.stopEntered()
	}
}
