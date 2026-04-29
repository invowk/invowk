// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type fakeProvisioner struct {
	result *provision.Result
	err    error
}

func (p fakeProvisioner) Provision(_ context.Context, _ container.ImageTag) (*provision.Result, error) {
	return p.result, p.err
}

// TestContainerRuntime_SetProvisionConfig tests updating provision config.
func TestContainerRuntime_SetProvisionConfig(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
	}

	// Get initial provisioner
	initialProvisioner := rt.provisioner

	// Set new config
	newCfg := &provision.Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath("/custom/invowk"),
		BinaryMountPath:  container.MountTargetPath("/opt/invowk"),
		ModulesMountPath: container.MountTargetPath("/opt/modules"),
	}
	if err := rt.SetProvisionConfig(newCfg); err != nil {
		t.Fatalf("SetProvisionConfig() unexpected error: %v", err)
	}

	// Provisioner should be replaced
	if rt.provisioner == initialProvisioner {
		t.Error("SetProvisionConfig() should create new provisioner")
	}
}

// TestContainerRuntime_SetProvisionConfig_Nil tests that nil config is handled.
func TestContainerRuntime_SetProvisionConfig_Nil(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
	}

	initialProvisioner := rt.provisioner

	// Setting nil config should not change provisioner
	if err := rt.SetProvisionConfig(nil); err != nil {
		t.Fatalf("SetProvisionConfig(nil) unexpected error: %v", err)
	}

	if rt.provisioner != initialProvisioner {
		t.Error("SetProvisionConfig(nil) should not change provisioner")
	}
}

// TestContainerRuntime_SupportsInteractive tests that container runtime supports interactive mode.
func TestContainerRuntime_SupportsInteractive(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
	}

	if !rt.SupportsInteractive() {
		t.Error("SupportsInteractive() = false, want true")
	}
}

// TestDefaultProvisionConfig_Defaults tests the default provisioning configuration values.
func TestDefaultProvisionConfig_Defaults(t *testing.T) {
	t.Parallel()

	cfg := provision.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Check defaults - values from provision package
	if cfg.BinaryMountPath != "/invowk/bin" {
		t.Errorf("BinaryMountPath = %q, want %q", cfg.BinaryMountPath, "/invowk/bin")
	}
	if cfg.ModulesMountPath != "/invowk/modules" {
		t.Errorf("ModulesMountPath = %q, want %q", cfg.ModulesMountPath, "/invowk/modules")
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
}

// TestContainerRuntime_generateImageTag tests the image tag generation.
func TestContainerRuntime_generateImageTag(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", err)
	}

	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	tag, err := rt.generateImageTag(invowkfilePath)
	if err != nil {
		t.Fatalf("generateImageTag() error: %v", err)
	}

	// Tag should start with "invowk-" and end with ":latest"
	if len(tag) < 20 {
		t.Errorf("generateImageTag() tag too short: %q", tag)
	}
	if tag[:7] != "invowk-" {
		t.Errorf("generateImageTag() tag should start with 'invowk-': %q", tag)
	}
	if tag[len(tag)-7:] != ":latest" {
		t.Errorf("generateImageTag() tag should end with ':latest': %q", tag)
	}

	// Same path should generate same tag
	tag2, _ := rt.generateImageTag(invowkfilePath)
	if tag != tag2 {
		t.Errorf("generateImageTag() should be deterministic: %q != %q", tag, tag2)
	}

	// Different path should generate different tag
	otherPath := filepath.Join(tmpDir, "other", "invowkfile.cue")
	tag3, _ := rt.generateImageTag(otherPath)
	if tag == tag3 {
		t.Errorf("generateImageTag() different paths should generate different tags")
	}
}

// TestBuildProvisionConfig_StrictPropagation tests that the Strict field
// from config.AutoProvisionConfig is propagated to provision.Config.
func TestBuildProvisionConfig_StrictPropagation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		strict bool
	}{
		{"strict enabled", true},
		{"strict disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := config.DefaultConfig()
			cfg.Container.AutoProvision.Strict = tt.strict

			provCfg := buildProvisionConfig(cfg)

			if provCfg.Strict != tt.strict {
				t.Errorf("buildProvisionConfig().Strict = %v, want %v", provCfg.Strict, tt.strict)
			}
		})
	}
}

