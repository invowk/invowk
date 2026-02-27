// SPDX-License-Identifier: MPL-2.0

package container

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// PortProtocolTCP is the TCP transport protocol for port mappings.
	PortProtocolTCP PortProtocol = "tcp"
	// PortProtocolUDP is the UDP transport protocol for port mappings.
	PortProtocolUDP PortProtocol = "udp"

	// SELinuxLabelNone means no SELinux label is applied to volume mounts.
	SELinuxLabelNone SELinuxLabel = ""
	// SELinuxLabelShared allows sharing the volume between containers.
	SELinuxLabelShared SELinuxLabel = "z"
	// SELinuxLabelPrivate restricts the volume to a single container.
	SELinuxLabelPrivate SELinuxLabel = "Z"
)

var (
	// ErrInvalidPortProtocol is the sentinel error wrapped by InvalidPortProtocolError.
	ErrInvalidPortProtocol = errors.New("invalid port protocol")

	// ErrInvalidSELinuxLabel is the sentinel error wrapped by InvalidSELinuxLabelError.
	ErrInvalidSELinuxLabel = errors.New("invalid SELinux label")

	// ErrInvalidNetworkPort is the sentinel error wrapped by InvalidNetworkPortError.
	ErrInvalidNetworkPort = errors.New("invalid network port")

	// ErrInvalidHostFilesystemPath is the sentinel error wrapped by InvalidHostFilesystemPathError.
	ErrInvalidHostFilesystemPath = errors.New("invalid host filesystem path")

	// ErrInvalidMountTargetPath is the sentinel error wrapped by InvalidMountTargetPathError.
	ErrInvalidMountTargetPath = errors.New("invalid container filesystem path")

	// ErrInvalidVolumeMount is the sentinel error wrapped by InvalidVolumeMountError.
	ErrInvalidVolumeMount = errors.New("invalid volume mount")

	// ErrInvalidPortMapping is the sentinel error wrapped by InvalidPortMappingError.
	ErrInvalidPortMapping = errors.New("invalid port mapping")
)

