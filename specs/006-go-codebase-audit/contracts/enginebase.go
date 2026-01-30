// SPDX-License-Identifier: MPL-2.0

//go:build ignore

// Package container provides container engine abstractions.
//
// This is a CONTRACT FILE for planning purposes. It defines the API that will be
// added to internal/container/engine_base.go.
//
// BaseCLIEngine provides shared implementation for CLI-based container engines
// (Docker, Podman). Concrete engines embed this struct and override only
// engine-specific behavior.
package container

import (
	"context"
	"os/exec"
)

// ExecCommandFunc is the function signature for creating exec.Cmd.
// This allows injection of mock implementations for testing.
type ExecCommandFunc func(name string, arg ...string) *exec.Cmd

// BaseCLIEngineOption configures a BaseCLIEngine.
type BaseCLIEngineOption func(*BaseCLIEngine)

// WithExecCommand sets a custom exec command function for testing.
func WithExecCommand(fn ExecCommandFunc) BaseCLIEngineOption

// BaseCLIEngine provides common implementation for CLI-based container engines.
// Docker and Podman engines embed this struct.
type BaseCLIEngine struct {
	binaryName  string
	execCommand ExecCommandFunc
}

// NewBaseCLIEngine creates a new base engine with the given binary name.
func NewBaseCLIEngine(binaryName string, opts ...BaseCLIEngineOption) *BaseCLIEngine

// BinaryName returns the CLI binary name (e.g., "docker", "podman").
func (e *BaseCLIEngine) BinaryName() string

// --- Argument Builders ---

// BuildArgs constructs arguments for a container build command.
// Returns arguments in the order expected by docker/podman build.
//
// Generated command: <binary> build [options] <context>
//
// Options include:
//   - -f <dockerfile> (if specified)
//   - -t <tag>
//   - --no-cache (if NoCache is true)
//   - --build-arg KEY=VALUE (for each build arg)
func (e *BaseCLIEngine) BuildArgs(opts BuildOptions) []string

// RunArgs constructs arguments for a container run command.
// Returns arguments in the order expected by docker/podman run.
//
// Generated command: <binary> run [options] <image> [command...]
//
// Options include:
//   - --rm (if Remove is true)
//   - -it (if Interactive is true)
//   - -v <host>:<container>[:ro][:Z] (for each volume)
//   - -p <host>:<container>[/protocol] (for each port)
//   - -e KEY=VALUE (for each env var)
//   - -w <workdir> (if Workdir is specified)
func (e *BaseCLIEngine) RunArgs(opts RunOptions) []string

// ExecArgs constructs arguments for a container exec command.
// Returns arguments in the order expected by docker/podman exec.
//
// Generated command: <binary> exec [options] <container> <command...>
func (e *BaseCLIEngine) ExecArgs(containerID string, command []string, interactive bool) []string

// --- Command Execution ---

// RunCommand executes a command and returns its output.
// This is the low-level execution method used by concrete engines.
func (e *BaseCLIEngine) RunCommand(ctx context.Context, args ...string) ([]byte, error)

// RunCommandInteractive executes a command with stdin/stdout/stderr attached.
func (e *BaseCLIEngine) RunCommandInteractive(ctx context.Context, args ...string) error

// --- Dockerfile Resolution ---

// ResolveDockerfilePath resolves a Dockerfile path relative to the build context.
// If the path is absolute, it is returned as-is.
// If the path is relative, it is resolved against the context path.
// Returns the resolved path or error if path traversal is detected.
func (e *BaseCLIEngine) ResolveDockerfilePath(contextPath, dockerfilePath string) (string, error)

// --- Volume Mount Formatting ---

// VolumeMount represents a volume mount specification.
type VolumeMount struct {
	HostPath      string
	ContainerPath string
	ReadOnly      bool
	SELinux       string // Empty, "z", or "Z"
}

// FormatVolumeMount formats a volume mount as a string for -v flag.
// Override this method in concrete engines for engine-specific behavior.
func (e *BaseCLIEngine) FormatVolumeMount(mount VolumeMount) string

// --- Port Mapping Formatting ---

// PortMapping represents a port mapping specification.
type PortMapping struct {
	HostPort      uint16
	ContainerPort uint16
	Protocol      string // "tcp" or "udp", defaults to "tcp"
}

// FormatPortMapping formats a port mapping as a string for -p flag.
func (e *BaseCLIEngine) FormatPortMapping(mapping PortMapping) string

// --- Engine Interface Methods (to be implemented by concrete engines) ---

// These methods are defined in the Engine interface and must be
// implemented by DockerEngine and PodmanEngine:
//
// Name() string                              - Returns engine name
// Available() bool                           - Checks if engine is installed
// Version(ctx context.Context) (string, error) - Returns engine version
// Build(ctx context.Context, opts BuildOptions) error
// Run(ctx context.Context, opts RunOptions) (*RunResult, error)
// Exec(ctx context.Context, containerID string, command []string) error
//
// Concrete engines embed BaseCLIEngine and use its helper methods
// to implement these interface methods.
