// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

// mockEngine implements container.Engine for testing provisioner logic
// without requiring real Docker/Podman.
type mockEngine struct {
	// imageExistsResult controls what ImageExists returns
	imageExistsResult bool
	// imageExistsErr controls the error ImageExists returns
	imageExistsErr error
	// buildErr controls the error Build returns
	buildErr error

	// buildCalls records Build invocations for assertion
	buildCalls []container.BuildOptions
	// imageExistsCalls records ImageExists invocations
	imageExistsCalls []string
}

func newMockEngine() *mockEngine {
	return &mockEngine{
		buildCalls:       make([]container.BuildOptions, 0),
		imageExistsCalls: make([]string, 0),
	}
}

func (m *mockEngine) Name() string                                 { return "mock" }
func (m *mockEngine) Available() bool                              { return true }
func (m *mockEngine) BinaryPath() string                           { return "/usr/bin/mock" }
func (m *mockEngine) BuildRunArgs(_ container.RunOptions) []string { return []string{"run"} }
func (m *mockEngine) PrepareRunCommand(ctx context.Context, opts container.RunOptions) *exec.Cmd {
	return exec.CommandContext(ctx, m.BinaryPath(), m.BuildRunArgs(opts)...)
}

func (m *mockEngine) Version(_ context.Context) (string, error) {
	return "mock-1.0.0", nil
}

func (m *mockEngine) Build(_ context.Context, opts container.BuildOptions) error {
	m.buildCalls = append(m.buildCalls, opts)
	return m.buildErr
}

func (m *mockEngine) Run(_ context.Context, _ container.RunOptions) (*container.RunResult, error) {
	return &container.RunResult{}, nil
}

func (m *mockEngine) Remove(_ context.Context, _ container.ContainerID, _ bool) error {
	return nil
}

func (m *mockEngine) ImageExists(_ context.Context, image container.ImageTag) (bool, error) {
	m.imageExistsCalls = append(m.imageExistsCalls, string(image))
	return m.imageExistsResult, m.imageExistsErr
}

func (m *mockEngine) RemoveImage(_ context.Context, _ container.ImageTag, _ bool) error {
	return nil
}

// --- Provision Tests ---

