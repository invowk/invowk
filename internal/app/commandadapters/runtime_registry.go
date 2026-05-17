// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/app/deps"
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

	//goplint:ignore -- private adapter session owns infrastructure handles rather than domain invariants.
	runtimeSession struct {
		registry               *runtime.Registry
		cleanup                func()
		diagnostics            []commandsvc.Diagnostic
		containerInitErr       error
		dependencyProbeFactory dependencyRuntimeProbeFactory
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
// container execution. Runtime-level fallback serialization is process-wide,
// but one runtime instance still keeps registry cleanup and provisioning state
// scoped to a single execution.
func (f RuntimeRegistryFactory) Create(cfg *config.Config, hostAccess commandsvc.HostAccess, selectedRuntime invowkfile.RuntimeMode) commandsvc.RuntimeSession {
	var hostCallbacks runtime.HostCallbackServer
	if provider, ok := hostAccess.(sshServerProvider); ok && hostAccess.Running() {
		hostCallbacks = provider.SSHServer()
	}

	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	session := &runtimeSession{
		registry: runtime.NewRegistry(),
		cleanup: func() {
			// Native and virtual runtimes do not allocate registry resources.
		},
		dependencyProbeFactory: NewDependencyRuntimeProbeFactory(),
	}
	session.registry.Register(runtime.RuntimeTypeNative, runtime.NewNativeRuntime())
	session.registry.Register(runtime.RuntimeTypeVirtual, runtime.NewVirtualRuntime(
		cfg.VirtualShell.EnableUrootUtils,
		runtime.WithInteractiveCommandFactory(virtualInteractiveCommand),
	))

	if !shouldInitializeContainerRuntime(selectedRuntime) {
		return session
	}

	factory := f.containerRuntimeFactory
	if factory == nil {
		factory = func(cfg *config.Config) (*runtime.ContainerRuntime, error) {
			return runtime.NewContainerRuntime(cfg)
		}
	}
	containerRT, err := factory(cfg)
	if err != nil {
		session.containerInitErr = err
		session.diagnostics = commandsvc.BridgeRuntimeDiagnostics([]runtime.InitDiagnostic{{
			Code:    runtime.CodeContainerRuntimeInitFailed,
			Message: fmt.Sprintf("container runtime unavailable: %v", err),
			Cause:   err,
		}})
		return session
	}

	if hostCallbacks != nil && hostCallbacks.IsRunning() {
		containerRT.SetHostCallbacks(hostCallbacks)
	}
	session.registry.Register(runtime.RuntimeTypeContainer, containerRT)
	session.cleanup = func() {
		if closeErr := containerRT.Close(); closeErr != nil {
			slog.Warn("container runtime cleanup failed", "error", closeErr)
		}
	}
	return session
}

func shouldInitializeContainerRuntime(selectedRuntime invowkfile.RuntimeMode) bool {
	return selectedRuntime == "" || selectedRuntime == invowkfile.RuntimeContainer
}

func (s *runtimeSession) NewExecutionID() runtime.ExecutionID {
	return s.registry.NewExecutionID()
}

func (s *runtimeSession) Validate() error {
	var errs []error
	for i := range s.diagnostics {
		errs = append(errs, s.diagnostics[i].Validate())
	}
	return errors.Join(errs...)
}

func (s *runtimeSession) Diagnostics() []commandsvc.Diagnostic {
	return s.diagnostics
}

func (s *runtimeSession) ContainerInitErr() error {
	return s.containerInitErr
}

func (s *runtimeSession) DependencyProbe(execCtx *runtime.ExecutionContext) deps.RuntimeDependencyProbe {
	return s.dependencyProbeFactory.Create(s.registry, execCtx)
}

func (s *runtimeSession) Execute(execCtx *runtime.ExecutionContext, cmdName invowkfile.CommandName, interactive bool, interactiveExecutor commandsvc.InteractiveExecutor) (*runtime.Result, invowkfile.RuntimeMode, error) {
	if !interactive {
		return s.registry.Execute(execCtx), "", nil
	}

	rt, err := s.registry.GetForContext(execCtx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get runtime: %w", err)
	}

	if interactiveRT, ok := rt.(commandsvc.RuntimeInteractiveCommand); ok {
		if interactiveExecutor == nil {
			return &runtime.Result{ExitCode: 1, Error: commandsvc.ErrInteractiveExecutorNotConfigured}, "", nil
		}
		return interactiveExecutor.Execute(execCtx, cmdName, interactiveRT), "", nil
	}

	return s.registry.Execute(execCtx), invowkfile.RuntimeMode(rt.Name()), nil //goplint:ignore -- runtime names are registered from runtime mode constants.
}

func (s *runtimeSession) Close() {
	if s.cleanup != nil {
		s.cleanup()
	}
}
