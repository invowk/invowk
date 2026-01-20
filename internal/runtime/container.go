// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invkfile"
)

// ContainerRuntime executes commands inside a container
type ContainerRuntime struct {
	engine      container.Engine
	sshServer   *sshserver.Server
	provisioner *LayerProvisioner
	cfg         *config.Config
}

// NewContainerRuntime creates a new container runtime
func NewContainerRuntime(cfg *config.Config) (*ContainerRuntime, error) {
	engineType := container.EngineType(cfg.ContainerEngine)
	engine, err := container.NewEngine(engineType)
	if err != nil {
		return nil, err
	}

	// Create provisioner with config
	provisionCfg := buildProvisionConfig(cfg)
	provisioner := NewLayerProvisioner(engine, provisionCfg)

	return &ContainerRuntime{
		engine:      engine,
		provisioner: provisioner,
		cfg:         cfg,
	}, nil
}

// NewContainerRuntimeWithEngine creates a container runtime with a specific engine
func NewContainerRuntimeWithEngine(engine container.Engine) *ContainerRuntime {
	provisionCfg := DefaultProvisionConfig()
	return &ContainerRuntime{
		engine:      engine,
		provisioner: NewLayerProvisioner(engine, provisionCfg),
	}
}

// buildProvisionConfig creates a ContainerProvisionConfig from the app config
func buildProvisionConfig(cfg *config.Config) *ContainerProvisionConfig {
	provisionCfg := DefaultProvisionConfig()

	if cfg == nil {
		return provisionCfg
	}

	// Apply config overrides
	autoProv := cfg.Container.AutoProvision
	provisionCfg.Enabled = autoProv.Enabled

	if autoProv.BinaryPath != "" {
		provisionCfg.InvowkBinaryPath = autoProv.BinaryPath
	}

	if len(autoProv.ModulesPaths) > 0 {
		provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, autoProv.ModulesPaths...)
	}

	if autoProv.CacheDir != "" {
		provisionCfg.CacheDir = autoProv.CacheDir
	}

	// Also add config search paths to modules paths
	provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, cfg.SearchPaths...)

	return provisionCfg
}

// SetProvisionConfig updates the provisioner configuration.
// This is useful for setting the invkfile path before execution.
func (r *ContainerRuntime) SetProvisionConfig(cfg *ContainerProvisionConfig) {
	if cfg != nil {
		r.provisioner = NewLayerProvisioner(r.engine, cfg)
	}
}

// SetSSHServer sets the SSH server for host access from containers
func (r *ContainerRuntime) SetSSHServer(srv *sshserver.Server) {
	r.sshServer = srv
}

// isWindowsContainerImage detects if an image is Windows-based by name convention.
// The container runtime only supports Linux containers. Windows container images
// (e.g., mcr.microsoft.com/windows/servercore) are not supported because the
// runtime executes scripts using /bin/sh which is not available in Windows containers.
func isWindowsContainerImage(image string) bool {
	imageLower := strings.ToLower(image)
	windowsPatterns := []string{
		"mcr.microsoft.com/windows/",
		"mcr.microsoft.com/powershell:",
		"microsoft/windowsservercore",
		"microsoft/nanoserver",
	}
	for _, pattern := range windowsPatterns {
		if strings.Contains(imageLower, pattern) {
			return true
		}
	}
	return false
}

// Name returns the runtime name
func (r *ContainerRuntime) Name() string {
	return "container"
}

// Available returns whether this runtime is available
func (r *ContainerRuntime) Available() bool {
	return r.engine != nil && r.engine.Available()
}

