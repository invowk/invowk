// SPDX-License-Identifier: MPL-2.0

package containerplan

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
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
	// TUIPlanRefused starts no TUI server and fails the interactive run
	// before start: direct rendering is not a usable fallback, because a
	// captured value component (name=$(invowk tui input)) renders into the
	// capture instead of the terminal.
	TUIPlanRefused TUIPlanKind = "refused"

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

	refusedPrefix = "interactive container run refused: "
)

var (
	// ErrInvalidTUIExposureFacts is returned when TUINetworkFacts are inconsistent.
	ErrInvalidTUIExposureFacts = errors.New("invalid TUI exposure facts")

	// ErrInvalidTUIExposurePlan is returned when a TUIExposurePlan breaks the
	// bind invariants of its kind (restrict-tui-server-bind D4, decision 25).
	ErrInvalidTUIExposurePlan = errors.New("invalid TUI exposure plan")

	// ErrTUIExposureRefused is wrapped by the error of a refused plan.
	ErrTUIExposureRefused = errors.New("interactive container run refused")

	// loopbackAddr is the address every narrow plan lists first.
	loopbackAddr = netip.AddrFrom4([4]byte{127, 0, 0, 1})

	// narrowRows are the D2 rows with a narrow plan.
	narrowRows = []TUIPlanRow{TUIRowHostRuntime, TUIRowDockerDesktop, TUIRowDockerBridge, TUIRowPodmanPasta}
	// optInRows are the rows container.tui_server_bind "all-interfaces" widens.
	optInRows = []TUIPlanRow{
		TUIRowRemoteEngine, TUIRowQueryFailed, TUIRowSnap, TUIRowDockerDesktop, TUIRowDockerRootless,
		TUIRowPodmanRootful, TUIRowPodmanPersistent, TUIRowPodmanNoPasta, TUIRowUnknown,
	}
	// refusedRows are the rows without a narrow plan.
	refusedRows = []TUIPlanRow{
		TUIRowFlatpakNoNetwork, TUIRowRemoteEngine, TUIRowQueryFailed, TUIRowSnap, TUIRowDockerRootless,
		TUIRowPodmanRootful, TUIRowPodmanPersistent, TUIRowPodmanNoPasta, TUIRowUnknown,
	}
)

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

	// TUIExposureMessage is a one-line message of a plan: the exposure
	// warning of a wide plan, or the error of a refused plan.
	TUIExposureMessage string

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
		// PodmanBackend reports a Docker CLI talking to Podman's
		// Docker-compatible API (rootful; rootless reports Rootless).
		PodmanBackend bool
	}

	// TUIExposurePlan is the bind plan for one interactive run. Build it with
	// NewTUIExposurePlanNarrow, NewTUIExposurePlanWide or
	// NewTUIExposurePlanRefused; Validate enforces each kind's shape.
	TUIExposurePlan struct {
		// row is the D2 table row that matched.
		row TUIPlanRow
		// kind selects narrow, wide or refused.
		kind TUIPlanKind
		// bindAddrs are the narrow plan's addresses, loopback first.
		bindAddrs []netip.Addr
		// dialHost is the host the command dials (IP literal or engine alias).
		dialHost types.HostServiceAddress
		// pastaForward asks for --network=pasta:-T,<port> on the container run.
		pastaForward bool
		// warning is the wide plan's exposure warning, printed once to stderr.
		warning TUIExposureMessage
		// refusal is the refused plan's error text.
		refusal TUIExposureMessage
	}

	// TUIExposureRefusedError is returned for an interactive run whose plan
	// is refused.
	TUIExposureRefusedError struct {
		Row     TUIPlanRow
		Message TUIExposureMessage
	}
)

// String returns the kind name.
func (k TUIPlanKind) String() string { return string(k) }

