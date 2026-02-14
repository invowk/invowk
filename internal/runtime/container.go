// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/internal/sshserver"
)

// Container host addresses for SSH tunneling
const (
	hostDockerInternal     = "host.docker.internal"
	hostContainersInternal = "host.containers.internal"
	hostGatewayMapping     = "host.docker.internal:host-gateway"
)

// Compile-time interface checks
var (
	_ Runtime            = (*ContainerRuntime)(nil)
	_ CapturingRuntime   = (*ContainerRuntime)(nil)
	_ InteractiveRuntime = (*ContainerRuntime)(nil)
)

type (
	// ContainerRuntime executes commands inside a container.
	//
	// WARNING: Only ONE ContainerRuntime instance should exist per process.
	// Process-wide serialization relies on this single-instance invariant,
	// enforced by createRuntimeRegistry() creating exactly one instance.
	// The runMu mutex provides intra-process fallback locking when flock-based
	// cross-process serialization is unavailable (non-Linux platforms).
	// Multiple instances would defeat this protection, reintroducing the
	// ping_group_range race on platforms without flock.
	ContainerRuntime struct {
		engine      container.Engine
		sshServer   *sshserver.Server
		provisioner *provision.LayerProvisioner
		cfg         *config.Config
		envBuilder  EnvBuilder
		// runMu is a fallback mutex used when flock-based cross-process
		// serialization is unavailable (non-Linux platforms, lock file errors).
		// See runWithRetry() in container_exec.go for usage details.
		runMu sync.Mutex
	}

	// ContainerRuntimeOption configures a ContainerRuntime.
	ContainerRuntimeOption func(*ContainerRuntime)

	// invowkfileContainerConfig is a local type for container config extracted from RuntimeConfig
	invowkfileContainerConfig struct {
		Containerfile string
		Image         string
		Volumes       []string
		Ports         []string
	}
)

// WithContainerEnvBuilder sets the environment builder for the container runtime.
// If not set, NewDefaultEnvBuilder() is used.
func WithContainerEnvBuilder(b EnvBuilder) ContainerRuntimeOption {
	return func(r *ContainerRuntime) {
		r.envBuilder = b
	}
}

// NewContainerRuntime creates a new container runtime with optional configuration.
func NewContainerRuntime(cfg *config.Config, opts ...ContainerRuntimeOption) (*ContainerRuntime, error) {
	engineType := container.EngineType(cfg.ContainerEngine)
	engine, err := container.NewEngine(engineType)
	if err != nil {
		return nil, err
	}

	// Create provisioner with config
	provisionCfg := buildProvisionConfig(cfg)
	provisioner := provision.NewLayerProvisioner(engine, provisionCfg)

	r := &ContainerRuntime{
		engine:      engine,
		provisioner: provisioner,
		cfg:         cfg,
		envBuilder:  NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// NewContainerRuntimeWithEngine creates a container runtime with a specific engine.
func NewContainerRuntimeWithEngine(engine container.Engine, opts ...ContainerRuntimeOption) *ContainerRuntime {
	provisionCfg := provision.DefaultConfig()
	r := &ContainerRuntime{
		engine:      engine,
		provisioner: provision.NewLayerProvisioner(engine, provisionCfg),
		envBuilder:  NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Close releases resources held by the container engine (e.g., the sysctl
// override temp file on Linux). Should be called when the runtime is no longer needed.
func (r *ContainerRuntime) Close() error {
	return container.CloseEngine(r.engine)
}

// SetProvisionConfig updates the provisioner configuration.
// This is useful for setting the invowkfile path before execution.
func (r *ContainerRuntime) SetProvisionConfig(cfg *provision.Config) {
	if cfg != nil {
		r.provisioner = provision.NewLayerProvisioner(r.engine, cfg)
	}
}

// SetSSHServer sets the SSH server for host access from containers
func (r *ContainerRuntime) SetSSHServer(srv *sshserver.Server) {
	r.sshServer = srv
}

// Name returns the runtime name
func (r *ContainerRuntime) Name() string {
	return "container"
}

// Available returns whether this runtime is available
func (r *ContainerRuntime) Available() bool {
	return r.engine != nil && r.engine.Available()
}

// Validate checks if a command can be executed
func (r *ContainerRuntime) Validate(ctx *ExecutionContext) error {
	if ctx.SelectedImpl == nil {
		return fmt.Errorf("no implementation selected for execution")
	}
	if ctx.SelectedImpl.Script == "" {
		return fmt.Errorf("implementation has no script to execute")
	}

	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return fmt.Errorf("runtime config not found for container runtime")
	}

	// Check for containerfile or image
	if rtConfig.Containerfile == "" && rtConfig.Image == "" {
		// Check for default Containerfile/Dockerfile
		invowkDir := filepath.Dir(ctx.Invowkfile.FilePath)
		containerfilePath := filepath.Join(invowkDir, "Containerfile")
		dockerfilePath := filepath.Join(invowkDir, "Dockerfile")
		if _, err := os.Stat(containerfilePath); err != nil {
			if _, err := os.Stat(dockerfilePath); err != nil {
				return fmt.Errorf("container runtime requires a Containerfile or Dockerfile at %s, or an image specified in the runtime config", invowkDir)
			}
		}
	}

	return nil
}
