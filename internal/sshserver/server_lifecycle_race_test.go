// SPDX-License-Identifier: MPL-2.0

package sshserver

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/core/serverbase"
	"github.com/invowk/invowk/internal/testutil"
)

// Bounds for the lifecycle race tests. A refused loopback dial takes about
// 2 s on Windows, so dialTimeout leaves room for it.
const (
	hookTimeout = 10 * time.Second
	callTimeout = 10 * time.Second
	waitTimeout = 5 * time.Second
	dialTimeout = 5 * time.Second
)

type (
	// lifecycleHold is a lifecycleHooks point a test controls: reached closes
	// when the hook runs, and the hook returns once release closes.
	lifecycleHold struct {
		reached     chan struct{}
		release     chan struct{}
		reachOnce   sync.Once
		releaseOnce sync.Once
	}

	// addrCapture records the address beforePublish reports.
	addrCapture struct {
		mu    sync.Mutex
		addr  string
		fired atomic.Int32
	}
)

func newLifecycleHold() *lifecycleHold {
	return &lifecycleHold{reached: make(chan struct{}), release: make(chan struct{})}
}

// Release lets the hook return. Safe to call more than once.
func (h *lifecycleHold) Release() { h.releaseOnce.Do(func() { close(h.release) }) }

func (h *lifecycleHold) markReached() { h.reachOnce.Do(func() { close(h.reached) }) }

// hold marks the hook reached and blocks until the hold is released.
func (h *lifecycleHold) hold() {
	h.markReached()
	<-h.release
}

// holdUntil marks the hook reached and blocks until cond holds or the hold
// is released. It polls; it never sleeps a fixed window.
func (h *lifecycleHold) holdUntil(cond func() bool) {
	h.markReached()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for !cond() {
		select {
		case <-h.release:
			return
		case <-ticker.C:
		}
	}
}

func (c *addrCapture) record(addr net.Addr) {
	c.fired.Add(1)
	c.mu.Lock()
	c.addr = addr.String()
	c.mu.Unlock()
}

func (c *addrCapture) get() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addr
}

// newTestServer builds a server on an ephemeral port; mutate may adjust the
// configuration first.
func newTestServer(t *testing.T, mutate func(*Config)) *Server {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Port = 0
	if mutate != nil {
		mutate(&cfg)
	}
	return mustNew(t, cfg)
}

// stopOnCleanup releases every hold and then stops srv when the test ends,
// so a failed assertion never leaves a goroutine blocked in a hook.
func stopOnCleanup(t *testing.T, srv *Server, holds ...*lifecycleHold) {
	t.Helper()
	t.Cleanup(func() {
		for _, h := range holds {
			h.Release()
		}
		testutil.MustStop(t, srv)
	})
}

func stateIs(srv *Server, want serverbase.State) func() bool {
	return func() bool { return srv.State() == want }
}

func startAsync(ctx context.Context, srv *Server) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- srv.Start(ctx) }()
	return ch
}

func stopAsync(srv *Server) <-chan error {
	ch := make(chan error, 1)
	go func() { ch <- srv.Stop() }()
	return ch
}

func awaitErr(t *testing.T, ch <-chan error, within time.Duration, what string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(within):
		t.Fatalf("%s did not return within %v", what, within)
		return nil
	}
}

func awaitClosed(t *testing.T, ch <-chan struct{}, within time.Duration, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(within):
		t.Fatalf("%s not reached within %v", what, within)
	}
}

func requireNotReturned(t *testing.T, ch <-chan error, what string) {
	t.Helper()
	select {
	case err := <-ch:
		t.Fatalf("%s already returned (err = %v), want it still blocked", what, err)
	default:
	}
}

// awaitWait calls Wait (after Start has returned) and requires it to return
// within waitTimeout.
func awaitWait(t *testing.T, srv *Server) error {
	t.Helper()
	ch := make(chan error, 1)
	go func() { ch <- srv.Wait() }()
	return awaitErr(t, ch, waitTimeout, "Wait()")
}

