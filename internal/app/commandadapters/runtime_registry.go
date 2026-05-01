// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"fmt"
	"log/slog"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// RuntimeRegistryFactory creates and populates the runtime registry for command
// execution.
//
//goplint:ignore -- stateless infrastructure adapter has no domain invariants.
type (
	RuntimeRegistryFactory struct {
		containerRuntimeFactory func(*config.Config) (*runtime.ContainerRuntime, error)
	}

	sshServerProvider interface {
		SSHServer() runtime.HostCallbackServer
	}
)

// NewRuntimeRegistryFactory creates a runtime-registry adapter.
func NewRuntimeRegistryFactory() (RuntimeRegistryFactory, error) {
	factory := RuntimeRegistryFactory{
		containerRuntimeFactory: func(cfg *config.Config) (*runtime.ContainerRuntime, error) {
			return runtime.NewContainerRuntime(cfg)
		},
	}
	if err := factory.Validate(); err != nil {
		return RuntimeRegistryFactory{}, err
	}
	return factory, nil
}

// Validate returns nil because RuntimeRegistryFactory is stateless.
func (RuntimeRegistryFactory) Validate() error {
	return nil
}

// Create builds a runtime registry and forwards an active SSH-backed host
// access server to the container runtime when available.
//
// INVARIANT: This method creates at most one ContainerRuntime instance per
// container execution. The ContainerRuntime.runMu mutex provides intra-process
// serialization as a fallback when flock-based cross-process locking is
// unavailable (non-Linux platforms). Creating multiple ContainerRuntime
// instances for one execution would give each its own mutex, defeating
// serialization and reintroducing the ping_group_range race. See
// TestCreateRuntimeRegistry_SingleContainerInstance.
func (f RuntimeRegistryFactory) Create(cfg *config.Config, hostAccess commandsvc.HostAccess, selectedRuntime invowkfile.RuntimeMode) commandsvc.RuntimeRegistryResult {
	var hostCallbacks runtime.HostCallbackServer
	if provider, ok := hostAccess.(sshServerProvider); ok && hostAccess.Running() {
		hostCallbacks = provider.SSHServer()
	}

	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	result := commandsvc.RuntimeRegistryResult{
		Registry: runtime.NewRegistry(),
		Cleanup:  func() {},
	}
	result.Registry.Register(runtime.RuntimeTypeNative, runtime.NewNativeRuntime())
	result.Registry.Register(runtime.RuntimeTypeVirtual, runtime.NewVirtualRuntime(
		cfg.VirtualShell.EnableUrootUtils,
		runtime.WithInteractiveCommandFactory(virtualInteractiveCommand),
	))

	if !shouldInitializeContainerRuntime(selectedRuntime) {
		return result
	}

	factory := f.containerRuntimeFactory
	if factory == nil {
		factory = func(cfg *config.Config) (*runtime.ContainerRuntime, error) {
			return runtime.NewContainerRuntime(cfg)
		}
	}
	containerRT, err := factory(cfg)
	if err != nil {
		result.ContainerInitErr = err
		result.Diagnostics = commandsvc.BridgeRuntimeDiagnostics([]runtime.InitDiagnostic{{
			Code:    runtime.CodeContainerRuntimeInitFailed,
			Message: fmt.Sprintf("container runtime unavailable: %v", err),
			Cause:   err,
		}})
		return result
	}

	if hostCallbacks != nil && hostCallbacks.IsRunning() {
		containerRT.SetHostCallbacks(hostCallbacks)
	}
	result.Registry.Register(runtime.RuntimeTypeContainer, containerRT)
	result.Cleanup = func() {
		if closeErr := containerRT.Close(); closeErr != nil {
			slog.Warn("container runtime cleanup failed", "error", closeErr)
		}
	}
	return result
}

func shouldInitializeContainerRuntime(selectedRuntime invowkfile.RuntimeMode) bool {
	return selectedRuntime == "" || selectedRuntime == invowkfile.RuntimeContainer
}
