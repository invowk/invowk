// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// maxBuildRetries is the number of attempts for container image builds.
	// Retries handle transient engine errors (exit code 125, network timeouts,
	// storage driver races) that occur on rootless Podman in CI environments.
	maxBuildRetries = 3

	// baseBuildBackoff is the initial backoff duration between build retries.
	// Builds are heavier than runs, so we use a longer base than the smoke test.
	baseBuildBackoff = 2 * time.Second

	// maxRunRetries is the number of attempts for container run operations.
	// Retries handle transient engine errors (rootless Podman ping_group_range
	// race, exit code 125/126, overlay mount races) that occur when multiple
	// containers start concurrently. Set to 5 (vs 3 for builds) because
	// run races are more frequent under heavy parallelism and runs are fast.
	maxRunRetries = 5

	// baseRunBackoff is the initial backoff duration between run retries.
	// Uses exponential backoff: 1s, 2s, 4s, 8s (total max ~15s).
	baseRunBackoff = 1 * time.Second

	containerWorkspaceRoot   = "/workspace"
	containerWorkspacePrefix = containerWorkspaceRoot + "/"
)

// errStrictModeProvisioning is returned when container provisioning fails
// and strict mode is enabled, preventing fallback to the base image.
var errStrictModeProvisioning = errors.New("container provisioning failed (strict mode enabled)")

// ensureProvisionedImage ensures the container image exists and is provisioned
// with invowk resources (binary, modules, etc.). This enables nested invowk commands
// inside containers.
func (r *ContainerRuntime) ensureProvisionedImage(ctx *ExecutionContext, cfg invowkfileContainerConfig, invowkDir string) (imageName string, envVars map[string]string, cleanup func(), diagnostics []InitDiagnostic, err error) {
	// First, ensure the base image exists
	baseImage, err := r.ensureImage(ctx, cfg, invowkDir)
	if err != nil {
		return "", nil, nil, nil, err
	}

	// If provisioning is disabled, return the base image
	if r.provisioner == nil || r.provisionConfig == nil || !r.provisionConfig.Enabled {
		return baseImage, nil, nil, nil, nil
	}

	// Provision the image with invowk resources
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.IO.Stdout, "Provisioning container with invowk resources...\n") // Verbose output; error non-critical
	}

	result, err := r.provisioner.Provision(ctx.Context, provision.Request{
		BaseImage:    container.ImageTag(baseImage),
		ForceRebuild: ctx.ForceRebuild,
		Stdout:       ctx.IO.Stderr,
		Stderr:       ctx.IO.Stderr,
	})
	if err != nil {
		if r.provisionConfig.Strict {
			// Multi-wrap (Go 1.20+): callers can match either sentinel via errors.Is
			return "", nil, nil, nil, fmt.Errorf("%w: %w", errStrictModeProvisioning, err)
		}
		diag := InitDiagnostic{
			Code: CodeContainerProvisioningFailed,
			Message: fmt.Sprintf("Container provisioning failed: %v. Running without invowk resources (binary, modules). "+
				"Nested invowk commands inside the container will not work. "+
				"To fail on provisioning errors, set container.auto_provision.strict = true.", err),
			Cause: err,
		}
		return baseImage, nil, nil, []InitDiagnostic{diag}, nil
	}
	diagnostics = make([]InitDiagnostic, 0, len(result.Warnings))
	for _, warning := range result.Warnings {
		diagnostics = append(diagnostics, InitDiagnostic{
			Code:    CodeContainerProvisioningWarning,
			Message: warning.Message.String(),
		})
	}

	return string(result.ImageTag), result.EnvVars, result.Cleanup, diagnostics, nil
}

