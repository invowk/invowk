// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/pkg/platform"
)

func stubTUIProbe(facts container.EngineNetworkFacts, err error, calls *int) tuiExposureProbe {
	return tuiExposureProbe{
		queryFacts: func(context.Context, container.NamedEngine) (container.EngineNetworkFacts, error) {
			*calls++
			return facts, err
		},
		sandbox:        func() platform.SandboxType { return platform.SandboxNone },
		flatpakNetwork: func() bool { return false },
		interfaceAddrs: func() ([]netip.Prefix, error) {
			return []netip.Prefix{netip.MustParsePrefix("127.0.0.1/8"), netip.MustParsePrefix("172.17.0.1/16")}, nil
		},
	}
}

func TestContainerRuntimePlanTUIExposure(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("172.17.0.1")
	podman := container.EngineNetworkFacts{Engine: container.EngineTypePodman, Rootless: true, PastaAvailable: true}
	tests := []struct {
		name       string
		facts      container.EngineNetworkFacts
		err        error
		bind       config.TUIServerBind
		persistent bool
		wantRow    containerplan.TUIPlanRow
		wantKind   containerplan.TUIPlanKind
	}{
		{name: "rootful docker", facts: container.EngineNetworkFacts{Engine: container.EngineTypeDocker, BridgeGateway: gw}, wantRow: 8, wantKind: containerplan.TUIPlanNarrow},
		{name: "gateway not local", facts: container.EngineNetworkFacts{Engine: container.EngineTypeDocker, BridgeGateway: netip.MustParseAddr("10.9.0.1")}, wantRow: 13, wantKind: containerplan.TUIPlanNone},
		{name: "pasta", facts: podman, wantRow: 11, wantKind: containerplan.TUIPlanNarrow},
		{name: "persistent podman", facts: podman, persistent: true, wantRow: 10, wantKind: containerplan.TUIPlanNone},
		{name: "persistent podman with opt-in", facts: podman, persistent: true, bind: config.TUIServerBindAllInterfaces, wantRow: 10, wantKind: containerplan.TUIPlanWide},
		{name: "query failure", err: errors.New("info failed"), wantRow: 4, wantKind: containerplan.TUIPlanNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			rt := newPersistentTestRuntime(t, NewMockEngine())
			rt.tuiProbe = stubTUIProbe(tt.facts, tt.err, &calls)
			cfg := config.DefaultConfig()
			if tt.bind != "" {
				cfg.Container.TUIServerBind = tt.bind
			}
			rt.cfg = cfg
			ctx := newPersistentExecutionContext(t, nil)
			if tt.persistent {
				ctx.ContainerNameOverride = "dev"
			}
			plan := rt.PlanTUIExposure(ctx)
			if plan.Row != tt.wantRow || plan.Kind != tt.wantKind {
				t.Fatalf("plan = row %d kind %s, want row %d kind %s", plan.Row, plan.Kind, tt.wantRow, tt.wantKind)
			}
			_ = rt.PlanTUIExposure(ctx)
			if calls != 1 {
				t.Fatalf("facts queried %d times, want once (cached)", calls)
			}
		})
	}
}

// TestContainerRuntimeConstructionQueriesNoFacts pins D11: building a runtime
// and running non-interactively never asks for TUI facts.
func TestContainerRuntimeConstructionQueriesNoFacts(t *testing.T) {
	t.Parallel()

	calls := 0
	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	rt.tuiProbe = stubTUIProbe(container.EngineNetworkFacts{}, nil, &calls)
	_ = rt.Execute(newPersistentExecutionContext(t, nil))
	if calls != 0 {
		t.Fatalf("non-interactive Execute queried TUI facts %d times, want 0", calls)
	}
	if len(engine.RunCalls) > 0 && engine.RunCalls[0].NetworkMode != "" {
		t.Fatalf("non-interactive run got network mode %q", engine.RunCalls[0].NetworkMode)
	}
}

// TestContainerPrepareCommandTUINetworkMode pins that the plan's network mode
// reaches a new container run, and that an empty TUI context injects no TUI
// variables and no host-gateway mapping.
func TestContainerPrepareCommandTUINetworkMode(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt := newPersistentTestRuntime(t, engine)
	ctx := newPersistentExecutionContext(t, nil)
	ctx.TUI = TUIContext{ServerURL: "http://127.0.0.1:43123", ServerToken: "tok", NetworkMode: "pasta:-T,43123"}
	prepared, err := rt.PrepareCommand(ctx)
	if err != nil {
		t.Fatalf("PrepareCommand(): %v", err)
	}
	prepared.Cleanup()

	noServer := newPersistentExecutionContext(t, nil)
	prepared, err = rt.PrepareCommand(noServer)
	if err != nil {
		t.Fatalf("PrepareCommand(no server): %v", err)
	}
	prepared.Cleanup()

	if len(engine.PrepareRunCalls) != 2 {
		t.Fatalf("PrepareRunCommand calls = %d, want 2", len(engine.PrepareRunCalls))
	}
	if got := engine.PrepareRunCalls[0].NetworkMode; got != "pasta:-T,43123" {
		t.Fatalf("NetworkMode = %q, want pasta:-T,43123", got)
	}
	second := engine.PrepareRunCalls[1]
	if second.NetworkMode != "" || len(second.ExtraHosts) != 0 {
		t.Fatalf("no-server run got network %q and hosts %v, want neither", second.NetworkMode, second.ExtraHosts)
	}
	for _, key := range []string{"INVOWK_TUI_ADDR", "INVOWK_TUI_TOKEN"} {
		if _, ok := second.Env[key]; ok {
			t.Fatalf("no-server run injected %s", key)
		}
	}
}
