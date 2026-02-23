// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// Container engine type constants.
const (
	// EngineTypePodman identifies the Podman container engine.
	EngineTypePodman EngineType = "podman"
	// EngineTypeDocker identifies the Docker container engine.
	EngineTypeDocker EngineType = "docker"
	// EngineTypeAny is used exclusively in EngineNotAvailableError when
	// AutoDetectEngine fails to find any engine — it is not a valid engine
	// type for normal operations.
	EngineTypeAny EngineType = "any"
)

var (
	// ErrNoEngineAvailable is returned when no container engine (Docker or Podman) is available.
	// Callers can check for this error using errors.Is(err, ErrNoEngineAvailable).
	ErrNoEngineAvailable = errors.New("no container engine available")
	// ErrInvalidEngineType is returned when an EngineType value is not recognized.
	ErrInvalidEngineType = errors.New("invalid engine type")
	// ErrInvalidContainerID is the sentinel error wrapped by InvalidContainerIDError.
	ErrInvalidContainerID = errors.New("invalid container ID")
	// ErrInvalidImageTag is the sentinel error wrapped by InvalidImageTagError.
	ErrInvalidImageTag = errors.New("invalid image tag")
	// ErrInvalidContainerName is the sentinel error wrapped by InvalidContainerNameError.
	ErrInvalidContainerName = errors.New("invalid container name")
	// ErrInvalidHostMapping is the sentinel error wrapped by InvalidHostMappingError.
	ErrInvalidHostMapping = errors.New("invalid host mapping")
)