type (
	// ExecCommandFunc is the function signature for creating exec.Cmd.
	// This allows injection of mock implementations for testing.
	ExecCommandFunc func(ctx context.Context, name string, arg ...string) *exec.Cmd

	// VolumeFormatFunc is a function that formats a volume mount spec as a string.
	// Podman uses this to add SELinux labels (:z/:Z) which are required in
	// SELinux-enforcing environments for proper volume isolation — without them,
	// container processes cannot access bind-mounted host paths.
	VolumeFormatFunc func(volume invowkfile.VolumeMountSpec) string

	// SELinuxCheckFunc is a function that checks if SELinux is enabled.
	// This allows injection of mock implementations for testing.
	SELinuxCheckFunc func() bool

	// RunArgsTransformer modifies run arguments after they're built.
	// Used by Podman to inject --userns=keep-id for rootless compatibility.
	RunArgsTransformer func(args []string) []string

	// BaseCLIEngineOption configures a BaseCLIEngine.
	BaseCLIEngineOption func(*BaseCLIEngine)

	// BaseCLIEngine provides common implementation for CLI-based container engines.
	// Docker and Podman engines embed this struct. Methods that are identical across
	// all CLI engines (Build, Run, Exec, Remove, RemoveImage, BuildRunArgs, InspectImage)
	// are implemented here; engine-specific methods (Available, Version, ImageExists)
	// remain on the concrete types.
	BaseCLIEngine struct {
		name string // Engine name for error messages (e.g., "docker", "podman")
		//plint:internal -- resolved at construction via exec.LookPath; not user-configurable
		binaryPath         HostFilesystemPath
		execCommand        ExecCommandFunc
		volumeFormatter    VolumeFormatFunc
		runArgsTransformer RunArgsTransformer
		//plint:internal -- injected by sysctl override setup; not a constructor option
		cmdEnvOverrides      map[string]string  // Per-command env var overrides (e.g., CONTAINERS_CONF_OVERRIDE)
		sysctlOverridePath   HostFilesystemPath // Temp file path for sysctl override (removed on Close)
		sysctlOverrideActive bool               // Whether the temp file sysctl override is in effect
	}

	// CmdCustomizer is implemented by engines that inject per-command overrides
	// (environment variables). Used by the runtime package to propagate overrides
	// to commands created outside the engine (e.g., interactive mode PTY commands).
	CmdCustomizer interface {
		CustomizeCmd(cmd *exec.Cmd)
	}

	// SysctlOverrideChecker is implemented by engines that may use a temp-file-based
	// CONTAINERS_CONF_OVERRIDE to prevent the rootless Podman ping_group_range race.
	// The runtime package uses this to decide whether run-level serialization (flock
	// or mutex fallback) is needed: if the override is active, the race is eliminated
	// at source and no serialization is needed; otherwise, runs must be serialized.
	//
	// Only PodmanEngine implements this interface. DockerEngine does not (Docker is
	// not susceptible to the ping_group_range race). SandboxAwareEngine forwards
	// to the wrapped engine.
	SysctlOverrideChecker interface {
		SysctlOverrideActive() bool
	}

	// EngineCloser is implemented by engines that hold resources requiring cleanup
	// (e.g., sysctl override temp files). Engines that don't hold resources
	// (e.g., DockerEngine) don't implement this interface.
	EngineCloser interface {
		Close() error
	}

	// BaseCLIProvider is implemented by engines that embed BaseCLIEngine.
	// Enables SandboxAwareEngine to access arg-building methods without
	// a concrete type switch, making it safe to add new engine types.
	BaseCLIProvider interface {
		BaseCLI() *BaseCLIEngine
	}

	// PortProtocol represents a network transport protocol for port mappings.
	// The zero value ("") is valid and means "default to tcp".
	PortProtocol string

	// InvalidPortProtocolError is returned when a PortProtocol is not a recognized protocol.
	InvalidPortProtocolError struct {
		Value PortProtocol
	}

	// SELinuxLabel represents an SELinux volume labeling option.
	// The zero value ("") means no SELinux label is applied.
	SELinuxLabel string

	// InvalidSELinuxLabelError is returned when an SELinuxLabel is not a recognized label.
	InvalidSELinuxLabelError struct {
		Value SELinuxLabel
	}

	// NetworkPort represents a TCP/UDP port number for container port mappings.
	// A valid port must be greater than zero.
	NetworkPort uint16

	// InvalidNetworkPortError is returned when a NetworkPort value is zero.
	InvalidNetworkPortError struct {
		Value NetworkPort
	}

	// HostFilesystemPath represents a filesystem path on the host for volume mounts.
	// A valid path must be non-empty and not whitespace-only.
	HostFilesystemPath string

	// InvalidHostFilesystemPathError is returned when a HostFilesystemPath is empty or whitespace-only.
	InvalidHostFilesystemPathError struct {
		Value HostFilesystemPath
	}

	// MountTargetPath represents a filesystem path inside a container for volume mounts.
	// A valid path must be non-empty and not whitespace-only.
	MountTargetPath string

	// InvalidMountTargetPathError is returned when a MountTargetPath is empty or whitespace-only.
	InvalidMountTargetPathError struct {
		Value MountTargetPath
	}

	// VolumeMount represents a volume mount specification.
	VolumeMount struct {
		HostPath      HostFilesystemPath
		ContainerPath MountTargetPath
		ReadOnly      bool
		SELinux       SELinuxLabel
	}

	// PortMapping represents a port mapping specification.
	PortMapping struct {
		HostPort      NetworkPort
		ContainerPort NetworkPort
		Protocol      PortProtocol
	}

	// InvalidVolumeMountError is returned when a VolumeMount has one or more invalid fields.
	// It wraps the individual field validation errors for inspection.
	InvalidVolumeMountError struct {
		Value     VolumeMount
		FieldErrs []error
	}

	// InvalidPortMappingError is returned when a PortMapping has one or more invalid fields.
	// It wraps the individual field validation errors for inspection.
	InvalidPortMappingError struct {
		Value     PortMapping
		FieldErrs []error
	}
)

