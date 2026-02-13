// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"invowk-cli/internal/container"
)

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
type LayerProvisioner struct {
	engine container.Engine
	config *Config
}

// NewLayerProvisioner creates a new LayerProvisioner.
func NewLayerProvisioner(engine container.Engine, cfg *Config) *LayerProvisioner {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &LayerProvisioner{
		engine: engine,
		config: cfg,
	}
}

// Config returns the provisioner's configuration.
func (p *LayerProvisioner) Config() *Config {
	return p.config
}

// Provision creates or retrieves a cached provisioned image based on the
// given base image. The returned Result contains the image tag
// to use and any cleanup functions.
func (p *LayerProvisioner) Provision(ctx context.Context, baseImage string) (*Result, error) {
	if !p.config.Enabled {
		return &Result{
			ImageTag: baseImage,
			EnvVars:  make(map[string]string),
		}, nil
	}

	// Calculate cache key
	cacheKey, err := p.calculateCacheKey(ctx, baseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate cache key: %w", err)
	}

	provisionedTag := p.buildProvisionedTag(cacheKey[:12])

	// Check if cached image exists (skip if ForceRebuild is set)
	if !p.config.ForceRebuild {
		exists, _ := p.engine.ImageExists(ctx, provisionedTag) //nolint:errcheck // Error treated as "not found"
		if exists {
			return &Result{
				ImageTag: provisionedTag,
				EnvVars:  p.buildEnvVars(),
			}, nil
		}
	}

	// Build the provisioned image
	if err := p.buildProvisionedImage(ctx, baseImage, provisionedTag); err != nil {
		return nil, fmt.Errorf("failed to build provisioned image: %w", err)
	}

	return &Result{
		ImageTag: provisionedTag,
		EnvVars:  p.buildEnvVars(),
	}, nil
}

// CleanupProvisionedImages removes all cached provisioned images.
// This can be called periodically to free up disk space.
func (p *LayerProvisioner) CleanupProvisionedImages(ctx context.Context) error {
	// List all images with the invowk-provisioned prefix
	// This would require adding a ListImages method to the Engine interface
	// For now, this is a placeholder
	return nil
}

// GetProvisionedImageTag returns the tag that would be used for a provisioned
// image without actually building it. Useful for checking if an image is cached.
func (p *LayerProvisioner) GetProvisionedImageTag(ctx context.Context, baseImage string) (string, error) {
	cacheKey, err := p.calculateCacheKey(ctx, baseImage)
	if err != nil {
		return "", err
	}
	return p.buildProvisionedTag(cacheKey[:12]), nil
}

// IsImageProvisioned checks if a provisioned image already exists in the cache.
func (p *LayerProvisioner) IsImageProvisioned(ctx context.Context, baseImage string) (bool, error) {
	tag, err := p.GetProvisionedImageTag(ctx, baseImage)
	if err != nil {
		return false, err
	}
	return p.engine.ImageExists(ctx, tag)
}

// buildProvisionedTag constructs the image tag with optional suffix.
// When TagSuffix is set, the tag format is "invowk-provisioned:<hash>-<suffix>".
// This enables test isolation by making each test's images unique.
func (p *LayerProvisioner) buildProvisionedTag(hash string) string {
	if p.config.TagSuffix != "" {
		return fmt.Sprintf("invowk-provisioned:%s-%s", hash, p.config.TagSuffix)
	}
	return fmt.Sprintf("invowk-provisioned:%s", hash)
}

