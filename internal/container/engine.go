// SPDX-License-Identifier: EPL-2.0

// Package container provides an abstraction layer for container runtimes (Docker/Podman).
package container

import (
	"context"
	"fmt"
	"io"
)

// Engine defines the interface for container operations
type Engine interface {
	// Name returns the engine name (docker or podman)
	Name() string
	// Available checks if the engine is available on the system
	Available() bool
	// Version returns the engine version
	Version(ctx context.Context) (string, error)

	// Build builds an image from a Dockerfile
	Build(ctx context.Context, opts BuildOptions) error
	// Run runs a command in a container
	Run(ctx context.Context, opts RunOptions) (*RunResult, error)
	// Remove removes a container
	Remove(ctx context.Context, containerID string, force bool) error
	// ImageExists checks if an image exists
	ImageExists(ctx context.Context, image string) (bool, error)
	// RemoveImage removes an image
	RemoveImage(ctx context.Context, image string, force bool) error
}

// BuildOptions contains options for building an image
type BuildOptions struct {
	// ContextDir is the build context directory
	ContextDir string
	// Dockerfile is the path to the Dockerfile (relative to ContextDir)
	Dockerfile string
	// Tag is the image tag
	Tag string
	// BuildArgs are build-time variables
	BuildArgs map[string]string
	// NoCache disables the build cache
	NoCache bool
	// Stdout is where to write build output
	Stdout io.Writer
	// Stderr is where to write build errors
	Stderr io.Writer
}

// RunOptions contains options for running a container
type RunOptions struct {
	// Image is the image to run
	Image string
	// Command is the command to run
	Command []string
	// WorkDir is the working directory inside the container
	WorkDir string
	// Env contains environment variables
	Env map[string]string
	// Volumes are volume mounts in "host:container" format
	Volumes []string
	// Ports are port mappings in "host:container" format
	Ports []string
	// Remove automatically removes the container after exit
	Remove bool
	// Name is the container name
	Name string
	// Stdin is the standard input
	Stdin io.Reader
	// Stdout is where to write standard output
	Stdout io.Writer
	// Stderr is where to write standard error
	Stderr io.Writer
	// Interactive enables interactive mode
	Interactive bool
	// TTY allocates a pseudo-TTY
	TTY bool
	// ExtraHosts are additional host-to-IP mappings (e.g., "host.docker.internal:host-gateway")
	ExtraHosts []string
}

// RunResult contains the result of running a container
type RunResult struct {
	// ContainerID is the container ID
	ContainerID string
	// ExitCode is the exit code
	ExitCode int
	// Error contains any error
	Error error
}

// EngineType identifies the container engine type
type EngineType string

const (
	EngineTypePodman EngineType = "podman"
	EngineTypeDocker EngineType = "docker"
)

// ErrEngineNotAvailable is returned when a container engine is not available
type ErrEngineNotAvailable struct {
	Engine string
	Reason string
}

func (e *ErrEngineNotAvailable) Error() string {
	return fmt.Sprintf("container engine '%s' is not available: %s", e.Engine, e.Reason)
}

// NewEngine creates a new container engine based on preference
func NewEngine(preferredType EngineType) (Engine, error) {
	switch preferredType {
	case EngineTypePodman:
		engine := NewPodmanEngine()
		if engine.Available() {
			return engine, nil
		}
		// Fall back to Docker
		dockerEngine := NewDockerEngine()
		if dockerEngine.Available() {
			return dockerEngine, nil
		}
		return nil, &ErrEngineNotAvailable{
			Engine: "podman",
			Reason: "podman is not installed or not accessible, and docker fallback is also not available",
		}

	case EngineTypeDocker:
		engine := NewDockerEngine()
		if engine.Available() {
			return engine, nil
		}
		// Fall back to Podman
		podmanEngine := NewPodmanEngine()
		if podmanEngine.Available() {
			return podmanEngine, nil
		}
		return nil, &ErrEngineNotAvailable{
			Engine: "docker",
			Reason: "docker is not installed or not accessible, and podman fallback is also not available",
		}

	default:
		return nil, fmt.Errorf("unknown container engine type: %s", preferredType)
	}
}

// AutoDetectEngine tries to find an available container engine
func AutoDetectEngine() (Engine, error) {
	// Try Podman first (more commonly available in rootless setups)
	podman := NewPodmanEngine()
	if podman.Available() {
		return podman, nil
	}

	// Try Docker
	docker := NewDockerEngine()
	if docker.Available() {
		return docker, nil
	}

	return nil, &ErrEngineNotAvailable{
		Engine: "any",
		Reason: "no container engine (podman or docker) is available on this system",
	}
}
