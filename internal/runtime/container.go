// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/internal/sshserver"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// Container host addresses for SSH tunneling
const (
	hostDockerInternal     = "host.docker.internal"
	hostContainersInternal = "host.containers.internal"
	hostGatewayMapping     = "host.docker.internal:host-gateway"
)

// ErrContainerBuildConfig is returned when a container runtime command specifies neither
// a container image nor a Containerfile/Dockerfile, and no default Containerfile exists
// in the invowkfile directory. Callers can use errors.Is to detect this condition
// programmatically without relying on message substring matching.
var (
	ErrContainerBuildConfig = errors.New("invalid container build configuration")

	// Compile-time interface checks.
	_ Runtime            = (*ContainerRuntime)(nil)
	_ CapturingRuntime   = (*ContainerRuntime)(nil)
	_ InteractiveRuntime = (*ContainerRuntime)(nil)
)

type (
	// ContainerRuntime executes commands inside a container.
	//
	// WARNING: Only ONE ContainerRuntime instance should exist per process.
	// Process-wide serialization relies on this single-instance invariant,
	// enforced by BuildRegistry() creating exactly one instance.
	// See TestCreateRuntimeRegistry_SingleContainerInstance for the enforcement test.
	//
	// The runMu mutex provides intra-process fallback locking when flock-based
	// cross-process serialization is unavailable (non-Linux platforms).
	// Multiple instances would defeat this protection, reintroducing the
	// ping_group_range race on platforms without flock. The ping_group_range
	// race is a Podman-specific sysctl contention issue where concurrent
	// container starts compete for /proc/sys/net/ipv4/ping_group_range,
	// causing transient "OCI runtime" errors. See container_exec.go for
	// the retry and serialization logic.
	ContainerRuntime struct {
		engine          container.Engine
		hostCallbacks   HostCallbackServer
		provisioner     provision.Provisioner
		provisionConfig *provision.Config
		cfg             *config.Config
		envBuilder      EnvBuilder
		//plint:internal -- fallback mutex for non-Linux flock; see runWithRetry()
		runMu sync.Mutex
		//plint:internal -- fallback ID counter for missing ExecutionID; see newExecutionID()
		fallbackIDCounter atomic.Uint64
	}

	// ContainerRuntimeOption configures a ContainerRuntime.
	ContainerRuntimeOption func(*ContainerRuntime)

	// invowkfileContainerConfig is a local type for container config extracted from RuntimeConfig
	invowkfileContainerConfig struct {
		Containerfile container.HostFilesystemPath
		Image         container.ImageTag
		Volumes       []invowkfile.VolumeMountSpec
		Ports         []invowkfile.PortMappingSpec
	}
)

// WithContainerEnvBuilder sets the environment builder for the container runtime.
// If not set, NewDefaultEnvBuilder() is used.
func WithContainerEnvBuilder(b EnvBuilder) ContainerRuntimeOption {
	return func(r *ContainerRuntime) {
		r.envBuilder = b
	}
}

// WithContainerProvisioner sets the provisioning port for tests and custom adapters.
func WithContainerProvisioner(p provision.Provisioner, cfg *provision.Config) ContainerRuntimeOption {
	return func(r *ContainerRuntime) {
		r.provisioner = p
		r.provisionConfig = cfg
	}
}

// NewContainerRuntime creates a new container runtime with optional configuration.
func NewContainerRuntime(cfg *config.Config, opts ...ContainerRuntimeOption) (*ContainerRuntime, error) {
	engineType := container.EngineType(cfg.ContainerEngine)
	engine, err := container.NewEngine(engineType)
	if err != nil {
		if errors.Is(err, container.ErrNoEngineAvailable) {
			return nil, fmt.Errorf("%w: %w", ErrContainerEngineUnavailable, err)
		}
		return nil, err
	}

	// Create provisioner with config
	provisionCfg := buildProvisionConfig(cfg)
	provisioner, err := provision.NewLayerProvisioner(engine, provisionCfg)
	if err != nil {
		return nil, fmt.Errorf("create provisioner: %w", err)
	}

	r := &ContainerRuntime{
		engine:          engine,
		provisioner:     provisioner,
		provisionConfig: provisionCfg,
		cfg:             cfg,
		envBuilder:      NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// NewContainerRuntimeWithEngine creates a container runtime with a specific engine.
func NewContainerRuntimeWithEngine(engine container.Engine, opts ...ContainerRuntimeOption) (*ContainerRuntime, error) {
	provisionCfg := provision.DefaultConfig()
	provisioner, err := provision.NewLayerProvisioner(engine, provisionCfg)
	if err != nil {
		return nil, fmt.Errorf("create provisioner: %w", err)
	}
	r := &ContainerRuntime{
		engine:          engine,
		provisioner:     provisioner,
		provisionConfig: provisionCfg,
		envBuilder:      NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Close releases resources held by the container engine (e.g., the sysctl
// override temp file on Linux). Should be called when the runtime is no longer needed.
func (r *ContainerRuntime) Close() error {
	return container.CloseEngine(r.engine)
}

// SetProvisionConfig updates the provisioner configuration.
// This is useful for setting the invowkfile path before execution.
func (r *ContainerRuntime) SetProvisionConfig(cfg *provision.Config) error {
	if cfg != nil {
		provisioner, err := provision.NewLayerProvisioner(r.engine, cfg)
		if err != nil {
			return fmt.Errorf("update provisioner: %w", err)
		}
		r.provisioner = provisioner
		r.provisionConfig = cfg
	}
	return nil
}

// SetHostCallbacks sets the host callback server for container access to host services.
func (r *ContainerRuntime) SetHostCallbacks(server HostCallbackServer) {
	r.hostCallbacks = server
}

// SetSSHServer sets the SSH server for host access from containers.
func (r *ContainerRuntime) SetSSHServer(srv *sshserver.Server) {
	r.SetHostCallbacks(srv)
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
		return errContainerNoImpl
	}
	if ctx.SelectedImpl.Script == "" {
		return errContainerNoScript
	}

	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return errors.New("runtime config not found for container runtime")
	}

	// Check for containerfile or image
	if rtConfig.Containerfile == "" && rtConfig.Image == "" {
		// Check for default Containerfile/Dockerfile
		invowkDir := filepath.Dir(string(ctx.Invowkfile.FilePath))
		containerfilePath := filepath.Join(invowkDir, "Containerfile")
		dockerfilePath := filepath.Join(invowkDir, "Dockerfile")
		if _, err := os.Stat(containerfilePath); err != nil {
			if _, err := os.Stat(dockerfilePath); err != nil {
				return fmt.Errorf("%w: container runtime requires a Containerfile or Dockerfile at %s, or an image specified in the runtime config", ErrContainerBuildConfig, invowkDir)
			}
		}
	}

	return nil
}
