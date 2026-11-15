// SPDX-License-Identifier: MPL-2.0

package containerplan

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

const (
	// TUIRowHostRuntime is D2 table row 1.
	TUIRowHostRuntime TUIPlanRow = 1
	// TUIRowFlatpakNoNetwork is D2 table row 2.
	TUIRowFlatpakNoNetwork TUIPlanRow = 2
	// TUIRowRemoteEngine is D2 table row 3.
	TUIRowRemoteEngine TUIPlanRow = 3
	// TUIRowQueryFailed is D2 table row 4.
	TUIRowQueryFailed TUIPlanRow = 4
	// TUIRowSnap is D2 table row 5.
	TUIRowSnap TUIPlanRow = 5
	// TUIRowDockerDesktop is D2 table row 6.
	TUIRowDockerDesktop TUIPlanRow = 6
	// TUIRowDockerRootless is D2 table row 7.
	TUIRowDockerRootless TUIPlanRow = 7
	// TUIRowDockerBridge is D2 table row 8.
	TUIRowDockerBridge TUIPlanRow = 8
	// TUIRowPodmanRootful is D2 table row 9.
	TUIRowPodmanRootful TUIPlanRow = 9
	// TUIRowPodmanPersistent is D2 table row 10.
	TUIRowPodmanPersistent TUIPlanRow = 10
	// TUIRowPodmanPasta is D2 table row 11.
	TUIRowPodmanPasta TUIPlanRow = 11
	// TUIRowPodmanNoPasta is D2 table row 12.
	TUIRowPodmanNoPasta TUIPlanRow = 12
	// TUIRowUnknown is D2 table row 13.
	TUIRowUnknown TUIPlanRow = 13

	// TUIPlanNarrow binds loopback plus, at most, one verified host route.
	TUIPlanNarrow TUIPlanKind = "narrow"
	// TUIPlanWide binds 0.0.0.0; reachable only through the user opt-in.
	TUIPlanWide TUIPlanKind = "wide"
	// TUIPlanNone starts no TUI server; the command renders TUI calls directly.
	TUIPlanNone TUIPlanKind = "none"

	// TUIEngineDocker identifies Docker (including Docker Desktop).
	TUIEngineDocker TUIEngineKind = "docker"
	// TUIEnginePodman identifies Podman.
	TUIEnginePodman TUIEngineKind = "podman"

	// TUISandboxNone means invowk runs directly on the host.
	TUISandboxNone TUISandboxKind = "none"
	// TUISandboxFlatpak means invowk runs inside Flatpak.
	TUISandboxFlatpak TUISandboxKind = "flatpak"
	// TUISandboxSnap means invowk runs inside Snap.
	TUISandboxSnap TUISandboxKind = "snap"

	// TUIOptInSetting is the config setting every restorable warning names.
	TUIOptInSetting = `container.tui_server_bind: "all-interfaces"`

	dockerHostAlias types.HostServiceAddress = "host.docker.internal"
	podmanHostAlias types.HostServiceAddress = "host.containers.internal"
	loopbackHost    types.HostServiceAddress = "127.0.0.1"
)

// ErrInvalidTUIExposureFacts is returned when TUINetworkFacts are inconsistent.
var ErrInvalidTUIExposureFacts = errors.New("invalid TUI exposure facts")

type (
	// TUIPlanKind says whether a run gets a narrow server, a wide server or none.
	TUIPlanKind string

	// TUIEngineKind identifies the container engine family behind the facts.
	TUIEngineKind string

	// TUISandboxKind identifies the sandbox invowk itself runs in.
	TUISandboxKind string

	// TUIPlanRow is the 1-based row of the D2 table in the
	// restrict-tui-server-bind design that selected a plan.
	TUIPlanRow int

	// TUIExposureWarning is the one-line stderr warning a plan asks for.
	TUIExposureWarning string

	// TUINetworkFacts are the engine and sandbox facts behind a plan. The
	// zero value describes a host runtime (row 1).
	TUINetworkFacts struct {
		// Container is false for native, virtual-sh and virtual-lua runs.
		Container bool
		// Sandbox is the sandbox invowk runs in.
		Sandbox TUISandboxKind
		// FlatpakNetwork reports whether the Flatpak shares the host network.
		FlatpakNetwork bool
		// QueryError is the failure of the engine facts query, if any.
		QueryError error
		// Engine is the engine family.
		Engine TUIEngineKind
		// RemoteEndpoint reports a tcp:// or ssh:// Docker endpoint, or a
		// remote Podman service (podman-remote, --remote, Podman machine).
		RemoteEndpoint bool
		// DockerDesktop reports Docker's OperatingSystem "Docker Desktop".
		DockerDesktop bool
		// Rootless reports a rootless engine.
		Rootless bool
		// PastaAvailable reports a non-empty Podman Pasta.Executable.
		PastaAvailable bool
		// PersistentTarget reports that the run execs into a persistent container.
		PersistentTarget bool
		// BridgeGateway is Docker's default bridge gateway, when known.
		BridgeGateway netip.Addr
		// GatewayLocal reports that BridgeGateway is on a local interface.
		GatewayLocal bool
	}

	// TUIExposurePlan is the bind plan for one interactive run.
	TUIExposurePlan struct {
		// Row is the D2 table row that matched.
		Row TUIPlanRow
		// Kind selects narrow, wide or no server.
		Kind TUIPlanKind
		// BindAddrs are the narrow plan's addresses, loopback first.
		BindAddrs []netip.Addr
		// DialHost is the host the command dials (IP literal or engine alias).
		DialHost types.HostServiceAddress
		// PastaForward asks for --network=pasta:-T,<port> on the container run.
		PastaForward bool
		// Warning is printed once to stderr before the run when non-empty.
		Warning TUIExposureWarning
	}
)

