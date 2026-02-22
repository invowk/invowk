// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
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
	allOpts := []BaseCLIEngineOption{WithName(string(EngineTypeDocker))}
	allOpts = append(allOpts, opts...)
	return &DockerEngine{
		BaseCLIEngine: NewBaseCLIEngine(path, allOpts...),
	}
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

// ImageExists checks if an image exists.
// Docker uses "image inspect" which returns detailed JSON on success.
func (e *DockerEngine) ImageExists(ctx context.Context, image string) (bool, error) {
	err := e.RunCommandStatus(ctx, "image", "inspect", image)
	return err == nil, nil
}
