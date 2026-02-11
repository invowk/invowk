// SPDX-License-Identifier: MPL-2.0

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
	"invowk-cli/internal/provision"
	"invowk-cli/pkg/invkfile"
)

// ensureProvisionedImage ensures the container image exists and is provisioned
// with invowk resources (binary, modules, etc.). This enables nested invowk commands
// inside containers.
func (r *ContainerRuntime) ensureProvisionedImage(ctx *ExecutionContext, cfg invkfileContainerConfig, invowkDir string) (imageName string, cleanup func(), err error) {
	// First, ensure the base image exists
	baseImage, err := r.ensureImage(ctx, cfg, invowkDir)
	if err != nil {
		return "", nil, err
	}

	// If provisioning is disabled, return the base image
	if r.provisioner == nil || !r.provisioner.Config().Enabled {
		return baseImage, nil, nil
	}

	// Update provisioner config with current invkfile path and ForceRebuild
	provCfg := r.provisioner.Config()
	provCfg.InvkfilePath = ctx.Invkfile.FilePath
	provCfg.ForceRebuild = ctx.ForceRebuild

	// Provision the image with invowk resources
	if ctx.Verbose {
		_, _ = fmt.Fprintf(ctx.IO.Stdout, "Provisioning container with invowk resources...\n") // Verbose output; error non-critical
	}

	result, err := r.provisioner.Provision(ctx.Context, baseImage)
	if err != nil {
		// If provisioning fails, warn but continue with base image
		_, _ = fmt.Fprintf(ctx.IO.Stderr, "Warning: failed to provision container, using base image: %v\n", err) // Warning output; error non-critical
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

	// Check if image already exists (skip if ForceRebuild is set)
	if !ctx.ForceRebuild {
		exists, err := r.engine.ImageExists(ctx.Context, imageTag)
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
		ContextDir: invowkDir,
		Dockerfile: containerfile,
		Tag:        imageTag,
		NoCache:    ctx.ForceRebuild,
		Stdout:     ctx.IO.Stdout,
		Stderr:     ctx.IO.Stderr,
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

// buildProvisionConfig creates a provision.Config from the app config.
func buildProvisionConfig(cfg *config.Config) *provision.Config {
	provisionCfg := provision.DefaultConfig()

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

	// Also include module paths from the config includes entries so provisioned
	// containers can resolve all configured modules.
	for _, inc := range cfg.Includes {
		if inc.IsModule() {
			provisionCfg.ModulesPaths = append(provisionCfg.ModulesPaths, inc.Path)
		}
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