// ensureImage ensures the container image exists, building if necessary
func (r *ContainerRuntime) ensureImage(ctx *ExecutionContext, cfg invowkfileContainerConfig, invowkDir string) (string, error) {
	// If an image is specified, use it directly
	if cfg.Image != "" {
		return string(cfg.Image), nil
	}

	// Build from Containerfile/Dockerfile
	containerfile := cfg.Containerfile
	if containerfile == "" {
		return "", fmt.Errorf("%w: container runtime requires either containerfile or image in the runtime config", ErrContainerBuildConfig)
	}

	containerfilePath := filepath.Join(invowkDir, string(containerfile))
	if _, err := os.Stat(containerfilePath); err != nil {
		return "", fmt.Errorf("containerfile not found at %s", containerfilePath)
	}

	// Generate a unique image tag based on invowkfile path
	imageTag, err := r.generateImageTag(string(ctx.Invowkfile.FilePath))
	if err != nil {
		return "", err
	}

	// Check if image already exists (skip if ForceRebuild is set)
	if !ctx.ForceRebuild {
		exists, err := r.engine.ImageExists(ctx.Context, container.ImageTag(imageTag))
		if err != nil {
			return "", fmt.Errorf("failed to check image existence: %w", err)
		}
		if exists {
			return imageTag, nil
		}
	}

	// Build the image
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.IO.Stdout, "Building container image from %s...\n", containerfilePath) // Verbose output; error non-critical
	}

	buildOpts := container.BuildOptions{
		ContextDir: container.HostFilesystemPath(invowkDir),
		Dockerfile: containerfile,
		Tag:        container.ImageTag(imageTag),
		NoCache:    ctx.ForceRebuild,
		Stdout:     ctx.IO.Stdout,
		Stderr:     ctx.IO.Stderr,
	}

	// Retry build on transient engine errors (exit code 125, network failures,
	// storage driver races). RetryWithBackoff checks ctx.Err() between retries,
	// preventing wasted build attempts after context cancellation.
	var prevBuildErr error
	retryErr := container.RetryWithBackoff(ctx.Context, maxBuildRetries, baseBuildBackoff,
		func(attempt int) (bool, error) {
			if attempt > 0 && ctx.Verbose {
				_, _ = fmt.Fprintf(ctx.IO.Stdout, "Retrying container build (attempt %d/%d) after transient error: %v\n", attempt+1, maxBuildRetries, prevBuildErr)
			}
			buildErr := r.engine.Build(ctx.Context, buildOpts)
			if buildErr != nil {
				prevBuildErr = buildErr
				return container.IsTransientError(buildErr), buildErr
			}
			return false, nil
		})
	if retryErr != nil {
		return "", retryErr
	}
	return imageTag, nil
}

// generateImageTag generates a unique image tag for an invowkfile
//
//plint:render
func (r *ContainerRuntime) generateImageTag(invowkfilePath string) (string, error) {
	absPath, err := filepath.Abs(invowkfilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve invowkfile path: %w", err)
	}
	hash := sha256.Sum256([]byte(absPath))
	shortHash := hex.EncodeToString(hash[:])[:12]
	return fmt.Sprintf("invowk-%s:latest", shortHash), nil
}

// buildProvisionConfig creates a provision.Config from the app config.
func buildProvisionConfig(cfg *config.Config) *provision.Config {
	provisionCfg := provision.DefaultConfig()
	applyHostProvisionDefaults(provisionCfg)

	if cfg == nil {
		return provisionCfg
	}

	// Apply config overrides
	autoProv := cfg.Container.AutoProvision
	provisionCfg.Enabled = autoProv.Enabled
	provisionCfg.Strict = autoProv.Strict

	if autoProv.BinaryPath != "" {
		provisionCfg.InvowkBinaryPath = types.FilesystemPath(autoProv.BinaryPath)
	}

	// Add modules from auto_provision includes (explicit provisioning paths).
	for _, inc := range autoProv.Includes {
		provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, types.FilesystemPath(inc.Path))
	}

	// Conditionally inherit root-level includes into provisioning.
	if autoProv.InheritIncludes {
		for _, inc := range cfg.Includes {
			provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, types.FilesystemPath(inc.Path))
		}
	}

	if autoProv.CacheDir != "" {
		provisionCfg.CacheDir = types.FilesystemPath(autoProv.CacheDir)
	}

	return provisionCfg
}

func applyHostProvisionDefaults(provisionCfg *provision.Config) {
	if provisionCfg == nil {
		return
	}

	if provisionCfg.InvowkBinaryPath == "" {
		if binaryPath, err := os.Executable(); err == nil {
			provisionCfg.InvowkBinaryPath = types.FilesystemPath(binaryPath) //goplint:ignore -- host path validated by provision.Config.Validate()
		}
	}
	if len(provisionCfg.ModulesPaths) == 0 {
		if userDir, err := config.CommandsDir(); err == nil {
			if info, statErr := os.Stat(string(userDir)); statErr == nil && info.IsDir() {
				provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, userDir)
			}
		}
	}
	if provisionCfg.CacheDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			provisionCfg.CacheDir = types.FilesystemPath(filepath.Join(home, ".cache", "invowk", "provision")) //goplint:ignore -- host path validated by provision.Config.Validate()
		}
	}
	if provisionCfg.TagSuffix == "" {
		provisionCfg.TagSuffix = os.Getenv("INVOWK_PROVISION_TAG_SUFFIX")
	}
}

// containerConfigFromRuntime extracts container config from RuntimeConfig
func containerConfigFromRuntime(rt *invowkfile.RuntimeConfig) invowkfileContainerConfig {
	if rt == nil {
		return invowkfileContainerConfig{}
	}
	return invowkfileContainerConfig{
		Containerfile: container.HostFilesystemPath(rt.Containerfile),
		Image:         container.ImageTag(rt.Image),
		Volumes:       containerVolumeSpecs(rt.Volumes),
		Ports:         containerPortSpecs(rt.Ports),
		Persistent:    rt.Persistent,
	}
}