// Error implements the error interface.
func (e *InvalidPortProtocolError) Error() string {
	return fmt.Sprintf("invalid port protocol %q (valid: tcp, udp)", e.Value)
}

// Unwrap returns ErrInvalidPortProtocol so callers can use errors.Is for programmatic detection.
func (e *InvalidPortProtocolError) Unwrap() error { return ErrInvalidPortProtocol }

// Validate returns an error if the PortProtocol is not one of the defined protocols.
// The zero value ("") is valid — it is treated as "tcp" by FormatPortMapping.
func (p PortProtocol) Validate() error {
	switch p {
	case PortProtocolTCP, PortProtocolUDP, "":
		return nil
	default:
		return &InvalidPortProtocolError{Value: p}
	}
}

// String returns the string representation of the PortProtocol.
func (p PortProtocol) String() string { return string(p) }

// Error implements the error interface.
func (e *InvalidSELinuxLabelError) Error() string {
	return fmt.Sprintf("invalid SELinux label %q (valid: empty, z, Z)", e.Value)
}

// Unwrap returns ErrInvalidSELinuxLabel so callers can use errors.Is for programmatic detection.
func (e *InvalidSELinuxLabelError) Unwrap() error { return ErrInvalidSELinuxLabel }

// Validate returns an error if the SELinuxLabel is not one of the defined labels.
// The zero value ("") is valid — it means no SELinux label.
func (s SELinuxLabel) Validate() error {
	switch s {
	case SELinuxLabelNone, SELinuxLabelShared, SELinuxLabelPrivate:
		return nil
	default:
		return &InvalidSELinuxLabelError{Value: s}
	}
}

// String returns the string representation of the SELinuxLabel.
func (s SELinuxLabel) String() string { return string(s) }

// String returns the string representation of the NetworkPort.
func (p NetworkPort) String() string { return fmt.Sprintf("%d", p) }