// Validate checks if a command can be executed
func (r *ContainerRuntime) Validate(ctx *ExecutionContext) error {
	if ctx.SelectedImpl == nil {
		return fmt.Errorf("no implementation selected for execution")
	}
	if ctx.SelectedImpl.Script == "" {
		return fmt.Errorf("implementation has no script to execute")
	}

	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return fmt.Errorf("runtime config not found for container runtime")
	}

	// Check for containerfile or image
	if rtConfig.Containerfile == "" && rtConfig.Image == "" {
		// Check for default Containerfile/Dockerfile
		invowkDir := filepath.Dir(ctx.Invkfile.FilePath)
		containerfilePath := filepath.Join(invowkDir, "Containerfile")
		dockerfilePath := filepath.Join(invowkDir, "Dockerfile")
		if _, err := os.Stat(containerfilePath); err != nil {
			if _, err := os.Stat(dockerfilePath); err != nil {
				return fmt.Errorf("container runtime requires a Containerfile or Dockerfile at %s, or an image specified in the runtime config", invowkDir)
			}
		}
	}

	return nil
}

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
		if r.sshServer == nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("enable_host_ssh is enabled but SSH server is not configured")}
		}
		if !r.sshServer.IsRunning() {
			return &Result{ExitCode: 1, Error: fmt.Errorf("enable_host_ssh is enabled but SSH server is not running")}
		}

		// Generate connection info with a unique token for this command execution
		executionID := ctx.ExecutionID
		if executionID == "" {
			executionID = newExecutionID()
		}
		commandID := fmt.Sprintf("%s-%s", ctx.Command.Name, executionID)
		sshConnInfo, err = r.sshServer.GetConnectionInfo(commandID)
		if err != nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("failed to generate SSH credentials: %w", err)}
		}

		// Add SSH connection info to environment
		// Use host.docker.internal for Docker or host.containers.internal for Podman
		hostAddr := "host.docker.internal"
		if r.engine.Name() == "podman" {
			hostAddr = "host.containers.internal"
		}

		env["INVOWK_SSH_HOST"] = hostAddr
		env["INVOWK_SSH_PORT"] = fmt.Sprintf("%d", sshConnInfo.Port)
		env["INVOWK_SSH_USER"] = sshConnInfo.User
		env["INVOWK_SSH_TOKEN"] = sshConnInfo.Token
		env["INVOWK_SSH_ENABLED"] = "true"

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
		extraHosts = append(extraHosts, "host.docker.internal:host-gateway")
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
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritNone)
	if err != nil {
		if provisionCleanup != nil {
			provisionCleanup()
		}
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}

	// Check if host SSH is enabled for this runtime
	hostSSHEnabled := ctx.SelectedImpl.GetHostSSHForRuntime(ctx.SelectedRuntime)

	// Handle host SSH access if enabled
	var sshConnInfo *sshserver.ConnectionInfo
	var cleanupSSH func()
	if hostSSHEnabled {
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
		sshConnInfo, err = r.sshServer.GetConnectionInfo(commandID)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SSH credentials: %w", err)
		}

		// Add SSH connection info to environment
		// Use host.docker.internal for Docker or host.containers.internal for Podman
		hostAddr := "host.docker.internal"
		if r.engine.Name() == "podman" {
			hostAddr = "host.containers.internal"
		}

		env["INVOWK_SSH_HOST"] = hostAddr
		env["INVOWK_SSH_PORT"] = fmt.Sprintf("%d", sshConnInfo.Port)
		env["INVOWK_SSH_USER"] = sshConnInfo.User
		env["INVOWK_SSH_TOKEN"] = sshConnInfo.Token
		env["INVOWK_SSH_ENABLED"] = "true"

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
	needsHostAccess := (hostSSHEnabled && sshConnInfo != nil) || ctx.TUIServerURL != ""
	if needsHostAccess {
		// Add host gateway for accessing host from container
		// This enables host.docker.internal (Docker) or host.containers.internal (Podman)
		extraHosts = append(extraHosts, "host.docker.internal:host-gateway")
	}

	// Add TUI server environment variables if set (for interactive mode)
	if ctx.TUIServerURL != "" {
		env["INVOWK_TUI_ADDR"] = ctx.TUIServerURL
	}
	if ctx.TUIServerToken != "" {
		env["INVOWK_TUI_TOKEN"] = ctx.TUIServerToken
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
		return "host.containers.internal"
	}
	return "host.docker.internal"
}

