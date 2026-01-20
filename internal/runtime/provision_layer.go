// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"invowk-cli/internal/container"
	"os"
	"path/filepath"
	"strings"
)

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
	config *ContainerProvisionConfig
}

// NewLayerProvisioner creates a new LayerProvisioner.
func NewLayerProvisioner(engine container.Engine, cfg *ContainerProvisionConfig) *LayerProvisioner {
	if cfg == nil {
		cfg = DefaultProvisionConfig()
	}
	return &LayerProvisioner{
		engine: engine,
		config: cfg,
	}
}

// Provision creates or retrieves a cached provisioned image based on the
// given base image. The returned ProvisionResult contains the image tag
// to use and any cleanup functions.
func (p *LayerProvisioner) Provision(ctx context.Context, baseImage string) (*ProvisionResult, error) {
	if !p.config.Enabled {
		return &ProvisionResult{
			ImageTag: baseImage,
			EnvVars:  make(map[string]string),
		}, nil
	}

	// Calculate cache key
	cacheKey, err := p.calculateCacheKey(ctx, baseImage)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate cache key: %w", err)
	}

	provisionedTag := fmt.Sprintf("invowk-provisioned:%s", cacheKey[:12])

	// Check if cached image exists
	exists, _ := p.engine.ImageExists(ctx, provisionedTag) //nolint:errcheck // Error treated as "not found"
	if exists {
		return &ProvisionResult{
			ImageTag: provisionedTag,
			EnvVars:  p.buildEnvVars(),
		}, nil
	}

	// Build the provisioned image
	if err := p.buildProvisionedImage(ctx, baseImage, provisionedTag); err != nil {
		return nil, fmt.Errorf("failed to build provisioned image: %w", err)
	}

	return &ProvisionResult{
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
	return fmt.Sprintf("invowk-provisioned:%s", cacheKey[:12]), nil
}

// IsImageProvisioned checks if a provisioned image already exists in the cache.
func (p *LayerProvisioner) IsImageProvisioned(ctx context.Context, baseImage string) (bool, error) {
	tag, err := p.GetProvisionedImageTag(ctx, baseImage)
	if err != nil {
		return false, err
	}
	return p.engine.ImageExists(ctx, tag)
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
		binaryHash, err := calculateFileHash(p.config.InvowkBinaryPath)
		if err != nil {
			return "", fmt.Errorf("failed to hash invowk binary: %w", err)
		}
		h.Write([]byte("binary:" + binaryHash))
	}

	// Include modules hash
	modules := discoverModules(p.config.ModulesPaths)
	for _, modulePath := range modules {
		moduleHash, err := calculateDirHash(modulePath)
		if err != nil {
			// Skip modules that can't be hashed
			continue
		}
		moduleName := filepath.Base(modulePath)
		h.Write([]byte("module:" + moduleName + ":" + moduleHash))
	}

	// Include invkfile directory hash if set
	if p.config.InvkfilePath != "" {
		invkfileDir := filepath.Dir(p.config.InvkfilePath)
		dirHash, err := calculateDirHash(invkfileDir)
		if err == nil {
			h.Write([]byte("invkfile:" + dirHash))
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// getImageIdentifier tries to get a stable identifier for an image.
func (p *LayerProvisioner) getImageIdentifier(ctx context.Context, image string) (string, error) {
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
		// Log the build context path for debugging
		fmt.Fprintf(os.Stderr, "Debug: Build context was at: %s\n", buildCtx)

		// List contents of build context for debugging
		entries, _ := os.ReadDir(buildCtx)
		fmt.Fprintf(os.Stderr, "Debug: Build context contents:\n")
		for _, e := range entries {
			fmt.Fprintf(os.Stderr, "  - %s\n", e.Name())
		}

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

	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		// Use a visible directory - Docker Snap can access this
		buildContextParent = filepath.Join(home, "invowk-build")
	} else {
		// Fallback: try current working directory
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
		if err := copyFile(p.config.InvowkBinaryPath, binaryDst); err != nil {
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

	modules := discoverModules(p.config.ModulesPaths)
	for _, modulePath := range modules {
		moduleName := filepath.Base(modulePath)
		moduleDst := filepath.Join(modulesDir, moduleName)
		if err := copyDir(modulePath, moduleDst); err != nil {
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

// generateDockerfile creates the Dockerfile content for the provisioned image.
func (p *LayerProvisioner) generateDockerfile(baseImage string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("FROM %s\n\n", baseImage))
	sb.WriteString("# Invowk auto-provisioned layer\n")
	sb.WriteString("# This layer adds invowk binary and modules to enable nested invowk commands\n\n")

	// Copy invowk binary
	if p.config.InvowkBinaryPath != "" {
		binaryPath := p.config.BinaryMountPath
		sb.WriteString("# Install invowk binary\n")
		sb.WriteString(fmt.Sprintf("COPY invowk %s/invowk\n", binaryPath))
		sb.WriteString(fmt.Sprintf("RUN chmod +x %s/invowk\n\n", binaryPath))
	}

	// Copy modules
	modulesPath := p.config.ModulesMountPath
	sb.WriteString("# Install modules\n")
	sb.WriteString(fmt.Sprintf("COPY modules/ %s/\n\n", modulesPath))

	// Set environment variables
	sb.WriteString("# Configure environment\n")
	if p.config.InvowkBinaryPath != "" {
		sb.WriteString(fmt.Sprintf("ENV PATH=\"%s:$PATH\"\n", p.config.BinaryMountPath))
	}
	sb.WriteString(fmt.Sprintf("ENV INVOWK_MODULE_PATH=\"%s\"\n", modulesPath)) //nolint:gocritic // Dockerfile ENV syntax requires literal quotes

	return sb.String()
}

// buildEnvVars returns environment variables to set in the container.
func (p *LayerProvisioner) buildEnvVars() map[string]string {
	env := make(map[string]string)

	// PATH is set in the Dockerfile, but we also set it here for consistency
	if p.config.InvowkBinaryPath != "" {
		env["PATH"] = p.config.BinaryMountPath + ":/usr/local/bin:/usr/bin:/bin"
	}

	env["INVOWK_MODULE_PATH"] = p.config.ModulesMountPath

	return env
}
