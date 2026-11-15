// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/containerplan"
	"github.com/invowk/invowk/pkg/platform"
)

type (
	// tuiExposureProbe gathers the facts behind a TUI exposure plan
	// (restrict-tui-server-bind D2). Engine facts are queried at most once
	// per runtime, and only when an interactive run asks for a plan.
	tuiExposureProbe struct {
		queryFacts     func(ctx context.Context, engine container.NamedEngine) (container.EngineNetworkFacts, error)
		sandbox        func() platform.SandboxType
		flatpakNetwork func() bool
		interfaceAddrs func() ([]netip.Prefix, error)

		once     sync.Once
		facts    container.EngineNetworkFacts
		factsErr error
	}
)

// Validate checks the cached engine facts once they have been queried.
func (p *tuiExposureProbe) Validate() error {
	if p.facts == (container.EngineNetworkFacts{}) {
		return nil
	}
	return p.facts.Validate()
}

// fillDefaults sets the production hooks that are still nil, so a
// ContainerRuntime built without a constructor still plans safely.
func (p *tuiExposureProbe) fillDefaults() {
	if p.queryFacts == nil {
		p.queryFacts = func(ctx context.Context, engine container.NamedEngine) (container.EngineNetworkFacts, error) {
			return container.QueryEngineNetworkFacts(ctx, engine, nil)
		}
	}
	if p.sandbox == nil {
		p.sandbox = platform.DetectSandbox
	}
	if p.flatpakNetwork == nil {
		p.flatpakNetwork = container.FlatpakSharesNetwork
	}
	if p.interfaceAddrs == nil {
		p.interfaceAddrs = localInterfacePrefixes
	}
}

// PlanTUIExposure selects the TUI server bind plan for an interactive run.
// The opt-in comes from the runtime's config, which only the
// config-source-resolution sources produce (never an invowkfile or module).
func (r *ContainerRuntime) PlanTUIExposure(ctx *ExecutionContext) containerplan.TUIExposurePlan {
	r.tuiProbe.fillDefaults()
	facts := containerplan.TUINetworkFacts{Container: true, Sandbox: containerplan.TUISandboxNone}
	switch r.tuiProbe.sandbox() {
	case platform.SandboxFlatpak:
		facts.Sandbox = containerplan.TUISandboxFlatpak
		facts.FlatpakNetwork = r.tuiProbe.flatpakNetwork()
	case platform.SandboxSnap:
		facts.Sandbox = containerplan.TUISandboxSnap
	case platform.SandboxNone:
	}

	goCtx := context.Background()
	if ctx != nil && ctx.Context != nil {
		goCtx = ctx.Context
	}
	engineFacts, err := r.engineNetworkFacts(goCtx)
	if err != nil {
		facts.QueryError = err
	}
	switch engineFacts.Engine {
	case container.EngineTypeDocker:
		facts.Engine = containerplan.TUIEngineDocker
	case container.EngineTypePodman:
		facts.Engine = containerplan.TUIEnginePodman
	case container.EngineTypeAny:
	}
	facts.DockerDesktop = engineFacts.DockerDesktop
	facts.Rootless = engineFacts.Rootless
	facts.RemoteEndpoint = engineFacts.RemoteEndpoint
	facts.PastaAvailable = engineFacts.PastaAvailable
	facts.BridgeGateway = engineFacts.BridgeGateway
	facts.GatewayLocal = container.IsLocalInterfaceAddr(engineFacts.BridgeGateway, r.tuiProbe.interfaceAddrs)
	facts.PersistentTarget = r.tuiPersistentTarget(ctx)

	allow := r.cfg != nil && r.cfg.Container.TUIServerBind.AllowsAllInterfaces()
	return containerplan.SelectTUIExposure(facts, allow)
}

func (r *ContainerRuntime) engineNetworkFacts(ctx context.Context) (container.EngineNetworkFacts, error) {
	r.tuiProbe.once.Do(func() {
		r.tuiProbe.facts, r.tuiProbe.factsErr = r.tuiProbe.queryFacts(ctx, r.engine)
	})
	return r.tuiProbe.facts, r.tuiProbe.factsErr
}

// tuiPersistentTarget reports whether the run execs into a persistent
// container. A resolution error counts as persistent, the narrower outcome.
func (r *ContainerRuntime) tuiPersistentTarget(ctx *ExecutionContext) bool {
	if ctx == nil {
		return false
	}
	if ctx.ContainerNameOverride != "" {
		return true
	}
	if ctx.SelectedImpl == nil {
		return false
	}
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return false
	}
	requested, err := persistentContainerRequested(ctx, containerConfigFromRuntime(rtConfig))
	return err != nil || requested
}

func localInterfacePrefixes() ([]netip.Prefix, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("list interface addresses: %w", err)
	}
	prefixes := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		ones, _ := ipNet.Mask.Size()
		prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), ones))
	}
	return prefixes, nil
}
