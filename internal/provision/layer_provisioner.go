// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

const provisionedLayerCacheVersion = "1"

// Compile-time interface check
var _ Provisioner = (*LayerProvisioner)(nil)

// LayerProvisioner creates ephemeral container image layers that include
// invowk resources (binary, modules, etc.) on top of a base image.
//
// The provisioned images are cached based on a hash of:
// - Base image digest
// - invowk binary hash
// - modules directory hash
//
// This allows fast reuse when resources haven't changed.
type (
	imageBuilder interface {
		ImageExists(context.Context, container.ImageTag) (bool, error)
		Build(context.Context, container.BuildOptions) error
	}

	LayerProvisioner struct {
		engine  imageBuilder
		config  *Config
		copyDir func(src, dst string) error
	}
)

// NewLayerProvisioner creates a new LayerProvisioner.
// It validates the Config if provided; nil defaults to DefaultConfig().
func NewLayerProvisioner(engine imageBuilder, cfg *Config) (*LayerProvisioner, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("provision config: %w", err)
	}
	return &LayerProvisioner{
		engine: engine,
		config: cfg,
	}, nil
}

// Config returns the provisioner's configuration.
func (p *LayerProvisioner) Config() *Config {
	return p.config
}

func writerOrDefault(w, fallback io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}

// Provision creates or retrieves a cached provisioned image based on the
// given base image. The returned Result contains the image tag
// to use and any cleanup functions.
func (p *LayerProvisioner) Provision(ctx context.Context, req Request) (*Result, error) {
	if req.BaseImage == "" {
		req.BaseImage = "debian:stable-slim"
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("provision request: %w", err)
	}
	if !p.config.Enabled {
		return &Result{
			ImageTag: req.BaseImage,
			EnvVars:  make(map[string]string),
		}, nil
	}

	// Calculate cache key
	cacheKey, err := p.calculateCacheKey(ctx, req.BaseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate cache key: %w", err)
	}

	provisionedTag := container.ImageTag(p.buildProvisionedTag(cacheKey[:12]))
	if validateErr := provisionedTag.Validate(); validateErr != nil {
		return nil, fmt.Errorf("provisioned image tag: %w", validateErr)
	}

	// Check if cached image exists (skip if ForceRebuild is set)
	if !req.ForceRebuild && !p.config.ForceRebuild {
		exists, _ := p.engine.ImageExists(ctx, provisionedTag) //nolint:errcheck // Error treated as "not found"
		if exists {
			return &Result{
				ImageTag: provisionedTag,
				EnvVars:  p.buildEnvVars(),
			}, nil
		}
	}

	// Build the provisioned image
	warnings, err := p.buildProvisionedImage(ctx, req, provisionedTag)
	if err != nil {
		return nil, fmt.Errorf("failed to build provisioned image: %w", err)
	}

	return &Result{
		ImageTag: provisionedTag,
		EnvVars:  p.buildEnvVars(),
		Warnings: warnings,
	}, nil
}

// CleanupProvisionedImages removes all cached provisioned images.
// This can be called periodically to free up disk space.
func (p *LayerProvisioner) CleanupProvisionedImages(_ context.Context) error {
	// List all images with the invowk-provisioned prefix
	// This would require adding a ListImages method to the Engine interface
	// For now, this is a placeholder
	return nil
}

// GetProvisionedImageTag returns the tag that would be used for a provisioned
// image without actually building it. Useful for checking if an image is cached.
func (p *LayerProvisioner) GetProvisionedImageTag(ctx context.Context, baseImage container.ImageTag) (string, error) {
	if err := baseImage.Validate(); err != nil {
		return "", fmt.Errorf("base image: %w", err)
	}
	cacheKey, err := p.calculateCacheKey(ctx, baseImage)
	if err != nil {
		return "", err
	}
	return p.buildProvisionedTag(cacheKey[:12]), nil
}