// Validate returns an error if the NetworkPort is invalid.
// A valid port must be greater than zero.
func (p NetworkPort) Validate() error {
	if p == 0 {
		return &InvalidNetworkPortError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidNetworkPortError.
func (e *InvalidNetworkPortError) Error() string {
	return fmt.Sprintf("invalid network port %d: must be greater than zero", e.Value)
}

// Unwrap returns ErrInvalidNetworkPort for errors.Is() compatibility.
func (e *InvalidNetworkPortError) Unwrap() error { return ErrInvalidNetworkPort }

// String returns the string representation of the HostFilesystemPath.
func (p HostFilesystemPath) String() string { return string(p) }

// Validate returns an error if the HostFilesystemPath is invalid.
// A valid path must be non-empty and not whitespace-only.
//
//goplint:nonzero
func (p HostFilesystemPath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidHostFilesystemPathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidHostFilesystemPathError.
func (e *InvalidHostFilesystemPathError) Error() string {
	return fmt.Sprintf("invalid host filesystem path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidHostFilesystemPath for errors.Is() compatibility.
func (e *InvalidHostFilesystemPathError) Unwrap() error { return ErrInvalidHostFilesystemPath }

// String returns the string representation of the MountTargetPath.
func (p MountTargetPath) String() string { return string(p) }

// Validate returns an error if the MountTargetPath is invalid.
// A valid path must be non-empty and not whitespace-only.
//
//goplint:nonzero
func (p MountTargetPath) Validate() error {
	if strings.TrimSpace(string(p)) == "" {
		return &InvalidMountTargetPathError{Value: p}
	}
	return nil
}

// Error implements the error interface for InvalidMountTargetPathError.
func (e *InvalidMountTargetPathError) Error() string {
	return fmt.Sprintf("invalid container filesystem path %q: must be non-empty", e.Value)
}

// Unwrap returns ErrInvalidMountTargetPath for errors.Is() compatibility.
func (e *InvalidMountTargetPathError) Unwrap() error {
	return ErrInvalidMountTargetPath
}

// Error implements the error interface for InvalidVolumeMountError.
func (e *InvalidVolumeMountError) Error() string {
	return fmt.Sprintf("invalid volume mount %s:%s: %d field error(s)",
		e.Value.HostPath, e.Value.ContainerPath, len(e.FieldErrs))
}

// Unwrap returns ErrInvalidVolumeMount for errors.Is() compatibility.
func (e *InvalidVolumeMountError) Unwrap() error { return ErrInvalidVolumeMount }

// Validate returns an error if any typed field of the VolumeMount is invalid.
// Validates HostPath, ContainerPath, and SELinux.
// ReadOnly is a bool and requires no validation.
func (v VolumeMount) Validate() error {
	var errs []error
	if err := v.HostPath.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := v.ContainerPath.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := v.SELinux.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// String returns the volume mount in "host:container[:selinux][:ro]" format.
func (v VolumeMount) String() string {
	s := string(v.HostPath) + ":" + string(v.ContainerPath)
	if v.SELinux != "" {
		s += ":" + string(v.SELinux)
	}
	if v.ReadOnly {
		s += ":ro"
	}
	return s
}

// Error implements the error interface for InvalidPortMappingError.
func (e *InvalidPortMappingError) Error() string {
	return fmt.Sprintf("invalid port mapping %d:%d/%s: %d field error(s)",
		e.Value.HostPort, e.Value.ContainerPort, e.Value.Protocol, len(e.FieldErrs))
}

// Unwrap returns ErrInvalidPortMapping for errors.Is() compatibility.
func (e *InvalidPortMappingError) Unwrap() error { return ErrInvalidPortMapping }

// Validate returns an error if any typed field of the PortMapping is invalid.
// Validates HostPort, ContainerPort, and Protocol.
func (p PortMapping) Validate() error {
	var errs []error
	if err := p.HostPort.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := p.ContainerPort.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := p.Protocol.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// String returns the port mapping in "host:container/protocol" format.
// Defaults to "tcp" when the protocol is empty.
func (p PortMapping) String() string {
	proto := p.Protocol
	if proto == "" {
		proto = PortProtocolTCP
	}
	return fmt.Sprintf("%d:%d/%s", p.HostPort, p.ContainerPort, proto)
}

// --- Option Functions ---

// WithName sets the engine name used in error messages.
func WithName(name string) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		e.name = name
	}
}

// WithExecCommand sets a custom exec command function for testing.
func WithExecCommand(fn ExecCommandFunc) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		e.execCommand = fn
	}
}

// WithVolumeFormatter sets a custom volume formatter function.
// This is used by Podman to add SELinux labels on Linux.
func WithVolumeFormatter(fn VolumeFormatFunc) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		e.volumeFormatter = fn
	}
}

// WithRunArgsTransformer sets a custom run args transformer.
// This is used by Podman to inject --userns=keep-id for rootless compatibility.
func WithRunArgsTransformer(fn RunArgsTransformer) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		e.runArgsTransformer = fn
	}
}

// WithCmdEnvOverride adds an environment variable override applied to every
// exec.Cmd created by this engine. Used by Podman to inject CONTAINERS_CONF_OVERRIDE.
func WithCmdEnvOverride(key, value string) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		if e.cmdEnvOverrides == nil {
			e.cmdEnvOverrides = make(map[string]string)
		}
		e.cmdEnvOverrides[key] = value
	}
}

