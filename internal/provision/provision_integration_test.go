// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

// TestLayerProvisioner_Integration tests the provisioner with a real container engine.
// These tests require Docker or Podman to be available.
func TestLayerProvisioner_Integration(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("skipping provision integration tests: no container engine available: %v", err)
	}
	if !engine.Available() {
		t.Skip("skipping provision integration tests: container engine not available")
	}

	t.Run("CacheMiss_BuildsImage", func(t *testing.T) {
		t.Parallel()
		testutil.AcquireContainerSemaphore(t)
		testCacheMissBuildsImage(t, engine)
	})
	t.Run("CacheHit_SkipsBuild", func(t *testing.T) {
		t.Parallel()
		testutil.AcquireContainerSemaphore(t)
		testCacheHitSkipsBuild(t, engine)
	})
	t.Run("ForceRebuild_RebuildsExistingImage", func(t *testing.T) {
		t.Parallel()
		testutil.AcquireContainerSemaphore(t)
		testForceRebuildRebuildsExistingImage(t, engine)
	})
	t.Run("IsImageProvisioned_TrueAfterProvision", func(t *testing.T) {
		t.Parallel()
		testutil.AcquireContainerSemaphore(t)
		testIsImageProvisionedTrueAfterProvision(t, engine)
	})
}

// setupProvisionTest creates a snap-safe temp directory with a minimal binary stub.
func setupProvisionTest(t *testing.T) string {
	t.Helper()

	tmpDir := testutil.ContainerSafeTempDir(t, "provision-test")

	// Write a minimal binary stub that the provisioner can copy into the image
	binaryPath := filepath.Join(tmpDir, "invowk")
	binaryContent := []byte("#!/bin/sh\nexec /bin/true\n")
	if writeErr := os.WriteFile(binaryPath, binaryContent, 0o755); writeErr != nil {
		t.Fatalf("Failed to write binary stub: %v", writeErr)
	}

	return tmpDir
}

// testTagSuffix returns a unique suffix derived from the test name to ensure
// provisioned image tags don't collide across parallel tests.
func testTagSuffix(t *testing.T) string {
	t.Helper()
	h := sha256.Sum256([]byte(t.Name()))
	return hex.EncodeToString(h[:6])
}

// newTestProvisionConfig returns a base Config for integration tests.
func newTestProvisionConfig(binaryPath, suffix string) *Config {
	return &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		TagSuffix:        suffix,
	}
}

// cleanupProvisionedImage removes the provisioned image after the test.
func cleanupProvisionedImage(t *testing.T, engine container.Engine, tag container.ImageTag) {
	t.Helper()
	t.Cleanup(func() {
		ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
		_ = engine.RemoveImage(ctx, tag, true) // Best-effort cleanup
	})
}

func testCacheMissBuildsImage(t *testing.T, engine container.Engine) {
	t.Helper()

	tmpDir := setupProvisionTest(t)
	binaryPath := filepath.Join(tmpDir, "invowk")
	suffix := testTagSuffix(t)

	cfg := newTestProvisionConfig(binaryPath, suffix)

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
	result, err := provisioner.Provision(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("Provision() unexpected error: %v", err)
	}

	if result.ImageTag == "" {
		t.Fatal("Provision() returned empty image tag")
	}
	if !strings.HasPrefix(string(result.ImageTag), "invowk-provisioned:") {
		t.Errorf("Provision() tag = %q, want prefix 'invowk-provisioned:'", result.ImageTag)
	}

	cleanupProvisionedImage(t, engine, result.ImageTag)

	// Verify the image actually exists in the engine
	exists, existsErr := engine.ImageExists(ctx, result.ImageTag)
	if existsErr != nil {
		t.Fatalf("ImageExists() unexpected error: %v", existsErr)
	}
	if !exists {
		t.Error("ImageExists() = false after Provision(), want true")
	}

	if result.Cleanup != nil {
		result.Cleanup()
	}
}

