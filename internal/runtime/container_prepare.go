// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"invowk-cli/internal/container"
	"invowk-cli/pkg/invkfile"
)

// SupportsInteractive returns true if the container runtime can run interactively.
// This requires a container engine to be available.
func (r *ContainerRuntime) SupportsInteractive() bool {
	return r.Available()
}

// PrepareInteractive prepares the container runtime for interactive execution.
// This is an alias for PrepareCommand to implement the InteractiveRuntime interface.
func (r *ContainerRuntime) PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error) {
	return r.PrepareCommand(ctx)
}

// PrepareCommand prepares the container execution for interactive mode.
// Instead of executing immediately, it returns a prepared command that can
// be attached to a PTY by the caller. This enables the interactive mode
// TUI overlay pattern where the parent process manages the PTY.
func (r *ContainerRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return nil, fmt.Errorf("runtime config not found for container runtime")
	}
	containerCfg := containerConfigFromRuntime(rtConfig)
	invowkDir := filepath.Dir(ctx.Invkfile.FilePath)

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return nil, err
	}

	// Determine the image to use (with provisioning if enabled)
	image, provisionCleanup, err := r.ensureProvisionedImage(ctx, containerCfg, invowkDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare container image: %w", err)
	}

	// Check for unsupported Windows container images
	if isWindowsContainerImage(image) {
		if provisionCleanup != nil {
			provisionCleanup()
		}
		return nil, fmt.Errorf("windows container images are not supported; the container runtime requires Linux-based images (e.g., debian:stable-slim); see https://invowk.io/docs/runtime-modes/container for details")
	}

	// Build environment
	env, err := r.envBuilder.Build(ctx, invkfile.EnvInheritNone)
	if err != nil {
		if provisionCleanup != nil {
			provisionCleanup()
		}
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}

	// Check if host SSH is enabled for this runtime
	hostSSHEnabled := ctx.SelectedImpl.GetHostSSHForRuntime(ctx.SelectedRuntime)

	// Handle host SSH access if enabled
	var cleanupSSH func()
	if hostSSHEnabled {
		sshConnInfo, sshErr := r.setupSSHConnection(ctx, env)
		if sshErr != nil {
			if provisionCleanup != nil {
				provisionCleanup()
			}
			return nil, sshErr
		}

		// Setup cleanup function for SSH token revocation
		tokenToRevoke := sshConnInfo.Token
		cleanupSSH = func() {
			r.sshServer.RevokeToken(tokenToRevoke)
		}
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
			if cleanupSSH != nil {
				cleanupSSH()
			}
			if provisionCleanup != nil {
				provisionCleanup()
			}
			return nil, err
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

	// Build extra hosts for accessing host services from container
	var extraHosts []string
	needsHostAccess := hostSSHEnabled || ctx.TUI.ServerURL != ""
	if needsHostAccess {
		// Add host gateway for accessing host from container
		// This enables hostDockerInternal (Docker) or hostContainersInternal (Podman)
		extraHosts = append(extraHosts, hostGatewayMapping)
	}

	// Add TUI server environment variables if set (for interactive mode)
	if ctx.TUI.ServerURL != "" {
		env["INVOWK_TUI_ADDR"] = ctx.TUI.ServerURL
	}
	if ctx.TUI.ServerToken != "" {
		env["INVOWK_TUI_TOKEN"] = ctx.TUI.ServerToken
	}

	// Build run options - enable TTY and Interactive for PTY attachment
	runOpts := container.RunOptions{
		Image:       image,
		Command:     shellCmd,
		WorkDir:     workDir,
		Env:         env,
		Volumes:     volumes,
		Ports:       containerCfg.Ports,
		Remove:      true, // Always remove after execution
		Interactive: true, // Enable -i for PTY
		TTY:         true, // Enable -t for PTY
		ExtraHosts:  extraHosts,
	}

	// Build the docker/podman run command arguments
	args := r.engine.BuildRunArgs(runOpts)

	// Create the exec.Cmd
	cmd := exec.CommandContext(ctx.Context, r.engine.BinaryPath(), args...)

	// Prepare cleanup function
	cleanup := func() {
		if tempScriptPath != "" {
			_ = os.Remove(tempScriptPath) // Cleanup temp file; error non-critical
		}
		if cleanupSSH != nil {
			cleanupSSH()
		}
		if provisionCleanup != nil {
			provisionCleanup()
		}
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: cleanup}, nil
}

// GetHostAddressForContainer returns the hostname that containers should use
// to access services on the host machine.
func (r *ContainerRuntime) GetHostAddressForContainer() string {
	if r.engine.Name() == "podman" {
		return hostContainersInternal
	}
	return hostDockerInternal
}

// CleanupImage removes the built image for an invkfile
func (r *ContainerRuntime) CleanupImage(ctx *ExecutionContext) error {
	imageTag, err := r.generateImageTag(ctx.Invkfile.FilePath)
	if err != nil {
		return err
	}
	return r.engine.RemoveImage(ctx.Context, imageTag, true)
}

// GetEngineName returns the name of the underlying container engine
func (r *ContainerRuntime) GetEngineName() string {
	if r.engine == nil {
		return "none"
	}
	return r.engine.Name()
}
