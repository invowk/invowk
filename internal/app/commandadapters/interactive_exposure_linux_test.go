// SPDX-License-Identifier: MPL-2.0

//go:build linux

package commandadapters

import (
	"bytes"
	"net/http"
	"net/netip"
	"net/url"
	"testing"

	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestInteractiveExecutorGatewayPlanUsesIPLiteral uses 127.0.0.2 (routed to lo
// on Linux) as a stand-in bridge gateway for a row 8 plan.
func TestInteractiveExecutorGatewayPlanUsesIPLiteral(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("127.0.0.2")
	plan, err := containerplan.NewTUIExposurePlanNarrow(containerplan.TUIRowDockerBridge,
		[]netip.Addr{netip.MustParseAddr("127.0.0.1"), gw}, "127.0.0.2", false)
	if err != nil {
		t.Fatalf("NewTUIExposurePlanNarrow(): %v", err)
	}
	rt := &spyPlannerRuntime{plan: plan}
	rt.probe = func(tuiCtx runtime.TUIContext) {
		for _, host := range []string{"127.0.0.1", "127.0.0.2"} {
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
	if err != nil || u.Hostname() != "127.0.0.2" {
		t.Fatalf("ServerURL = %q, want the gateway IP literal", rt.prepareTUI.ServerURL)
	}
	if rt.prepareTUI.NetworkMode != "" {
		t.Fatalf("NetworkMode = %q, want none for a bridge plan", rt.prepareTUI.NetworkMode)
	}
}