// IsImageProvisioned checks if a provisioned image already exists in the cache.
func (p *LayerProvisioner) IsImageProvisioned(ctx context.Context, baseImage container.ImageTag) (bool, error) {
	tag, err := p.GetProvisionedImageTag(ctx, baseImage)
	if err != nil {
		return false, err
	}
	imgTag := container.ImageTag(tag)
	if err := imgTag.Validate(); err != nil {
		return false, fmt.Errorf("provisioned image tag: %w", err)
	}
	return p.engine.ImageExists(ctx, imgTag)
}

// buildProvisionedTag constructs the image tag with optional suffix.
// When TagSuffix is set, the tag format is "invowk-provisioned:<hash>-<suffix>".
// This enables test isolation by making each test's images unique.
func (p *LayerProvisioner) buildProvisionedTag(hash string) string {
	if p.config.TagSuffix != "" {
		return fmt.Sprintf("invowk-provisioned:%s-%s", hash, p.config.TagSuffix)
	}
	return "invowk-provisioned:" + hash
}

// calculateCacheKey generates a unique key based on all provisioned layer resources.
func (p *LayerProvisioner) calculateCacheKey(ctx context.Context, baseImage container.ImageTag) (string, error) {
	h := sha256.New()

	h.Write([]byte("provision_layer_version:" + provisionedLayerCacheVersion))

	// Include base image identifier
	// Try to get image digest for more accurate caching
	imageID, err := p.getImageIdentifier(ctx, baseImage)
	if err != nil {
		// Fall back to image name if we can't get the ID
		imageID = string(baseImage)
	}
	h.Write([]byte("image:" + imageID))

	// Include invowk binary hash
	if p.config.InvowkBinaryPath != "" {
		binaryHash, err := CalculateFileHash(string(p.config.InvowkBinaryPath))
		if err != nil {
			return "", fmt.Errorf("failed to hash invowk binary: %w", err)
		}
		h.Write([]byte("binary:" + binaryHash))
		h.Write([]byte("binary_mount:" + string(p.config.BinaryMountPath)))
	}

	// Include modules hash
	h.Write([]byte("modules_mount:" + string(p.config.ModulesMountPath)))
	modules := DiscoverModules(p.config.ModulesPaths)
	for _, modulePath := range modules {
		moduleHash, err := CalculateDirHash(modulePath)
		if err != nil {
			// Skip modules that can't be hashed
			continue
		}
		moduleName := filepath.Base(modulePath)
		h.Write([]byte("module:" + moduleName + ":" + moduleHash))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// getImageIdentifier tries to get a stable identifier for an image.
func (p *LayerProvisioner) getImageIdentifier(_ context.Context, image container.ImageTag) (string, error) {
	// For now, just use the image name
	// In a more complete implementation, we'd inspect the image to get its digest
	return string(image), nil
}

// buildProvisionedImage creates the ephemeral image layer.
func (p *LayerProvisioner) buildProvisionedImage(ctx context.Context, req Request, tag container.ImageTag) ([]Warning, error) {
	// Create temporary build context
	buildCtx, warnings, cleanup, err := p.prepareBuildContext(req.BaseImage)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// Verify the build context exists and is accessible
	if _, err := os.Stat(buildCtx); os.IsNotExist(err) {
		return nil, fmt.Errorf("build context directory does not exist: %s", buildCtx)
	}

	// Verify Dockerfile exists
	dockerfilePath := filepath.Join(buildCtx, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("dockerfile not found in build context: %s", dockerfilePath)
	}

	// Build the image
	ctxDir := container.HostFilesystemPath(buildCtx)
	if err := ctxDir.Validate(); err != nil {
		return nil, fmt.Errorf("build context directory: %w", err)
	}
	buildOpts := container.BuildOptions{
		ContextDir: ctxDir,
		Dockerfile: "Dockerfile",
		Tag:        tag,
		Stdout:     writerOrDefault(req.Stdout, os.Stderr),
		Stderr:     writerOrDefault(req.Stderr, os.Stderr),
	}

	if err := p.engine.Build(ctx, buildOpts); err != nil {
		return nil, err
	}

	return warnings, nil
}

// prepareBuildContext creates a temporary directory with all resources
// needed to build the provisioned image.
//
// Note: Docker installed via Snap has limited filesystem access:
// - Cannot access /tmp (different namespace)
// - Cannot access hidden directories like ~/.cache (home interface restriction)
// - CAN access visible directories in $HOME like ~/invowk-build
//
// We use a visible directory in the user's home as the build context location.
func (p *LayerProvisioner) prepareBuildContext(baseImage container.ImageTag) (buildContextDir string, warnings []Warning, cleanup func(), err error) {
	buildContextParent, parentCleanup, err := p.resolveBuildContextParent()
	if err != nil {
		return "", nil, nil, err
	}

	// Ensure the parent directory exists
	if mkdirErr := os.MkdirAll(string(buildContextParent), 0o755); mkdirErr != nil {
		return "", nil, nil, fmt.Errorf("failed to create build context parent directory: %w", mkdirErr)
	}

	tmpDir, err := os.MkdirTemp(string(buildContextParent), "ctx-*")
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir) // Cleanup temp dir; error non-critical
		if parentCleanup != nil {
			parentCleanup()
		}
	}

	// Copy invowk binary
	if p.config.InvowkBinaryPath != "" {
		binaryDst := filepath.Join(tmpDir, "invowk")
		if err := CopyFile(string(p.config.InvowkBinaryPath), binaryDst); err != nil {
			cleanup()
			return "", nil, nil, fmt.Errorf("failed to copy invowk binary: %w", err)
		}
		// Ensure binary is executable
		_ = os.Chmod(binaryDst, 0o755) // Best-effort; execution may still work
	}

	// Copy modules
	modulesDir := filepath.Join(tmpDir, "modules")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		cleanup()
		return "", nil, nil, fmt.Errorf("failed to create modules directory: %w", err)
	}

	copyDir := p.copyDir
	if copyDir == nil {
		copyDir = CopyDir
	}
	modules := DiscoverModules(p.config.ModulesPaths)
	for _, modulePath := range modules {
		moduleName := filepath.Base(modulePath)
		moduleDst := filepath.Join(modulesDir, moduleName)
		if err := copyDir(modulePath, moduleDst); err != nil {
			warnings = append(warnings, Warning{
				Message: WarningMessage(fmt.Sprintf("failed to copy module %s: %v", moduleName, err)),
			})
			continue
		}
	}

	// Generate Dockerfile
	dockerfile := p.generateDockerfile(string(baseImage))
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
		cleanup()
		return "", nil, nil, fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	return tmpDir, warnings, cleanup, nil
}