// WithSysctlOverridePath records the temp file path for the sysctl override.
// The path is cleaned up when Close() is called on the engine.
func WithSysctlOverridePath(path HostFilesystemPath) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		e.sysctlOverridePath = path
	}
}

// WithSysctlOverrideActive marks the engine as having an active temp-file-based
// sysctl override. When true, the runtime layer skips run-level serialization
// because the override eliminates the ping_group_range race at source.
func WithSysctlOverrideActive(active bool) BaseCLIEngineOption {
	return func(e *BaseCLIEngine) {
		e.sysctlOverrideActive = active
	}
}

// --- Constructor ---

// NewBaseCLIEngine creates a new base engine with the given binary path.
func NewBaseCLIEngine(binaryPath HostFilesystemPath, opts ...BaseCLIEngineOption) *BaseCLIEngine {
	e := &BaseCLIEngine{
		binaryPath:  binaryPath,
		execCommand: exec.CommandContext,
		// Identity functions by default
		volumeFormatter:    func(v invowkfile.VolumeMountSpec) string { return string(v) },
		runArgsTransformer: func(args []string) []string { return args },
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// --- Accessor Methods ---

// Name returns the engine name used in error messages.
func (e *BaseCLIEngine) Name() string {
	return e.name
}

// BinaryPath returns the path to the container engine binary.
func (e *BaseCLIEngine) BinaryPath() string {
	return string(e.binaryPath)
}

// BaseCLI returns the BaseCLIEngine itself.
// This satisfies the BaseCLIProvider interface and is promoted by embedding
// engines (DockerEngine, PodmanEngine), enabling SandboxAwareEngine to access
// arg-building methods without a concrete type switch.
func (e *BaseCLIEngine) BaseCLI() *BaseCLIEngine {
	return e
}

// --- Argument Builders ---

// BuildArgs constructs arguments for a container build command.
// Returns arguments in the order expected by docker/podman build.
//
// Generated command: <binary> build [options] <context>
func (e *BaseCLIEngine) BuildArgs(opts BuildOptions) []string {
	args := []string{"build"}

	if opts.Dockerfile != "" {
		// Resolve Dockerfile path relative to context directory.
		// If ContextDir is empty, the Dockerfile path is used as-is
		// (assumed resolvable from CWD by the container engine).
		dockerfilePath := string(opts.Dockerfile)
		if !filepath.IsAbs(dockerfilePath) && opts.ContextDir != "" {
			dockerfilePath = filepath.Join(string(opts.ContextDir), dockerfilePath)
		}
		args = append(args, "-f", dockerfilePath)
	}

	if opts.Tag != "" {
		args = append(args, "-t", string(opts.Tag))
	}

	if opts.NoCache {
		args = append(args, "--no-cache")
	}

	for k, v := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, string(opts.ContextDir))

	return args
}

// RunArgs constructs arguments for a container run command.
// Returns arguments in the order expected by docker/podman run.
//
// Generated command: <binary> run [options] <image> [command...]
func (e *BaseCLIEngine) RunArgs(opts RunOptions) []string {
	args := []string{"run"}

	if opts.Remove {
		args = append(args, "--rm")
	}

	if opts.Name != "" {
		args = append(args, "--name", string(opts.Name))
	}

	if opts.WorkDir != "" {
		args = append(args, "-w", string(opts.WorkDir))
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
		args = append(args, "-v", e.volumeFormatter(v))
	}

	for _, p := range opts.Ports {
		args = append(args, "-p", string(p))
	}

	for _, h := range opts.ExtraHosts {
		args = append(args, "--add-host", string(h))
	}

	args = append(args, string(opts.Image))
	args = append(args, opts.Command...)

	return e.runArgsTransformer(args)
}

// ExecArgs constructs arguments for a container exec command.
// Returns arguments in the order expected by docker/podman exec.
//
// Generated command: <binary> exec [options] <container> <command...>
func (e *BaseCLIEngine) ExecArgs(containerID ContainerID, command []string, opts RunOptions) []string {
	args := []string{"exec"}

	if opts.Interactive {
		args = append(args, "-i")
	}

	if opts.TTY {
		args = append(args, "-t")
	}

	if opts.WorkDir != "" {
		args = append(args, "-w", string(opts.WorkDir))
	}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, string(containerID))
	args = append(args, command...)

	return args
}

