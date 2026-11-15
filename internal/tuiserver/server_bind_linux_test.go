// SPDX-License-Identifier: MPL-2.0

//go:build linux

package tuiserver

import (
	"net"
	"net/http"
	"net/netip"
	"testing"
)

// TestServerBindsTwoLoopbackAddressesAtOnePort uses 127.0.0.2, which Linux
// routes to lo without configuration, as a stand-in for a bridge gateway.
func TestServerBindsTwoLoopbackAddressesAtOnePort(t *testing.T) {
	t.Parallel()

	plan, err := NewBindPlan(netip.MustParseAddr("127.0.0.2"))
	if err != nil {
		t.Fatalf("NewBindPlan(): %v", err)
	}
	server, err := NewWithBindPlan(plan)
	if err != nil {
		t.Fatalf("NewWithBindPlan(): %v", err)
	}
	if startErr := server.Start(t.Context()); startErr != nil {
		t.Fatalf("Start(): %v", startErr)
	}

	addrs := server.ListenAddrs()
	if len(addrs) != 2 {
		t.Fatalf("ListenAddrs() = %v, want two listeners", addrs)
	}
	port := server.Port().String()
	for i, want := range []string{"127.0.0.1", "127.0.0.2"} {
		if got := addrs[i].String(); got != net.JoinHostPort(want, port) {
			t.Errorf("listener %d = %s, want %s", i, got, net.JoinHostPort(want, port))
		}
		code, _ := getWithToken(t, URL("http://"+net.JoinHostPort(want, port)), "/health", server.Token())
		if code != http.StatusOK {
			t.Errorf("authenticated /health on %s = %d, want 200", want, code)
		}
	}
	assertNonLoopbackRefused(t, port, nil)

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop(): %v", err)
	}
	for _, want := range []string{"127.0.0.1", "127.0.0.2"} {
		var dialer net.Dialer
		if conn, dialErr := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort(want, port)); dialErr == nil {
			_ = conn.Close()
			t.Errorf("%s:%s still accepts connections after Stop", want, port)
		}
	}
}
