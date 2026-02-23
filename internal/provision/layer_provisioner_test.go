// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"os"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/types"
)

func TestLayerProvisionerGenerateDockerfile(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath("/usr/bin/invowk"),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	dockerfile := provisioner.generateDockerfile("debian:stable-slim")

	// Verify Dockerfile content
	if !strings.Contains(dockerfile, "FROM debian:stable-slim") {
		t.Error("Expected FROM debian:stable-slim")
	}

	if !strings.Contains(dockerfile, "COPY invowk /invowk/bin/invowk") {
		t.Error("Expected COPY invowk")
	}

	if !strings.Contains(dockerfile, "COPY modules/ /invowk/modules/") {
		t.Error("Expected COPY modules/")
	}

	if !strings.Contains(dockerfile, "ENV PATH=\"/invowk/bin:$PATH\"") {
		t.Error("Expected PATH env var")
	}

	if !strings.Contains(dockerfile, "ENV INVOWK_MODULE_PATH=\"/invowk/modules\"") {
		t.Error("Expected INVOWK_MODULE_PATH env var")
	}
}

func TestLayerProvisionerBuildEnvVars(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: types.FilesystemPath("/usr/bin/invowk"),
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	envVars := provisioner.buildEnvVars()

	if envVars["INVOWK_MODULE_PATH"] != "/invowk/modules" {
		t.Errorf("Expected INVOWK_MODULE_PATH=/invowk/modules, got %s", envVars["INVOWK_MODULE_PATH"])
	}

	if !strings.Contains(envVars["PATH"], "/invowk/bin") {
		t.Errorf("Expected PATH to contain /invowk/bin, got %s", envVars["PATH"])
	}
}

func TestLayerProvisionerConfigAccessor(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:      true,
		ForceRebuild: true,
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	if provisioner.Config() != cfg {
		t.Error("Config() should return the provisioner's config")
	}

	if !provisioner.Config().ForceRebuild {
		t.Error("Expected ForceRebuild to be true")
	}
}

func TestNewLayerProvisionerWithNilConfig(t *testing.T) {
	t.Parallel()

	// NewLayerProvisioner should use DefaultConfig when passed nil
	provisioner := NewLayerProvisioner(nil, nil)

	if provisioner.config == nil {
		t.Fatal("Expected config to be set to defaults when nil passed")
	}

	if !provisioner.config.Enabled {
		t.Error("Expected Enabled to be true by default")
	}

	if provisioner.config.BinaryMountPath != "/invowk/bin" {
		t.Errorf("Expected BinaryMountPath to be /invowk/bin, got %s", provisioner.config.BinaryMountPath)
	}
}

func TestBuildProvisionedTagWithoutSuffix(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:   true,
		TagSuffix: "", // No suffix
	}
	provisioner := &LayerProvisioner{config: cfg}

	tag := provisioner.buildProvisionedTag("abc123def456")

	expected := "invowk-provisioned:abc123def456"
	if tag != expected {
		t.Errorf("buildProvisionedTag without suffix: got %q, want %q", tag, expected)
	}
}

func TestBuildProvisionedTagWithSuffix(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:   true,
		TagSuffix: "test1234",
	}
	provisioner := &LayerProvisioner{config: cfg}

	tag := provisioner.buildProvisionedTag("abc123def456")

	expected := "invowk-provisioned:abc123def456-test1234"
	if tag != expected {
		t.Errorf("buildProvisionedTag with suffix: got %q, want %q", tag, expected)
	}
}

func TestUniqueSuffixesProduceUniqueTags(t *testing.T) {
	t.Parallel()

	hash := "abc123def456"

	cfg1 := &Config{TagSuffix: "suffix1"}
	cfg2 := &Config{TagSuffix: "suffix2"}

	p1 := &LayerProvisioner{config: cfg1}
	p2 := &LayerProvisioner{config: cfg2}

	tag1 := p1.buildProvisionedTag(hash)
	tag2 := p2.buildProvisionedTag(hash)

	if tag1 == tag2 {
		t.Errorf("Unique suffixes should produce unique tags, but both are: %s", tag1)
	}

	// Verify both contain the hash
	if !strings.Contains(tag1, hash) {
		t.Errorf("tag1 should contain hash %q: %s", hash, tag1)
	}
	if !strings.Contains(tag2, hash) {
		t.Errorf("tag2 should contain hash %q: %s", hash, tag2)
	}
}

func TestWithTagSuffixOption(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	// Apply the WithTagSuffix option
	cfg.Apply(WithTagSuffix("my-suffix"))

	if cfg.TagSuffix != "my-suffix" {
		t.Errorf("WithTagSuffix option: got %q, want %q", cfg.TagSuffix, "my-suffix")
	}
}

func TestDefaultConfigReadsTagSuffixFromEnv(t *testing.T) {
	// Set the environment variable
	const testSuffix = "env-test-suffix"
	os.Setenv("INVOWK_PROVISION_TAG_SUFFIX", testSuffix)
	defer os.Unsetenv("INVOWK_PROVISION_TAG_SUFFIX")

	cfg := DefaultConfig()

	if cfg.TagSuffix != testSuffix {
		t.Errorf("DefaultConfig should read TagSuffix from env: got %q, want %q", cfg.TagSuffix, testSuffix)
	}
}

func TestLayerProvisionerGenerateDockerfile_NoBinary(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: "", // No binary path
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	dockerfile := provisioner.generateDockerfile("debian:stable-slim")

	// Should have FROM instruction
	if !strings.Contains(dockerfile, "FROM debian:stable-slim") {
		t.Error("Expected FROM debian:stable-slim")
	}

	// Should NOT have COPY invowk instruction
	if strings.Contains(dockerfile, "COPY invowk") {
		t.Error("Should not have COPY invowk when binary path is empty")
	}

	// Should NOT have RUN chmod instruction
	if strings.Contains(dockerfile, "RUN chmod") {
		t.Error("Should not have RUN chmod when binary path is empty")
	}

	// Should NOT have PATH env var
	if strings.Contains(dockerfile, "ENV PATH=") {
		t.Error("Should not set PATH when binary path is empty")
	}

	// Should still have modules COPY and module path env var
	if !strings.Contains(dockerfile, "COPY modules/") {
		t.Error("Expected COPY modules/ even without binary")
	}

	if !strings.Contains(dockerfile, "ENV INVOWK_MODULE_PATH=") {
		t.Error("Expected INVOWK_MODULE_PATH even without binary")
	}
}

func TestLayerProvisionerBuildEnvVars_NoBinary(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: "", // No binary path
		BinaryMountPath:  container.MountTargetPath("/invowk/bin"),
		ModulesMountPath: container.MountTargetPath("/invowk/modules"),
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	envVars := provisioner.buildEnvVars()

	// PATH should NOT be set when there's no binary
	if _, ok := envVars["PATH"]; ok {
		t.Error("PATH should not be set when binary path is empty")
	}

	// INVOWK_MODULE_PATH should still be set
	if envVars["INVOWK_MODULE_PATH"] != "/invowk/modules" {
		t.Errorf("Expected INVOWK_MODULE_PATH=/invowk/modules, got %q", envVars["INVOWK_MODULE_PATH"])
	}
}