// String returns the kind name.
func (k TUIPlanKind) String() string { return string(k) }

// Validate returns an error for an unknown plan kind.
func (k TUIPlanKind) Validate() error {
	switch k {
	case TUIPlanNarrow, TUIPlanWide, TUIPlanNone:
		return nil
	default:
		return fmt.Errorf("%w: plan kind %q", ErrInvalidTUIExposureFacts, string(k))
	}
}

// String returns the engine name.
func (k TUIEngineKind) String() string { return string(k) }

// Validate returns an error for an unknown engine kind; empty means unknown.
func (k TUIEngineKind) Validate() error {
	switch k {
	case "", TUIEngineDocker, TUIEnginePodman:
		return nil
	default:
		return fmt.Errorf("%w: engine %q", ErrInvalidTUIExposureFacts, string(k))
	}
}

// String returns the sandbox name.
func (k TUISandboxKind) String() string { return string(k) }

// Validate returns an error for an unknown sandbox kind; empty means none.
func (k TUISandboxKind) Validate() error {
	switch k {
	case "", TUISandboxNone, TUISandboxFlatpak, TUISandboxSnap:
		return nil
	default:
		return fmt.Errorf("%w: sandbox %q", ErrInvalidTUIExposureFacts, string(k))
	}
}

// String returns the row number.
func (r TUIPlanRow) String() string { return strconv.Itoa(int(r)) }

// Validate returns an error for a row outside the D2 table.
func (r TUIPlanRow) Validate() error {
	if r < TUIRowHostRuntime || r > TUIRowUnknown {
		return fmt.Errorf("%w: plan row %d", ErrInvalidTUIExposureFacts, int(r))
	}
	return nil
}

// String returns the warning text.
func (w TUIExposureWarning) String() string { return string(w) }

// Validate rejects multi-line text: a plan's warning is one stderr line.
func (w TUIExposureWarning) Validate() error {
	if strings.ContainsAny(string(w), "\r\n") {
		return fmt.Errorf("%w: warning must be one line", ErrInvalidTUIExposureFacts)
	}
	return nil
}

// Validate checks the enum fields of the facts.
func (f TUINetworkFacts) Validate() error {
	if err := f.Engine.Validate(); err != nil {
		return err
	}
	return f.Sandbox.Validate()
}

