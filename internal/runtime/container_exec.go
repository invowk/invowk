// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"fmt"
	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invkfile"
	"os"
	"path/filepath"
)

// Execute runs a command in a container
func (r *ContainerRuntime) Execute(ctx *ExecutionContext) *Result {
	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("runtime config not found for container runtime")}
	}
	containerCfg := containerConfigFromRuntime(rtConfig)
	invowkDir := filepath.Dir(ctx.Invkfile.FilePath)

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Determine the image to use (with provisioning if enabled)
	image, provisionCleanup, err := r.ensureProvisionedImage(ctx, containerCfg, invowkDir)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to prepare container image: %w", err)}
	}
	if provisionCleanup != nil {
		defer provisionCleanup()
	}

	// Check for unsupported Windows container images
	if isWindowsContainerImage(image) {
		return &Result{
			ExitCode: 1,
			Error:    fmt.Errorf("windows container images are not supported; the container runtime requires Linux-based images (e.g., debian:stable-slim); see https://invowk.io/docs/runtime-modes/container for details"),
		}
	}

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}

	// Check if host SSH is enabled for this runtime
	hostSSHEnabled := ctx.SelectedImpl.GetHostSSHForRuntime(ctx.SelectedRuntime)

	// Handle host SSH access if enabled
	var sshConnInfo *sshserver.ConnectionInfo
	if hostSSHEnabled {
		sshConnInfo, err = r.setupSSHConnection(ctx, env)
		if err != nil {
			return &Result{ExitCode: 1, Error: err}
		}

		// Defer token revocation
		defer func() {
			if sshConnInfo != nil {
				r.sshServer.RevokeToken(sshConnInfo.Token)
			}
		}()
	}

	// Prepare volumes
	volumes := containerCfg.Volumes
	// Always mount the invkfile directory
	volumes = append(volumes, fmt.Sprintf("%s:/workspace", invowkDir))

	// Resolve interpreter (defaults to "auto" which parses shebang)
	interpInfo := rtConfig.ResolveInterpreterFromScript(script)

	// Build shell command based on interpreter
	var shellCmd []string
	var tempScriptPath string // Track temp file for cleanup

	if interpInfo.Found {
		// Use the resolved interpreter
		shellCmd, tempScriptPath, err = r.buildInterpreterCommand(ctx, script, interpInfo, invowkDir)
		if err != nil {
			return &Result{ExitCode: 1, Error: err}
		}
		if tempScriptPath != "" {
			defer func() { _ = os.Remove(tempScriptPath) }() // Cleanup temp file; error non-critical
		}
	} else {
		// Use default shell execution
		// We wrap the script in a shell to handle multi-line scripts
		// For POSIX shells: /bin/sh -c 'script' invowk arg1 arg2 ... (args become $1, $2, etc.)
		shellCmd = []string{"/bin/sh", "-c", script}
		if len(ctx.PositionalArgs) > 0 {
			shellCmd = append(shellCmd, "invowk") // $0 placeholder
			shellCmd = append(shellCmd, ctx.PositionalArgs...)
		}
	}

	// Determine working directory using the hierarchical override model
	workDir := r.getContainerWorkDir(ctx, invowkDir)

	// Build extra hosts for SSH server access
	var extraHosts []string
	if hostSSHEnabled && sshConnInfo != nil {
		// Add host gateway for accessing host from container
		extraHosts = append(extraHosts, hostGatewayMapping)
	}

	// Run the container
	runOpts := container.RunOptions{
		Image:       image,
		Command:     shellCmd,
		WorkDir:     workDir,
		Env:         env,
		Volumes:     volumes,
		Ports:       containerCfg.Ports,
		Remove:      true, // Always remove after execution
		Stdin:       ctx.Stdin,
		Stdout:      ctx.Stdout,
		Stderr:      ctx.Stderr,
		Interactive: ctx.Stdin != nil,
		ExtraHosts:  extraHosts,
	}

	result, err := r.engine.Run(ctx.Context, runOpts)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to run container: %w", err)}
	}

	return &Result{
		ExitCode: result.ExitCode,
		Error:    result.Error,
	}
}

