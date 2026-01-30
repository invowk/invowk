// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"invowk-cli/internal/config"
	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"os"
	"path/filepath"
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
	// ContainerRuntime executes commands inside a container
	ContainerRuntime struct {
		engine      container.Engine
		sshServer   *sshserver.Server
		provisioner *LayerProvisioner
		cfg         *config.Config
	}

	// invkfileContainerConfig is a local type for container config extracted from RuntimeConfig
	invkfileContainerConfig struct {
		Containerfile string
		Image         string
		Volumes       []string
		Ports         []string
	}
)

// NewContainerRuntime creates a new container runtime
func NewContainerRuntime(cfg *config.Config) (*ContainerRuntime, error) {
	engineType := container.EngineType(cfg.ContainerEngine)
	engine, err := container.NewEngine(engineType)
	if err != nil {
		return nil, err
	}

	// Create provisioner with config
	provisionCfg := buildProvisionConfig(cfg)
	provisioner := NewLayerProvisioner(engine, provisionCfg)

	return &ContainerRuntime{
		engine:      engine,
		provisioner: provisioner,
		cfg:         cfg,
	}, nil
}

// NewContainerRuntimeWithEngine creates a container runtime with a specific engine
func NewContainerRuntimeWithEngine(engine container.Engine) *ContainerRuntime {
	provisionCfg := DefaultProvisionConfig()
	return &ContainerRuntime{
		engine:      engine,
		provisioner: NewLayerProvisioner(engine, provisionCfg),
	}
}

// SetProvisionConfig updates the provisioner configuration.
// This is useful for setting the invkfile path before execution.
func (r *ContainerRuntime) SetProvisionConfig(cfg *ContainerProvisionConfig) {
	if cfg != nil {
		r.provisioner = NewLayerProvisioner(r.engine, cfg)
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
		invowkDir := filepath.Dir(ctx.Invkfile.FilePath)
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
