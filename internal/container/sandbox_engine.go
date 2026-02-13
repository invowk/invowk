// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"os/exec"

	"invowk-cli/pkg/platform"
)

// SandboxAwareEngine wraps a container Engine to handle execution from within
// application sandboxes (Flatpak, Snap).
//
// When running inside a sandbox, container engines like Docker/Podman run on the
// host system, not inside the sandbox. This causes path mismatches when mounting
// volumes - the sandbox has its own filesystem namespace, so paths like /tmp
// inside the sandbox don't correspond to /tmp on the host.
//
// This wrapper solves the problem by executing container commands via the
// sandbox's host spawn mechanism (e.g., flatpak-spawn --host). This runs the
// entire command on the host where paths resolve correctly.
//
// When not in a sandbox, this wrapper passes through to the underlying engine
// without modification.
type SandboxAwareEngine struct {
	wrapped     Engine
	sandboxType platform.SandboxType
}

// NewSandboxAwareEngine wraps an Engine with sandbox awareness.
// If not running in a sandbox, the engine is returned unwrapped.
func NewSandboxAwareEngine(engine Engine) Engine {
	sandboxType := platform.DetectSandbox()
	if sandboxType == platform.SandboxNone {
		return engine
	}
	return &SandboxAwareEngine{
		wrapped:     engine,
		sandboxType: sandboxType,
	}
}

// newSandboxAwareEngineForTesting creates a SandboxAwareEngine with a specific
// sandbox type for testing purposes.
func newSandboxAwareEngineForTesting(engine Engine, sandboxType platform.SandboxType) *SandboxAwareEngine {
	return &SandboxAwareEngine{
		wrapped:     engine,
		sandboxType: sandboxType,
	}
}

// Name returns the wrapped engine name.
func (e *SandboxAwareEngine) Name() string {
	return e.wrapped.Name()
}

// Available checks if the wrapped engine is available.
// In a sandbox environment, we check if the engine is available via the host spawn.
func (e *SandboxAwareEngine) Available() bool {
	// For availability check, we use the wrapped engine's check.
	// The spawn command overhead doesn't affect availability status.
	return e.wrapped.Available()
}

// Version returns the wrapped engine version.
func (e *SandboxAwareEngine) Version(ctx context.Context) (string, error) {
	return e.wrapped.Version(ctx)
}

// BinaryPath returns the path to the container engine binary.
func (e *SandboxAwareEngine) BinaryPath() string {
	return e.wrapped.BinaryPath()
}

// BuildRunArgs builds the argument slice for a 'run' command.
// When in a sandbox, this prepends the spawn command and args.
func (e *SandboxAwareEngine) BuildRunArgs(opts RunOptions) []string {
	baseArgs := e.wrapped.BuildRunArgs(opts)
	return e.wrapArgs(baseArgs)
}

// Build builds an image from a Dockerfile.
// In sandbox mode, the build command is executed via the host spawn mechanism.
func (e *SandboxAwareEngine) Build(ctx context.Context, opts BuildOptions) error {
	if e.sandboxType == platform.SandboxNone {
		return e.wrapped.Build(ctx, opts)
	}

	// Get the build args from the underlying engine
	// We need to access BaseCLIEngine's BuildArgs method
	baseEngine, ok := e.getBaseCLIEngine()
	if !ok {
		// Fallback to wrapped engine if we can't get base args
		return e.wrapped.Build(ctx, opts)
	}

	buildArgs := baseEngine.BuildArgs(opts)
	fullArgs := e.buildSpawnArgs(e.wrapped.BinaryPath(), buildArgs)

	cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
	e.CustomizeCmd(cmd)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return buildContainerError(e.wrapped.Name(), opts, err)
	}

	return nil
}

