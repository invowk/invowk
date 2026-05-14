// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
	"unicode"

	"github.com/invowk/invowk/pkg/containerargs"
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

	// availabilityTimeout is the maximum time to wait for a container engine
	// to respond during availability checks. Prevents indefinite hangs when
	// the engine daemon is unresponsive or starting up.
	availabilityTimeout = 3 * time.Second

	// engineUnavailablePodmanThenDocker explains Podman-first initialization failures.
	// Note: shell aliases are not used by binary discovery (exec.LookPath).
	engineUnavailablePodmanThenDocker = "podman/podman-remote is not installed or not accessible, and docker fallback is also not available; shell aliases are not considered for engine discovery"
	// engineUnavailableDockerThenPodman explains Docker-first initialization failures.
	// Note: shell aliases are not used by binary discovery (exec.LookPath).
	engineUnavailableDockerThenPodman = "docker is not installed or not accessible, and podman/podman-remote fallback is also not available; shell aliases are not considered for engine discovery"
	// engineUnavailableAutoDetect explains automatic engine detection failures.
	engineUnavailableAutoDetect = "no container engine (podman, podman-remote, or docker) is available on this system; shell aliases are not considered for engine discovery"
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
	ErrInvalidContainerName = containerargs.ErrInvalidContainerName
	// ErrInvalidHostMapping is the sentinel error wrapped by InvalidHostMappingError.
	ErrInvalidHostMapping = errors.New("invalid host mapping")
	// ErrInvalidBuildOptions is the sentinel error wrapped by InvalidBuildOptionsError.
	ErrInvalidBuildOptions = errors.New("invalid build options")
	// ErrInvalidRunOptions is the sentinel error wrapped by InvalidRunOptionsError.
	ErrInvalidRunOptions = errors.New("invalid run options")
	// ErrInvalidCreateOptions is the sentinel error wrapped by InvalidCreateOptionsError.
	ErrInvalidCreateOptions = errors.New("invalid create options")
	// ErrContainerNotFound is returned when a named container does not exist.
	ErrContainerNotFound = errors.New("container not found")
	// ErrContainerNameConflict is returned when a container name is already in use.
	ErrContainerNameConflict = errors.New("container name conflict")
	// ErrContainerOperationFailed is wrapped by container engine operation failures.
	ErrContainerOperationFailed = errors.New("container operation failed")
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

	// ContainerName is a user-assigned portable name for a container instance.
	// The zero value ("") means "no explicit name" (engine assigns one).
	ContainerName = containerargs.ContainerName //nolint:revive // DDD value type keeps the domain noun explicit across package boundaries.

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
	InvalidContainerNameError = containerargs.InvalidContainerNameError

	// InvalidHostMappingError is returned when a HostMapping value is invalid.
	// DDD Value Type error struct — wraps ErrInvalidHostMapping for errors.Is().
	InvalidHostMappingError struct {
		Value HostMapping
	}

	// InvalidBuildOptionsError is returned when BuildOptions has one or more invalid fields.
	// It wraps ErrInvalidBuildOptions for errors.Is() compatibility.
	InvalidBuildOptionsError struct {
		FieldErrors []error
	}

	// InvalidRunOptionsError is returned when RunOptions has one or more invalid fields.
	// It wraps ErrInvalidRunOptions for errors.Is() compatibility.
	InvalidRunOptionsError struct {
		FieldErrors []error
	}

	// InvalidCreateOptionsError is returned when CreateOptions has one or more invalid fields.
	// It wraps ErrInvalidCreateOptions for errors.Is() compatibility.
	InvalidCreateOptionsError struct {
		FieldErrors []error
	}

	// ContainerNotFoundError is returned when a named container does not exist.
	ContainerNotFoundError struct { //nolint:revive // DDD error type mirrors the sentinel for errors.As clarity.
		Name ContainerName
	}

	// ContainerNameConflictError is returned when a container name is already in use.
	ContainerNameConflictError struct { //nolint:revive // DDD error type mirrors the sentinel for errors.As clarity.
		Name ContainerName
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
		// InspectContainer inspects a container by name
		InspectContainer(ctx context.Context, name ContainerName) (*ContainerInfo, error)
		// Create creates a stopped container
		Create(ctx context.Context, opts CreateOptions) (*CreateResult, error)
		// Start starts a stopped container by ID or name
		Start(ctx context.Context, containerID ContainerID) error
		// Exec runs a command in a running container
		Exec(ctx context.Context, containerID ContainerID, command []string, opts RunOptions) (*RunResult, error)
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

		// PrepareRunCommand creates a configured command for a container run.
		PrepareRunCommand(ctx context.Context, opts RunOptions) *exec.Cmd
	}

	//goplint:validate-all
	//
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

	//goplint:validate-all
	//
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
		// Volumes are volume mounts in "host:container[:options]" format.
		Volumes []VolumeMountSpec
		// Ports are port mappings in Docker/Podman "-p" format.
		Ports []PortMappingSpec
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

	//goplint:validate-all
	//
	// CreateOptions contains options for creating a persistent container.
	CreateOptions struct {
		// Image is the image to create the container from.
		Image ImageTag //goplint:ignore -- required for container creation; CreateOptions.Validate enforces non-empty validity.
		// Command is the idle command to keep the container alive after start.
		Command []string //goplint:ignore -- exec boundary (container idle command argv).
		// WorkDir is the default working directory inside the container.
		WorkDir MountTargetPath //goplint:ignore -- optional creation-time working directory validated when non-empty.
		// Env contains creation-time environment variables.
		Env map[string]string //goplint:ignore -- exec/OS boundary (container creation env map).
		// Labels are container metadata labels.
		Labels map[string]string //goplint:ignore -- container labels are stringly typed by Docker/Podman APIs.
		// Volumes are volume mounts in "host:container[:options]" format.
		Volumes []VolumeMountSpec
		// Ports are port mappings in Docker/Podman "-p" format.
		Ports []PortMappingSpec
		// Name is the container name.
		Name ContainerName
		// ExtraHosts are additional host-to-IP mappings.
		ExtraHosts []HostMapping
	}

	// CreateResult contains the result of creating a container.
	CreateResult struct {
		// ContainerID is the created container ID.
		ContainerID ContainerID //goplint:ignore -- required result field; CreateResult.Validate enforces non-empty validity.
	}

	//goplint:validate-all
	//
	// RunResult contains the result of running a container
	RunResult struct {
		// ContainerID is the container ID
		ContainerID ContainerID
		// ExitCode is the exit code
		ExitCode types.ExitCode
		// Error contains any error
		Error error
	}

	// ContainerInfo contains inspect metadata for a container.
	ContainerInfo struct { //nolint:revive // Domain DTO name stays explicit when referenced outside this package.
		// ContainerID is the container ID.
		ContainerID ContainerID //goplint:ignore -- required inspect result field; ContainerInfo.Validate enforces non-empty validity.
		// Name is the container name.
		Name ContainerName
		// Running reports whether the container is currently running.
		Running bool
		// Status is the engine-specific status string.
		Status string //goplint:ignore -- display-only engine status text.
		// Labels contains container metadata labels.
		Labels map[string]string //goplint:ignore -- container labels are stringly typed by Docker/Podman APIs.
	}

	// EngineNotAvailableError is returned when a container engine is not available
	EngineNotAvailableError struct {
		Engine EngineType
		Reason string
	}

	// OperationError describes a failed container engine operation.
	OperationError struct {
		Engine    string
		Operation string
		Resource  string
		Err       error
	}

	engineDiscovery interface {
		NewPodman() Engine
		NewDocker() Engine
	}

	defaultEngineDiscovery struct{}
)