// dialConnects dials addr once and reports whether a connection was made.
func dialConnects(ctx context.Context, addr string) bool {
	d := net.Dialer{Timeout: dialTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func assertDialRefused(t *testing.T, addr string) {
	t.Helper()
	if addr == "" {
		t.Fatal("beforePublish captured no bound address")
	}
	if dialConnects(t.Context(), addr) {
		t.Errorf("dial %s succeeded, want the connection refused", addr)
	}
}

// holdPort binds a loopback port for the whole test, so any bind attempt on
// it by Start fails with a listen error.
func holdPort(t *testing.T) ListenPort {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold port: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("held listener address %T, want *net.TCPAddr", ln.Addr())
	}
	return ListenPort(tcpAddr.Port)
}

func TestServeGate(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	t.Run("nil gate is a no-op", func(t *testing.T) {
		t.Parallel()
		var g *serveGate
		g.open()
		if !g.wait(cancelled) {
			t.Error("nil gate wait() = false, want true")
		}
	})
	t.Run("open is idempotent", func(t *testing.T) {
		t.Parallel()
		g := newServeGate()
		g.open()
		g.open()
		if !g.wait(t.Context()) {
			t.Error("wait() after open = false, want true")
		}
	})
	t.Run("open gate wins over an ended context", func(t *testing.T) {
		t.Parallel()
		g := newServeGate()
		g.open()
		if !g.wait(cancelled) {
			t.Error("wait() on an open gate with an ended context = false, want true")
		}
	})
	t.Run("wait returns false when its context ends first", func(t *testing.T) {
		t.Parallel()
		g := newServeGate()
		ctx, cancelTimeout := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancelTimeout()
		if g.wait(ctx) {
			t.Error("wait() on a closed gate = true, want false")
		}
	})
}

func TestReadyListenerSignalsOnceBeforeFirstAccept(t *testing.T) {
	t.Parallel()

	acceptErr := errors.New("accept failed")
	var calls atomic.Int32
	l := &readyListener{
		serverListener: failingServerListener{err: acceptErr},
		onReady:        func() { calls.Add(1) },
	}
	for range 3 {
		if _, err := l.Accept(); !errors.Is(err, acceptErr) {
			t.Fatalf("Accept() error = %v, want %v", err, acceptErr)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("onReady ran %d times, want 1", n)
	}
}

func TestServerServeWithoutServerOpensGate(t *testing.T) {
	t.Parallel()

	srv := &Server{base: serverbase.NewBase()}
	if err := srv.base.TransitionToStarting(t.Context()); err != nil {
		t.Fatalf("TransitionToStarting() error = %v", err)
	}

	gate := newServeGate()
	srv.base.AddGoroutine()
	go srv.serve(gate)

	ctx, cancel := context.WithTimeout(t.Context(), waitTimeout)
	defer cancel()
	if !gate.wait(ctx) {
		t.Fatal("gate did not open after serve returned early without a server")
	}
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestServerStartWaitsForServeLoop(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, nil)
	hold := newLifecycleHold()
	srv.hooks = &lifecycleHooks{beforeServe: hold.hold}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "beforeServe")

	if st := srv.State(); st != serverbase.StateStarting {
		t.Fatalf("State() = %s while the serve loop is held, want %s (readiness reported before the serve loop)", st, serverbase.StateStarting)
	}
	requireNotReturned(t, startCh, "Start")

	hold.Release()
	if err := awaitErr(t, startCh, callTimeout, "Start"); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if st := srv.State(); st != serverbase.StateRunning {
		t.Errorf("State() after Start = %s, want %s", st, serverbase.StateRunning)
	}
}

// TestServerStopWhileServeLoopDelayed checks that a Stop while the serve goroutine is held
// before the serve loop waits for it, runs the graceful Shutdown once, and
// makes the overtaken Start fail.
//
// Not parallel: it asserts that a freed port refuses a dial, and a parallel
// test of this package binding an ephemeral port could reuse that port first.
//
//nolint:paralleltest // Dials a freed port; see above.
func TestServerStopWhileServeLoopDelayed(t *testing.T) {
	srv := newTestServer(t, nil)
	hold := newLifecycleHold()
	capture := &addrCapture{}
	var shutdowns atomic.Int32
	srv.hooks = &lifecycleHooks{
		beforePublish:  capture.record,
		beforeServe:    func() { hold.holdUntil(stateIs(srv, serverbase.StateStopping)) },
		beforeShutdown: func() { shutdowns.Add(1) },
	}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "beforeServe")

	if st := srv.State(); st != serverbase.StateStarting {
		t.Fatalf("State() = %s while the serve loop is held, want %s (readiness reported before the serve loop)", st, serverbase.StateStarting)
	}
	requireNotReturned(t, startCh, "Start")

	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	startErr := awaitErr(t, startCh, callTimeout, "Start")
	if !errors.Is(startErr, errStoppedDuringStartup) {
		t.Errorf("Start() error = %v, want errStoppedDuringStartup", startErr)
	}
	if n := shutdowns.Load(); n != 1 {
		t.Errorf("graceful Shutdown ran %d times, want 1", n)
	}
	assertDialRefused(t, capture.get())
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

// TestServerStopTimesOutBeforeServeLoop checks that Stop skips the graceful Shutdown
// and returns a timeout error when the serve loop is not reached within
// ShutdownTimeout.
//
// Not parallel: it asserts that a freed port refuses a dial, and a parallel
// test of this package binding an ephemeral port could reuse that port first.
//
//nolint:paralleltest // Dials a freed port; see above.
func TestServerStopTimesOutBeforeServeLoop(t *testing.T) {
	const shutdownTimeout = 50 * time.Millisecond
	srv := newTestServer(t, func(cfg *Config) { cfg.ShutdownTimeout = shutdownTimeout })
	hold := newLifecycleHold()
	capture := &addrCapture{}
	stoppingSeen := make(chan struct{})
	var shutdowns atomic.Int32
	srv.hooks = &lifecycleHooks{
		beforePublish: capture.record,
		beforeServe: func() {
			hold.holdUntil(stateIs(srv, serverbase.StateStopping))
			close(stoppingSeen)
			// No hook reports the teardown's timeout: this margin, ten times
			// the shutdown timeout, is the synchronisation.
			select {
			case <-time.After(10 * shutdownTimeout):
			case <-hold.release:
			}
		},
		beforeShutdown: func() { shutdowns.Add(1) },
	}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "beforeServe")

	stopCh := stopAsync(srv)
	awaitClosed(t, stoppingSeen, hookTimeout, "Stopping state in beforeServe")
	requireNotReturned(t, stopCh, "Stop")

	if err := awaitErr(t, stopCh, callTimeout, "Stop"); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Stop() error = %v, want context.DeadlineExceeded", err)
	}
	if n := shutdowns.Load(); n != 0 {
		t.Errorf("graceful Shutdown ran %d times, want 0 (skipped after the timeout)", n)
	}
	if err := awaitErr(t, startCh, callTimeout, "Start"); err == nil {
		t.Error("Start() error = nil, want an error")
	}
	assertDialRefused(t, capture.get())
	awaitWait(t, srv) // Only the bounded return matters here.
}