func containerVolumeSpecs(specs []invowkfile.VolumeMountSpec) []container.VolumeMountSpec {
	volumes := make([]container.VolumeMountSpec, 0, len(specs))
	for _, spec := range specs {
		volumes = append(volumes, container.VolumeMountSpec(spec))
	}
	return volumes
}

func containerPortSpecs(specs []invowkfile.PortMappingSpec) []container.PortMappingSpec {
	ports := make([]container.PortMappingSpec, 0, len(specs))
	for _, spec := range specs {
		ports = append(ports, container.PortMappingSpec(spec))
	}
	return ports
}

// buildInterpreterCommand builds the command array for interpreter-based execution.
// For inline scripts, it creates a temp file in the workspace directory (mounted in container).
// Returns: (command []string, tempFilePath string, error)
// The caller is responsible for cleaning up tempFilePath if non-empty.
func (r *ContainerRuntime) buildInterpreterCommand(ctx *ExecutionContext, script string, interp invowkfile.ShebangInfo, invowkDir string) (command []string, tempFile string, err error) {
	var cmd []string
	cmd = append(cmd, interp.Interpreter)
	cmd = append(cmd, interp.Args...)

	var tempFilePath string

	if ctx.SelectedImpl.IsScriptFile() {
		// File script: use the relative path within /workspace
		scriptPath := ctx.SelectedScriptFilePath()
		// Convert host path to container path (relative to /workspace)
		relPath, err := filepath.Rel(invowkDir, string(scriptPath))
		if err != nil {
			// Fall back to just the filename
			relPath = filepath.Base(string(scriptPath))
		}
		// Use forward slashes for container path
		containerPath := containerWorkspacePrefix + filepath.ToSlash(relPath)
		cmd = append(cmd, containerPath)
	} else {
		// Inline script: create temp file in workspace directory
		// This ensures the script is accessible from within the container
		ext := invowkfile.GetExtensionForInterpreter(interp.Interpreter)
		tempF, createErr := os.CreateTemp(invowkDir, "invowk-script-*"+ext)
		if createErr != nil {
			return nil, "", fmt.Errorf("failed to create temp script in workspace: %w", createErr)
		}

		if _, writeErr := tempF.WriteString(script); writeErr != nil {
			_ = tempF.Close()           // Best-effort close on error path
			_ = os.Remove(tempF.Name()) // Best-effort cleanup on error path
			return nil, "", fmt.Errorf("failed to write temp script: %w", writeErr)
		}

		if closeErr := tempF.Close(); closeErr != nil {
			_ = os.Remove(tempF.Name()) // Best-effort cleanup on error path
			return nil, "", fmt.Errorf("failed to close temp script: %w", closeErr)
		}

		// Make executable
		_ = os.Chmod(tempF.Name(), 0o755) // Best-effort; execution may still work

		tempFilePath = tempF.Name()

		// Get the filename for container path
		containerPath := containerWorkspacePrefix + filepath.Base(tempF.Name())
		cmd = append(cmd, containerPath)
	}

	// Add positional arguments
	cmd = append(cmd, ctx.PositionalArgs...)

	return cmd, tempFilePath, nil
}

// getContainerWorkDir determines the working directory for container execution.
// Uses the hierarchical override model (CLI > Implementation > Command > Root > Default).
// The invowkfile directory is mounted at /workspace, so relative paths are mapped there.
func (r *ContainerRuntime) getContainerWorkDir(ctx *ExecutionContext, invowkDir string) string {
	// Get the effective workdir using the standard resolution logic
	// Note: ctx.WorkDir is the CLI override passed through ExecutionContext
	effectiveWorkDir := ctx.Invowkfile.GetEffectiveWorkDir(ctx.Command, ctx.SelectedImpl, ctx.WorkDir)

	// If no workdir was specified at any level, default to /workspace
	if string(effectiveWorkDir) == invowkDir {
		return containerWorkspaceRoot
	}

	// If it's an absolute path, use it directly in the container
	if filepath.IsAbs(string(effectiveWorkDir)) {
		// Check if the path is inside the invowkfile directory (mounted at /workspace)
		relPath, err := filepath.Rel(invowkDir, string(effectiveWorkDir))
		if err == nil && relPath != ".." && !strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			// Path is within invowkfile dir - map to /workspace
			return containerWorkspacePrefix + filepath.ToSlash(relPath)
		}
		// Path is outside invowkfile dir - use as-is (must exist in container or be a mounted path)
		return string(effectiveWorkDir)
	}
	if isContainerAbsolutePath(effectiveWorkDir) {
		return string(effectiveWorkDir)
	}

	// Relative path - join with /workspace
	return containerWorkspacePrefix + filepath.ToSlash(string(effectiveWorkDir))
}

func isContainerAbsolutePath(path types.FilesystemPath) bool {
	return strings.HasPrefix(path.String(), "/")
}