// calculateCacheKey generates a unique key based on all provisioned resources.
func (p *LayerProvisioner) calculateCacheKey(ctx context.Context, baseImage string) (string, error) {
	h := sha256.New()

	// Include base image identifier
	// Try to get image digest for more accurate caching
	imageID, err := p.getImageIdentifier(ctx, baseImage)
	if err != nil {
		// Fall back to image name if we can't get the ID
		imageID = baseImage
	}
	h.Write([]byte("image:" + imageID))

	// Include invowk binary hash
	if p.config.InvowkBinaryPath != "" {
		binaryHash, err := CalculateFileHash(p.config.InvowkBinaryPath)
		if err != nil {
			return "", fmt.Errorf("failed to hash invowk binary: %w", err)
		}
		h.Write([]byte("binary:" + binaryHash))
	}

	// Include modules hash
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

	// Include invowkfile directory hash if set
	if p.config.InvowkfilePath != "" {
		invowkfileDir := filepath.Dir(p.config.InvowkfilePath)
		dirHash, err := CalculateDirHash(invowkfileDir)
		if err == nil {
			h.Write([]byte("invowkfile:" + dirHash))
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// getImageIdentifier tries to get a stable identifier for an image.
func (p *LayerProvisioner) getImageIdentifier(_ context.Context, image string) (string, error) {
	// For now, just use the image name
	// In a more complete implementation, we'd inspect the image to get its digest
	return image, nil
}

// buildProvisionedImage creates the ephemeral image layer.
func (p *LayerProvisioner) buildProvisionedImage(ctx context.Context, baseImage, tag string) error {
	// Create temporary build context
	buildCtx, cleanup, err := p.prepareBuildContext(baseImage)
	if err != nil {
		return err
	}
	defer cleanup()

	// Verify the build context exists and is accessible
	if _, err := os.Stat(buildCtx); os.IsNotExist(err) {
		return fmt.Errorf("build context directory does not exist: %s", buildCtx)
	}

	// Verify Dockerfile exists
	dockerfilePath := filepath.Join(buildCtx, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found in build context: %s", dockerfilePath)
	}

	// Build the image
	buildOpts := container.BuildOptions{
		ContextDir: buildCtx,
		Dockerfile: "Dockerfile",
		Tag:        tag,
		Stdout:     os.Stderr, // Show build progress on stderr
		Stderr:     os.Stderr,
	}

	if err := p.engine.Build(ctx, buildOpts); err != nil {
		return err
	}

	return nil
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
func (p *LayerProvisioner) prepareBuildContext(baseImage string) (buildContextDir string, cleanup func(), err error) {
	// Use a visible directory in user's home that Docker Snap can access
	// Snap's home interface doesn't expose hidden directories (starting with .)
	var buildContextParent string

	// Try HOME first, but verify it actually exists (handles cases like testscript
	// setting HOME=/no-home or misconfigured environments)
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		if _, statErr := os.Stat(home); statErr == nil {
			// Use a visible directory - Docker Snap can access this
			buildContextParent = filepath.Join(home, "invowk-build")
		}
	}

	// Fallback if HOME is unavailable or doesn't exist
	if buildContextParent == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			buildContextParent = filepath.Join(cwd, ".invowk-build")
		} else {
			// Last resort: use system temp (may fail with Snap Docker)
			buildContextParent = filepath.Join(os.TempDir(), "invowk-build")
		}
	}

	// Ensure the parent directory exists
	if mkdirErr := os.MkdirAll(buildContextParent, 0o755); mkdirErr != nil {
		return "", nil, fmt.Errorf("failed to create build context parent directory: %w", mkdirErr)
	}

	tmpDir, err := os.MkdirTemp(buildContextParent, "ctx-*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanup = func() {
		_ = os.RemoveAll(tmpDir) // Cleanup temp dir; error non-critical
	}

	// Copy invowk binary
	if p.config.InvowkBinaryPath != "" {
		binaryDst := filepath.Join(tmpDir, "invowk")
		if err := CopyFile(p.config.InvowkBinaryPath, binaryDst); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("failed to copy invowk binary: %w", err)
		}
		// Ensure binary is executable
		_ = os.Chmod(binaryDst, 0o755) // Best-effort; execution may still work
	}

	// Copy modules
	modulesDir := filepath.Join(tmpDir, "modules")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to create modules directory: %w", err)
	}

	modules := DiscoverModules(p.config.ModulesPaths)
	for _, modulePath := range modules {
		moduleName := filepath.Base(modulePath)
		moduleDst := filepath.Join(modulesDir, moduleName)
		if err := CopyDir(modulePath, moduleDst); err != nil {
			// Log warning but continue - don't fail the whole provision for one module
			fmt.Fprintf(os.Stderr, "Warning: failed to copy module %s: %v\n", moduleName, err)
			continue
		}
	}

	// Generate Dockerfile
	dockerfile := p.generateDockerfile(baseImage)
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	return tmpDir, cleanup, nil
}
