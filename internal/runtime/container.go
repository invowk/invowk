package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"invowk-cli/internal/config"
	"invowk-cli/internal/container"
	"invowk-cli/internal/sshserver"
	"invowk-cli/pkg/invowkfile"
)

// ContainerRuntime executes commands inside a container
type ContainerRuntime struct {
	engine    container.Engine
	sshServer *sshserver.Server
}

// NewContainerRuntime creates a new container runtime
func NewContainerRuntime(cfg *config.Config) (*ContainerRuntime, error) {
	engineType := container.EngineType(cfg.ContainerEngine)
	engine, err := container.NewEngine(engineType)
	if err != nil {
		return nil, err
	}
	return &ContainerRuntime{engine: engine}, nil
}

// NewContainerRuntimeWithEngine creates a container runtime with a specific engine
func NewContainerRuntimeWithEngine(engine container.Engine) *ContainerRuntime {
	return &ContainerRuntime{engine: engine}
}

// SetSSHServer sets the SSH server for host access from containers
func (r *ContainerRuntime) SetSSHServer(srv *sshserver.Server) {
	r.sshServer = srv
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
		invowkDir := filepath.Dir(ctx.Invowkfile.FilePath)
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
	invowkDir := filepath.Dir(ctx.Invowkfile.FilePath)

	// Resolve the script content (from file or inline)
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invowkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Determine the image to use
	image, err := r.ensureImage(ctx, containerCfg, invowkDir)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to prepare container image: %w", err)}
	}

	// Build environment
	env := r.buildEnv(ctx)

	// Check if host SSH is enabled for this runtime
	hostSSHEnabled := ctx.SelectedImpl.GetHostSSHForRuntime(ctx.SelectedRuntime)

	// Handle host SSH access if enabled
	var sshConnInfo *sshserver.ConnectionInfo
	if hostSSHEnabled {
		if r.sshServer == nil {
			return &Result{ExitCode: 1, Error: fmt.Errorf("host_ssh is enabled but SSH server is not configured")}
		}
		if !r.sshServer.IsRunning() {
			return &Result{ExitCode: 1, Error: fmt.Errorf("host_ssh is enabled but SSH server is not running")}
		}

		// Generate connection info with a unique token for this command execution
		commandID := fmt.Sprintf("%s-%d", ctx.Command.Name, ctx.Context.Value("execution_id"))
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
	// Always mount the invowkfile directory
	volumes = append(volumes, fmt.Sprintf("%s:/workspace", invowkDir))

	// Create shell script command
	// We wrap the script in a shell to handle multi-line scripts
	shellCmd := []string{"/bin/sh", "-c", script}

	// Determine working directory
	workDir := "/workspace"
	if ctx.Command.WorkDir != "" {
		if filepath.IsAbs(ctx.Command.WorkDir) {
			workDir = ctx.Command.WorkDir
		} else {
			workDir = filepath.Join("/workspace", ctx.Command.WorkDir)
		}
	}

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

// ensureImage ensures the container image exists, building if necessary
func (r *ContainerRuntime) ensureImage(ctx *ExecutionContext, cfg invowkfileContainerConfig, invowkDir string) (string, error) {
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

	// Generate a unique image tag based on invowkfile path
	imageTag := r.generateImageTag(ctx.Invowkfile.FilePath)

	// Check if image already exists
	exists, _ := r.engine.ImageExists(ctx.Context, imageTag)
	if exists {
		// TODO: Add an option to force rebuild
		return imageTag, nil
	}

	// Build the image
	if ctx.Verbose {
		fmt.Fprintf(ctx.Stdout, "Building container image from %s...\n", containerfilePath)
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

// generateImageTag generates a unique image tag for an invowkfile
func (r *ContainerRuntime) generateImageTag(invowkfilePath string) string {
	absPath, _ := filepath.Abs(invowkfilePath)
	hash := sha256.Sum256([]byte(absPath))
	shortHash := hex.EncodeToString(hash[:])[:12]
	return fmt.Sprintf("invowk-%s:latest", shortHash)
}

// buildEnv builds the environment for the command
func (r *ContainerRuntime) buildEnv(ctx *ExecutionContext) map[string]string {
	env := make(map[string]string)

	// Platform-level env from the selected implementation
	currentPlatform := invowkfile.GetCurrentHostOS()
	for _, p := range ctx.SelectedImpl.Target.Platforms {
		if p.Name == currentPlatform {
			for k, v := range p.Env {
				env[k] = v
			}
			break
		}
	}

	// Command-level env
	for k, v := range ctx.Command.Env {
		env[k] = v
	}

	// Extra env from context
	for k, v := range ctx.ExtraEnv {
		env[k] = v
	}

	return env
}

// invowkfileContainerConfig is a local type for container config extracted from RuntimeConfig
type invowkfileContainerConfig struct {
	Containerfile string
	Image         string
	Volumes       []string
	Ports         []string
}

// containerConfigFromRuntime extracts container config from RuntimeConfig
func containerConfigFromRuntime(rt *invowkfile.RuntimeConfig) invowkfileContainerConfig {
	if rt == nil {
		return invowkfileContainerConfig{}
	}
	return invowkfileContainerConfig{
		Containerfile: rt.Containerfile,
		Image:         rt.Image,
		Volumes:       append([]string{}, rt.Volumes...),
		Ports:         append([]string{}, rt.Ports...),
	}
}

// CleanupImage removes the built image for an invowkfile
func (r *ContainerRuntime) CleanupImage(ctx *ExecutionContext) error {
	imageTag := r.generateImageTag(ctx.Invowkfile.FilePath)
	return r.engine.RemoveImage(ctx.Context, imageTag, true)
}

// GetEngineName returns the name of the underlying container engine
func (r *ContainerRuntime) GetEngineName() string {
	if r.engine == nil {
		return "none"
	}
	return r.engine.Name()
}

// ShellInContainer opens an interactive shell in the container
func (r *ContainerRuntime) ShellInContainer(ctx *ExecutionContext, shell string) *Result {
	if shell == "" {
		shell = "/bin/sh"
	}

	// Get the container runtime config
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("runtime config not found for container runtime")}
	}
	containerCfg := containerConfigFromRuntime(rtConfig)
	invowkDir := filepath.Dir(ctx.Invowkfile.FilePath)

	image, err := r.ensureImage(ctx, containerCfg, invowkDir)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	volumes := containerCfg.Volumes
	volumes = append(volumes, fmt.Sprintf("%s:/workspace", invowkDir))

	runOpts := container.RunOptions{
		Image:       image,
		Command:     []string{shell},
		WorkDir:     "/workspace",
		Env:         r.buildEnv(ctx),
		Volumes:     volumes,
		Remove:      true,
		Stdin:       ctx.Stdin,
		Stdout:      ctx.Stdout,
		Stderr:      ctx.Stderr,
		Interactive: true,
		TTY:         true,
	}

	result, err := r.engine.Run(ctx.Context, runOpts)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	return &Result{ExitCode: result.ExitCode, Error: result.Error}
}

// imageName sanitizes a path for use as part of an image name
func imageName(path string) string {
	path = strings.ToLower(path)
	path = strings.ReplaceAll(path, string(filepath.Separator), "-")
	path = strings.ReplaceAll(path, " ", "-")
	return path
}
