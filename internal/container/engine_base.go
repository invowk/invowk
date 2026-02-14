// SPDX-License-Identifier: MPL-2.0

package container

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/issue"
)

type (
	// ExecCommandFunc is the function signature for creating exec.Cmd.
	// This allows injection of mock implementations for testing.
	ExecCommandFunc func(ctx context.Context, name string, arg ...string) *exec.Cmd

	// VolumeFormatFunc is a function that formats a volume string.
	// Podman uses this to add SELinux labels (:z/:Z) which are required in
	// SELinux-enforcing environments for proper volume isolation — without them,
	// container processes cannot access bind-mounted host paths.
	VolumeFormatFunc func(volume string) string

	// SELinuxCheckFunc is a function that checks if SELinux is enabled.
	// This allows injection of mock implementations for testing.
	SELinuxCheckFunc func() bool

	// RunArgsTransformer modifies run arguments after they're built.
	// Used by Podman to inject --userns=keep-id for rootless compatibility.
	RunArgsTransformer func(args []string) []string

	// BaseCLIEngineOption configures a BaseCLIEngine.
	BaseCLIEngineOption func(*BaseCLIEngine)

	// BaseCLIEngine provides common implementation for CLI-based container engines.
	// Docker and Podman engines embed this struct.
	BaseCLIEngine struct {
		binaryPath           string
		execCommand          ExecCommandFunc
		volumeFormatter      VolumeFormatFunc
		runArgsTransformer   RunArgsTransformer
		cmdEnvOverrides      map[string]string // Per-command env var overrides (e.g., CONTAINERS_CONF_OVERRIDE)
		sysctlOverridePath   string            // Temp file path for sysctl override (removed on Close)
		sysctlOverrideActive bool              // Whether the temp file sysctl override is in effect
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

	// VolumeMount represents a volume mount specification.
	VolumeMount struct {
		HostPath      string
		ContainerPath string
		ReadOnly      bool
		SELinux       string // Empty, "z", or "Z"
	}

	// PortMapping represents a port mapping specification.
	PortMapping struct {
		HostPort      uint16
		ContainerPort uint16
		Protocol      string // "tcp" or "udp", defaults to "tcp"
	}
)

// --- Option Functions ---

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
func WithSysctlOverridePath(path string) BaseCLIEngineOption {
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
func NewBaseCLIEngine(binaryPath string, opts ...BaseCLIEngineOption) *BaseCLIEngine {
	e := &BaseCLIEngine{
		binaryPath:  binaryPath,
		execCommand: exec.CommandContext,
		// Identity functions by default
		volumeFormatter:    func(v string) string { return v },
		runArgsTransformer: func(args []string) []string { return args },
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// --- Accessor Methods ---

// BinaryPath returns the path to the container engine binary.
func (e *BaseCLIEngine) BinaryPath() string {
	return e.binaryPath
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
		args = append(args, "-v", e.volumeFormatter(v))
	}

	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}

	for _, h := range opts.ExtraHosts {
		args = append(args, "--add-host", h)
	}

	args = append(args, opts.Image)
	args = append(args, opts.Command...)

	return e.runArgsTransformer(args)
}

// ExecArgs constructs arguments for a container exec command.
// Returns arguments in the order expected by docker/podman exec.
//
// Generated command: <binary> exec [options] <container> <command...>
func (e *BaseCLIEngine) ExecArgs(containerID string, command []string, opts RunOptions) []string {
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

	return args
}

// RemoveArgs constructs arguments for a container remove command.
func (e *BaseCLIEngine) RemoveArgs(containerID string, force bool) []string {
	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)
	return args
}

// RemoveImageArgs constructs arguments for an image remove command.
func (e *BaseCLIEngine) RemoveImageArgs(image string, force bool) []string {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, image)
	return args
}

// --- Command Execution ---

