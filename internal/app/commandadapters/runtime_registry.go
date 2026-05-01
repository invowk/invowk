// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
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
	RuntimeRegistryFactory struct{}

	sshServerProvider interface {
		SSHServer() runtime.HostCallbackServer
	}
)

// NewRuntimeRegistryFactory creates a runtime-registry adapter.
func NewRuntimeRegistryFactory() (RuntimeRegistryFactory, error) {
	factory := RuntimeRegistryFactory{}
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
func (RuntimeRegistryFactory) Create(cfg *config.Config, hostAccess commandsvc.HostAccess, selectedRuntime invowkfile.RuntimeMode) commandsvc.RuntimeRegistryResult {
	var hostCallbacks runtime.HostCallbackServer
	if provider, ok := hostAccess.(sshServerProvider); ok && hostAccess.Running() {
		hostCallbacks = provider.SSHServer()
	}

	built := runtime.BuildRegistry(runtime.BuildRegistryOptions{
		Config:          cfg,
		HostCallbacks:   hostCallbacks,
		SelectedRuntime: selectedRuntime,
	})

	return commandsvc.RuntimeRegistryResult{
		Registry:         built.Registry,
		Cleanup:          built.Cleanup,
		Diagnostics:      commandsvc.BridgeRuntimeDiagnostics(built.Diagnostics),
		ContainerInitErr: built.ContainerInitErr,
	}
}