func TestLayerProvisioner_Provision_Disabled(t *testing.T) {
	t.Parallel()

	engine := newMockEngine()
	cfg := &Config{
		Enabled:          false,
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	result, err := provisioner.Provision(t.Context(), Request{BaseImage: container.ImageTag("debian:stable-slim")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// When disabled, should return the base image unchanged
	if result.ImageTag != "debian:stable-slim" {
		t.Errorf("expected base image tag, got %q", result.ImageTag)
	}

	if result.EnvVars == nil {
		t.Error("expected non-nil EnvVars map")
	}

	// Should not call engine at all
	if len(engine.buildCalls) > 0 {
		t.Error("expected no build calls when disabled")
	}
	if len(engine.imageExistsCalls) > 0 {
		t.Error("expected no image exists calls when disabled")
	}
}

func TestLayerProvisioner_ProvisionRejectsInvalidBaseImage(t *testing.T) {
	t.Parallel()

	engine := newMockEngine()
	cfg := &Config{
		Enabled:          false,
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	_, err := provisioner.Provision(t.Context(), Request{BaseImage: container.ImageTag("debian:stable-slim\nRUN echo bad")})
	if err == nil {
		t.Fatal("Provision() returned nil error, want invalid base image error")
	}
	if !errors.Is(err, container.ErrInvalidImageTag) {
		t.Errorf("Provision() error = %v, want ErrInvalidImageTag", err)
	}
	if len(engine.buildCalls) > 0 {
		t.Error("expected no build calls for invalid request")
	}
	if len(engine.imageExistsCalls) > 0 {
		t.Error("expected no image exists calls for invalid request")
	}
}

func TestLayerProvisioner_Provision_CacheHit(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a fake binary so hash calculation succeeds
	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	engine := newMockEngine()
	engine.imageExistsResult = true // Simulate cached image exists

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	result, err := provisioner.Provision(t.Context(), Request{BaseImage: container.ImageTag("debian:stable-slim")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return a provisioned tag
	if !strings.HasPrefix(string(result.ImageTag), "invowk-provisioned:") {
		t.Errorf("expected provisioned tag, got %q", result.ImageTag)
	}

	// Should check ImageExists but NOT build
	if len(engine.imageExistsCalls) != 1 {
		t.Errorf("expected 1 ImageExists call, got %d", len(engine.imageExistsCalls))
	}
	if len(engine.buildCalls) != 0 {
		t.Error("expected no build calls on cache hit")
	}

	// Should include env vars
	if result.EnvVars["INVOWK_MODULE_PATH"] != "/invowk/modules" {
		t.Errorf("expected INVOWK_MODULE_PATH=/invowk/modules, got %q", result.EnvVars["INVOWK_MODULE_PATH"])
	}
}

func TestLayerProvisioner_Provision_ForceRebuild(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a fake binary
	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	engine := newMockEngine()
	engine.imageExistsResult = true // Would be a cache hit normally

	cfg := &Config{
		Enabled:          true,
		ForceRebuild:     true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	result, err := provisioner.Provision(t.Context(), Request{BaseImage: container.ImageTag("debian:stable-slim")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still return a provisioned tag
	if !strings.HasPrefix(string(result.ImageTag), "invowk-provisioned:") {
		t.Errorf("expected provisioned tag, got %q", result.ImageTag)
	}

	// Should NOT check ImageExists (skipped due to ForceRebuild)
	if len(engine.imageExistsCalls) != 0 {
		t.Errorf("expected no ImageExists calls with ForceRebuild, got %d", len(engine.imageExistsCalls))
	}

	// Should call Build
	if len(engine.buildCalls) != 1 {
		t.Fatalf("expected 1 build call, got %d", len(engine.buildCalls))
	}

	// Verify build options
	buildOpts := engine.buildCalls[0]
	if buildOpts.Tag == "" {
		t.Error("expected non-empty tag in build options")
	}
	if buildOpts.Dockerfile != "Dockerfile" {
		t.Errorf("expected Dockerfile name 'Dockerfile', got %q", buildOpts.Dockerfile)
	}
}

func TestLayerProvisioner_Provision_CacheMiss(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a fake binary
	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	engine := newMockEngine()
	engine.imageExistsResult = false // Cache miss

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	result, err := provisioner.Provision(t.Context(), Request{BaseImage: container.ImageTag("debian:stable-slim")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return a provisioned tag
	if !strings.HasPrefix(string(result.ImageTag), "invowk-provisioned:") {
		t.Errorf("expected provisioned tag, got %q", result.ImageTag)
	}

	// Should check ImageExists AND build
	if len(engine.imageExistsCalls) != 1 {
		t.Errorf("expected 1 ImageExists call, got %d", len(engine.imageExistsCalls))
	}
	if len(engine.buildCalls) != 1 {
		t.Fatalf("expected 1 build call, got %d", len(engine.buildCalls))
	}
}

func TestLayerProvisioner_ProvisionUsesRequestWriters(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	engine := newMockEngine()
	engine.imageExistsResult = false
	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}
	provisioner, provErr := NewLayerProvisioner(engine, cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	var stdout, stderr bytes.Buffer
	_, err := provisioner.Provision(t.Context(), Request{
		BaseImage: container.ImageTag("debian:stable-slim"),
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if err != nil {
		t.Fatalf("Provision() = %v", err)
	}
	if len(engine.buildCalls) != 1 {
		t.Fatalf("build calls = %d, want 1", len(engine.buildCalls))
	}
	if engine.buildCalls[0].Stdout != &stdout {
		t.Fatal("BuildOptions.Stdout did not use request stdout")
	}
	if engine.buildCalls[0].Stderr != &stderr {
		t.Fatal("BuildOptions.Stderr did not use request stderr")
	}
}

// --- GetProvisionedImageTag Tests ---

func TestLayerProvisioner_GetProvisionedImageTag(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
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

	tag, err := provisioner.GetProvisionedImageTag(t.Context(), container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(tag, "invowk-provisioned:") {
		t.Errorf("expected provisioned tag prefix, got %q", tag)
	}

	// Verify determinism: same inputs produce same tag
	tag2, err := provisioner.GetProvisionedImageTag(t.Context(), container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tag != tag2 {
		t.Errorf("expected deterministic tag, got %q and %q", tag, tag2)
	}

	// Different base image should produce different tag
	tag3, err := provisioner.GetProvisionedImageTag(t.Context(), container.ImageTag("registry.example.com/base/app:22.04"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tag == tag3 {
		t.Errorf("expected different tag for different base image, both are %q", tag)
	}
}

func TestLayerProvisioner_GetProvisionedImageTagRejectsInvalidBaseImage(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:          true,
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}
	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	_, err := provisioner.GetProvisionedImageTag(t.Context(), container.ImageTag("debian:stable-slim\nRUN echo bad"))
	if err == nil {
		t.Fatal("GetProvisionedImageTag() returned nil error, want invalid base image error")
	}
	if !errors.Is(err, container.ErrInvalidImageTag) {
		t.Fatalf("GetProvisionedImageTag() error = %v, want ErrInvalidImageTag", err)
	}
}

// --- IsImageProvisioned Tests ---

func TestLayerProvisioner_IsImageProvisioned(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	t.Run("image exists", func(t *testing.T) {
		t.Parallel()

		engine := newMockEngine()
		engine.imageExistsResult = true

		cfg := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binaryPath),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}

		provisioner, provErr := NewLayerProvisioner(engine, cfg)
		if provErr != nil {
			t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
		}

		exists, err := provisioner.IsImageProvisioned(t.Context(), container.ImageTag("debian:stable-slim"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !exists {
			t.Error("expected image to exist")
		}
	})

	t.Run("image does not exist", func(t *testing.T) {
		t.Parallel()

		engine := newMockEngine()
		engine.imageExistsResult = false

		cfg := &Config{
			Enabled:          true,
			InvowkBinaryPath: types.FilesystemPath(binaryPath),
			BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
			ModulesMountPath: container.MountTargetPath("/invowk/modules"),
		}

		provisioner, provErr := NewLayerProvisioner(engine, cfg)
		if provErr != nil {
			t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
		}

		exists, err := provisioner.IsImageProvisioned(t.Context(), container.ImageTag("debian:stable-slim"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if exists {
			t.Error("expected image to not exist")
		}
	})
}

// --- calculateCacheKey Tests ---

// --- prepareBuildContext Tests ---

func TestLayerProvisioner_PrepareBuildContext(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
		t.Fatalf("failed to create fake binary: %v", err)
	}

	// Create a module directory
	modulesDir := filepath.Join(tmpDir, "modules")
	modPath := filepath.Join(modulesDir, "example.invowkmod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modPath, "invowkmod.cue"), []byte("test module"), 0o644); err != nil {
		t.Fatalf("failed to write module file: %v", err)
	}

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath(binaryPath),
		ModulesPaths:     []types.FilesystemPath{types.FilesystemPath(modulesDir)},
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	buildCtx, warnings, cleanup, err := provisioner.prepareBuildContext(container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	// Verify build context directory exists
	if _, statErr := os.Stat(buildCtx); os.IsNotExist(statErr) {
		t.Fatal("build context directory does not exist")
	}

	// Verify Dockerfile was generated
	dockerfilePath := filepath.Join(buildCtx, "Dockerfile")
	dockerfileContent, readErr := os.ReadFile(dockerfilePath)
	if readErr != nil {
		t.Fatalf("failed to read Dockerfile: %v", readErr)
	}

	if !strings.Contains(string(dockerfileContent), "FROM debian:stable-slim") {
		t.Error("Dockerfile should contain FROM instruction")
	}

	// Verify binary was copied
	copiedBinary := filepath.Join(buildCtx, "invowk")
	if _, statErr := os.Stat(copiedBinary); os.IsNotExist(statErr) {
		t.Error("invowk binary should be copied to build context")
	}

	// Verify modules directory exists
	copiedModules := filepath.Join(buildCtx, "modules")
	if _, statErr := os.Stat(copiedModules); os.IsNotExist(statErr) {
		t.Error("modules directory should exist in build context")
	}
}

func TestLayerProvisioner_PrepareBuildContext_NoBinary(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: "", // No binary
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	buildCtx, warnings, cleanup, err := provisioner.prepareBuildContext(container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	// Should still create the build context (with Dockerfile and modules dir)
	if _, statErr := os.Stat(buildCtx); os.IsNotExist(statErr) {
		t.Fatal("build context directory should exist")
	}

	// Binary should NOT be in the build context
	copiedBinary := filepath.Join(buildCtx, "invowk")
	if _, statErr := os.Stat(copiedBinary); !os.IsNotExist(statErr) {
		t.Error("invowk binary should not be in build context when path is empty")
	}

	// Dockerfile should still be generated
	dockerfilePath := filepath.Join(buildCtx, "Dockerfile")
	content, readErr := os.ReadFile(dockerfilePath)
	if readErr != nil {
		t.Fatalf("failed to read Dockerfile: %v", readErr)
	}

	// Should NOT have COPY invowk instruction
	if strings.Contains(string(content), "COPY invowk") {
		t.Error("Dockerfile should not contain COPY invowk when binary path is empty")
	}
}

func TestLayerProvisioner_PrepareBuildContext_UsesCacheDir(t *testing.T) {
	t.Parallel()

	cacheDir := filepath.Join(t.TempDir(), "provision-cache")
	cfg := &Config{
		Enabled:          true,
		CacheDir:         types.FilesystemPath(cacheDir),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}

	buildCtx, warnings, cleanup, err := provisioner.prepareBuildContext(container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("prepareBuildContext() error = %v", err)
	}
	defer cleanup()
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if filepath.Dir(buildCtx) != cacheDir {
		t.Fatalf("build context parent = %q, want %q", filepath.Dir(buildCtx), cacheDir)
	}
	if filepath.Base(buildCtx) == "" || !strings.HasPrefix(filepath.Base(buildCtx), "ctx-") {
		t.Fatalf("build context = %q, want ctx-* child", buildCtx)
	}
}

func TestLayerProvisioner_PrepareBuildContext_Cleanup(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "invowk")
	if err := os.WriteFile(binaryPath, []byte("fake-binary"), 0o755); err != nil {
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

	buildCtx, warnings, cleanup, err := provisioner.prepareBuildContext(container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	// Verify build context exists before cleanup
	if _, err := os.Stat(buildCtx); os.IsNotExist(err) {
		t.Fatal("build context should exist before cleanup")
	}

	// Call cleanup
	cleanup()

	// Verify build context is removed after cleanup
	if _, err := os.Stat(buildCtx); !os.IsNotExist(err) {
		t.Error("build context should be removed after cleanup")
	}
}

func TestLayerProvisioner_PrepareBuildContext_ModuleCopyWarnings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	modulesDir := filepath.Join(tmpDir, "modules")
	modPath := filepath.Join(modulesDir, "broken.invowkmod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	cfg := &Config{
		Enabled:          true,
		ModulesPaths:     []types.FilesystemPath{types.FilesystemPath(modulesDir)},
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner, provErr := NewLayerProvisioner(newMockEngine(), cfg)
	if provErr != nil {
		t.Fatalf("NewLayerProvisioner() unexpected error: %v", provErr)
	}
	provisioner.copyDir = func(_, _ string) error {
		return errors.New("copy failed")
	}

	buildCtx, warnings, cleanup, err := provisioner.prepareBuildContext(container.ImageTag("debian:stable-slim"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()
	if buildCtx == "" {
		t.Fatal("expected build context to be created despite module copy warning")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	warning := warnings[0].Message.String()
	if !strings.Contains(warning, "failed to copy module broken.invowkmod") || !strings.Contains(warning, "copy failed") {
		t.Fatalf("warning = %q, want module name and copy error", warning)
	}
}

// --- Config Options Coverage ---

func TestConfigOptions_WithModulesPaths(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	paths := []types.FilesystemPath{types.FilesystemPath("/path/to/modules1"), types.FilesystemPath("/path/to/modules2")}
	cfg.Apply(WithModulesPaths(paths))

	if len(cfg.ModulesPaths) != 2 {
		t.Errorf("expected 2 module paths, got %d", len(cfg.ModulesPaths))
	}

	if cfg.ModulesPaths[0] != "/path/to/modules1" {
		t.Errorf("expected first path '/path/to/modules1', got %q", cfg.ModulesPaths[0])
	}
}