// buildInterpreterCommand builds the command array for interpreter-based execution.
// For inline scripts, it creates a temp file in the workspace directory (mounted in container).
// Returns: (command []string, tempFilePath string, error)
// The caller is responsible for cleaning up tempFilePath if non-empty.
func (r *ContainerRuntime) buildInterpreterCommand(ctx *ExecutionContext, script string, interp invkfile.ShebangInfo, invowkDir string) (command []string, tempFile string, err error) {
	var cmd []string
	cmd = append(cmd, interp.Interpreter)
	cmd = append(cmd, interp.Args...)

	var tempFilePath string

	if ctx.SelectedImpl.IsScriptFile() {
		// File script: use the relative path within /workspace
		scriptPath := ctx.SelectedImpl.GetScriptFilePath(ctx.Invkfile.FilePath)
		// Convert host path to container path (relative to /workspace)
		relPath, err := filepath.Rel(invowkDir, scriptPath)
		if err != nil {
			// Fall back to just the filename
			relPath = filepath.Base(scriptPath)
		}
		// Use forward slashes for container path
		containerPath := "/workspace/" + filepath.ToSlash(relPath)
		cmd = append(cmd, containerPath)
	} else {
		// Inline script: create temp file in workspace directory
		// This ensures the script is accessible from within the container
		ext := invkfile.GetExtensionForInterpreter(interp.Interpreter)
		tempFile, err := os.CreateTemp(invowkDir, "invowk-script-*"+ext)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create temp script in workspace: %w", err)
		}

		if _, err := tempFile.WriteString(script); err != nil {
			_ = tempFile.Close()            // Best-effort close on error path
			_ = os.Remove(tempFile.Name())  // Best-effort cleanup on error path
			return nil, "", fmt.Errorf("failed to write temp script: %w", err)
		}

		if err := tempFile.Close(); err != nil {
			_ = os.Remove(tempFile.Name()) // Best-effort cleanup on error path
			return nil, "", fmt.Errorf("failed to close temp script: %w", err)
		}

		// Make executable
		_ = os.Chmod(tempFile.Name(), 0o755) // Best-effort; execution may still work

		tempFilePath = tempFile.Name()

		// Get the filename for container path
		containerPath := "/workspace/" + filepath.Base(tempFile.Name())
		cmd = append(cmd, containerPath)
	}

	// Add positional arguments
	cmd = append(cmd, ctx.PositionalArgs...)

	return cmd, tempFilePath, nil
}

// ensureProvisionedImage ensures the container image exists and is provisioned
// with invowk resources (binary, packs, etc.). This enables nested invowk commands
// inside containers.
func (r *ContainerRuntime) ensureProvisionedImage(ctx *ExecutionContext, cfg invkfileContainerConfig, invowkDir string) (imageName string, cleanup func(), err error) {
	// First, ensure the base image exists
	baseImage, err := r.ensureImage(ctx, cfg, invowkDir)
	if err != nil {
		return "", nil, err
	}

	// If provisioning is disabled, return the base image
	if r.provisioner == nil || !r.provisioner.config.Enabled {
		return baseImage, nil, nil
	}

	// Update provisioner config with current invkfile path
	r.provisioner.config.InvkfilePath = ctx.Invkfile.FilePath

	// Provision the image with invowk resources
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.Stdout, "Provisioning container with invowk resources...\n") // Verbose output; error non-critical
	}

	result, err := r.provisioner.Provision(ctx.Context, baseImage)
	if err != nil {
		// If provisioning fails, warn but continue with base image
		_, _ = fmt.Fprintf(ctx.Stderr, "Warning: failed to provision container, using base image: %v\n", err) // Warning output; error non-critical
		return baseImage, nil, nil
	}

	return result.ImageTag, result.Cleanup, nil
}

