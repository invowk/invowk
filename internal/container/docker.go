// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// DockerEngine implements the Engine interface using Docker CLI.
// It embeds BaseCLIEngine for common CLI operations.
type DockerEngine struct {
	*BaseCLIEngine
}

// NewDockerEngine creates a new Docker engine.
func NewDockerEngine(opts ...BaseCLIEngineOption) *DockerEngine {
	path, _ := exec.LookPath("docker")
	return &DockerEngine{
		BaseCLIEngine: NewBaseCLIEngine(path, opts...),
	}
}

// Name returns the engine name.
func (e *DockerEngine) Name() string {
	return string(EngineTypeDocker)
}

// Available checks if Docker is available.
func (e *DockerEngine) Available() bool {
	if e.BinaryPath() == "" {
		return false
	}
	cmd := e.CreateCommand(context.Background(), "version", "--format", "{{.Server.Version}}")
	return cmd.Run() == nil
}

// Version returns the Docker version.
func (e *DockerEngine) Version(ctx context.Context) (string, error) {
	out, err := e.RunCommandWithOutput(ctx, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return "", fmt.Errorf("failed to get docker version: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Build builds an image from a Dockerfile.
func (e *DockerEngine) Build(ctx context.Context, opts BuildOptions) error {
	args := e.BuildArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return buildContainerError("docker", opts, err)
	}

	return nil
}

// Run runs a command in a container.
func (e *DockerEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	args := e.RunArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// Remove removes a container.
func (e *DockerEngine) Remove(ctx context.Context, containerID string, force bool) error {
	args := e.RemoveArgs(containerID, force)
	return e.RunCommandStatus(ctx, args...)
}

// ImageExists checks if an image exists.
func (e *DockerEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	err := e.RunCommandStatus(ctx, "image", "inspect", image)
	return err == nil, nil
}

// RemoveImage removes an image.
func (e *DockerEngine) RemoveImage(ctx context.Context, image string, force bool) error {
	args := e.RemoveImageArgs(image, force)
	return e.RunCommandStatus(ctx, args...)
}

// Exec runs a command in a running container.
func (e *DockerEngine) Exec(ctx context.Context, containerID string, command []string, opts RunOptions) (*RunResult, error) {
	args := e.ExecArgs(containerID, command, opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{ContainerID: containerID}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// InspectImage returns information about an image.
func (e *DockerEngine) InspectImage(ctx context.Context, image string) (string, error) {
	return e.RunCommandWithOutput(ctx, "image", "inspect", image)
}

// BuildRunArgs builds the argument slice for a 'run' command without executing.
// Returns the full argument slice including 'run' and all options.
// This is used for interactive mode where the command needs to be attached to a PTY.
// Note: This delegates to BaseCLIEngine.RunArgs() for the actual implementation.
func (e *DockerEngine) BuildRunArgs(opts RunOptions) []string {
	return e.RunArgs(opts)
}