// ExecuteCapture runs a command in a container and captures its stdout/stderr.
// This implements the CapturingRuntime interface, enabling container-based
// dependency validation through custom checks that need to capture output.
func (r *ContainerRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("runtime config not found for container runtime")}
	}
	containerCfg := containerConfigFromRuntime(rtConfig)
	invowkDir := filepath.Dir(ctx.Invkfile.FilePath)

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Determine the image to use (with provisioning if enabled)
	image, provisionCleanup, err := r.ensureProvisionedImage(ctx, containerCfg, invowkDir)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to prepare container image: %w", err)}
	}
	if provisionCleanup != nil {
		defer provisionCleanup()
	}

	// Check for unsupported Windows container images
	if isWindowsContainerImage(image) {
		return &Result{
			ExitCode: 1,
			Error:    fmt.Errorf("windows container images are not supported; the container runtime requires Linux-based images (e.g., debian:stable-slim); see https://invowk.io/docs/runtime-modes/container for details"),
		}
	}

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}

	// Check if host SSH is enabled for this runtime
	hostSSHEnabled := ctx.SelectedImpl.GetHostSSHForRuntime(ctx.SelectedRuntime)

	// Handle host SSH access if enabled
	var sshConnInfo *sshserver.ConnectionInfo
	if hostSSHEnabled {
		sshConnInfo, err = r.setupSSHConnection(ctx, env)
		if err != nil {
			return &Result{ExitCode: 1, Error: err}
		}

		// Defer token revocation
		defer func() {
			if sshConnInfo != nil {
				r.sshServer.RevokeToken(sshConnInfo.Token)
			}
		}()
	}

	// Prepare volumes
	volumes := containerCfg.Volumes
	// Always mount the invkfile directory
	volumes = append(volumes, fmt.Sprintf("%s:/workspace", invowkDir))

	// Resolve interpreter (defaults to "auto" which parses shebang)
	interpInfo := rtConfig.ResolveInterpreterFromScript(script)

	// Build shell command based on interpreter
	var shellCmd []string
	var tempScriptPath string // Track temp file for cleanup

	if interpInfo.Found {
		// Use the resolved interpreter
		shellCmd, tempScriptPath, err = r.buildInterpreterCommand(ctx, script, interpInfo, invowkDir)
		if err != nil {
			return &Result{ExitCode: 1, Error: err}
		}
		if tempScriptPath != "" {
			defer func() { _ = os.Remove(tempScriptPath) }() // Cleanup temp file; error non-critical
		}
	} else {
		// Use default shell execution
		// We wrap the script in a shell to handle multi-line scripts
		// For POSIX shells: /bin/sh -c 'script' invowk arg1 arg2 ... (args become $1, $2, etc.)
		shellCmd = []string{"/bin/sh", "-c", script}
		if len(ctx.PositionalArgs) > 0 {
			shellCmd = append(shellCmd, "invowk") // $0 placeholder
			shellCmd = append(shellCmd, ctx.PositionalArgs...)
		}
	}

	// Determine working directory using the hierarchical override model
	workDir := r.getContainerWorkDir(ctx, invowkDir)

	// Build extra hosts for SSH server access
	var extraHosts []string
	if hostSSHEnabled && sshConnInfo != nil {
		// Add host gateway for accessing host from container
		extraHosts = append(extraHosts, hostGatewayMapping)
	}

	// Capture stdout and stderr into buffers
	var stdout, stderr bytes.Buffer

	// Run the container with output capture
	runOpts := container.RunOptions{
		Image:       image,
		Command:     shellCmd,
		WorkDir:     workDir,
		Env:         env,
		Volumes:     volumes,
		Ports:       containerCfg.Ports,
		Remove:      true,    // Always remove after execution
		Stdin:       nil,     // No stdin for capture mode
		Stdout:      &stdout, // Capture stdout
		Stderr:      &stderr, // Capture stderr
		Interactive: false,   // Non-interactive for capture mode
		ExtraHosts:  extraHosts,
	}

	result, err := r.engine.Run(ctx.Context, runOpts)
	if err != nil {
		return &Result{
			ExitCode:  1,
			Error:     fmt.Errorf("failed to run container: %w", err),
			Output:    stdout.String(),
			ErrOutput: stderr.String(),
		}
	}

	return &Result{
		ExitCode:  result.ExitCode,
		Error:     result.Error,
		Output:    stdout.String(),
		ErrOutput: stderr.String(),
	}
}

// setupSSHConnection sets up SSH connection for container host access
func (r *ContainerRuntime) setupSSHConnection(ctx *ExecutionContext, env map[string]string) (*sshserver.ConnectionInfo, error) {
	if r.sshServer == nil {
		return nil, fmt.Errorf("enable_host_ssh is enabled but SSH server is not configured")
	}
	if !r.sshServer.IsRunning() {
		return nil, fmt.Errorf("enable_host_ssh is enabled but SSH server is not running")
	}

	// Generate connection info with a unique token for this command execution
	executionID := ctx.ExecutionID
	if executionID == "" {
		executionID = newExecutionID()
	}
	commandID := fmt.Sprintf("%s-%s", ctx.Command.Name, executionID)
	sshConnInfo, err := r.sshServer.GetConnectionInfo(commandID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SSH credentials: %w", err)
	}

	// Add SSH connection info to environment
	// Use hostDockerInternal for Docker or hostContainersInternal for Podman
	hostAddr := hostDockerInternal
	if r.engine.Name() == "podman" {
		hostAddr = hostContainersInternal
	}

	env["INVOWK_SSH_HOST"] = hostAddr
	env["INVOWK_SSH_PORT"] = fmt.Sprintf("%d", sshConnInfo.Port)
	env["INVOWK_SSH_USER"] = sshConnInfo.User
	env["INVOWK_SSH_TOKEN"] = sshConnInfo.Token
	env["INVOWK_SSH_ENABLED"] = "true"

	return sshConnInfo, nil
}
