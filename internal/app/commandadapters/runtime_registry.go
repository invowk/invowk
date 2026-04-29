// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/sshserver"
)

// RuntimeRegistryFactory creates and populates the runtime registry for command
// execution.
//
//goplint:ignore -- stateless infrastructure adapter has no domain invariants.
type (
	RuntimeRegistryFactory struct{}

	sshServerProvider interface {
		SSHServer() *sshserver.Server
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
// INVARIANT: This method creates exactly one ContainerRuntime instance per
// call. The ContainerRuntime.runMu mutex provides intra-process serialization
// as a fallback when flock-based cross-process locking is unavailable
// (non-Linux platforms). Creating multiple ContainerRuntime instances would
// give each its own mutex, defeating serialization and reintroducing the
// ping_group_range race. See TestCreateRuntimeRegistry_SingleContainerInstance.
func (RuntimeRegistryFactory) Create(cfg *config.Config, hostAccess commandsvc.HostAccess) commandsvc.RuntimeRegistryResult {
	var sshServer *sshserver.Server
	if provider, ok := hostAccess.(sshServerProvider); ok && hostAccess.Running() {
		sshServer = provider.SSHServer()
	}

	built := runtime.BuildRegistry(runtime.BuildRegistryOptions{
		Config:    cfg,
		SSHServer: sshServer,
	})

	return commandsvc.RuntimeRegistryResult{
		Registry:         built.Registry,
		Cleanup:          built.Cleanup,
		Diagnostics:      commandsvc.BridgeRuntimeDiagnostics(built.Diagnostics),
		ContainerInitErr: built.ContainerInitErr,
	}
}
