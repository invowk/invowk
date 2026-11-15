// SPDX-License-Identifier: MPL-2.0

package tuiclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// acceptingListener accepts connections and hands each one to handle. It
// stands in for an engine proxy that completes the handshake without reaching
// the real listener.
func acceptingListener(t *testing.T, handle func(net.Conn)) URL {
	t.Helper()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(): %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go handle(conn)
		}
	}()
	return URL("http://" + listener.Addr().String())
}

// holdConns returns a connection handler that keeps every connection open
// without answering, closing them at cleanup.
func holdConns(t *testing.T) func(net.Conn) {
	t.Helper()
	var (
		mu    sync.Mutex
		conns []net.Conn
	)
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range conns {
			_ = conn.Close()
		}
	})
	return func(c net.Conn) {
		mu.Lock()
		defer mu.Unlock()
		conns = append(conns, c)
	}
}

func TestClientUnreachableServerFailsFast(t *testing.T) {
	t.Parallel()

	// A port that was just closed refuses connections.
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(): %v", err)
	}
	refusedAddr := listener.Addr().String()
	_ = listener.Close()

	held := holdConns(t)
	acceptClose := acceptingListener(t, func(c net.Conn) { _ = c.Close() })
	acceptReset := acceptingListener(t, func(c net.Conn) {
		if tcp, ok := c.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0) // RST instead of FIN.
		}
		buf := make([]byte, 1)
		_, _ = c.Read(buf)
		_ = c.Close()
	})
	acceptSilent := acceptingListener(t, held)
	notTUI := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notTUI.Close)

	tests := []struct {
		name    string
		addr    URL
		timeout time.Duration
	}{
		{name: "refused port", addr: URL("http://" + refusedAddr), timeout: reachTimeout},
		// 192.0.2.1 (TEST-NET-1) is not routed, so the dial either times out
		// or fails with "network unreachable"; both must be bounded.
		{name: "blackholed address", addr: URL("http://192.0.2.1:9"), timeout: 300 * time.Millisecond},
		{name: "accept then close", addr: acceptClose, timeout: reachTimeout},
		{name: "accept then reset", addr: acceptReset, timeout: reachTimeout},
		{name: "accept then silent", addr: acceptSilent, timeout: 300 * time.Millisecond},
		{name: "not a TUI server", addr: URL(notTUI.URL), timeout: reachTimeout},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := newClientWithReachTimeout(tt.addr, "token", tt.timeout)
			start := time.Now()
			_, confirmErr := client.ConfirmContext(t.Context(), ConfirmRequest{Title: "x"})
			elapsed := time.Since(start)
			if !errors.Is(confirmErr, ErrTUIServerUnreachable) {
				t.Fatalf("ConfirmContext() error = %v, want ErrTUIServerUnreachable", confirmErr)
			}
			var unreachable *UnreachableError
			if !errors.As(confirmErr, &unreachable) || unreachable.Addr != tt.addr {
				t.Fatalf("error %v does not carry address %s", confirmErr, tt.addr)
			}
			for _, want := range []string{string(tt.addr), "container.tui_server_bind", "all-interfaces"} {
				if !strings.Contains(confirmErr.Error(), want) {
					t.Errorf("error %q does not name %q", confirmErr, want)
				}
			}
			if limit := tt.timeout + 2*time.Second; elapsed > limit {
				t.Errorf("ConfirmContext() took %s, want under %s", elapsed, limit)
			}
		})
	}
}

// TestClientReachErrorsThatAreNotUnreachable pins the two failures the
// preflight must not report as an unreachable server: a rejected token and
// the caller's own cancellation.
func TestClientReachErrorsThatAreNotUnreachable(t *testing.T) {
	t.Parallel()

	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(unauthorized.Close)
	client := newClientWithReachTimeout(URL(unauthorized.URL), "token", reachTimeout)
	_, err := client.ConfirmContext(t.Context(), ConfirmRequest{Title: "x"})
	if err == nil || errors.Is(err, ErrTUIServerUnreachable) || !strings.Contains(err.Error(), "401") {
		t.Fatalf("401 preflight: error = %v, want a 401 server error that is not ErrTUIServerUnreachable", err)
	}

	silent := acceptingListener(t, holdConns(t))
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	client = newClientWithReachTimeout(silent, "token", reachTimeout)
	_, err = client.ConfirmContext(ctx, ConfirmRequest{Title: "x"})
	if err == nil || errors.Is(err, ErrTUIServerUnreachable) {
		t.Fatalf("cancelled caller: error = %v, want the cancellation, not ErrTUIServerUnreachable", err)
	}
}

// TestClientReachTimeoutDoesNotBoundInteraction pins that only the connect
// and the preflight are bounded: a component answer that arrives after the
// reach timeout still succeeds.
func TestClientReachTimeoutDoesNotBoundInteraction(t *testing.T) {
	t.Parallel()

	const reach = 100 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		time.Sleep(3 * reach)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"confirmed":true}}`))
	}))
	t.Cleanup(server.Close)

	client := newClientWithReachTimeout(URL(server.URL), "token", reach)
	confirmed, err := client.ConfirmContext(t.Context(), ConfirmRequest{Title: "x"})
	if err != nil {
		t.Fatalf("ConfirmContext() error = %v, want success after a slow answer", err)
	}
	if !confirmed {
		t.Fatal("ConfirmContext() = false, want true")
	}
}

// TestClientTransportIgnoresProxyEnvironment pins that the TUI request, and
// its bearer token, never go to an HTTP_PROXY from the environment.
func TestClientTransportIgnoresProxyEnvironment(t *testing.T) {
	t.Parallel()

	client := NewClient("http://172.17.0.1:43123", "token")
	transport, ok := client.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport is %T, want *http.Transport", client.client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("transport.Proxy is set, want nil (no proxy for the TUI server)")
	}
}
