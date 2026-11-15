// SPDX-License-Identifier: MPL-2.0

//go:build linux

package commandadapters

import (
	"bytes"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"

	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// localNonLoopbackIPv4 returns an IPv4 address assigned to a local
// interface that is not loopback, or skips the test when there is none.
func localNonLoopbackIPv4(t *testing.T) netip.Addr {
	t.Helper()
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("list interface addresses: %v", err)
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if ok && addr.Unmap().Is4() && !addr.IsLoopback() && !addr.IsLinkLocalUnicast() {
			return addr.Unmap()
		}
	}
	t.Skip("no local non-loopback IPv4 address")
	return netip.Addr{}
}

// TestInteractiveExecutorGatewayPlanUsesIPLiteral uses a local non-loopback
// IPv4 address as a stand-in bridge gateway for a row 8 plan. Row 8 requires
// a non-loopback gateway (decision 34), so 127.0.0.2 no longer qualifies.
func TestInteractiveExecutorGatewayPlanUsesIPLiteral(t *testing.T) {
	t.Parallel()

	gw := localNonLoopbackIPv4(t)
	plan, err := containerplan.NewTUIExposurePlanNarrow(containerplan.TUIRowDockerBridge,
		[]netip.Addr{netip.MustParseAddr("127.0.0.1"), gw}, types.HostServiceAddress(gw.String()), false)
	if err != nil {
		t.Fatalf("NewTUIExposurePlanNarrow(): %v", err)
	}
	rt := &spyPlannerRuntime{plan: plan}
	rt.probe = func(tuiCtx runtime.TUIContext) {
		for _, host := range []string{"127.0.0.1", gw.String()} {
			if code := authedHealth(t, tuiCtx, host); code != http.StatusOK {
				t.Errorf("authenticated /health on %s = %d, want 200", host, code)
			}
		}
	}
	result := InteractiveExecutor{runSession: (&fakeSession{}).run}.Execute(newExposureTestContext(t, &bytes.Buffer{}), invowkfile.CommandName("probe"), rt)
	if result.Error != nil {
		t.Fatalf("Execute(): %v", result.Error)
	}
	u, err := url.Parse(string(rt.prepareTUI.ServerURL))
	if err != nil || u.Hostname() != gw.String() {
		t.Fatalf("ServerURL = %q, want the gateway IP literal", rt.prepareTUI.ServerURL)
	}
	if rt.prepareTUI.NetworkMode != "" {
		t.Fatalf("NetworkMode = %q, want none for a bridge plan", rt.prepareTUI.NetworkMode)
	}
}