type (
	// EngineType identifies the container engine type
	EngineType string

	// ContainerID identifies a running or stopped container instance.
	// DDD Value Type — name is intentionally explicit for cross-package clarity.
	ContainerID string //nolint:revive // DDD Value Type pattern: explicit name preferred over stuttering-free abbreviation

	// ImageTag identifies a container image by name and optional tag/digest.
	// Examples: "debian:stable-slim", "invowk-provisioned:abc123"
	ImageTag string

	// ContainerName is a user-assigned name for a container instance.
	// The zero value ("") means "no explicit name" (engine assigns one).
	ContainerName string //nolint:revive // DDD Value Type pattern: explicit name preferred over stuttering-free abbreviation

	// HostMapping is a host-to-IP mapping entry for --add-host (e.g., "host.docker.internal:host-gateway").
	HostMapping string

	// InvalidEngineTypeError is returned when an EngineType value is not recognized.
	// It wraps ErrInvalidEngineType for errors.Is() compatibility.
	InvalidEngineTypeError struct {
		Value EngineType
	}

	// InvalidContainerIDError is returned when a ContainerID value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidContainerID for errors.Is().
	InvalidContainerIDError struct {
		Value ContainerID
	}

	// InvalidImageTagError is returned when an ImageTag value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidImageTag for errors.Is().
	InvalidImageTagError struct {
		Value ImageTag
	}

	// InvalidContainerNameError is returned when a ContainerName value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidContainerName for errors.Is().
	InvalidContainerNameError struct {
		Value ContainerName
	}

	// InvalidHostMappingError is returned when a HostMapping value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidHostMapping for errors.Is().
	InvalidHostMappingError struct {
		Value HostMapping
	}

	// Engine defines the interface for container operations
	Engine interface {
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
		// Remove removes a container by its ID
		Remove(ctx context.Context, containerID ContainerID, force bool) error
		// ImageExists checks if an image exists
		ImageExists(ctx context.Context, image ImageTag) (bool, error)
		// RemoveImage removes an image
		RemoveImage(ctx context.Context, image ImageTag, force bool) error

		// BinaryPath returns the path to the container engine binary.
		// This is used when preparing commands for PTY attachment in interactive mode.
		BinaryPath() string

		// BuildRunArgs builds the argument slice for a 'run' command without executing.
		// Returns the full argument slice including 'run' and all options.
		// This is used for interactive mode where the command needs to be attached to a PTY.
		BuildRunArgs(opts RunOptions) []string
	}

	// BuildOptions contains options for building an image
	BuildOptions struct {
		// ContextDir is the build context directory
		ContextDir HostFilesystemPath
		// Dockerfile is the path to the Dockerfile (relative to ContextDir)
		Dockerfile HostFilesystemPath
		// Tag is the image tag to assign to the built image
		Tag ImageTag
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
	RunOptions struct {
		// Image is the image to run
		Image ImageTag
		// Command is the command to run
		Command []string
		// WorkDir is the working directory inside the container
		WorkDir MountTargetPath
		// Env contains environment variables
		Env map[string]string
		// Volumes are volume mounts in "host:container[:options]" format
		Volumes []invowkfile.VolumeMountSpec
		// Ports are port mappings in "host:container[/protocol]" format
		Ports []invowkfile.PortMappingSpec
		// Remove automatically removes the container after exit
		Remove bool
		// Name is the container name
		Name ContainerName
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
		ExtraHosts []HostMapping
	}

	// RunResult contains the result of running a container
	RunResult struct {
		// ContainerID is the container ID
		ContainerID ContainerID
		// ExitCode is the exit code
		ExitCode types.ExitCode
		// Error contains any error
		Error error
	}

	// EngineNotAvailableError is returned when a container engine is not available
	EngineNotAvailableError struct {
		Engine EngineType
		Reason string
	}
)

func (e *EngineNotAvailableError) Error() string {
	return fmt.Sprintf("container engine '%s' is not available: %s", e.Engine, e.Reason)
}

// Unwrap returns the underlying sentinel error for errors.Is compatibility.
func (e *EngineNotAvailableError) Unwrap() error {
	return ErrNoEngineAvailable
}

// Error implements the error interface for InvalidEngineTypeError.
func (e *InvalidEngineTypeError) Error() string {
	return fmt.Sprintf("invalid engine type %q (valid: podman, docker)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidEngineTypeError) Unwrap() error {
	return ErrInvalidEngineType
}

// String returns the string representation of the ContainerID.
func (c ContainerID) String() string { return string(c) }

// IsValid returns whether the ContainerID is valid.
// A valid ContainerID is non-empty and not whitespace-only.
func (c ContainerID) IsValid() (bool, []error) {
	if strings.TrimSpace(string(c)) == "" {
		return false, []error{&InvalidContainerIDError{Value: c}}
	}
	return true, nil
}

// Error implements the error interface for InvalidContainerIDError.
func (e *InvalidContainerIDError) Error() string {
	return fmt.Sprintf("invalid container ID %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidContainerID for errors.Is() compatibility.
func (e *InvalidContainerIDError) Unwrap() error { return ErrInvalidContainerID }

// String returns the string representation of the ImageTag.
func (t ImageTag) String() string { return string(t) }

// IsValid returns whether the ImageTag is valid.
// A valid ImageTag is non-empty and not whitespace-only.
func (t ImageTag) IsValid() (bool, []error) {
	if strings.TrimSpace(string(t)) == "" {
		return false, []error{&InvalidImageTagError{Value: t}}
	}
	return true, nil
}

// Error implements the error interface for InvalidImageTagError.
func (e *InvalidImageTagError) Error() string {
	return fmt.Sprintf("invalid image tag %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidImageTag for errors.Is() compatibility.
func (e *InvalidImageTagError) Unwrap() error { return ErrInvalidImageTag }

// String returns the string representation of the ContainerName.
func (n ContainerName) String() string { return string(n) }

// IsValid returns whether the ContainerName is valid.
// The zero value ("") is valid (means no explicit name). Non-zero: not whitespace-only.
func (n ContainerName) IsValid() (bool, []error) {
	if n == "" {
		return true, nil
	}
	if strings.TrimSpace(string(n)) == "" {
		return false, []error{&InvalidContainerNameError{Value: n}}
	}
	return true, nil
}

// Error implements the error interface for InvalidContainerNameError.
func (e *InvalidContainerNameError) Error() string {
	return fmt.Sprintf("invalid container name %q: non-empty value must not be whitespace-only", e.Value)
}

// Unwrap returns ErrInvalidContainerName for errors.Is() compatibility.
func (e *InvalidContainerNameError) Unwrap() error { return ErrInvalidContainerName }

// String returns the string representation of the HostMapping.
func (h HostMapping) String() string { return string(h) }

// IsValid returns whether the HostMapping is valid.
// A valid HostMapping is non-empty and not whitespace-only.
func (h HostMapping) IsValid() (bool, []error) {
	if strings.TrimSpace(string(h)) == "" {
		return false, []error{&InvalidHostMappingError{Value: h}}
	}
	return true, nil
}

// Error implements the error interface for InvalidHostMappingError.
func (e *InvalidHostMappingError) Error() string {
	return fmt.Sprintf("invalid host mapping %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidHostMapping for errors.Is() compatibility.
func (e *InvalidHostMappingError) Unwrap() error { return ErrInvalidHostMapping }

// IsValid returns whether the EngineType is one of the defined engine types,
// and a list of validation errors if it is not.
func (et EngineType) IsValid() (bool, []error) {
	switch et {
	case EngineTypePodman, EngineTypeDocker:
		return true, nil
	case EngineTypeAny:
		// EngineTypeAny is only valid in error reporting contexts, not as an engine type
		return false, []error{&InvalidEngineTypeError{Value: et}}
	default:
		return false, []error{&InvalidEngineTypeError{Value: et}}
	}
}

// NewEngine creates a new container engine based on preference.
// The returned engine is automatically wrapped with sandbox awareness
// when running inside Flatpak or Snap sandboxes.
func NewEngine(preferredType EngineType) (Engine, error) {
	if isValid, errs := preferredType.IsValid(); !isValid {
		return nil, errs[0]
	}

	var engine Engine

	switch preferredType {
	case EngineTypePodman:
		podman := NewPodmanEngine()
		if podman.Available() {
			engine = podman
		} else {
			// Fall back to Docker
			docker := NewDockerEngine()
			if docker.Available() {
				engine = docker
			} else {
				return nil, &EngineNotAvailableError{
					Engine: EngineTypePodman,
					Reason: "podman is not installed or not accessible, and docker fallback is also not available",
				}
			}
		}

	case EngineTypeDocker:
		docker := NewDockerEngine()
		if docker.Available() {
			engine = docker
		} else {
			// Fall back to Podman
			podman := NewPodmanEngine()
			if podman.Available() {
				engine = podman
			} else {
				return nil, &EngineNotAvailableError{
					Engine: EngineTypeDocker,
					Reason: "docker is not installed or not accessible, and podman fallback is also not available",
				}
			}
		}

	case EngineTypeAny:
		// Unreachable: IsValid() rejects EngineTypeAny before reaching this switch.
		return nil, fmt.Errorf("EngineTypeAny is not a valid engine type for initialization")

	default:
		// Unreachable: IsValid() guard above ensures only valid types reach here.
		return nil, fmt.Errorf("unknown container engine type: %s", preferredType)
	}

	// Wrap with sandbox awareness for Flatpak/Snap environments
	return NewSandboxAwareEngine(engine), nil
}

// CloseEngine releases resources held by a container engine. It is a no-op for
// engines that don't implement EngineCloser. Safe to call with nil.
func CloseEngine(engine Engine) error {
	if c, ok := engine.(EngineCloser); ok {
		return c.Close()
	}
	return nil
}

// AutoDetectEngine tries to find an available container engine.
// The returned engine is automatically wrapped with sandbox awareness
// when running inside Flatpak or Snap sandboxes.
func AutoDetectEngine() (Engine, error) {
	// Try Podman first (more commonly available in rootless setups)
	podman := NewPodmanEngine()
	if podman.Available() {
		return NewSandboxAwareEngine(podman), nil
	}

	// Try Docker
	docker := NewDockerEngine()
	if docker.Available() {
		return NewSandboxAwareEngine(docker), nil
	}

	return nil, &EngineNotAvailableError{
		Engine: EngineTypeAny,
		Reason: "no container engine (podman or docker) is available on this system",
	}
}
