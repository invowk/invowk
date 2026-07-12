// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

const testProvisionBaseImage container.ImageTag = "debian:stable-slim"

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

	key1, err := provisioner.calculateCacheKey(t.Context(), testProvisionBaseImage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key2, err := provisioner.calculateCacheKey(t.Context(), testProvisionBaseImage)
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

	baseConfig := func(binaryPath string) *Config {
		return &Config{
			Enabled: true, InvowkBinaryPath: types.FilesystemPath(binaryPath),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}
	}
	tests := []struct {
		name  string
		setup func(*testing.T) (*Config, container.ImageTag, *Config, container.ImageTag)
	}{
		{
			name: "different base image",
			setup: func(*testing.T) (*Config, container.ImageTag, *Config, container.ImageTag) {
				return baseConfig(binary1), testProvisionBaseImage, baseConfig(binary1), "registry.example.com/base/app:22.04"
			},
		},
		{
			name: "different binary",
			setup: func(*testing.T) (*Config, container.ImageTag, *Config, container.ImageTag) {
				return baseConfig(binary1), testProvisionBaseImage, baseConfig(binary2), testProvisionBaseImage
			},
		},
		{
			name: "with modules",
			setup: func(t *testing.T) (*Config, container.ImageTag, *Config, container.ImageTag) {
				t.Helper()

				modulesDir := filepath.Join(t.TempDir(), "modules")
				createProvisioningModule(t, modulesDir, "test.invowkmod", "test")
				withModules := baseConfig(binary1)
				withModules.ModulesPaths = []types.FilesystemPath{types.FilesystemPath(modulesDir)}
				return withModules, testProvisionBaseImage, baseConfig(binary1), testProvisionBaseImage
			},
		},
		{
			name: "different mount paths",
			setup: func(*testing.T) (*Config, container.ImageTag, *Config, container.ImageTag) {
				alternate := baseConfig(binary1)
				alternate.BinaryMountPath = container.MountTargetPath("/opt/invowk")
				alternate.ModulesMountPath = container.MountTargetPath("/opt/modules")
				return baseConfig(binary1), testProvisionBaseImage, alternate, testProvisionBaseImage
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg1, image1, cfg2, image2 := tt.setup(t)
			p1, err := NewLayerProvisioner(newMockEngine(), cfg1)
			if err != nil {
				t.Fatalf("NewLayerProvisioner(first) error = %v", err)
			}
			p2, err := NewLayerProvisioner(newMockEngine(), cfg2)
			if err != nil {
				t.Fatalf("NewLayerProvisioner(second) error = %v", err)
			}
			key1, err := p1.calculateCacheKey(t.Context(), image1)
			if err != nil {
				t.Fatalf("calculateCacheKey(first) error = %v", err)
			}
			key2, err := p2.calculateCacheKey(t.Context(), image2)
			if err != nil {
				t.Fatalf("calculateCacheKey(second) error = %v", err)
			}
			if key1 == key2 {
				t.Error("calculateCacheKey() values are equal, want input-sensitive keys")
			}
		})
	}
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
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}
	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	key1, err := provisioner.calculateCacheKey(t.Context(), testProvisionBaseImage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writeErr := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("changed workspace content"), 0o644); writeErr != nil {
		t.Fatalf("failed to write workspace file: %v", writeErr)
	}
	key2, err := provisioner.calculateCacheKey(t.Context(), testProvisionBaseImage)
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

	key, err := provisioner.calculateCacheKey(t.Context(), testProvisionBaseImage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if key == "" {
		t.Error("expected non-empty cache key even without binary")
	}
}
