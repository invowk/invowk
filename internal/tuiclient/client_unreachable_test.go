// SPDX-License-Identifier: MPL-2.0

package tuiclient

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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

	tests := []struct {
		name    string
		addr    URL
		timeout time.Duration
	}{
		{name: "refused port", addr: URL("http://" + refusedAddr), timeout: dialTimeout},
		// 192.0.2.1 (TEST-NET-1) is not routed, so the dial either times out
		// or fails with "network unreachable"; both must be bounded.
		{name: "blackholed address", addr: URL("http://192.0.2.1:9"), timeout: 300 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := newClientWithDialTimeout(tt.addr, "token", tt.timeout)
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

// TestClientDialTimeoutDoesNotBoundInteraction pins that only the connect is
// bounded: a handler that answers after the dial timeout still succeeds.
func TestClientDialTimeoutDoesNotBoundInteraction(t *testing.T) {
	t.Parallel()

	const dial = 100 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(3 * dial)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"confirmed":true}}`))
	}))
	t.Cleanup(server.Close)

	client := newClientWithDialTimeout(URL(server.URL), "token", dial)
	confirmed, err := client.ConfirmContext(t.Context(), ConfirmRequest{Title: "x"})
	if err != nil {
		t.Fatalf("ConfirmContext() error = %v, want success after a slow answer", err)
	}
	if !confirmed {
		t.Fatal("ConfirmContext() = false, want true")
	}
}