func testCacheHitSkipsBuild(t *testing.T, engine container.Engine) {
	t.Helper()

	tmpDir := setupProvisionTest(t)
	binaryPath := filepath.Join(tmpDir, "invowk")
	suffix := testTagSuffix(t)

	cfg := newTestProvisionConfig(binaryPath, suffix)

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)

	// First call: builds the image (cache miss)
	result1, err := provisioner.Provision(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("Provision() first call unexpected error: %v", err)
	}
	cleanupProvisionedImage(t, engine, result1.ImageTag)
	if result1.Cleanup != nil {
		result1.Cleanup()
	}

	// Second call: should reuse the cached image (cache hit)
	result2, err := provisioner.Provision(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("Provision() second call unexpected error: %v", err)
	}
	if result2.Cleanup != nil {
		result2.Cleanup()
	}

	// Both calls should produce the same image tag (same inputs = same hash)
	if result1.ImageTag != result2.ImageTag {
		t.Errorf("cache hit should return same tag: first=%q, second=%q", result1.ImageTag, result2.ImageTag)
	}
}

func testForceRebuildRebuildsExistingImage(t *testing.T, engine container.Engine) {
	t.Helper()

	tmpDir := setupProvisionTest(t)
	binaryPath := filepath.Join(tmpDir, "invowk")
	suffix := testTagSuffix(t)

	// First, provision normally to populate the cache
	cfg := newTestProvisionConfig(binaryPath, suffix)

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
	result1, err := provisioner.Provision(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("Provision() initial call unexpected error: %v", err)
	}
	cleanupProvisionedImage(t, engine, result1.ImageTag)
	if result1.Cleanup != nil {
		result1.Cleanup()
	}

	// Now provision with ForceRebuild enabled
	cfgForce := newTestProvisionConfig(binaryPath, suffix)
	cfgForce.ForceRebuild = true

	provisionerForce, provForceErr := NewLayerProvisioner(engine, cfgForce)
	if provForceErr != nil {
		t.Fatalf("NewLayerProvisioner(force) unexpected error: %v", provForceErr)
	}

	result2, err := provisionerForce.Provision(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("Provision(ForceRebuild) unexpected error: %v", err)
	}
	if result2.Cleanup != nil {
		result2.Cleanup()
	}

	// ForceRebuild should succeed and produce a valid provisioned tag
	if !strings.HasPrefix(string(result2.ImageTag), "invowk-provisioned:") {
		t.Errorf("Provision(ForceRebuild) tag = %q, want prefix 'invowk-provisioned:'", result2.ImageTag)
	}

	// Verify the rebuilt image exists
	exists, existsErr := engine.ImageExists(ctx, result2.ImageTag)
	if existsErr != nil {
		t.Fatalf("ImageExists() after ForceRebuild unexpected error: %v", existsErr)
	}
	if !exists {
		t.Error("ImageExists() = false after ForceRebuild, want true")
	}
}

func testIsImageProvisionedTrueAfterProvision(t *testing.T, engine container.Engine) {
	t.Helper()

	tmpDir := setupProvisionTest(t)
	binaryPath := filepath.Join(tmpDir, "invowk")
	suffix := testTagSuffix(t)

	cfg := newTestProvisionConfig(binaryPath, suffix)

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)

	// Before provisioning, the image should not exist
	existsBefore, err := provisioner.IsImageProvisioned(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		// Non-fatal: the image might not exist yet, which is expected.
		// Some engines return error for non-existent images.
		t.Logf("IsImageProvisioned() before provision returned error (expected): %v", err)
	} else if existsBefore {
		// If it already exists from a previous run, that's acceptable but worth noting
		t.Log("IsImageProvisioned() = true before provision (stale cache from previous run)")
	}

	// Provision the image
	result, provisionErr := provisioner.Provision(ctx, container.ImageTag("debian:stable-slim"))
	if provisionErr != nil {
		t.Fatalf("Provision() unexpected error: %v", provisionErr)
	}
	cleanupProvisionedImage(t, engine, result.ImageTag)
	if result.Cleanup != nil {
		result.Cleanup()
	}

	// After provisioning, IsImageProvisioned should return true
	existsAfter, err := provisioner.IsImageProvisioned(ctx, container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("IsImageProvisioned() after provision unexpected error: %v", err)
	}
	if !existsAfter {
		t.Error("IsImageProvisioned() = false after Provision(), want true")
	}
}