// TestEnsureProvisionedImage_StrictMode tests that strict provisioning mode
// returns a hard error when provisioning fails.
func TestEnsureProvisionedImage_StrictMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	cmd := &invowkfile.Command{
		Name: "strict-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
		},
	}

	engine := NewMockEngine().WithImageExists(false).WithBuildError(errors.New("disk full"))

	// Configure provisioner with strict=true and a non-existent binary path
	// to force Provision() to fail during resource hash computation.
	provCfg := &provision.Config{
		Enabled:          true,
		Strict:           true,
		InvowkBinaryPath: types.FilesystemPath(filepath.Join(tmpDir, "nonexistent-invowk")),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}
	rt, rtErr := NewContainerRuntimeWithEngine(engine)
	if rtErr != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", rtErr)
	}
	if err := rt.SetProvisionConfig(provCfg); err != nil {
		t.Fatalf("SetProvisionConfig() unexpected error: %v", err)
	}

	execCtx := NewExecutionContext(t.Context(), cmd, inv)

	var stderr bytes.Buffer
	execCtx.IO.Stderr = &stderr
	execCtx.IO.Stdout = &bytes.Buffer{}

	cfg := invowkfileContainerConfig{Image: container.ImageTag("debian:stable-slim")}
	_, _, _, err := rt.ensureProvisionedImage(execCtx, cfg, tmpDir)

	if err == nil {
		t.Fatal("ensureProvisionedImage() with strict=true should return error on provisioning failure")
	}
	if !errors.Is(err, errStrictModeProvisioning) {
		t.Errorf("error should wrap errStrictModeProvisioning, got: %v", err)
	}
}

// TestEnsureProvisionedImage_NonStrictMode tests that non-strict provisioning mode
// falls back to the base image with a warning when provisioning fails.
func TestEnsureProvisionedImage_NonStrictMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}

	cmd := &invowkfile.Command{
		Name: "non-strict-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
		},
	}

	engine := NewMockEngine().WithImageExists(false).WithBuildError(errors.New("disk full"))

	// Configure provisioner with strict=false and a non-existent binary path
	provCfg := &provision.Config{
		Enabled:          true,
		Strict:           false,
		InvowkBinaryPath: types.FilesystemPath(filepath.Join(tmpDir, "nonexistent-invowk")),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}
	rt, rtErr := NewContainerRuntimeWithEngine(engine)
	if rtErr != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() unexpected error: %v", rtErr)
	}
	if err := rt.SetProvisionConfig(provCfg); err != nil {
		t.Fatalf("SetProvisionConfig() unexpected error: %v", err)
	}

	execCtx := NewExecutionContext(t.Context(), cmd, inv)

	var stderr bytes.Buffer
	execCtx.IO.Stderr = &stderr
	execCtx.IO.Stdout = &bytes.Buffer{}

	cfg := invowkfileContainerConfig{Image: container.ImageTag("debian:stable-slim")}
	imageName, _, _, err := rt.ensureProvisionedImage(execCtx, cfg, tmpDir)
	if err != nil {
		t.Fatalf("ensureProvisionedImage() with strict=false should not return error, got: %v", err)
	}
	if imageName != "debian:stable-slim" {
		t.Errorf("imageName = %q, want %q (should fall back to base image)", imageName, "debian:stable-slim")
	}

	// Verify the warning message contains actionable information
	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "WARNING") {
		t.Error("stderr should contain WARNING")
	}
	if !strings.Contains(stderrOutput, "strict") {
		t.Error("stderr should mention strict mode as the remedy")
	}
	if !strings.Contains(stderrOutput, "Nested invowk commands") {
		t.Error("stderr should explain consequences (nested invowk commands won't work)")
	}
}

func TestPrepareCommandIncludesProvisionedEnvVars(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
	}
	cmd := &invowkfile.Command{
		Name: "env-test",
		Implementations: []invowkfile.Implementation{
			{
				Script:    "echo hello",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
				Platforms: invowkfile.AllPlatformConfigs(),
			},
		},
	}

	engine := NewMockEngine()
	rt, err := NewContainerRuntimeWithEngine(
		engine,
		WithContainerProvisioner(
			fakeProvisioner{
				result: &provision.Result{
					ImageTag: container.ImageTag("invowk-provisioned:test"),
					EnvVars: map[string]string{
						"INVOWK_BIN":         "/invowk/bin/invowk",
						"INVOWK_MODULE_PATH": "/invowk/modules",
					},
				},
			},
			&provision.Config{Enabled: true},
		),
	)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}

	execCtx := NewExecutionContext(t.Context(), cmd, inv)
	prepared, err := rt.PrepareCommand(execCtx)
	if err != nil {
		t.Fatalf("PrepareCommand() error = %v", err)
	}
	prepared.Cleanup()

	if len(engine.PrepareRunCalls) != 1 {
		t.Fatalf("PrepareRunCommand calls = %d, want 1", len(engine.PrepareRunCalls))
	}
	env := engine.PrepareRunCalls[0].Env
	if env["INVOWK_BIN"] != "/invowk/bin/invowk" {
		t.Errorf("INVOWK_BIN = %q, want /invowk/bin/invowk", env["INVOWK_BIN"])
	}
	if env["INVOWK_MODULE_PATH"] != "/invowk/modules" {
		t.Errorf("INVOWK_MODULE_PATH = %q, want /invowk/modules", env["INVOWK_MODULE_PATH"])
	}
}