// TestServerStopBeforeServeReadsResources checks that a teardown that clears the
// server fields before serve reads them does not wait for ShutdownTimeout.
//
// Not parallel: it asserts that a freed port refuses a dial, and a parallel
// test of this package binding an ephemeral port could reuse that port first.
//
//nolint:paralleltest // Dials a freed port; see above.
func TestServerStopBeforeServeReadsResources(t *testing.T) {
	srv := newTestServer(t, func(cfg *Config) { cfg.ShutdownTimeout = 5 * time.Second })
	hold := newLifecycleHold()
	capture := &addrCapture{}
	srv.hooks = &lifecycleHooks{
		beforePublish: capture.record,
		serveEntered: func() {
			hold.holdUntil(func() bool {
				if srv.State() != serverbase.StateStopping {
					return false
				}
				srv.srvMu.Lock()
				defer srv.srvMu.Unlock()
				return srv.srv == nil
			})
		},
	}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "serveEntered")

	began := time.Now()
	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if elapsed := time.Since(began); elapsed >= time.Second {
		t.Errorf("Stop() took %v, want under 1s (it waited for the shutdown timeout)", elapsed)
	}
	if err := awaitErr(t, startCh, callTimeout, "Start"); !errors.Is(err, errStoppedDuringStartup) {
		t.Errorf("Start() error = %v, want errStoppedDuringStartup", err)
	}
	assertDialRefused(t, capture.get())
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

// TestServerStopBeforeStartPublishes checks that Start launches nothing and closes
// its listener when a Stop finished during its setup.
//
// Not parallel: it asserts that a freed port refuses a dial, and a parallel
// test of this package binding an ephemeral port could reuse that port first.
//
//nolint:paralleltest // Dials a freed port; see above.
func TestServerStopBeforeStartPublishes(t *testing.T) {
	srv := newTestServer(t, nil)
	hold := newLifecycleHold()
	capture := &addrCapture{}
	srv.hooks = &lifecycleHooks{
		beforePublish: func(addr net.Addr) {
			capture.record(addr)
			hold.holdUntil(stateIs(srv, serverbase.StateStopped))
		},
	}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "beforePublish")

	if err := srv.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
	if err := awaitErr(t, startCh, callTimeout, "Start"); !errors.Is(err, errStoppedDuringStartup) {
		t.Errorf("Start() error = %v, want errStoppedDuringStartup", err)
	}
	assertDialRefused(t, capture.get())
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