// Run runs a command in a container.
// In sandbox mode, the run command is executed via the host spawn mechanism.
func (e *SandboxAwareEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if e.sandboxType == platform.SandboxNone {
		return e.wrapped.Run(ctx, opts)
	}

	// Get the run args from the underlying engine
	baseArgs := e.wrapped.BuildRunArgs(opts)
	fullArgs := e.buildSpawnArgs(e.wrapped.BinaryPath(), baseArgs)

	cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
	e.CustomizeCmd(cmd)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// Remove removes a container.
func (e *SandboxAwareEngine) Remove(ctx context.Context, containerID string, force bool) error {
	if e.sandboxType == platform.SandboxNone {
		return e.wrapped.Remove(ctx, containerID, force)
	}

	baseEngine, ok := e.getBaseCLIEngine()
	if !ok {
		return e.wrapped.Remove(ctx, containerID, force)
	}

	removeArgs := baseEngine.RemoveArgs(containerID, force)
	fullArgs := e.buildSpawnArgs(e.wrapped.BinaryPath(), removeArgs)

	cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
	e.CustomizeCmd(cmd)
	return cmd.Run()
}

// ImageExists checks if an image exists.
func (e *SandboxAwareEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	if e.sandboxType == platform.SandboxNone {
		return e.wrapped.ImageExists(ctx, image)
	}

	// Build image exists check command
	// Podman: "image exists <image>"
	// Docker: "image inspect <image>"
	var checkArgs []string
	if e.wrapped.Name() == string(EngineTypePodman) {
		checkArgs = []string{"image", "exists", image}
	} else {
		checkArgs = []string{"image", "inspect", image}
	}

	fullArgs := e.buildSpawnArgs(e.wrapped.BinaryPath(), checkArgs)
	cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
	e.CustomizeCmd(cmd)
	err := cmd.Run()
	return err == nil, nil
}

// RemoveImage removes an image.
func (e *SandboxAwareEngine) RemoveImage(ctx context.Context, image string, force bool) error {
	if e.sandboxType == platform.SandboxNone {
		return e.wrapped.RemoveImage(ctx, image, force)
	}

	baseEngine, ok := e.getBaseCLIEngine()
	if !ok {
		return e.wrapped.RemoveImage(ctx, image, force)
	}

	removeArgs := baseEngine.RemoveImageArgs(image, force)
	fullArgs := e.buildSpawnArgs(e.wrapped.BinaryPath(), removeArgs)

	cmd := exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
	e.CustomizeCmd(cmd)
	return cmd.Run()
}

// CustomizeCmd applies the wrapped engine's overrides (env vars, extra files)
// to a command created outside the wrapped engine's CreateCommand method.
func (e *SandboxAwareEngine) CustomizeCmd(cmd *exec.Cmd) {
	if c, ok := e.wrapped.(CmdCustomizer); ok {
		c.CustomizeCmd(cmd)
	}
}

// Close forwards to the wrapped engine's Close method if it implements
// EngineCloser. Returns nil if the wrapped engine has no Close method.
func (e *SandboxAwareEngine) Close() error {
	if c, ok := e.wrapped.(EngineCloser); ok {
		return c.Close()
	}
	return nil
}

// SysctlOverrideActive forwards to the wrapped engine's SysctlOverrideChecker
// if it implements the interface. Returns false if the wrapped engine doesn't
// implement SysctlOverrideChecker (e.g., DockerEngine).
func (e *SandboxAwareEngine) SysctlOverrideActive() bool {
	if checker, ok := e.wrapped.(SysctlOverrideChecker); ok {
		return checker.SysctlOverrideActive()
	}
	return false
}

// buildSpawnArgs constructs the full argument list for spawning a command on the host.
// For Flatpak: ["flatpak-spawn", "--host", <binary>, <args...>]
// For Snap: ["snap", "run", "--shell", <binary>, <args...>]
func (e *SandboxAwareEngine) buildSpawnArgs(binary string, args []string) []string {
	spawnCmd, spawnArgs := e.getSpawnInfo()

	// Build: [spawn-cmd, spawn-args..., binary, args...]
	result := make([]string, 0, 1+len(spawnArgs)+1+len(args))
	result = append(result, spawnCmd)
	result = append(result, spawnArgs...)
	result = append(result, binary)
	result = append(result, args...)

	return result
}

// getSpawnInfo returns the spawn command and arguments based on the engine's sandbox type.
// This uses the engine's stored sandbox type, not the global detection, allowing tests
// to override the sandbox type.
func (e *SandboxAwareEngine) getSpawnInfo() (cmd string, args []string) {
	switch e.sandboxType {
	case platform.SandboxNone:
		return "", nil
	case platform.SandboxFlatpak:
		return "flatpak-spawn", []string{"--host"}
	case platform.SandboxSnap:
		return "snap", []string{"run", "--shell"}
	}
	return "", nil
}

// wrapArgs prepends spawn command to existing args if in sandbox.
// This is used for BuildRunArgs which returns args starting from "run".
func (e *SandboxAwareEngine) wrapArgs(args []string) []string {
	if e.sandboxType == platform.SandboxNone {
		return args
	}
	return e.buildSpawnArgs(e.wrapped.BinaryPath(), args)
}

// getBaseCLIEngine attempts to extract the BaseCLIEngine from the wrapped engine.
// This is needed to access argument building methods.
func (e *SandboxAwareEngine) getBaseCLIEngine() (*BaseCLIEngine, bool) {
	switch engine := e.wrapped.(type) {
	case *PodmanEngine:
		return engine.BaseCLIEngine, true
	case *DockerEngine:
		return engine.BaseCLIEngine, true
	default:
		return nil, false
	}
}