// RunCommand executes a command and returns its output.
// This is the low-level execution method used by concrete engines.
func (e *BaseCLIEngine) RunCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := e.CreateCommand(ctx, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("command %s %v failed: %w", e.binaryPath, args, err)
	}
	return out, nil
}

// RunCommandCombined executes a command and returns combined stdout/stderr.
func (e *BaseCLIEngine) RunCommandCombined(ctx context.Context, args ...string) ([]byte, error) {
	cmd := e.CreateCommand(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("command %s %v failed: %w", e.binaryPath, args, err)
	}
	return out, nil
}

// RunCommandStatus executes a command and returns only the error status.
func (e *BaseCLIEngine) RunCommandStatus(ctx context.Context, args ...string) error {
	cmd := e.CreateCommand(ctx, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w", e.binaryPath, args, err)
	}
	return nil
}

// RunCommandWithOutput executes a command with stdout captured to a buffer.
func (e *BaseCLIEngine) RunCommandWithOutput(ctx context.Context, args ...string) (string, error) {
	cmd := e.CreateCommand(ctx, args...)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command %s %v failed: %w", e.binaryPath, args, err)
	}

	return out.String(), nil
}

// CreateCommand creates an exec.Cmd for the given arguments.
// This is useful when the caller needs to customize stdin/stdout/stderr.
// Engine-level overrides (env vars, extra files) are applied automatically.
func (e *BaseCLIEngine) CreateCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := e.execCommand(ctx, e.binaryPath, args...)
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
		err := os.Remove(e.sysctlOverridePath)
		e.sysctlOverridePath = ""
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove sysctl override file: %w", err)
		}
	}
	return nil
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
func FormatVolumeMount(mount VolumeMount) string {
	var result strings.Builder
	result.WriteString(mount.HostPath)
	result.WriteString(":")
	result.WriteString(mount.ContainerPath)

	var options []string
	if mount.ReadOnly {
		options = append(options, "ro")
	}
	if mount.SELinux != "" {
		options = append(options, mount.SELinux)
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
func ParseVolumeMount(volume string) VolumeMount {
	mount := VolumeMount{}

	parts := strings.Split(volume, ":")

	if len(parts) >= 1 {
		mount.HostPath = parts[0]
	}
	if len(parts) >= 2 {
		mount.ContainerPath = parts[1]
	}
	if len(parts) >= 3 {
		options := parts[2]
		for opt := range strings.SplitSeq(options, ",") {
			switch opt {
			case "ro":
				mount.ReadOnly = true
			case "z", "Z":
				mount.SELinux = opt
			}
		}
	}

	return mount
}

// --- Port Mapping Formatting ---

// FormatPortMapping formats a port mapping as a string for -p flag.
func FormatPortMapping(mapping PortMapping) string {
	result := fmt.Sprintf("%d:%d", mapping.HostPort, mapping.ContainerPort)
	if mapping.Protocol != "" && mapping.Protocol != "tcp" {
		result += "/" + mapping.Protocol
	}
	return result
}

// --- Actionable Error Helpers ---

// buildContainerError creates an actionable error for container build failures.
func buildContainerError(engine string, opts BuildOptions, cause error) error {
	ctx := issue.NewErrorContext().
		WithOperation("build container image")

	// Determine resource (Dockerfile or image tag)
	switch {
	case opts.Dockerfile != "":
		ctx.WithResource(opts.Dockerfile)
	case opts.ContextDir != "":
		ctx.WithResource(opts.ContextDir + "/Dockerfile")
	case opts.Tag != "":
		ctx.WithResource(opts.Tag)
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
		WithResource(opts.Image)

	ctx.WithSuggestion("Verify the image exists (try: " + engine + " images)")
	ctx.WithSuggestion("Check that volume mount paths exist on the host")
	ctx.WithSuggestion("Ensure port mappings don't conflict with running services")
	ctx.WithSuggestion("Run with --ivk-verbose to see full container output")

	return ctx.Wrap(cause).BuildError()
}
