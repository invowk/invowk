// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
)

// Container host addresses for SSH tunneling
const (
	hostDockerInternal     HostServiceAddress = "host.docker.internal"
	hostContainersInternal HostServiceAddress = "host.containers.internal"
	hostGatewayMapping                        = "host.docker.internal:host-gateway"
)

// ErrContainerBuildConfig is returned when a container runtime command specifies neither
// a container image nor an explicit Containerfile. Callers can use errors.Is to detect
// this condition programmatically without relying on message substring matching.
var (
	ErrContainerBuildConfig = errors.New("invalid container build configuration")

	containerRunFallbackMu  sync.Mutex
	acquireContainerRunLock = acquireRunLock

	// Compile-time interface checks.
	_ Runtime                    = (*ContainerRuntime)(nil)
	_ CapturingRuntime           = (*ContainerRuntime)(nil)
	_ InteractiveRuntime         = (*ContainerRuntime)(nil)
	_ HostServiceAddressProvider = (*ContainerRuntime)(nil)
)

type (
	// ContainerRuntime executes commands inside a container.
	//
	// Process-wide fallback serialization is shared across instances when
	// flock-based cross-process serialization is unavailable.
	ContainerRuntime struct {
		engine          containerEngine
		hostCallbacks   HostCallbackServer
		provisioner     provision.Provisioner
		provisionConfig *provision.Config
		cfg             *config.Config
		envBuilder      EnvBuilder
		retrySleep      func(context.Context, time.Duration) error
		//plint:internal -- fallback ID counter for missing ExecutionID; see newExecutionID()
		fallbackIDCounter atomic.Uint64
	}

	// ContainerRuntimeOption configures a ContainerRuntime.
	ContainerRuntimeOption func(*ContainerRuntime)

	// invowkfileContainerConfig is a local type for container config extracted from RuntimeConfig
	invowkfileContainerConfig struct {
		Containerfile container.HostFilesystemPath
		Image         container.ImageTag
		Volumes       []container.VolumeMountSpec
		Ports         []container.PortMappingSpec
	}

	containerEngine interface {
		Name() string
		Available() bool
		Build(context.Context, container.BuildOptions) error
		Run(context.Context, container.RunOptions) (*container.RunResult, error)
		ImageExists(context.Context, container.ImageTag) (bool, error)
		RemoveImage(context.Context, container.ImageTag, bool) error
	}

	containerEngineCloser interface {
		Close() error
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
		retrySleep:      sleepWithContext,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// NewContainerRuntimeWithEngine creates a container runtime with a specific engine.
func NewContainerRuntimeWithEngine(engine containerEngine, opts ...ContainerRuntimeOption) (*ContainerRuntime, error) {
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
		retrySleep:      sleepWithContext,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Close releases resources held by the container engine (e.g., the sysctl
// override temp file on Linux). Should be called when the runtime is no longer needed.
func (r *ContainerRuntime) Close() error {
	if closer, ok := r.engine.(containerEngineCloser); ok {
		return closer.Close()
	}
	return nil
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
		return fmt.Errorf("%w: container runtime requires either containerfile or image in the runtime config", ErrContainerBuildConfig)
	}
	if rtConfig.Image != "" {
		if err := container.ValidateSupportedRuntimeImage(container.ImageTag(rtConfig.Image)); err != nil {
			return err
		}
	}

	return nil
}