func (p *LayerProvisioner) resolveBuildContextParent() (parent types.FilesystemPath, cleanup func(), err error) {
	if p.config.CacheDir != "" {
		return p.config.CacheDir, nil, nil
	}

	// Try HOME first, but verify it actually exists (handles cases like testscript
	// setting HOME=/no-home or misconfigured environments). Use a visible
	// directory because Docker Snap cannot access hidden directories.
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		if _, statErr := os.Stat(home); statErr == nil {
			parentPath := types.FilesystemPath(filepath.Join(home, "invowk-build"))
			if pathErr := parentPath.Validate(); pathErr != nil {
				return "", nil, pathErr
			}
			return parentPath, nil, nil
		}
	}

	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		parentPath := types.FilesystemPath(filepath.Join(cwd, ".invowk-build"))
		if pathErr := parentPath.Validate(); pathErr != nil {
			return "", nil, pathErr
		}
		return parentPath, nil, nil
	}

	// Last resort: create a random parent in the system temp directory. This
	// avoids predictable paths in world-writable temp locations.
	tempParent, tempErr := os.MkdirTemp("", "invowk-build-*")
	if tempErr != nil {
		return "", nil, fmt.Errorf("failed to create fallback build context parent directory: %w", tempErr)
	}
	tempCleanup := func() {
		_ = os.RemoveAll(tempParent) // Best-effort cleanup of fallback parent dir
	}
	parentPath := types.FilesystemPath(tempParent)
	if pathErr := parentPath.Validate(); pathErr != nil {
		tempCleanup()
		return "", nil, pathErr
	}
	return parentPath, tempCleanup, nil
}