// Validate returns an error for an unknown plan kind.
func (k TUIPlanKind) Validate() error {
	switch k {
	case TUIPlanNarrow, TUIPlanWide, TUIPlanRefused:
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

// String returns the message text.
func (m TUIExposureMessage) String() string { return string(m) }

// Validate rejects multi-line text: a plan's message is one stderr line.
func (m TUIExposureMessage) Validate() error {
	if strings.ContainsAny(string(m), "\r\n") {
		return fmt.Errorf("%w: message must be one line", ErrInvalidTUIExposurePlan)
	}
	return nil
}

// Error implements error.
func (e *TUIExposureRefusedError) Error() string { return e.Message.String() }

// Unwrap returns ErrTUIExposureRefused.
func (e *TUIExposureRefusedError) Unwrap() error { return ErrTUIExposureRefused }

// Validate checks the enum fields of the facts.
func (f TUINetworkFacts) Validate() error {
	if err := f.Engine.Validate(); err != nil {
		return err
	}
	return f.Sandbox.Validate()
}

// NewTUIExposurePlanNarrow returns a validated narrow plan: loopback first,
// then at most the verified host route, and the host the command dials.
func NewTUIExposurePlanNarrow(row TUIPlanRow, bindAddrs []netip.Addr, dialHost types.HostServiceAddress, pastaForward bool) (TUIExposurePlan, error) {
	plan := TUIExposurePlan{row: row, kind: TUIPlanNarrow, bindAddrs: slices.Clone(bindAddrs), dialHost: dialHost, pastaForward: pastaForward}
	if err := plan.Validate(); err != nil {
		return TUIExposurePlan{}, err
	}
	return plan, nil
}

// NewTUIExposurePlanWide returns a validated wildcard plan; only an opt-in
// row may carry one.
func NewTUIExposurePlanWide(row TUIPlanRow, alias types.HostServiceAddress, warning TUIExposureMessage) (TUIExposurePlan, error) {
	plan := TUIExposurePlan{row: row, kind: TUIPlanWide, dialHost: alias, warning: warning}
	if err := plan.Validate(); err != nil {
		return TUIExposurePlan{}, err
	}
	return plan, nil
}

// NewTUIExposurePlanRefused returns a validated refused plan.
func NewTUIExposurePlanRefused(row TUIPlanRow, refusal TUIExposureMessage) (TUIExposurePlan, error) {
	plan := TUIExposurePlan{row: row, kind: TUIPlanRefused, refusal: refusal}
	if err := plan.Validate(); err != nil {
		return TUIExposurePlan{}, err
	}
	return plan, nil
}

// Validate checks the plan against the bind invariants of its kind: a narrow
// plan lists 127.0.0.1 first and only specified IPv4 addresses; a wildcard
// plan exists only on an opt-in row; a refused plan carries no address.
func (p TUIExposurePlan) Validate() error {
	if err := p.row.Validate(); err != nil {
		return invalidPlan(err.Error())
	}
	if err := p.kind.Validate(); err != nil {
		return invalidPlan(err.Error())
	}
	if err := p.warning.Validate(); err != nil {
		return err
	}
	if err := p.refusal.Validate(); err != nil {
		return err
	}
	if p.dialHost != "" {
		if err := p.dialHost.Validate(); err != nil {
			return invalidPlan(err.Error())
		}
	}
	switch p.kind {
	case TUIPlanNarrow:
		return p.validateNarrow()
	case TUIPlanWide:
		return p.validateWide()
	case TUIPlanRefused:
		return p.validateRefused()
	default:
		return invalidPlan("unknown kind")
	}
}

// RefusalError returns the error of a refused plan, or nil for any other kind.
func (p TUIExposurePlan) RefusalError() error {
	if p.kind != TUIPlanRefused {
		return nil
	}
	return &TUIExposureRefusedError{Row: p.row, Message: p.refusal}
}

// Row returns the D2 table row that matched.
func (p TUIExposurePlan) Row() TUIPlanRow { return p.row }

// Kind returns narrow, wide or refused.
func (p TUIExposurePlan) Kind() TUIPlanKind { return p.kind }

// BindAddrs returns a copy of the narrow plan's addresses, loopback first.
func (p TUIExposurePlan) BindAddrs() []netip.Addr { return slices.Clone(p.bindAddrs) }

// DialHost returns the host the command dials (IP literal or engine alias).
func (p TUIExposurePlan) DialHost() types.HostServiceAddress { return p.dialHost }

// PastaForward reports whether the run needs --network=pasta:-T,<port>.
func (p TUIExposurePlan) PastaForward() bool { return p.pastaForward }

// Warning returns the wide plan's exposure warning, or "".
func (p TUIExposurePlan) Warning() TUIExposureMessage { return p.warning }

// Refusal returns the refused plan's error text, or "".
func (p TUIExposurePlan) Refusal() TUIExposureMessage { return p.refusal }

func (p TUIExposurePlan) validateNarrow() error {
	if !slices.Contains(narrowRows, p.row) {
		return invalidPlan("row " + p.row.String() + " has no narrow plan")
	}
	if len(p.bindAddrs) == 0 || p.bindAddrs[0] != loopbackAddr {
		return invalidPlan("a narrow plan lists 127.0.0.1 first")
	}
	// Row shapes (D2): the bridge row binds loopback plus one non-loopback
	// gateway; every other narrow row binds loopback only.
	if p.row == TUIRowDockerBridge {
		if len(p.bindAddrs) != 2 || p.bindAddrs[1].IsLoopback() {
			return invalidPlan("row 8 binds 127.0.0.1 and exactly one non-loopback gateway")
		}
	} else if len(p.bindAddrs) != 1 {
		return invalidPlan("row " + p.row.String() + " binds 127.0.0.1 only")
	}
	for i, addr := range p.bindAddrs {
		switch {
		case !addr.Is4():
			return invalidPlan("bind address " + addr.String() + " is not IPv4")
		case addr.IsUnspecified():
			return invalidPlan("unspecified address " + addr.String() + " outside the opt-in plan")
		case slices.Contains(p.bindAddrs[:i], addr):
			return invalidPlan("duplicate bind address " + addr.String())
		}
	}
	if p.dialHost == "" {
		return invalidPlan("a narrow plan needs a dial host")
	}
	if p.pastaForward && p.row != TUIRowPodmanPasta {
		return invalidPlan("only the loopback-only pasta row forwards a port")
	}
	if p.warning != "" || p.refusal != "" {
		return invalidPlan("a narrow plan carries no message")
	}
	return nil
}

func (p TUIExposurePlan) validateWide() error {
	if !slices.Contains(optInRows, p.row) {
		return invalidPlan("the opt-in does not apply to row " + p.row.String())
	}
	if len(p.bindAddrs) != 0 || p.pastaForward {
		return invalidPlan("a wildcard plan lists no addresses and forwards no port")
	}
	if p.dialHost == "" {
		return invalidPlan("a wildcard plan needs the engine alias")
	}
	if p.warning == "" || p.refusal != "" {
		return invalidPlan("a wildcard plan carries the exposure warning only")
	}
	return nil
}

func (p TUIExposurePlan) validateRefused() error {
	if !slices.Contains(refusedRows, p.row) {
		return invalidPlan("row " + p.row.String() + " is not refused")
	}
	if len(p.bindAddrs) != 0 || p.dialHost != "" || p.pastaForward {
		return invalidPlan("a refused plan carries no address, dial host or forward")
	}
	if p.refusal == "" || p.warning != "" {
		return invalidPlan("a refused plan carries its error only")
	}
	return nil
}

// SelectTUIExposure evaluates the D2 table top to bottom; the first match
// wins. allowAllInterfaces is the user opt-in from container.tui_server_bind;
// it widens only restorable rows and Docker Desktop (row 6), never a host
// runtime, a verified narrow plan (rows 8, 11) or Flatpak without network.
func SelectTUIExposure(f TUINetworkFacts, allowAllInterfaces bool) TUIExposurePlan {
	plan, err := selectTUIExposure(f, allowAllInterfaces)
	if err != nil {
		// A constructor rejected the plan: fail closed with a refusal.
		return TUIExposurePlan{row: TUIRowUnknown, kind: TUIPlanRefused, refusal: tuiMessage(refusedPrefix + "the TUI exposure plan is invalid (" + err.Error() + ")")}
	}
	return plan
}

func selectTUIExposure(f TUINetworkFacts, allowAllInterfaces bool) (TUIExposurePlan, error) {
	restorable := func(row TUIPlanRow, reason string) (TUIExposurePlan, error) {
		if allowAllInterfaces {
			return wideTUIPlan(row, f.Engine)
		}
		return NewTUIExposurePlanRefused(row, tuiMessage(refusedPrefix+reason+
			". Run the command without -i, or, to accept a 0.0.0.0 bind knowingly, set "+TUIOptInSetting+" in your invowk config"))
	}

	switch {
	case !f.Container:
		return NewTUIExposurePlanNarrow(TUIRowHostRuntime, []netip.Addr{loopbackAddr}, loopbackHost, false)
	case f.Sandbox == TUISandboxFlatpak && !f.FlatpakNetwork:
		return NewTUIExposurePlanRefused(TUIRowFlatpakNoNetwork, tuiMessage(refusedPrefix+
			"the Flatpak sandbox's network is private, so no TUI server bind is reachable from the engine's containers. "+
			"Run the command without -i, or grant the sandbox --share=network"))
	case f.RemoteEndpoint:
		return restorable(TUIRowRemoteEngine, "the container engine is remote, so its containers cannot reach a loopback-bound server "+
			"(the opt-in helps only if the engine's host alias leads back to this machine)")
	case f.QueryError != nil:
		return restorable(TUIRowQueryFailed, "the container engine's network facts could not be read ("+f.QueryError.Error()+")")
	case f.Sandbox == TUISandboxSnap:
		return restorable(TUIRowSnap, "no narrow TUI server route is verified for Snap")
	case f.Engine == TUIEngineDocker && f.DockerDesktop:
		if allowAllInterfaces {
			return wideTUIPlan(TUIRowDockerDesktop, f.Engine)
		}
		return NewTUIExposurePlanNarrow(TUIRowDockerDesktop, []netip.Addr{loopbackAddr}, dockerHostAlias, false)
	case f.Engine == TUIEngineDocker && f.Rootless:
		return restorable(TUIRowDockerRootless, "no narrow TUI server route is verified for rootless Docker")
	case f.Engine == TUIEngineDocker && f.PodmanBackend:
		return restorable(TUIRowPodmanRootful, "the docker CLI talks to Podman's Docker-compatible API, and no narrow TUI server route is verified for rootful Podman")
	case f.Engine == TUIEngineDocker && f.BridgeGateway.Is4() && f.GatewayLocal && !f.BridgeGateway.IsUnspecified() && !f.BridgeGateway.IsLoopback():
		return NewTUIExposurePlanNarrow(TUIRowDockerBridge, []netip.Addr{loopbackAddr, f.BridgeGateway}, gatewayDialHost(f.BridgeGateway), false)
	case f.Engine == TUIEnginePodman && !f.Rootless:
		return restorable(TUIRowPodmanRootful, "no narrow TUI server route is verified for rootful Podman")
	case f.Engine == TUIEnginePodman && f.PersistentTarget:
		return restorable(TUIRowPodmanPersistent, "a persistent rootless Podman container has a fixed network, so it cannot get a per-run forwarded port")
	case f.Engine == TUIEnginePodman && f.PastaAvailable:
		return NewTUIExposurePlanNarrow(TUIRowPodmanPasta, []netip.Addr{loopbackAddr}, loopbackHost, true)
	case f.Engine == TUIEnginePodman:
		return restorable(TUIRowPodmanNoPasta, "rootless Podman has no pasta binary for a single forwarded port (installing pasta, the passt package, enables the narrow plan)")
	default:
		return restorable(TUIRowUnknown, "no narrow TUI server route is known for this engine mode (for example, the bridge gateway is not on a local interface)")
	}
}

func wideTUIPlan(row TUIPlanRow, engine TUIEngineKind) (TUIExposurePlan, error) {
	alias := dockerHostAlias
	if engine == TUIEnginePodman {
		alias = podmanHostAlias
	}
	return NewTUIExposurePlanWide(row, alias, tuiMessage(
		"invowk: warning: "+TUIOptInSetting+" is set, so the TUI server listens on 0.0.0.0 and its port is reachable from the network "+
			"(guarded by the per-session token)"))
}

//goplint:ignore -- diagnostic reason text, wrapped into ErrInvalidTUIExposurePlan.
func invalidPlan(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidTUIExposurePlan, reason)
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

// tuiMessage returns text as a validated one-line message; multi-line text
// (for example an engine error quoted in a reason) is folded onto one line.
//
//goplint:ignore -- composed message text, validated as TUIExposureMessage here.
func tuiMessage(text string) TUIExposureMessage {
	raw := strings.NewReplacer("\r", " ", "\n", " ").Replace(text)
	message := TUIExposureMessage(raw)
	if err := message.Validate(); err != nil {
		return ""
	}
	return message
}
