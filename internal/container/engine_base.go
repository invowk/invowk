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
	"time"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	commandFailedFmt = "command %s %v failed: %w"

	// cmdWaitDelay bounds how long cmd.Run waits for I/O pipes to close after
	// context cancellation kills the container engine process. Without this,
	// container child processes (the actual container) can keep pipes open
	// indefinitely, blocking cmd.Run far past the context deadline.
	cmdWaitDelay = 10 * time.Second
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
		binaryPath HostFilesystemPath
		//plint:internal -- engine-specific CLI subcommand; not a domain value
		imageExistsSubCmd  string // "exists" for Podman, "inspect" for Docker; defaults to "inspect"
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
)

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

// WithImageExistsSubCmd sets the subcommand used for image existence checks.
// Docker uses "inspect" (returns detailed JSON on success); Podman uses "exists"
// (returns exit code 0/1, more efficient). Defaults to "inspect" if not set.
func WithImageExistsSubCmd(subCmd string) BaseCLIEngineOption { //goplint:ignore -- internal CLI subcommand name, not a domain value
	return func(e *BaseCLIEngine) {
		e.imageExistsSubCmd = subCmd
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
		binaryPath:        binaryPath,
		imageExistsSubCmd: "inspect", // Docker-compatible default
		execCommand:       exec.CommandContext,
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

// ImageExistsArgs returns the argument slice for checking image existence.
// Docker uses "image inspect <image>"; Podman uses "image exists <image>".
func (e *BaseCLIEngine) ImageExistsArgs(image ImageTag) []string { //goplint:ignore -- raw CLI args for exec.Command
	return []string{"image", e.imageExistsSubCmd, string(image)}
}

// --- Command Execution ---

// RunCommand executes a command and returns its output.
// This is the low-level execution method used by concrete engines.
func (e *BaseCLIEngine) RunCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := e.CreateCommand(ctx, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(commandFailedFmt, string(e.binaryPath), args, err)
	}
	return out, nil
}

// RunCommandCombined executes a command and returns combined stdout/stderr.
func (e *BaseCLIEngine) RunCommandCombined(ctx context.Context, args ...string) ([]byte, error) {
	cmd := e.CreateCommand(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf(commandFailedFmt, string(e.binaryPath), args, err)
	}
	return out, nil
}

// RunCommandStatus executes a command and returns only the error status.
func (e *BaseCLIEngine) RunCommandStatus(ctx context.Context, args ...string) error {
	cmd := e.CreateCommand(ctx, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf(commandFailedFmt, string(e.binaryPath), args, err)
	}
	return nil
}

// RunCommandWithOutput executes a command with stdout captured to a buffer.
func (e *BaseCLIEngine) RunCommandWithOutput(ctx context.Context, args ...string) (string, error) {
	cmd := e.CreateCommand(ctx, args...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf(commandFailedFmt, string(e.binaryPath), args, err)
	}

	return out.String(), nil
}

// CreateCommand creates an exec.Cmd for the given arguments.
// This is useful when the caller needs to customize stdin/stdout/stderr.
// Engine-level overrides (env vars, extra files) are applied automatically.
func (e *BaseCLIEngine) CreateCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := e.execCommand(ctx, string(e.binaryPath), args...)
	cmd.WaitDelay = cmdWaitDelay
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

// runResultFromExecError extracts exit code from a command execution error
// into a RunResult. For exec.ExitError, the exit code is validated. For other
// errors, exit code defaults to 1. The errContext labels validation error messages
// (e.g., "container run", "sandbox run").
//
//goplint:ignore -- errContext is a format label for error messages, not a domain type.
func runResultFromExecError(err error, errContext string) (*RunResult, error) {
	result := &RunResult{}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode := types.ExitCode(exitErr.ExitCode())
			if validateErr := exitCode.Validate(); validateErr != nil {
				return nil, fmt.Errorf("%s exit code: %w", errContext, validateErr)
			}
			result.ExitCode = exitCode
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}
	return result, nil
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

	return runResultFromExecError(cmd.Run(), "container run")
}

// Exec runs a command in a running container.
func (e *BaseCLIEngine) Exec(ctx context.Context, containerID ContainerID, command []string, opts RunOptions) (*RunResult, error) {
	args := e.ExecArgs(containerID, command, opts)

	cmd := e.CreateCommand(ctx, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr

	result, err := runResultFromExecError(cmd.Run(), "container exec")
	if err != nil {
		return nil, err
	}
	result.ContainerID = containerID
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

// PrepareRunCommand creates a configured command for a container run.
func (e *BaseCLIEngine) PrepareRunCommand(ctx context.Context, opts RunOptions) *exec.Cmd {
	return e.CreateCommand(ctx, e.BuildRunArgs(opts)...)
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
