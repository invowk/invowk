// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/invowk/invowk/internal/container"
)

type (
	containerRunCommandPreparer interface {
		PrepareRunCommand(ctx context.Context, opts container.RunOptions) *exec.Cmd
	}

	containerExecCommandPreparer interface {
		PrepareExecCommand(ctx context.Context, containerID container.ContainerID, command []string, opts container.RunOptions) *exec.Cmd
	}
)

// SupportsInteractive returns true if the container runtime can run interactively.
// This requires a container engine to be available.
func (r *ContainerRuntime) SupportsInteractive() bool {
	return r.Available()
}

// PrepareInteractive prepares the container runtime for interactive execution.
// This is an alias for PrepareCommand to implement the InteractiveRuntime interface.
func (r *ContainerRuntime) PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error) {
	return r.PrepareCommand(ctx)
}

// PrepareCommand prepares the container execution for interactive mode.
// Instead of executing immediately, it returns a prepared command that can
// be attached to a PTY by the caller. This enables the interactive mode
// TUI overlay pattern where the parent process manages the PTY.
func (r *ContainerRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
	prep, errResult := r.prepareContainerExecution(ctx, containerExecOptions{interactiveTUI: true})
	if errResult != nil {
		return nil, errResult.Error
	}

	if persistentContainerRequested(ctx, prep.containerCfg) {
		containerID, err := r.ensurePersistentContainer(ctx, prep)
		if err != nil {
			prep.cleanup()
			return nil, err
		}
		runOpts := execOptionsForPersistent(ctx, prep, nil, nil)
		runOpts.Interactive = true
		runOpts.TTY = true
		if err := validatePersistentExecOptions(runOpts); err != nil {
			prep.cleanup()
			return nil, err
		}
		preparer, ok := r.engine.(containerExecCommandPreparer)
		if !ok {
			prep.cleanup()
			return nil, errors.New("container engine does not support interactive exec preparation")
		}
		cmd := preparer.PrepareExecCommand(ctx.Context, containerID, prep.shellCmd, runOpts)
		return &PreparedCommand{Cmd: cmd, Cleanup: prep.cleanup}, nil
	}

	runOpts := container.RunOptions{
		Image:       prep.image,
		Command:     prep.shellCmd,
		WorkDir:     prep.workDir,
		Env:         prep.env,
		Volumes:     prep.volumes,
		Ports:       prep.ports,
		Remove:      true, // Always remove after execution
		Interactive: true, // Enable -i for PTY
		TTY:         true, // Enable -t for PTY
		ExtraHosts:  prep.extraHosts,
	}
	if err := runOpts.Validate(); err != nil {
		return nil, fmt.Errorf("container run options: %w", err)
	}

	preparer, ok := r.engine.(containerRunCommandPreparer)
	if !ok {
		return nil, errors.New("container engine does not support interactive command preparation")
	}
	cmd := preparer.PrepareRunCommand(ctx.Context, runOpts)
	return &PreparedCommand{Cmd: cmd, Cleanup: prep.cleanup}, nil
}

func validatePersistentExecOptions(opts container.RunOptions) error {
	if opts.WorkDir != "" {
		if err := opts.WorkDir.Validate(); err != nil {
			return fmt.Errorf("container exec work dir: %w", err)
		}
	}
	for _, h := range opts.ExtraHosts {
		if err := h.Validate(); err != nil {
			return fmt.Errorf("container exec extra host: %w", err)
		}
	}
	return nil
}

// HostServiceAddress returns the hostname containers should use to access
// services on the host machine.
func (r *ContainerRuntime) HostServiceAddress() HostServiceAddress {
	if r.engine.Name() == "podman" {
		return hostContainersInternal
	}
	return hostDockerInternal
}

// GetHostAddressForContainer returns the hostname that containers should use
// to access services on the host machine.
func (r *ContainerRuntime) GetHostAddressForContainer() string {
	return r.HostServiceAddress().String()
}

// CleanupImage removes the built image for an invowkfile
func (r *ContainerRuntime) CleanupImage(ctx *ExecutionContext) error {
	imageTag, err := r.generateImageTag(string(ctx.Invowkfile.FilePath))
	if err != nil {
		return err
	}
	tag := container.ImageTag(imageTag)
	if err := tag.Validate(); err != nil {
		return fmt.Errorf("cleanup image tag: %w", err)
	}
	return r.engine.RemoveImage(ctx.Context, tag, true)
}

// GetEngineName returns the name of the underlying container engine
func (r *ContainerRuntime) GetEngineName() string {
	if r.engine == nil {
		return "none"
	}
	return r.engine.Name()
}
