// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/testutil"
)

func TestNewBindPlanRejectsUnsafeAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr netip.Addr
	}{
		{name: "unspecified", addr: netip.IPv4Unspecified()},
		{name: "ipv6", addr: netip.IPv6Loopback()},
		{name: "invalid", addr: netip.Addr{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewBindPlan(tt.addr); !errors.Is(err, ErrInvalidBindPlan) {
				t.Fatalf("NewBindPlan(%v) error = %v, want ErrInvalidBindPlan", tt.addr, err)
			}
		})
	}

	plan, err := NewBindPlan(netip.MustParseAddr("172.17.0.1"), loopbackAddr)
	if err != nil {
		t.Fatalf("NewBindPlan(): %v", err)
	}
	got := plan.Addresses()
	if len(got) != 2 || got[0] != loopbackAddr || got[1] != netip.MustParseAddr("172.17.0.1") {
		t.Fatalf("Addresses() = %v, want [127.0.0.1 172.17.0.1]", got)
	}
	if err := (BindPlan{addrs: []netip.Addr{netip.MustParseAddr("172.17.0.1")}}).Validate(); err == nil {
		t.Fatal("Validate() accepted a plan whose first address is not loopback")
	}
	if err := (BindPlan{wildcard: true, addrs: []netip.Addr{loopbackAddr}}).Validate(); err == nil {
		t.Fatal("Validate() accepted a wildcard plan with addresses")
	}
}

func TestServerDefaultPlanBindsLoopbackOnly(t *testing.T) {
	t.Parallel()

	server, err := New()
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	if startErr := server.Start(t.Context()); startErr != nil {
		t.Fatalf("Start(): %v", startErr)
	}
	defer testutil.MustStop(t, server)

	addrs := server.ListenAddrs()
	if len(addrs) != 1 {
		t.Fatalf("ListenAddrs() = %v, want one listener", addrs)
	}
	tcp, ok := addrs[0].(*net.TCPAddr)
	if !ok || !tcp.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Fatalf("listener = %v, want 127.0.0.1", addrs[0])
	}
	assertNonLoopbackRefused(t, server.Port().String(), nil)
}

func TestServerWildcardPlanIsSingleListener(t *testing.T) {
	t.Parallel()

	server, err := NewWithBindPlan(WildcardBindPlan())
	if err != nil {
		t.Fatalf("NewWithBindPlan(): %v", err)
	}
	if startErr := server.Start(t.Context()); startErr != nil {
		t.Fatalf("Start(): %v", startErr)
	}
	defer testutil.MustStop(t, server)

	addrs := server.ListenAddrs()
	if len(addrs) != 1 {
		t.Fatalf("ListenAddrs() = %v, want one listener", addrs)
	}
	tcp, ok := addrs[0].(*net.TCPAddr)
	if !ok || !tcp.IP.IsUnspecified() {
		t.Fatalf("listener = %v, want 0.0.0.0", addrs[0])
	}
}

func TestServerRejectsUnspecifiedAddressInRestrictedPlan(t *testing.T) {
	t.Parallel()

	plan := BindPlan{addrs: []netip.Addr{loopbackAddr, netip.IPv4Unspecified()}}
	if _, err := NewWithBindPlan(plan); !errors.Is(err, ErrInvalidBindPlan) {
		t.Fatalf("NewWithBindPlan(0.0.0.0) error = %v, want ErrInvalidBindPlan", err)
	}
}

func TestServerUnbindableSecondAddressFails(t *testing.T) {
	t.Parallel()

	// 192.0.2.1 (TEST-NET-1) is never assigned to a local interface.
	plan, err := NewBindPlan(netip.MustParseAddr("192.0.2.1"))
	if err != nil {
		t.Fatalf("NewBindPlan(): %v", err)
	}
	server, err := NewWithBindPlan(plan)
	if err != nil {
		t.Fatalf("NewWithBindPlan(): %v", err)
	}
	startErr := server.Start(t.Context())
	if startErr == nil {
		testutil.MustStop(t, server)
		t.Fatal("Start() succeeded binding 192.0.2.1")
	}
	if server.State().String() != "failed" {
		t.Fatalf("State() = %s, want failed", server.State())
	}
	if len(server.ListenAddrs()) != 0 {
		t.Fatalf("ListenAddrs() = %v after failed Start, want none", server.ListenAddrs())
	}
	if want := strconv.Itoa(maxBindAttempts) + " attempts"; !strings.Contains(startErr.Error(), want) {
		t.Fatalf("Start() error = %v, want it to mention %q", startErr, want)
	}
	if err := server.Stop(); err != nil {
		t.Fatalf("Stop(): %v", err)
	}
}

// assertNonLoopbackRefused dials every non-loopback local IPv4 address that is
// not in allowed at port and fails if any accepts. It skips with a message
// when the host has no such address.
func assertNonLoopbackRefused(t *testing.T, port string, allowed []net.IP) {
	t.Helper()
	ifaceAddrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatalf("InterfaceAddrs(): %v", err)
	}
	checked := 0
	for _, a := range ifaceAddrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.To4() == nil || ipNet.IP.IsLoopback() || containsIP(allowed, ipNet.IP) {
			continue
		}
		checked++
		dialer := net.Dialer{Timeout: time.Second}
		conn, dialErr := dialer.DialContext(t.Context(), "tcp", net.JoinHostPort(ipNet.IP.String(), port))
		if dialErr == nil {
			_ = conn.Close()
			t.Errorf("port %s accepted a connection on non-plan address %s", port, ipNet.IP)
		}
	}
	if checked == 0 {
		t.Log("no non-loopback IPv4 address on this host; exposure check skipped")
	}
}

func containsIP(ips []net.IP, ip net.IP) bool {
	for _, candidate := range ips {
		if candidate.Equal(ip) {
			return true
		}
	}
	return false
}
