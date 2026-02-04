// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"strings"
	"testing"
)

func TestLayerProvisionerGenerateDockerfile(t *testing.T) {
	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: "/usr/bin/invowk",
		BinaryMountPath:  "/invowk/bin",
		ModulesMountPath: "/invowk/modules",
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
	cfg := &Config{
		Enabled:          true,
		InvowkBinaryPath: "/usr/bin/invowk",
		BinaryMountPath:  "/invowk/bin",
		ModulesMountPath: "/invowk/modules",
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