// RemoveArgs constructs arguments for a container remove command.
func (e *BaseCLIEngine) RemoveArgs(containerID ContainerID, force bool) []string {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, string(containerID))
	return args
}

// RemoveImageArgs constructs arguments for an image remove command.
func (e *BaseCLIEngine) RemoveImageArgs(image ImageTag, force bool) []string {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, string(image))
	return args
}

// --- Command Execution ---

// RunCommand executes a command and returns its output.
// This is the low-level execution method used by concrete engines.
func (e *BaseCLIEngine) RunCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := e.CreateCommand(ctx, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command %s %v failed: %w", string(e.binaryPath), args, err)
	}
	return out, nil
}

// RunCommandCombined executes a command and returns combined stdout/stderr.
func (e *BaseCLIEngine) RunCommandCombined(ctx context.Context, args ...string) ([]byte, error) {
	cmd := e.CreateCommand(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("command %s %v failed: %w", string(e.binaryPath), args, err)
	}
	return out, nil
}

// RunCommandStatus executes a command and returns only the error status.
func (e *BaseCLIEngine) RunCommandStatus(ctx context.Context, args ...string) error {
	cmd := e.CreateCommand(ctx, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", string(e.binaryPath), args, err)
	}
	return nil
}

// RunCommandWithOutput executes a command with stdout captured to a buffer.
func (e *BaseCLIEngine) RunCommandWithOutput(ctx context.Context, args ...string) (string, error) {
	cmd := e.CreateCommand(ctx, args...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command %s %v failed: %w", string(e.binaryPath), args, err)
	}

	return out.String(), nil
}

// CreateCommand creates an exec.Cmd for the given arguments.
// This is useful when the caller needs to customize stdin/stdout/stderr.
// Engine-level overrides (env vars, extra files) are applied automatically.
func (e *BaseCLIEngine) CreateCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := e.execCommand(ctx, string(e.binaryPath), args...)
	e.customizeCmd(cmd)
	return cmd
}

// CustomizeCmd applies engine-level overrides (env vars) to a command.
// This is the public interface for external callers (runtime package, sandbox wrapper)
// that create exec.Cmd instances outside of CreateCommand.
func (e *BaseCLIEngine) CustomizeCmd(cmd *exec.Cmd) {
	e.customizeCmd(cmd)
}

// Close removes temporary resources associated with this engine (e.g., the
// sysctl override temp file). It is safe to call multiple times.
func (e *BaseCLIEngine) Close() error {
	if e.sysctlOverridePath != "" {
		err := os.Remove(string(e.sysctlOverridePath))
		e.sysctlOverridePath = ""
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove sysctl override file: %w", err)
		}
	}
	return nil
}

// --- Promoted Engine Methods (shared by Docker and Podman) ---

// Build builds an image from a Dockerfile.
// It validates BuildOptions before executing to catch invalid fields early.
func (e *BaseCLIEngine) Build(ctx context.Context, opts BuildOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	args := e.BuildArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	if err := cmd.Run(); err != nil {
		return buildContainerError(e.name, opts, err)
	}

	return nil
}