func (e *EngineNotAvailableError) Error() string {
	return fmt.Sprintf("container engine '%s' is not available: %s", e.Engine, e.Reason)
}

// Unwrap returns the underlying sentinel error for errors.Is compatibility.
func (e *EngineNotAvailableError) Unwrap() error {
	return ErrNoEngineAvailable
}

func (e *OperationError) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("%s %s failed for %s: %v", e.Engine, e.Operation, e.Resource, e.Err)
	}
	return fmt.Sprintf("%s %s failed: %v", e.Engine, e.Operation, e.Err)
}

func (e *OperationError) Unwrap() error {
	return errors.Join(ErrContainerOperationFailed, e.Err)
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

// Validate returns an error if the ContainerID is invalid.
// A valid ContainerID is non-empty and not whitespace-only.
//
//goplint:nonzero
func (c ContainerID) Validate() error {
	if strings.TrimSpace(string(c)) == "" {
		return &InvalidContainerIDError{Value: c}
	}
	return nil
}

// Error implements the error interface for InvalidContainerIDError.
func (e *InvalidContainerIDError) Error() string {
	return fmt.Sprintf("invalid container ID %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidContainerID for errors.Is() compatibility.
func (e *InvalidContainerIDError) Unwrap() error { return ErrInvalidContainerID }

// String returns the string representation of the ImageTag.
func (t ImageTag) String() string { return string(t) }

// Validate returns an error if the ImageTag is invalid.
// A valid ImageTag is non-empty and contains no whitespace or control characters.
//
//goplint:nonzero
func (t ImageTag) Validate() error {
	tag := string(t)
	if strings.TrimSpace(tag) == "" {
		return &InvalidImageTagError{Value: t}
	}
	for _, r := range tag {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return &InvalidImageTagError{Value: t}
		}
	}
	return nil
}

// Error implements the error interface for InvalidImageTagError.
func (e *InvalidImageTagError) Error() string {
	return fmt.Sprintf("invalid image tag %q: must be non-empty and contain no whitespace or control characters", e.Value)
}

// Unwrap returns ErrInvalidImageTag for errors.Is() compatibility.
func (e *InvalidImageTagError) Unwrap() error { return ErrInvalidImageTag }

// String returns the string representation of the HostMapping.
func (h HostMapping) String() string { return string(h) }

// Validate returns an error if the HostMapping is invalid.
// A valid HostMapping is non-empty and not whitespace-only.
//
//goplint:nonzero
func (h HostMapping) Validate() error {
	if strings.TrimSpace(string(h)) == "" {
		return &InvalidHostMappingError{Value: h}
	}
	return nil
}

// Error implements the error interface for InvalidHostMappingError.
func (e *InvalidHostMappingError) Error() string {
	return fmt.Sprintf("invalid host mapping %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidHostMapping for errors.Is() compatibility.
func (e *InvalidHostMappingError) Unwrap() error { return ErrInvalidHostMapping }

// Error implements the error interface for InvalidBuildOptionsError.
func (e *InvalidBuildOptionsError) Error() string {
	return types.FormatFieldErrors("build options", e.FieldErrors)
}

// Unwrap returns ErrInvalidBuildOptions for errors.Is() compatibility.
func (e *InvalidBuildOptionsError) Unwrap() error { return ErrInvalidBuildOptions }

// Validate returns an error if any typed field of the BuildOptions is invalid.
// Validates ContextDir, Dockerfile, and Tag.
// Dockerfile and Tag use zero-value-is-valid semantics: empty means "use default".
func (o BuildOptions) Validate() error {
	var errs []error
	if err := o.ContextDir.Validate(); err != nil {
		errs = append(errs, err)
	}
	if o.Dockerfile != "" {
		if err := o.Dockerfile.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.Tag != "" {
		if err := o.Tag.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidBuildOptionsError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidRunOptionsError.
func (e *InvalidRunOptionsError) Error() string {
	return types.FormatFieldErrors("run options", e.FieldErrors)
}

// Unwrap returns ErrInvalidRunOptions for errors.Is() compatibility.
func (e *InvalidRunOptionsError) Unwrap() error { return ErrInvalidRunOptions }

// Error implements the error interface for InvalidCreateOptionsError.
func (e *InvalidCreateOptionsError) Error() string {
	return types.FormatFieldErrors("create options", e.FieldErrors)
}

// Unwrap returns ErrInvalidCreateOptions for errors.Is() compatibility.
func (e *InvalidCreateOptionsError) Unwrap() error { return ErrInvalidCreateOptions }

// Error implements the error interface for ContainerNotFoundError.
func (e *ContainerNotFoundError) Error() string {
	return fmt.Sprintf("container %q not found", e.Name)
}

// Unwrap returns ErrContainerNotFound for errors.Is() compatibility.
func (e *ContainerNotFoundError) Unwrap() error { return ErrContainerNotFound }

// Error implements the error interface for ContainerNameConflictError.
func (e *ContainerNameConflictError) Error() string {
	return fmt.Sprintf("container name %q is already in use", e.Name)
}

// Unwrap returns ErrContainerNameConflict for errors.Is() compatibility.
func (e *ContainerNameConflictError) Unwrap() error { return ErrContainerNameConflict }

// Validate returns an error if any typed field of the RunOptions is invalid.
// Validates Image, WorkDir, Name, ExtraHosts, Volumes, and Ports.
func (o RunOptions) Validate() error {
	var errs []error
	if err := o.Image.Validate(); err != nil {
		errs = append(errs, err)
	}
	if o.WorkDir != "" {
		if err := o.WorkDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := o.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	for _, h := range o.ExtraHosts {
		if err := h.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, v := range o.Volumes {
		if err := v.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, p := range o.Ports {
		if err := p.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidRunOptionsError{FieldErrors: errs}
	}
	return nil
}

// Validate returns an error if any typed field of the CreateOptions is invalid.
func (o CreateOptions) Validate() error {
	var errs []error
	if err := o.Image.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := o.Name.Validate(); err != nil {
		errs = append(errs, err)
	}
	if o.Name == "" {
		errs = append(errs, errors.New("container name is required"))
	}
	if len(o.Command) == 0 {
		errs = append(errs, errors.New("container command is required"))
	}
	if o.WorkDir != "" {
		if err := o.WorkDir.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, h := range o.ExtraHosts {
		if err := h.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, v := range o.Volumes {
		if err := v.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, p := range o.Ports {
		if err := p.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidCreateOptionsError{FieldErrors: errs}
	}
	return nil
}

// String returns the string representation of the EngineType.
func (et EngineType) String() string { return string(et) }

// Validate returns an error if the EngineType is not one of the defined engine types.
func (et EngineType) Validate() error {
	switch et {
	case EngineTypePodman, EngineTypeDocker:
		return nil
	case EngineTypeAny:
		// EngineTypeAny is only valid in error reporting contexts, not as an engine type
		return &InvalidEngineTypeError{Value: et}
	default:
		return &InvalidEngineTypeError{Value: et}
	}
}

// NewEngine creates a new container engine based on preference.
// The returned engine is automatically wrapped with sandbox awareness
// when running inside Flatpak or Snap sandboxes.
func NewEngine(preferredType EngineType) (Engine, error) {
	return newEngineWithDiscovery(preferredType, defaultEngineDiscovery{})
}

func newEngineWithDiscovery(preferredType EngineType, discovery engineDiscovery) (Engine, error) {
	if err := preferredType.Validate(); err != nil {
		return nil, err
	}

	var engine Engine

	switch preferredType {
	case EngineTypePodman:
		podman := NewSandboxAwareEngine(discovery.NewPodman())
		if podman.Available() {
			engine = podman
		} else if docker := NewSandboxAwareEngine(discovery.NewDocker()); docker.Available() {
			// Fall back to Docker
			engine = docker
		} else {
			return nil, &EngineNotAvailableError{
				Engine: EngineTypePodman,
				Reason: engineUnavailablePodmanThenDocker,
			}
		}

	case EngineTypeDocker:
		docker := NewSandboxAwareEngine(discovery.NewDocker())
		if docker.Available() {
			engine = docker
		} else if podman := NewSandboxAwareEngine(discovery.NewPodman()); podman.Available() {
			// Fall back to Podman
			engine = podman
		} else {
			return nil, &EngineNotAvailableError{
				Engine: EngineTypeDocker,
				Reason: engineUnavailableDockerThenPodman,
			}
		}

	case EngineTypeAny:
		// Unreachable: Validate() rejects EngineTypeAny before reaching this switch.
		return nil, errors.New("EngineTypeAny is not a valid engine type for initialization")

	default:
		// Unreachable: Validate() guard above ensures only valid types reach here.
		return nil, fmt.Errorf("unknown container engine type: %s", preferredType)
	}

	return engine, nil
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
	return autoDetectEngineWithDiscovery(defaultEngineDiscovery{})
}

func autoDetectEngineWithDiscovery(discovery engineDiscovery) (Engine, error) {
	// Try Podman first (more commonly available in rootless setups)
	podman := NewSandboxAwareEngine(discovery.NewPodman())
	if podman.Available() {
		return podman, nil
	}

	// Try Docker
	docker := NewSandboxAwareEngine(discovery.NewDocker())
	if docker.Available() {
		return docker, nil
	}

	return nil, &EngineNotAvailableError{
		Engine: EngineTypeAny,
		Reason: engineUnavailableAutoDetect,
	}
}

func (defaultEngineDiscovery) NewPodman() Engine {
	return NewPodmanEngine()
}

func (defaultEngineDiscovery) NewDocker() Engine {
	return NewDockerEngine()
}
