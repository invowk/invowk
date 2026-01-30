// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PodmanEngine implements the Engine interface using Podman CLI.
// It embeds BaseCLIEngine for common CLI operations.
type PodmanEngine struct {
	*BaseCLIEngine
}

// NewPodmanEngine creates a new Podman engine.
// On Linux with SELinux enabled, volume mounts are automatically labeled with :z.
func NewPodmanEngine(opts ...BaseCLIEngineOption) *PodmanEngine {
	path, _ := exec.LookPath("podman")

	// Podman needs SELinux volume labels on Linux (prepend to user options)
	allOpts := append([]BaseCLIEngineOption{WithVolumeFormatter(addSELinuxLabel)}, opts...)

	return &PodmanEngine{
		BaseCLIEngine: NewBaseCLIEngine(path, allOpts...),
	}
}

// Name returns the engine name.
func (e *PodmanEngine) Name() string {
	return string(EngineTypePodman)
}

// Available checks if Podman is available.
func (e *PodmanEngine) Available() bool {
	if e.BinaryPath() == "" {
		return false
	}
	cmd := e.CreateCommand(context.Background(), "version", "--format", "{{.Version}}")
	return cmd.Run() == nil
}

// Version returns the Podman version.
func (e *PodmanEngine) Version(ctx context.Context) (string, error) {
	out, err := e.RunCommandWithOutput(ctx, "version", "--format", "{{.Version}}")
	if err != nil {
		return "", fmt.Errorf("failed to get podman version: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// Build builds an image from a Dockerfile.
func (e *PodmanEngine) Build(ctx context.Context, opts BuildOptions) error {
	args := e.BuildArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return buildContainerError("podman", opts, err)
	}

	return nil
}

// Run runs a command in a container.
// Volume mounts are automatically labeled with SELinux labels if needed.
func (e *PodmanEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
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
func (e *PodmanEngine) Remove(ctx context.Context, containerID string, force bool) error {
	args := e.RemoveArgs(containerID, force)
	return e.RunCommandStatus(ctx, args...)
}

// ImageExists checks if an image exists.
func (e *PodmanEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	err := e.RunCommandStatus(ctx, "image", "exists", image)
	return err == nil, nil
}

// RemoveImage removes an image.
func (e *PodmanEngine) RemoveImage(ctx context.Context, image string, force bool) error {
	args := e.RemoveImageArgs(image, force)
	return e.RunCommandStatus(ctx, args...)
}

// Exec runs a command in a running container.
func (e *PodmanEngine) Exec(ctx context.Context, containerID string, command []string, opts RunOptions) (*RunResult, error) {
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
func (e *PodmanEngine) InspectImage(ctx context.Context, image string) (string, error) {
	return e.RunCommandWithOutput(ctx, "image", "inspect", image)
}

// isSELinuxEnabled checks if SELinux is enabled on the system
func isSELinuxEnabled() bool {
	// Check /sys/fs/selinux/enforce for SELinux status
	data, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// addSELinuxLabel adds the :z label to a volume mount if SELinux is enabled
// and the volume doesn't already have an SELinux label (:z or :Z)
func addSELinuxLabel(volume string) string {
	if !isSELinuxEnabled() {
		return volume
	}

	// Parse the volume string to check if it already has SELinux labels
	// Volume format: host_path:container_path[:options]
	// Options can include: ro, rw, z, Z, and others
	parts := strings.Split(volume, ":")

	// Need at least host:container
	if len(parts) < 2 {
		return volume
	}

	// Check if options already contain SELinux label
	if len(parts) >= 3 {
		options := parts[len(parts)-1]
		// Check for :z or :Z in options
		for opt := range strings.SplitSeq(options, ",") {
			if opt == "z" || opt == "Z" {
				// Already has SELinux label
				return volume
			}
		}
		// Append :z to existing options
		return volume + ",z"
	}

	// No options specified, add :z
	return volume + ":z"
}

// BuildRunArgs builds the argument slice for a 'run' command without executing.
// Returns the full argument slice including 'run' and all options.
// This is used for interactive mode where the command needs to be attached to a PTY.
// Note: Volume mounts are automatically labeled with SELinux labels if needed
// (via the volume formatter set in the constructor).
func (e *PodmanEngine) BuildRunArgs(opts RunOptions) []string {
	return e.RunArgs(opts)
}