// TestServerStartupFailureTearsDown checks that a Start failing on its startup timeout
// or a cancelled context closes its listener and leaves Stop and Wait able to
// return.
//
// Not parallel: it asserts that a freed port refuses a dial, and a parallel
// test of this package binding an ephemeral port could reuse that port first.
//
//nolint:paralleltest // Dials a freed port; see above.
func TestServerStartupFailureTearsDown(t *testing.T) {
	tests := []struct {
		name            string
		startupTimeout  time.Duration
		shutdownTimeout time.Duration
		cancel          bool
		holdPastStart   bool
		want            error
	}{
		{
			name:            "timeout",
			startupTimeout:  100 * time.Millisecond,
			shutdownTimeout: 2 * time.Second,
			want:            context.DeadlineExceeded,
		},
		{
			name:            "cancel",
			startupTimeout:  5 * time.Second,
			shutdownTimeout: 2 * time.Second,
			cancel:          true,
			want:            context.Canceled,
		},
		{
			name:            "held past shutdown timeout",
			startupTimeout:  100 * time.Millisecond,
			shutdownTimeout: 100 * time.Millisecond,
			holdPastStart:   true,
			want:            context.DeadlineExceeded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t, func(cfg *Config) {
				cfg.StartupTimeout = tc.startupTimeout
				cfg.ShutdownTimeout = tc.shutdownTimeout
			})
			hold := newLifecycleHold()
			capture := &addrCapture{}
			beforeServe := func() { hold.holdUntil(stateIs(srv, serverbase.StateFailed)) }
			if tc.holdPastStart {
				beforeServe = hold.hold
			}
			srv.hooks = &lifecycleHooks{beforePublish: capture.record, beforeServe: beforeServe}
			stopOnCleanup(t, srv, hold)

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			bound := tc.startupTimeout + tc.shutdownTimeout + 2*time.Second

			startCh := startAsync(ctx, srv)
			awaitClosed(t, hold.reached, hookTimeout, "beforeServe")
			if tc.cancel {
				cancel()
			}

			if err := awaitErr(t, startCh, bound, "Start"); !errors.Is(err, tc.want) {
				t.Errorf("Start() error = %v, want %v", err, tc.want)
			}
			// In the held case the serve goroutine is still held here.
			assertDialRefused(t, capture.get())

			hold.Release()
			if err := awaitErr(t, stopAsync(srv), waitTimeout, "Stop"); err != nil {
				t.Errorf("Stop() error = %v, want nil", err)
			}
			if err := awaitWait(t, srv); !errors.Is(err, tc.want) {
				t.Errorf("Wait() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestServerConcurrentStopsDuringStart(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, nil)
	hold := newLifecycleHold()
	var stopEntries atomic.Int32
	srv.hooks = &lifecycleHooks{
		beforeServe: hold.hold,
		stopEntered: func() { stopEntries.Add(1) },
	}
	stopOnCleanup(t, srv, hold)

	startCh := startAsync(t.Context(), srv)
	awaitClosed(t, hold.reached, hookTimeout, "beforeServe")

	stop1 := stopAsync(srv)
	stop2 := stopAsync(srv)
	testutil.RequirePollUntil(t, hookTimeout, time.Millisecond, "both Stop calls in progress", func() bool {
		return stopEntries.Load() == 2 && srv.State() == serverbase.StateStopping
	})
	requireNotReturned(t, stop1, "first Stop")
	requireNotReturned(t, stop2, "second Stop")

	hold.Release()
	if err := awaitErr(t, stop1, callTimeout, "first Stop"); err != nil {
		t.Errorf("first Stop() error = %v, want nil", err)
	}
	if err := awaitErr(t, stop2, callTimeout, "second Stop"); err != nil {
		t.Errorf("second Stop() error = %v, want nil", err)
	}
	if err := awaitErr(t, startCh, callTimeout, "Start"); !errors.Is(err, errStoppedDuringStartup) {
		t.Errorf("Start() error = %v, want errStoppedDuringStartup", err)
	}
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

func TestServerStartAfterStopWithCancelledContext(t *testing.T) {
	t.Parallel()

	port := holdPort(t)
	srv := newTestServer(t, func(cfg *Config) { cfg.Port = port })
	capture := &addrCapture{}
	srv.hooks = &lifecycleHooks{beforePublish: capture.record}

	if err := srv.Stop(); err != nil {
		t.Fatalf("Stop() on a fresh server error = %v, want nil", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := srv.Start(ctx)
	if !errors.Is(err, errStoppedDuringStartup) {
		t.Errorf("Start() error = %v, want errStoppedDuringStartup", err)
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		t.Errorf("Start() error = %v, want no bind attempt", err)
	}
	if n := capture.fired.Load(); n != 0 {
		t.Errorf("beforePublish fired %d times, want 0", n)
	}
	if st := srv.State(); st != serverbase.StateStopped {
		t.Errorf("State() = %s, want %s", st, serverbase.StateStopped)
	}
	if err := awaitWait(t, srv); err != nil {
		t.Errorf("Wait() error = %v, want nil", err)
	}
}

// TestServerStopRacesStartWithCancelledContext is not parallel: it ends with a
// process-wide goroutine-leak assertion.
//
//nolint:paralleltest // Leak assertion attributes by package import path.
func TestServerStopRacesStartWithCancelledContext(t *testing.T) {
	port := holdPort(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for i := range 50 {
		srv := newTestServer(t, func(cfg *Config) { cfg.Port = port })
		capture := &addrCapture{}
		srv.hooks = &lifecycleHooks{beforePublish: capture.record}

		release := make(chan struct{})
		stopCh := make(chan error, 1)
		startCh := make(chan error, 1)
		go func() {
			<-release
			stopCh <- srv.Stop()
		}()
		go func() {
			<-release
			startCh <- srv.Start(ctx)
		}()
		close(release)

		startErr := awaitErr(t, startCh, waitTimeout, "Start")
		stopErr := awaitErr(t, stopCh, waitTimeout, "Stop")
		if startErr == nil {
			t.Errorf("iteration %d: Start() error = nil, want an error", i)
		}
		if _, ok := errors.AsType[*net.OpError](startErr); ok {
			t.Errorf("iteration %d: Start() error = %v, want no bind attempt", i, startErr)
		}
		if stopErr != nil {
			t.Errorf("iteration %d: Stop() error = %v, want nil", i, stopErr)
		}
		if n := capture.fired.Load(); n != 0 {
			t.Errorf("iteration %d: beforePublish fired %d times, want 0", i, n)
		}
		awaitWait(t, srv) // Only the bounded return matters here.
	}

	testutil.AssertNoGoroutineLeaks(t, "github.com/invowk/invowk/internal/sshserver")
}

// TestServerStartStopLoop is not parallel: it ends with a process-wide
// goroutine-leak assertion.
//
//nolint:paralleltest // Leak assertion attributes by package import path.
func TestServerStartStopLoop(t *testing.T) {
	const iterations = 25
	type dialResult struct {
		addr     string
		connects <-chan bool
	}
	var dials []dialResult
	dialAsync := func(addr string) {
		ch := make(chan bool, 1)
		go func() { ch <- dialConnects(t.Context(), addr) }()
		dials = append(dials, dialResult{addr: addr, connects: ch})
	}

	for i := range iterations {
		srv := newTestServer(t, func(cfg *Config) { cfg.ShutdownTimeout = 5 * time.Second })
		capture := &addrCapture{}
		srv.hooks = &lifecycleHooks{beforePublish: capture.record}

		if err := srv.Start(t.Context()); err != nil {
			t.Fatalf("back-to-back iteration %d: Start() error = %v", i, err)
		}
		began := time.Now()
		if err := srv.Stop(); err != nil {
			t.Errorf("back-to-back iteration %d: Stop() error = %v", i, err)
		}
		if elapsed := time.Since(began); elapsed >= time.Second {
			t.Errorf("back-to-back iteration %d: Stop() took %v, want under 1s", i, elapsed)
		}
		dialAsync(capture.get())
	}

	for i := range iterations {
		srv := newTestServer(t, func(cfg *Config) { cfg.ShutdownTimeout = 5 * time.Second })
		capture := &addrCapture{}
		srv.hooks = &lifecycleHooks{beforePublish: capture.record}

		startCh := startAsync(t.Context(), srv)
		began := time.Now()
		stopErr := srv.Stop()
		stopElapsed := time.Since(began)
		startErr := awaitErr(t, startCh, callTimeout, "Start")

		if startErr == nil {
			// nil only for a server that reached Running.
			select {
			case <-srv.base.StartedChannel():
			default:
				t.Errorf("racing iteration %d: Start() = nil, but the server never reached Running", i)
			}
		}
		if stopErr != nil {
			t.Errorf("racing iteration %d: Stop() error = %v, want nil", i, stopErr)
		}
		if stopElapsed >= time.Second {
			t.Errorf("racing iteration %d: Stop() took %v, want under 1s", i, stopElapsed)
		}
		awaitWait(t, srv) // Only the bounded return matters here.
		if addr := capture.get(); addr != "" {
			dialAsync(addr)
		}
	}

	for _, d := range dials {
		if <-d.connects {
			t.Errorf("dial %s succeeded, want the connection refused", d.addr)
		}
	}
	testutil.AssertNoGoroutineLeaks(t, "github.com/invowk/invowk/internal/sshserver")
}
