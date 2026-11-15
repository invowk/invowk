// SPDX-License-Identifier: MPL-2.0

package containerplan

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func TestSelectTUIExposureRows(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("172.17.0.1")
	docker := TUINetworkFacts{Container: true, Sandbox: TUISandboxNone, Engine: TUIEngineDocker, BridgeGateway: gw, GatewayLocal: true}
	podman := TUINetworkFacts{Container: true, Sandbox: TUISandboxNone, Engine: TUIEnginePodman, Rootless: true, PastaAvailable: true}
	with := func(base TUINetworkFacts, mut func(*TUINetworkFacts)) TUINetworkFacts { mut(&base); return base }

	tests := []struct {
		name     string
		facts    TUINetworkFacts
		wantRow  TUIPlanRow
		wantKind TUIPlanKind
		wantHost string
		wantWarn []string
		noWarn   []string
	}{
		{name: "host runtime", facts: TUINetworkFacts{}, wantRow: 1, wantKind: TUIPlanNarrow, wantHost: "127.0.0.1"},
		{name: "flatpak without network beats remote", facts: with(docker, func(f *TUINetworkFacts) { f.Sandbox = TUISandboxFlatpak; f.RemoteEndpoint = true }), wantRow: 2, wantKind: TUIPlanRefused, wantWarn: []string{"private"}, noWarn: []string{"all-interfaces"}},
		{name: "flatpak with network falls through", facts: with(docker, func(f *TUINetworkFacts) { f.Sandbox = TUISandboxFlatpak; f.FlatpakNetwork = true }), wantRow: 8, wantKind: TUIPlanNarrow, wantHost: "172.17.0.1"},
		{name: "remote beats docker desktop", facts: with(docker, func(f *TUINetworkFacts) { f.RemoteEndpoint = true; f.DockerDesktop = true }), wantRow: 3, wantKind: TUIPlanRefused, wantWarn: []string{TUIOptInSetting, "leads back to this machine"}},
		{name: "remote podman", facts: with(podman, func(f *TUINetworkFacts) { f.RemoteEndpoint = true }), wantRow: 3, wantKind: TUIPlanRefused},
		{name: "query failure", facts: with(docker, func(f *TUINetworkFacts) { f.QueryError = errors.New("boom") }), wantRow: 4, wantKind: TUIPlanRefused, wantWarn: []string{"boom", TUIOptInSetting}},
		{name: "snap", facts: with(docker, func(f *TUINetworkFacts) { f.Sandbox = TUISandboxSnap }), wantRow: 5, wantKind: TUIPlanRefused, wantWarn: []string{TUIOptInSetting}},
		{name: "docker desktop beats gateway", facts: with(docker, func(f *TUINetworkFacts) { f.DockerDesktop = true }), wantRow: 6, wantKind: TUIPlanNarrow, wantHost: "host.docker.internal"},
		{name: "rootless docker", facts: with(docker, func(f *TUINetworkFacts) { f.Rootless = true }), wantRow: 7, wantKind: TUIPlanRefused},
		{name: "rootful docker", facts: docker, wantRow: 8, wantKind: TUIPlanNarrow, wantHost: "172.17.0.1"},
		{name: "rootful podman", facts: with(podman, func(f *TUINetworkFacts) { f.Rootless = false }), wantRow: 9, wantKind: TUIPlanRefused},
		{name: "persistent rootless podman", facts: with(podman, func(f *TUINetworkFacts) { f.PersistentTarget = true }), wantRow: 10, wantKind: TUIPlanRefused, wantWarn: []string{TUIOptInSetting}},
		{name: "ephemeral rootless podman with pasta", facts: podman, wantRow: 11, wantKind: TUIPlanNarrow, wantHost: "127.0.0.1"},
		{name: "rootless podman without pasta", facts: with(podman, func(f *TUINetworkFacts) { f.PastaAvailable = false }), wantRow: 12, wantKind: TUIPlanRefused, wantWarn: []string{"pasta", TUIOptInSetting}},
		{name: "gateway not local", facts: with(docker, func(f *TUINetworkFacts) { f.GatewayLocal = false }), wantRow: 13, wantKind: TUIPlanRefused, wantWarn: []string{TUIOptInSetting}},
		{name: "unknown engine", facts: TUINetworkFacts{Container: true}, wantRow: 13, wantKind: TUIPlanRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := SelectTUIExposure(tt.facts, false)
			if plan.Row() != tt.wantRow || plan.Kind() != tt.wantKind {
				t.Fatalf("plan = row %d kind %s, want row %d kind %s", plan.Row(), plan.Kind(), tt.wantRow, tt.wantKind)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("plan.Validate(): %v", err)
			}
			if tt.wantHost != "" && plan.DialHost().String() != tt.wantHost {
				t.Errorf("DialHost = %q, want %q", plan.DialHost(), tt.wantHost)
			}
			if plan.Kind() == TUIPlanNarrow {
				if len(plan.BindAddrs()) == 0 || plan.BindAddrs()[0] != netip.MustParseAddr("127.0.0.1") {
					t.Errorf("BindAddrs = %v, want loopback first", plan.BindAddrs())
				}
				if plan.Warning() != "" {
					t.Errorf("narrow plan carries warning %q", plan.Warning())
				}
			}
			if plan.Kind() == TUIPlanRefused {
				if plan.Refusal() == "" || plan.Warning() != "" {
					t.Errorf("refused plan = %+v, want an error text and no warning", plan)
				}
				refusal := plan.RefusalError()
				if !errors.Is(refusal, ErrTUIExposureRefused) || !strings.Contains(refusal.Error(), "without -i") {
					t.Errorf("RefusalError() = %v, want ErrTUIExposureRefused naming the run without -i", refusal)
				}
			} else if plan.RefusalError() != nil {
				t.Errorf("%s plan returned a refusal error", plan.Kind())
			}
			for _, want := range tt.wantWarn {
				if !strings.Contains(plan.Refusal().String(), want) {
					t.Errorf("refusal %q does not contain %q", plan.Refusal(), want)
				}
			}
			for _, bad := range tt.noWarn {
				if strings.Contains(plan.Refusal().String(), bad) {
					t.Errorf("refusal %q must not contain %q", plan.Refusal(), bad)
				}
			}
		})
	}
}