// Run runs a command in a container and returns the result.
// A non-zero exit code is captured in RunResult.ExitCode (not returned as error).
// Only infrastructure failures (binary not found, etc.) set RunResult.Error.
// It validates RunOptions before executing to catch invalid fields early.
func (e *BaseCLIEngine) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	args := e.RunArgs(opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			result.ExitCode = types.ExitCode(exitErr.ExitCode())
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// Exec runs a command in a running container.
func (e *BaseCLIEngine) Exec(ctx context.Context, containerID ContainerID, command []string, opts RunOptions) (*RunResult, error) {
	args := e.ExecArgs(containerID, command, opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	err := cmd.Run()

	result := &RunResult{ContainerID: containerID}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			result.ExitCode = types.ExitCode(exitErr.ExitCode())
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	return result, nil
}

// Remove removes a container.
func (e *BaseCLIEngine) Remove(ctx context.Context, containerID ContainerID, force bool) error {
	args := e.RemoveArgs(containerID, force)
	return e.RunCommandStatus(ctx, args...)
}

// RemoveImage removes an image.
func (e *BaseCLIEngine) RemoveImage(ctx context.Context, image ImageTag, force bool) error {
	args := e.RemoveImageArgs(image, force)
	return e.RunCommandStatus(ctx, args...)
}

// BuildRunArgs builds the argument slice for a 'run' command without executing.
// Returns the full argument slice including 'run' and all options.
// This is used for interactive mode where the command needs to be attached to a PTY.
func (e *BaseCLIEngine) BuildRunArgs(opts RunOptions) []string {
	return e.RunArgs(opts)
}

// InspectImage returns information about an image.
func (e *BaseCLIEngine) InspectImage(ctx context.Context, image ImageTag) (string, error) {
	return e.RunCommandWithOutput(ctx, "image", "inspect", string(image))
}

// customizeCmd applies env overrides to a command.
func (e *BaseCLIEngine) customizeCmd(cmd *exec.Cmd) {
	if len(e.cmdEnvOverrides) > 0 {
		// Start with the parent process environment, then overlay overrides.
		// exec.Cmd.Env being nil means "inherit everything", but once set to
		// a non-nil slice, only the listed vars are passed to the child.
		cmd.Env = os.Environ()
		for k, v := range e.cmdEnvOverrides {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
}

// --- Dockerfile Resolution ---

// ResolveDockerfilePath resolves a Dockerfile path relative to the build context.
// If the path is absolute, it is returned as-is.
// If the path is relative, it is resolved against the context path.
// Returns the resolved path or error if path traversal is detected.
func ResolveDockerfilePath(contextPath, dockerfilePath string) (string, error) {
	if dockerfilePath == "" {
		return "", nil
	}

	if filepath.IsAbs(dockerfilePath) {
		return dockerfilePath, nil
	}

	resolved := filepath.Join(contextPath, dockerfilePath)

	// Check for path traversal: the resolved path should be within the context
	resolvedClean := filepath.Clean(resolved)
	contextClean := filepath.Clean(contextPath)

	// Ensure resolved path starts with context path
	if !strings.HasPrefix(resolvedClean, contextClean) {
		return "", fmt.Errorf("dockerfile path %q escapes context directory %q", dockerfilePath, contextPath)
	}

	return resolved, nil
}

// --- Volume Mount Formatting ---

// FormatVolumeMount formats a volume mount as a string for -v flag.
//
//plint:render
func FormatVolumeMount(mount VolumeMount) string {
	var result strings.Builder
	result.WriteString(string(mount.HostPath))
	result.WriteString(":")
	result.WriteString(string(mount.ContainerPath))

	var options []string
	if mount.ReadOnly {
		options = append(options, "ro")
	}
	if mount.SELinux != "" {
		options = append(options, string(mount.SELinux))
	}

	if len(options) > 0 {
		result.WriteString(":")
		result.WriteString(strings.Join(options, ","))
	}

	return result.String()
}

// ParseVolumeMount parses a volume string into a VolumeMount struct.
// Volume format: host_path:container_path[:options]
// Options can include: ro, rw, z, Z, and others.
// After parsing, the result is validated via VolumeMount.Validate().
func ParseVolumeMount(volume string) (VolumeMount, error) {
	mount := VolumeMount{}

	parts := strings.Split(volume, ":")

	if len(parts) >= 1 {
		mount.HostPath = HostFilesystemPath(parts[0])
	}
	if len(parts) >= 2 {
		mount.ContainerPath = MountTargetPath(parts[1])
	}
	if len(parts) >= 3 {
		options := parts[2]
		for opt := range strings.SplitSeq(options, ",") {
			switch opt {
			case "ro":
				mount.ReadOnly = true
			case "z", "Z":
				mount.SELinux = SELinuxLabel(opt)
			}
		}
	}

	if err := mount.Validate(); err != nil {
		return mount, err
	}
	return mount, nil
}

// --- Port Mapping Formatting ---

// FormatPortMapping formats a port mapping as a string for -p flag.
//
//plint:render
func FormatPortMapping(mapping PortMapping) string {
	result := fmt.Sprintf("%d:%d", mapping.HostPort, mapping.ContainerPort)
	if mapping.Protocol != "" && mapping.Protocol != PortProtocolTCP {
		result += "/" + string(mapping.Protocol)
	}
	return result
}

// ParsePortMapping parses a port mapping string in "hostPort:containerPort[/protocol]" format
// into a PortMapping struct. After parsing, the result is validated via PortMapping.Validate().
func ParsePortMapping(portStr string) (PortMapping, error) {
	mapping := PortMapping{}

	parts := strings.SplitN(portStr, ":", 2)
	if len(parts) != 2 {
		return mapping, fmt.Errorf("invalid port mapping format %q: must contain ':' separator", portStr)
	}

	hostPort, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return mapping, fmt.Errorf("invalid host port %q: %w", parts[0], err)
	}
	mapping.HostPort = NetworkPort(hostPort)

	// Split container part on "/" to get port number and optional protocol
	containerParts := strings.SplitN(parts[1], "/", 2)
	containerPort, err := strconv.ParseUint(containerParts[0], 10, 16)
	if err != nil {
		return mapping, fmt.Errorf("invalid container port %q: %w", containerParts[0], err)
	}
	mapping.ContainerPort = NetworkPort(containerPort)

	if len(containerParts) == 2 {
		mapping.Protocol = PortProtocol(containerParts[1])
	}

	if err := mapping.Validate(); err != nil {
		return mapping, err
	}
	return mapping, nil
}

// --- Actionable Error Helpers ---

// buildContainerError creates an actionable error for container build failures.
func buildContainerError(engine string, opts BuildOptions, cause error) error {
	ctx := issue.NewErrorContext().
		WithOperation("build container image")

	// Determine resource (Dockerfile or image tag)
	switch {
	case opts.Dockerfile != "":
		ctx.WithResource(string(opts.Dockerfile))
	case opts.ContextDir != "":
		ctx.WithResource(string(opts.ContextDir) + "/Dockerfile")
	case opts.Tag != "":
		ctx.WithResource(string(opts.Tag))
	}

	// Add suggestions based on common build issues
	ctx.WithSuggestion("Check Dockerfile syntax for errors")
	ctx.WithSuggestion("Verify the build context path exists and is accessible")
	ctx.WithSuggestion("Ensure base images are available (try: " + engine + " pull <base-image>)")
	ctx.WithSuggestion("Run with --ivk-verbose to see full build output")

	return ctx.Wrap(cause).BuildError()
}

// runContainerError creates an actionable error for container run failures.
func runContainerError(engine string, opts RunOptions, cause error) error {
	ctx := issue.NewErrorContext().
		WithOperation("run container").
		WithResource(string(opts.Image))

	ctx.WithSuggestion("Verify the image exists (try: " + engine + " images)")
	ctx.WithSuggestion("Check that volume mount paths exist on the host")
	ctx.WithSuggestion("Ensure port mappings don't conflict with running services")
	ctx.WithSuggestion("Run with --ivk-verbose to see full container output")

	return ctx.Wrap(cause).BuildError()
}
