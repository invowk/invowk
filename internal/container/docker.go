package container

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// DockerEngine implements the Engine interface using Docker CLI
type DockerEngine struct {
	binaryPath string
}

// NewDockerEngine creates a new Docker engine
func NewDockerEngine() *DockerEngine {
	path, _ := exec.LookPath("docker")
	return &DockerEngine{binaryPath: path}
}

// Name returns the engine name
func (e *DockerEngine) Name() string {
	return "docker"
}

// Available checks if Docker is available
func (e *DockerEngine) Available() bool {
	if e.binaryPath == "" {
		return false
	}
	cmd := exec.Command(e.binaryPath, "version", "--format", "{{.Server.Version}}")
	return cmd.Run() == nil
}

// Version returns the Docker version
func (e *DockerEngine) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, "version", "--format", "{{.Server.Version}}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get docker version: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Build builds an image from a Dockerfile
func (e *DockerEngine) Build(ctx context.Context, opts BuildOptions) error {
	args := []string{"build"}

	if opts.Dockerfile != "" {
		// Docker buildx requires the full path to the Dockerfile when using -f flag
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
		return fmt.Errorf("docker build failed: %w", err)
	}

	return nil
}

// Run runs a command in a container
func (e *DockerEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
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

	for _, v := range opts.Volumes {
		args = append(args, "-v", v)
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
func (e *DockerEngine) Remove(ctx context.Context, containerID string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	return cmd.Run()
}

// ImageExists checks if an image exists
func (e *DockerEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, "image", "inspect", image)
	err := cmd.Run()
	return err == nil, nil
}

// RemoveImage removes an image
func (e *DockerEngine) RemoveImage(ctx context.Context, image string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, image)

	cmd := exec.CommandContext(ctx, e.binaryPath, args...)
	return cmd.Run()
}

// Exec runs a command in a running container
func (e *DockerEngine) Exec(ctx context.Context, containerID string, command []string, opts RunOptions) (*RunResult, error) {
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
func (e *DockerEngine) InspectImage(ctx context.Context, image string) (string, error) {
	cmd := exec.CommandContext(ctx, e.binaryPath, "image", "inspect", image)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return out.String(), nil
}
