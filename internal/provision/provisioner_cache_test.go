// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

func TestLayerProvisioner_CalculateCacheKey_Determinism(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary-content"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	key1, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key2, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key1 != key2 {
		t.Errorf("expected deterministic cache key, got %q and %q", key1, key2)
	}

	if key1 == "" {
		t.Error("expected non-empty cache key")
	}
}

func TestLayerProvisioner_CalculateCacheKey_DifferentInputs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	binary1 := filepath.Join(tmpDir, "invowk1")
	if err := os.WriteFile(binary1, []byte("binary-v1"), 0o755); err != nil {
		t.Fatalf("failed to create binary1: %v", err)
	}

	binary2 := filepath.Join(tmpDir, "invowk2")
	if err := os.WriteFile(binary2, []byte("binary-v2"), 0o755); err != nil {
		t.Fatalf("failed to create binary2: %v", err)
	}

	engine := newMockEngine()

	t.Run("different base image", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary1),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}
		provisioner, provErr := NewLayerProvisioner(engine, cfg)
		if provErr != nil {
			t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
		}

		key1, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		key2, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("registry.example.com/base/app:22.04"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if key1 == key2 {
			t.Error("expected different keys for different base images")
		}
	})

	t.Run("different binary", func(t *testing.T) {
		t.Parallel()

		cfg1 := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary1),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}
		cfg2 := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary2),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}

		p1, p1Err := NewLayerProvisioner(engine, cfg1)
		if p1Err != nil {
			t.Fatalf("NewLayerProvisioner(cfg1) unexpected error: %v", p1Err)
		}
		p2, p2Err := NewLayerProvisioner(engine, cfg2)
		if p2Err != nil {
			t.Fatalf("NewLayerProvisioner(cfg2) unexpected error: %v", p2Err)
		}

		key1, err := p1.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		key2, err := p2.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if key1 == key2 {
			t.Error("expected different keys for different binaries")
		}
	})

	t.Run("with modules", func(t *testing.T) {
		t.Parallel()

		modulesDir := filepath.Join(tmpDir, "modules")
		modPath := filepath.Join(modulesDir, "test.invowkmod")
		if err := os.MkdirAll(modPath, 0o755); err != nil {
			t.Fatalf("failed to create module dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(modPath, "invowkmod.cue"), []byte("module content"), 0o644); err != nil {
			t.Fatalf("failed to write module file: %v", err)
		}

		cfgWithMods := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary1),
			ModulesPaths:     []types.FilesystemPath{types.FilesystemPath(modulesDir)},
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}
		cfgWithoutMods := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary1),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}

		p1, p1Err := NewLayerProvisioner(engine, cfgWithMods)
		if p1Err != nil {
			t.Fatalf("NewLayerProvisioner(cfgWithMods) unexpected error: %v", p1Err)
		}
		p2, p2Err := NewLayerProvisioner(engine, cfgWithoutMods)
		if p2Err != nil {
			t.Fatalf("NewLayerProvisioner(cfgWithoutMods) unexpected error: %v", p2Err)
		}

		key1, err := p1.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		key2, err := p2.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if key1 == key2 {
			t.Error("expected different keys with and without modules")
		}
	})

	t.Run("different mount paths", func(t *testing.T) {
		t.Parallel()

		cfg1 := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary1),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}
		cfg2 := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binary1),
			BinaryMountPath:  container.MountTargetPath("/opt/invowk"),
			ModulesMountPath: container.MountTargetPath("/opt/modules"),
		}

		p1, p1Err := NewLayerProvisioner(engine, cfg1)
		if p1Err != nil {
			t.Fatalf("NewLayerProvisioner(cfg1) unexpected error: %v", p1Err)
		}
		p2, p2Err := NewLayerProvisioner(engine, cfg2)
		if p2Err != nil {
			t.Fatalf("NewLayerProvisioner(cfg2) unexpected error: %v", p2Err)
		}

		key1, err := p1.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		key2, err := p2.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if key1 == key2 {
			t.Error("expected different keys for different mount paths")
		}
	})
}

func TestLayerProvisioner_CalculateCacheKey_IgnoresWorkspaceContent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary-content"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}
	workspace := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("failed to create workspace: %v", err)
	}
	invowkfilePath := filepath.Join(workspace, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte("cmds: []"), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile: %v", err)
	}

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		InvowkfilePath:   types.FilesystemPath(invowkfilePath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}
	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	key1, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), cfg.InvowkfilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeErr := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("changed workspace content"), 0o644); writeErr != nil {
		t.Fatalf("failed to write workspace file: %v", writeErr)
	}
	key2, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), cfg.InvowkfilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key1 != key2 {
		t.Fatalf("cache key changed after workspace-only content edit: %q != %q", key1, key2)
	}
}

func TestLayerProvisioner_CalculateCacheKey_NoBinary(t *testing.T) {
	t.Parallel()

	engine := newMockEngine()
	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: "", // No binary
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	key, err := provisioner.calculateCacheKey(t.Context(), container.ImageTag("debian:stable-slim"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key == "" {
		t.Error("expected non-empty cache key even without binary")
	}
}
