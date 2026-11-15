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
		{name: "flatpak without network beats remote", facts: with(docker, func(f *TUINetworkFacts) { f.Sandbox = TUISandboxFlatpak; f.RemoteEndpoint = true }), wantRow: 2, wantKind: TUIPlanNone, wantWarn: []string{"private"}, noWarn: []string{"all-interfaces"}},
		{name: "flatpak with network falls through", facts: with(docker, func(f *TUINetworkFacts) { f.Sandbox = TUISandboxFlatpak; f.FlatpakNetwork = true }), wantRow: 8, wantKind: TUIPlanNarrow, wantHost: "172.17.0.1"},
		{name: "remote beats docker desktop", facts: with(docker, func(f *TUINetworkFacts) { f.RemoteEndpoint = true; f.DockerDesktop = true }), wantRow: 3, wantKind: TUIPlanNone, wantWarn: []string{TUIOptInSetting, "leads back to this machine"}},
		{name: "remote podman", facts: with(podman, func(f *TUINetworkFacts) { f.RemoteEndpoint = true }), wantRow: 3, wantKind: TUIPlanNone},
		{name: "query failure", facts: with(docker, func(f *TUINetworkFacts) { f.QueryError = errors.New("boom") }), wantRow: 4, wantKind: TUIPlanNone, wantWarn: []string{"boom", TUIOptInSetting}},
		{name: "snap", facts: with(docker, func(f *TUINetworkFacts) { f.Sandbox = TUISandboxSnap }), wantRow: 5, wantKind: TUIPlanNone, wantWarn: []string{TUIOptInSetting}},
		{name: "docker desktop beats gateway", facts: with(docker, func(f *TUINetworkFacts) { f.DockerDesktop = true }), wantRow: 6, wantKind: TUIPlanNarrow, wantHost: "host.docker.internal"},
		{name: "rootless docker", facts: with(docker, func(f *TUINetworkFacts) { f.Rootless = true }), wantRow: 7, wantKind: TUIPlanNone},
		{name: "rootful docker", facts: docker, wantRow: 8, wantKind: TUIPlanNarrow, wantHost: "172.17.0.1"},
		{name: "rootful podman", facts: with(podman, func(f *TUINetworkFacts) { f.Rootless = false }), wantRow: 9, wantKind: TUIPlanNone},
		{name: "persistent rootless podman", facts: with(podman, func(f *TUINetworkFacts) { f.PersistentTarget = true }), wantRow: 10, wantKind: TUIPlanNone, wantWarn: []string{TUIOptInSetting}},
		{name: "ephemeral rootless podman with pasta", facts: podman, wantRow: 11, wantKind: TUIPlanNarrow, wantHost: "127.0.0.1"},
		{name: "rootless podman without pasta", facts: with(podman, func(f *TUINetworkFacts) { f.PastaAvailable = false }), wantRow: 12, wantKind: TUIPlanNone, wantWarn: []string{"pasta", TUIOptInSetting}},
		{name: "gateway not local", facts: with(docker, func(f *TUINetworkFacts) { f.GatewayLocal = false }), wantRow: 13, wantKind: TUIPlanNone, wantWarn: []string{TUIOptInSetting}},
		{name: "unknown engine", facts: TUINetworkFacts{Container: true}, wantRow: 13, wantKind: TUIPlanNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := SelectTUIExposure(tt.facts, false)
			if plan.Row != tt.wantRow || plan.Kind != tt.wantKind {
				t.Fatalf("plan = row %d kind %s, want row %d kind %s", plan.Row, plan.Kind, tt.wantRow, tt.wantKind)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("plan.Validate(): %v", err)
			}
			if tt.wantHost != "" && plan.DialHost.String() != tt.wantHost {
				t.Errorf("DialHost = %q, want %q", plan.DialHost, tt.wantHost)
			}
			if plan.Kind == TUIPlanNarrow {
				if len(plan.BindAddrs) == 0 || plan.BindAddrs[0] != netip.MustParseAddr("127.0.0.1") {
					t.Errorf("BindAddrs = %v, want loopback first", plan.BindAddrs)
				}
				if plan.Warning != "" {
					t.Errorf("narrow plan carries warning %q", plan.Warning)
				}
			}
			if plan.Kind == TUIPlanNone && plan.Warning == "" {
				t.Error("no-server plan has no warning")
			}
			for _, want := range tt.wantWarn {
				if !strings.Contains(plan.Warning.String(), want) {
					t.Errorf("warning %q does not contain %q", plan.Warning, want)
				}
			}
			for _, bad := range tt.noWarn {
				if strings.Contains(plan.Warning.String(), bad) {
					t.Errorf("warning %q must not contain %q", plan.Warning, bad)
				}
			}
		})
	}
}

func TestSelectTUIExposureRowEightBindsGateway(t *testing.T) {
	t.Parallel()

	gw := netip.MustParseAddr("172.17.0.1")
	plan := SelectTUIExposure(TUINetworkFacts{Container: true, Engine: TUIEngineDocker, BridgeGateway: gw, GatewayLocal: true}, false)
	if len(plan.BindAddrs) != 2 || plan.BindAddrs[1] != gw || plan.PastaForward {
		t.Fatalf("row 8 plan = %+v, want loopback + gateway and no pasta", plan)
	}
	pasta := SelectTUIExposure(TUINetworkFacts{Container: true, Engine: TUIEnginePodman, Rootless: true, PastaAvailable: true}, false)
	if !pasta.PastaForward || len(pasta.BindAddrs) != 1 {
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
		{name: "flatpak without network unchanged", facts: TUINetworkFacts{Container: true, Sandbox: TUISandboxFlatpak, Engine: TUIEngineDocker}, wantKind: TUIPlanNone, wantWarn: true, wantRowEq: 2},
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
			if plan.Kind != tt.wantKind || plan.Row != tt.wantRowEq {
				t.Fatalf("plan = row %d kind %s, want row %d kind %s", plan.Row, plan.Kind, tt.wantRowEq, tt.wantKind)
			}
			if tt.wantHost != "" && plan.DialHost.String() != tt.wantHost {
				t.Errorf("DialHost = %q, want %q", plan.DialHost, tt.wantHost)
			}
			if (plan.Warning != "") != tt.wantWarn {
				t.Errorf("warning = %q, want present=%v", plan.Warning, tt.wantWarn)
			}
			if plan.Kind == TUIPlanWide && !strings.Contains(plan.Warning.String(), "0.0.0.0") {
				t.Errorf("wide warning %q does not name 0.0.0.0", plan.Warning)
			}
		})
	}
}