// Validate checks the plan's internal consistency.
func (p TUIExposurePlan) Validate() error {
	if err := p.Row.Validate(); err != nil {
		return err
	}
	if err := p.Kind.Validate(); err != nil {
		return err
	}
	if err := p.Warning.Validate(); err != nil {
		return err
	}
	if p.Kind != TUIPlanNone {
		if err := p.DialHost.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// HasServer reports whether the plan starts a TUI server.
func (p TUIExposurePlan) HasServer() bool {
	return p.Kind != TUIPlanNone
}

// SelectTUIExposure evaluates the D2 table top to bottom; the first match
// wins. allowAllInterfaces is the user opt-in from container.tui_server_bind;
// it widens only restorable rows and Docker Desktop (row 6), never a host
// runtime, a verified narrow plan (rows 8, 11) or Flatpak without network.
func SelectTUIExposure(f TUINetworkFacts, allowAllInterfaces bool) TUIExposurePlan {
	loopback := netip.AddrFrom4([4]byte{127, 0, 0, 1})
	restorable := func(row TUIPlanRow, reason string) TUIExposurePlan {
		if allowAllInterfaces {
			return wideTUIPlan(row, f.Engine)
		}
		return TUIExposurePlan{Row: row, Kind: TUIPlanNone, Warning: tuiWarning(
			"invowk: interactive TUI overlay disabled for this container run: " + reason +
				"; invowk tui commands render directly in the command's terminal. To restore the wide 0.0.0.0 bind knowingly, set " +
				TUIOptInSetting + " in your invowk config")}
	}

	switch {
	case !f.Container:
		return TUIExposurePlan{Row: TUIRowHostRuntime, Kind: TUIPlanNarrow, BindAddrs: []netip.Addr{loopback}, DialHost: loopbackHost}
	case f.Sandbox == TUISandboxFlatpak && !f.FlatpakNetwork:
		return TUIExposurePlan{Row: TUIRowFlatpakNoNetwork, Kind: TUIPlanNone, Warning: "invowk: interactive TUI overlay disabled for this container run: " +
			"the Flatpak sandbox's network is private, so no TUI server bind is reachable from the engine's containers; " +
			"invowk tui commands render directly in the command's terminal (grant --share=network to restore the overlay)"}
	case f.RemoteEndpoint:
		return restorable(TUIRowRemoteEngine, "the container engine is remote, so its containers cannot reach a loopback-bound server "+
			"(the opt-in below helps only if the engine's host alias leads back to this machine)")
	case f.QueryError != nil:
		return restorable(TUIRowQueryFailed, "the container engine's network facts could not be read ("+f.QueryError.Error()+")")
	case f.Sandbox == TUISandboxSnap:
		return restorable(TUIRowSnap, "no narrow TUI server route is verified for Snap")
	case f.Engine == TUIEngineDocker && f.DockerDesktop:
		if allowAllInterfaces {
			return wideTUIPlan(TUIRowDockerDesktop, f.Engine)
		}
		return TUIExposurePlan{Row: TUIRowDockerDesktop, Kind: TUIPlanNarrow, BindAddrs: []netip.Addr{loopback}, DialHost: dockerHostAlias}
	case f.Engine == TUIEngineDocker && f.Rootless:
		return restorable(TUIRowDockerRootless, "no narrow TUI server route is verified for rootless Docker")
	case f.Engine == TUIEngineDocker && f.BridgeGateway.Is4() && f.GatewayLocal && !f.BridgeGateway.IsUnspecified() && !f.BridgeGateway.IsLoopback():
		return TUIExposurePlan{
			Row: TUIRowDockerBridge, Kind: TUIPlanNarrow,
			BindAddrs: []netip.Addr{loopback, f.BridgeGateway},
			DialHost:  gatewayDialHost(f.BridgeGateway),
		}
	case f.Engine == TUIEnginePodman && !f.Rootless:
		return restorable(TUIRowPodmanRootful, "no narrow TUI server route is verified for rootful Podman")
	case f.Engine == TUIEnginePodman && f.PersistentTarget:
		return restorable(TUIRowPodmanPersistent, "a persistent rootless Podman container has a fixed network, so it cannot get a per-run forwarded port")
	case f.Engine == TUIEnginePodman && f.PastaAvailable:
		return TUIExposurePlan{Row: TUIRowPodmanPasta, Kind: TUIPlanNarrow, BindAddrs: []netip.Addr{loopback}, DialHost: loopbackHost, PastaForward: true}
	case f.Engine == TUIEnginePodman:
		return restorable(TUIRowPodmanNoPasta, "rootless Podman has no pasta binary for a single forwarded port (install pasta, the passt package, to restore the overlay)")
	default:
		return restorable(TUIRowUnknown, "no narrow TUI server route is known for this engine mode (for example, the bridge gateway is not on a local interface)")
	}
}

func wideTUIPlan(row TUIPlanRow, engine TUIEngineKind) TUIExposurePlan {
	alias := dockerHostAlias
	if engine == TUIEnginePodman {
		alias = podmanHostAlias
	}
	return TUIExposurePlan{Row: row, Kind: TUIPlanWide, DialHost: alias, Warning: tuiWarning(
		"invowk: warning: " + TUIOptInSetting + " is set, so the TUI server listens on 0.0.0.0 and its port is reachable from the network " +
			"(guarded by the per-session token)")}
}

// gatewayDialHost returns the gateway IP literal as the dial host.
func gatewayDialHost(gateway netip.Addr) types.HostServiceAddress {
	raw := gateway.String()
	host := types.HostServiceAddress(raw)
	if err := host.Validate(); err != nil {
		return loopbackHost
	}
	return host
}

// tuiWarning returns text as a validated one-line warning; multi-line text
// (a defect in a reason string) is folded onto one line.
//
//goplint:ignore -- composed message text, validated as TUIExposureWarning here.
func tuiWarning(text string) TUIExposureWarning {
	raw := strings.NewReplacer("\r", " ", "\n", " ").Replace(text)
	warning := TUIExposureWarning(raw)
	if err := warning.Validate(); err != nil {
		return ""
	}
	return warning
}