func TestSelectTUIExposureRowEightBindsGateway(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("172.17.0.1")
	plan := SelectTUIExposure(TUINetworkFacts{Container: true, Engine: TUIEngineDocker, BridgeGateway: gw, GatewayLocal: true}, false)
	if len(plan.BindAddrs()) != 2 || plan.BindAddrs()[1] != gw || plan.PastaForward() {
		t.Fatalf("row 8 plan = %+v, want loopback + gateway and no pasta", plan)
	}
	pasta := SelectTUIExposure(TUINetworkFacts{Container: true, Engine: TUIEnginePodman, Rootless: true, PastaAvailable: true}, false)
	if !pasta.PastaForward() || len(pasta.BindAddrs()) != 1 {
		t.Fatalf("row 11 plan = %+v, want loopback only with pasta forward", pasta)
	}
}

func TestSelectTUIExposureOptIn(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("172.17.0.1")
	tests := []struct {
		name      string
		facts     TUINetworkFacts
		wantKind  TUIPlanKind
		wantHost  string
		wantWarn  bool
		wantRowEq TUIPlanRow
	}{
		{name: "host runtime unchanged", facts: TUINetworkFacts{}, wantKind: TUIPlanNarrow, wantHost: "127.0.0.1", wantRowEq: 1},
		{name: "flatpak without network unchanged", facts: TUINetworkFacts{Container: true, Sandbox: TUISandboxFlatpak, Engine: TUIEngineDocker}, wantKind: TUIPlanRefused, wantWarn: true, wantRowEq: 2},
		{name: "rootful docker unchanged", facts: TUINetworkFacts{Container: true, Engine: TUIEngineDocker, BridgeGateway: gw, GatewayLocal: true}, wantKind: TUIPlanNarrow, wantHost: "172.17.0.1", wantRowEq: 8},
		{name: "pasta unchanged", facts: TUINetworkFacts{Container: true, Engine: TUIEnginePodman, Rootless: true, PastaAvailable: true}, wantKind: TUIPlanNarrow, wantHost: "127.0.0.1", wantRowEq: 11},
		{name: "docker desktop widens", facts: TUINetworkFacts{Container: true, Engine: TUIEngineDocker, DockerDesktop: true}, wantKind: TUIPlanWide, wantHost: "host.docker.internal", wantWarn: true, wantRowEq: 6},
		{name: "persistent podman widens", facts: TUINetworkFacts{Container: true, Engine: TUIEnginePodman, Rootless: true, PastaAvailable: true, PersistentTarget: true}, wantKind: TUIPlanWide, wantHost: "host.containers.internal", wantWarn: true, wantRowEq: 10},
		{name: "remote widens", facts: TUINetworkFacts{Container: true, Engine: TUIEngineDocker, RemoteEndpoint: true}, wantKind: TUIPlanWide, wantHost: "host.docker.internal", wantWarn: true, wantRowEq: 3},
		{name: "rootless docker widens", facts: TUINetworkFacts{Container: true, Engine: TUIEngineDocker, Rootless: true}, wantKind: TUIPlanWide, wantHost: "host.docker.internal", wantWarn: true, wantRowEq: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := SelectTUIExposure(tt.facts, true)
			if plan.Kind() != tt.wantKind || plan.Row() != tt.wantRowEq {
				t.Fatalf("plan = row %d kind %s, want row %d kind %s", plan.Row(), plan.Kind(), tt.wantRowEq, tt.wantKind)
			}
			if tt.wantHost != "" && plan.DialHost().String() != tt.wantHost {
				t.Errorf("DialHost = %q, want %q", plan.DialHost(), tt.wantHost)
			}
			if (plan.Warning() != "" || plan.Refusal() != "") != tt.wantWarn {
				t.Errorf("warning = %q, refusal = %q, want a message=%v", plan.Warning(), plan.Refusal(), tt.wantWarn)
			}
			if plan.Kind() == TUIPlanWide && !strings.Contains(plan.Warning().String(), "0.0.0.0") {
				t.Errorf("wide warning %q does not name 0.0.0.0", plan.Warning())
			}
		})
	}
}