// ensureImage ensures the container image exists, building if necessary
func (r *ContainerRuntime) ensureImage(ctx *ExecutionContext, cfg invkfileContainerConfig, invowkDir string) (string, error) {
	// If an image is specified, use it directly
	if cfg.Image != "" {
		return cfg.Image, nil
	}

	// Build from Containerfile/Dockerfile
	containerfile := cfg.Containerfile
	if containerfile == "" {
		// Try Containerfile first, then Dockerfile
		containerfilePath := filepath.Join(invowkDir, "Containerfile")
		if _, err := os.Stat(containerfilePath); err == nil {
			containerfile = "Containerfile"
		} else {
			containerfile = "Dockerfile"
		}
	}

	containerfilePath := filepath.Join(invowkDir, containerfile)
	if _, err := os.Stat(containerfilePath); err != nil {
		return "", fmt.Errorf("containerfile not found at %s", containerfilePath)
	}

	// Generate a unique image tag based on invkfile path
	imageTag, err := r.generateImageTag(ctx.Invkfile.FilePath)
	if err != nil {
		return "", err
	}

	// Check if image already exists
	exists, err := r.engine.ImageExists(ctx.Context, imageTag)
	if err != nil {
		return "", fmt.Errorf("failed to check image existence: %w", err)
	}
	if exists {
		// TODO: Add an option to force rebuild
		return imageTag, nil
	}

	// Build the image
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.Stdout, "Building container image from %s...\n", containerfilePath) // Verbose output; error non-critical
	}

	buildOpts := container.BuildOptions{
		ContextDir: invowkDir,
		Dockerfile: containerfile,
		Tag:        imageTag,
		Stdout:     ctx.Stdout,
		Stderr:     ctx.Stderr,
	}

	if err := r.engine.Build(ctx.Context, buildOpts); err != nil {
		return "", err
	}

	return imageTag, nil
}

// generateImageTag generates a unique image tag for an invkfile
func (r *ContainerRuntime) generateImageTag(invkfilePath string) (string, error) {
	absPath, err := filepath.Abs(invkfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve invkfile path: %w", err)
	}
	hash := sha256.Sum256([]byte(absPath))
	shortHash := hex.EncodeToString(hash[:])[:12]
	return fmt.Sprintf("invowk-%s:latest", shortHash), nil
}

// getContainerWorkDir determines the working directory for container execution.
// Uses the hierarchical override model (CLI > Implementation > Command > Root > Default).
// The invkfile directory is mounted at /workspace, so relative paths are mapped there.
func (r *ContainerRuntime) getContainerWorkDir(ctx *ExecutionContext, invowkDir string) string {
	// Get the effective workdir using the standard resolution logic
	// Note: ctx.WorkDir is the CLI override passed through ExecutionContext
	effectiveWorkDir := ctx.Invkfile.GetEffectiveWorkDir(ctx.Command, ctx.SelectedImpl, ctx.WorkDir)

	// If no workdir was specified at any level, default to /workspace
	if effectiveWorkDir == invowkDir {
		return "/workspace"
	}

	// If it's an absolute path, use it directly in the container
	if filepath.IsAbs(effectiveWorkDir) {
		// Check if the path is inside the invkfile directory (mounted at /workspace)
		relPath, err := filepath.Rel(invowkDir, effectiveWorkDir)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			// Path is within invkfile dir - map to /workspace
			return "/workspace/" + filepath.ToSlash(relPath)
		}
		// Path is outside invkfile dir - use as-is (must exist in container or be a mounted path)
		return effectiveWorkDir
	}

	// Relative path - join with /workspace
	return "/workspace/" + filepath.ToSlash(effectiveWorkDir)
}

// invkfileContainerConfig is a local type for container config extracted from RuntimeConfig
type invkfileContainerConfig struct {
	Containerfile string
	Image         string
	Volumes       []string
	Ports         []string
}

// containerConfigFromRuntime extracts container config from RuntimeConfig
func containerConfigFromRuntime(rt *invkfile.RuntimeConfig) invkfileContainerConfig {
	if rt == nil {
		return invkfileContainerConfig{}
	}
	return invkfileContainerConfig{
		Containerfile: rt.Containerfile,
		Image:         rt.Image,
		Volumes:       append([]string{}, rt.Volumes...),
		Ports:         append([]string{}, rt.Ports...),
	}
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
