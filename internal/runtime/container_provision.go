// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/pkg/invowkfile"
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
)

// ensureProvisionedImage ensures the container image exists and is provisioned
// with invowk resources (binary, modules, etc.). This enables nested invowk commands
// inside containers.
func (r *ContainerRuntime) ensureProvisionedImage(ctx *ExecutionContext, cfg invowkfileContainerConfig, invowkDir string) (imageName string, cleanup func(), err error) {
	// First, ensure the base image exists
	baseImage, err := r.ensureImage(ctx, cfg, invowkDir)
	if err != nil {
		return "", nil, err
	}

	// If provisioning is disabled, return the base image
	if r.provisioner == nil || !r.provisioner.Config().Enabled {
		return baseImage, nil, nil
	}

	// Update provisioner config with current invowkfile path and ForceRebuild
	provCfg := r.provisioner.Config()
	provCfg.InvowkfilePath = string(ctx.Invowkfile.FilePath)
	provCfg.ForceRebuild = ctx.ForceRebuild

	// Provision the image with invowk resources
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.IO.Stdout, "Provisioning container with invowk resources...\n") // Verbose output; error non-critical
	}

	result, err := r.provisioner.Provision(ctx.Context, baseImage)
	if err != nil {
		if r.provisioner.Config().Strict {
			return "", nil, fmt.Errorf("container provisioning failed (strict mode enabled): %w", err)
		}
		_, _ = fmt.Fprintf(ctx.IO.Stderr,
			"WARNING: Container provisioning failed: %v\n"+
				"  The container will run WITHOUT invowk resources (binary, modules).\n"+
				"  Nested invowk commands inside the container will not work.\n"+
				"  To fail on provisioning errors, set: container.auto_provision.strict = true\n", err)
		return baseImage, nil, nil
	}

	return result.ImageTag, result.Cleanup, nil
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
		Dockerfile: container.HostFilesystemPath(containerfile),
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

	if cfg == nil {
		return provisionCfg
	}

	// Apply config overrides
	autoProv := cfg.Container.AutoProvision
	provisionCfg.Enabled = autoProv.Enabled
	provisionCfg.Strict = autoProv.Strict

	if autoProv.BinaryPath != "" {
		provisionCfg.InvowkBinaryPath = string(autoProv.BinaryPath)
	}

	// Add modules from auto_provision includes (explicit provisioning paths).
	for _, inc := range autoProv.Includes {
		provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, string(inc.Path))
	}

	// Conditionally inherit root-level includes into provisioning.
	if autoProv.InheritIncludes {
		for _, inc := range cfg.Includes {
			provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, string(inc.Path))
		}
	}

	if autoProv.CacheDir != "" {
		provisionCfg.CacheDir = string(autoProv.CacheDir)
	}

	return provisionCfg
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

// isAlpineContainerImage detects Alpine-based image references by repository name.
// Alpine images are intentionally unsupported because musl-based environments have
// subtle behavioral differences that reduce runtime reliability.
//
// Detection is segment-aware: only the last path segment of the image name is
// checked, so images like "go-alpine-builder:v1" or "myorg/alpine-tools" are
// NOT matched. Matches: alpine, alpine:3.20, docker.io/library/alpine:latest.
func isAlpineContainerImage(image string) bool {
	imageLower := strings.ToLower(strings.TrimSpace(image))
	if imageLower == "" {
		return false
	}

	// Strip tag/digest suffix for name-only matching.
	name := imageLower
	if idx := strings.LastIndex(name, ":"); idx != -1 {
		name = name[:idx]
	}
	if idx := strings.LastIndex(name, "@"); idx != -1 {
		name = name[:idx]
	}

	// Check bare name or last path segment.
	// Matches: alpine, alpine:3.20, docker.io/library/alpine:latest
	// Does NOT match: go-alpine-builder, myorg/alpine-tools
	return name == "alpine" || strings.HasSuffix(name, "/alpine")
}

// validateSupportedContainerImage enforces the container runtime image policy.
func validateSupportedContainerImage(image string) error {
	if isWindowsContainerImage(image) {
		return fmt.Errorf("windows container images are not supported; the container runtime requires Linux-based images (e.g., debian:stable-slim); see https://invowk.io/docs/runtime-modes/container for details")
	}
	if isAlpineContainerImage(image) {
		return fmt.Errorf("alpine-based container images are not supported; use a Debian-based image (e.g., debian:stable-slim) for reliable execution; see https://invowk.io/docs/runtime-modes/container for details")
	}

	return nil
}

// containerConfigFromRuntime extracts container config from RuntimeConfig
func containerConfigFromRuntime(rt *invowkfile.RuntimeConfig) invowkfileContainerConfig {
	if rt == nil {
		return invowkfileContainerConfig{}
	}
	volumes := make([]string, len(rt.Volumes))
	for i, v := range rt.Volumes {
		volumes[i] = string(v)
	}
	ports := make([]string, len(rt.Ports))
	for i, p := range rt.Ports {
		ports[i] = string(p)
	}
	return invowkfileContainerConfig{
		Containerfile: string(rt.Containerfile),
		Image:         string(rt.Image),
		Volumes:       volumes,
		Ports:         ports,
	}
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
		scriptPath := ctx.SelectedImpl.GetScriptFilePath(string(ctx.Invowkfile.FilePath))
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
		containerPath := "/workspace/" + filepath.Base(tempF.Name())
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
	if effectiveWorkDir == invowkDir {
		return "/workspace"
	}

	// If it's an absolute path, use it directly in the container
	if filepath.IsAbs(effectiveWorkDir) {
		// Check if the path is inside the invowkfile directory (mounted at /workspace)
		relPath, err := filepath.Rel(invowkDir, effectiveWorkDir)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			// Path is within invowkfile dir - map to /workspace
			return "/workspace/" + filepath.ToSlash(relPath)
		}
		// Path is outside invowkfile dir - use as-is (must exist in container or be a mounted path)
		return effectiveWorkDir
	}

	// Relative path - join with /workspace
	return "/workspace/" + filepath.ToSlash(effectiveWorkDir)
}