func TestTUIExposurePlanValidate(t *testing.T) {
	t.Parallel()

	loopback := netip.MustParseAddr("127.0.0.1")
	gw := netip.MustParseAddr("172.17.0.1")
	tests := []struct {
		name    string
		plan    TUIExposurePlan
		wantErr bool
	}{
		{name: "narrow loopback", plan: TUIExposurePlan{row: 1, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback}, dialHost: "127.0.0.1"}},
		{name: "narrow gateway", plan: TUIExposurePlan{row: 8, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback, gw}, dialHost: "172.17.0.1"}},
		{name: "narrow pasta", plan: TUIExposurePlan{row: 11, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback}, dialHost: "127.0.0.1", pastaForward: true}},
		{name: "wide opt-in row", plan: TUIExposurePlan{row: 10, kind: TUIPlanWide, dialHost: "host.containers.internal", warning: "w"}},
		{name: "refused row", plan: TUIExposurePlan{row: 10, kind: TUIPlanRefused, refusal: "r"}},
		{name: "narrow with unspecified address", plan: TUIExposurePlan{row: 8, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback, netip.IPv4Unspecified()}, dialHost: "172.17.0.1"}, wantErr: true},
		{name: "narrow unspecified only", plan: TUIExposurePlan{row: 1, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{netip.IPv4Unspecified()}, dialHost: "127.0.0.1"}, wantErr: true},
		{name: "narrow without loopback first", plan: TUIExposurePlan{row: 8, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{gw, loopback}, dialHost: "172.17.0.1"}, wantErr: true},
		{name: "narrow without addresses", plan: TUIExposurePlan{row: 1, kind: TUIPlanNarrow, dialHost: "127.0.0.1"}, wantErr: true},
		{name: "narrow IPv6", plan: TUIExposurePlan{row: 8, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback, netip.IPv6Unspecified()}, dialHost: "x"}, wantErr: true},
		{name: "narrow duplicate", plan: TUIExposurePlan{row: 8, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback, loopback}, dialHost: "x"}, wantErr: true},
		{name: "narrow on a refused row", plan: TUIExposurePlan{row: 10, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback}, dialHost: "127.0.0.1"}, wantErr: true},
		{name: "narrow without dial host", plan: TUIExposurePlan{row: 1, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback}}, wantErr: true},
		{name: "pasta forward off the pasta row", plan: TUIExposurePlan{row: 8, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback}, dialHost: "127.0.0.1", pastaForward: true}, wantErr: true},
		{name: "narrow with a message", plan: TUIExposurePlan{row: 1, kind: TUIPlanNarrow, bindAddrs: []netip.Addr{loopback}, dialHost: "127.0.0.1", warning: "w"}, wantErr: true},
		{name: "wide for a host runtime", plan: TUIExposurePlan{row: 1, kind: TUIPlanWide, dialHost: "host.docker.internal", warning: "w"}, wantErr: true},
		{name: "wide for rootful docker", plan: TUIExposurePlan{row: 8, kind: TUIPlanWide, dialHost: "host.docker.internal", warning: "w"}, wantErr: true},
		{name: "wide for pasta", plan: TUIExposurePlan{row: 11, kind: TUIPlanWide, dialHost: "host.containers.internal", warning: "w"}, wantErr: true},
		{name: "wide for flatpak without network", plan: TUIExposurePlan{row: 2, kind: TUIPlanWide, dialHost: "host.docker.internal", warning: "w"}, wantErr: true},
		{name: "wide with addresses", plan: TUIExposurePlan{row: 10, kind: TUIPlanWide, bindAddrs: []netip.Addr{loopback}, dialHost: "x", warning: "w"}, wantErr: true},
		{name: "wide without warning", plan: TUIExposurePlan{row: 10, kind: TUIPlanWide, dialHost: "x"}, wantErr: true},
		{name: "refused on a narrow row", plan: TUIExposurePlan{row: 8, kind: TUIPlanRefused, refusal: "r"}, wantErr: true},
		{name: "refused with an address", plan: TUIExposurePlan{row: 10, kind: TUIPlanRefused, bindAddrs: []netip.Addr{loopback}, refusal: "r"}, wantErr: true},
		{name: "refused with a dial host", plan: TUIExposurePlan{row: 10, kind: TUIPlanRefused, dialHost: "x", refusal: "r"}, wantErr: true},
		{name: "refused without text", plan: TUIExposurePlan{row: 10, kind: TUIPlanRefused}, wantErr: true},
		{name: "multi-line refusal", plan: TUIExposurePlan{row: 10, kind: TUIPlanRefused, refusal: "a\nb"}, wantErr: true},
		{name: "zero value", plan: TUIExposurePlan{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.plan.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTUIExposurePlanConstructorsValidate(t *testing.T) {
	t.Parallel()

	if _, err := NewTUIExposurePlanNarrow(TUIRowDockerBridge, []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.IPv4Unspecified()}, "x", false); !errors.Is(err, ErrInvalidTUIExposurePlan) {
		t.Fatalf("NewTUIExposurePlanNarrow(0.0.0.0) error = %v, want ErrInvalidTUIExposurePlan", err)
	}
	if _, err := NewTUIExposurePlanWide(TUIRowDockerBridge, "host.docker.internal", "w"); !errors.Is(err, ErrInvalidTUIExposurePlan) {
		t.Fatalf("NewTUIExposurePlanWide(row 8) error = %v, want ErrInvalidTUIExposurePlan", err)
	}
	if _, err := NewTUIExposurePlanRefused(TUIRowHostRuntime, "r"); !errors.Is(err, ErrInvalidTUIExposurePlan) {
		t.Fatalf("NewTUIExposurePlanRefused(row 1) error = %v, want ErrInvalidTUIExposurePlan", err)
	}
	addrs := []netip.Addr{netip.MustParseAddr("127.0.0.1")}
	plan, err := NewTUIExposurePlanNarrow(TUIRowHostRuntime, addrs, "127.0.0.1", false)
	if err != nil {
		t.Fatalf("NewTUIExposurePlanNarrow(): %v", err)
	}
	addrs[0] = netip.IPv4Unspecified()
	if err := plan.Validate(); err != nil {
		t.Fatalf("plan shares the caller's slice: %v", err)
	}
}

func TestSelectTUIExposureAlwaysValid(t *testing.T) {
	t.Parallel()

	// Every combination of the boolean facts, both engines, every sandbox
	// and with or without the opt-in must yield a valid plan.
	engines := []TUIEngineKind{"", TUIEngineDocker, TUIEnginePodman}
	sandboxes := []TUISandboxKind{TUISandboxNone, TUISandboxFlatpak, TUISandboxSnap}
	gateways := []netip.Addr{{}, netip.MustParseAddr("172.17.0.1"), netip.IPv4Unspecified(), netip.MustParseAddr("127.0.0.1")}
	for mask := range 1 << 9 {
		bit := func(i int) bool { return mask&(1<<i) != 0 }
		for _, engine := range engines {
			for _, sandbox := range sandboxes {
				for _, gw := range gateways {
					f := TUINetworkFacts{
						Container: bit(0), Sandbox: sandbox, FlatpakNetwork: bit(1), Engine: engine,
						RemoteEndpoint: bit(2), DockerDesktop: bit(3), Rootless: bit(4), PastaAvailable: bit(5),
						PersistentTarget: bit(6), BridgeGateway: gw, GatewayLocal: bit(7),
					}
					plan := SelectTUIExposure(f, bit(8))
					if err := plan.Validate(); err != nil {
						t.Fatalf("SelectTUIExposure(%+v, %v) = %+v, invalid: %v", f, bit(8), plan, err)
					}
					if plan.Kind() == TUIPlanWide && !bit(8) {
						t.Fatalf("SelectTUIExposure(%+v) selected the wide plan without the opt-in", f)
					}
				}
			}
		}
	}
}
