package container

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PodmanEngine implements the Engine interface using Podman CLI
type PodmanEngine struct {
	binaryPath string
}

// NewPodmanEngine creates a new Podman engine
func NewPodmanEngine() *PodmanEngine {
	path, _ := exec.LookPath("podman")
	return &PodmanEngine{binaryPath: path}
}

// Name returns the engine name
func (e *PodmanEngine) Name() string {
	return "podman"
}

// Available checks if Podman is available
func (e *PodmanEngine) Available() bool {
	if e.binaryPath == "" {
		return false
	}
	cmd := exec.Command(e.binaryPath, "version", "--format", "{{.Version}}")
	return cmd.Run() == nil
}

// Version returns the Podman version
func (e *PodmanEngine) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, "version", "--format", "{{.Version}}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get podman version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Build builds an image from a Dockerfile
func (e *PodmanEngine) Build(ctx context.Context, opts BuildOptions) error {
	args := []string{"build"}

	if opts.Dockerfile != "" {
		// Podman buildah may need the full path to the Dockerfile when using -f flag
		// to properly resolve the file. Join with context directory if relative path.
		dockerfilePath := opts.Dockerfile
		if !filepath.IsAbs(opts.Dockerfile) && opts.ContextDir != "" {
			dockerfilePath = filepath.Join(opts.ContextDir, opts.Dockerfile)
		}
		args = append(args, "-f", dockerfilePath)
	}

	if opts.Tag != "" {
		args = append(args, "-t", opts.Tag)
	}

	if opts.NoCache {
		args = append(args, "--no-cache")
	}

	for k, v := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, opts.ContextDir)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman build failed: %w", err)
	}

	return nil
}

// Run runs a command in a container
func (e *PodmanEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	args := []string{"run"}

	if opts.Remove {
		args = append(args, "--rm")
	}

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}

	if opts.Interactive {
		args = append(args, "-i")
	}

	if opts.TTY {
		args = append(args, "-t")
	}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add volumes with SELinux labels if needed
	for _, v := range opts.Volumes {
		args = append(args, "-v", addSELinuxLabel(v))
	}

	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}

	for _, h := range opts.ExtraHosts {
		args = append(args, "--add-host", h)
	}

	args = append(args, opts.Image)
	args = append(args, opts.Command...)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// Remove removes a container
func (e *PodmanEngine) Remove(ctx context.Context, containerID string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	return cmd.Run()
}

// ImageExists checks if an image exists
func (e *PodmanEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, "image", "exists", image)
	err := cmd.Run()
	return err == nil, nil
}

// RemoveImage removes an image
func (e *PodmanEngine) RemoveImage(ctx context.Context, image string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, image)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	return cmd.Run()
}

// Exec runs a command in a running container
func (e *PodmanEngine) Exec(ctx context.Context, containerID string, command []string, opts RunOptions) (*RunResult, error) {
	args := []string{"exec"}

	if opts.Interactive {
		args = append(args, "-i")
	}

	if opts.TTY {
		args = append(args, "-t")
	}

	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, containerID)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{ContainerID: containerID}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// InspectImage returns information about an image
func (e *PodmanEngine) InspectImage(ctx context.Context, image string) (string, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, "image", "inspect", image)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return out.String(), nil
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
		for _, opt := range strings.Split(options, ",") {
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
