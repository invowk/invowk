// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"invowk-cli/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultProvisionConfig(t *testing.T) {
	cfg := DefaultProvisionConfig()

	if !cfg.Enabled {
		t.Error("Expected Enabled to be true by default")
	}

	if cfg.InvowkBinaryPath == "" {
		t.Error("Expected InvowkBinaryPath to be set from os.Executable()")
	}

	if cfg.BinaryMountPath != "/invowk/bin" {
		t.Errorf("Expected BinaryMountPath to be /invowk/bin, got %s", cfg.BinaryMountPath)
	}

	if cfg.ModulesMountPath != "/invowk/modules" {
		t.Errorf("Expected ModulesMountPath to be /invowk/modules, got %s", cfg.ModulesMountPath)
	}
}

func TestCalculateFileHash(t *testing.T) {
	// Create a temp file with known content
	tmpFile, err := os.CreateTemp("", "test-hash-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }() // Cleanup temp file; error non-critical

	content := "test content for hashing"
	if _, writeErr := tmpFile.WriteString(content); writeErr != nil {
		t.Fatalf("Failed to write temp file: %v", writeErr)
	}
	testutil.MustClose(t, tmpFile)

	// Calculate hash
	hash1, err := calculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("calculateFileHash failed: %v", err)
	}

	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Calculate again - should be the same
	hash2, err := calculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("calculateFileHash failed second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Expected same hash, got %s and %s", hash1, hash2)
	}
}

func TestCalculateDirHash(t *testing.T) {
	// Create a temp directory with files
	tmpDir := t.TempDir()

	// Create some files
	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Calculate hash
	hash1, err := calculateDirHash(tmpDir)
	if err != nil {
		t.Fatalf("calculateDirHash failed: %v", err)
	}

	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Calculate again - should be the same
	hash2, err := calculateDirHash(tmpDir)
	if err != nil {
		t.Fatalf("calculateDirHash failed second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Expected same hash, got %s and %s", hash1, hash2)
	}
}

func TestDiscoverModules(t *testing.T) {
	// Create a temp directory structure with modules
	tmpDir := t.TempDir()

	// Create some module directories
	module1 := filepath.Join(tmpDir, "mymodule.invkmod")
	if err := os.MkdirAll(module1, 0o755); err != nil {
		t.Fatalf("Failed to create module1: %v", err)
	}

	module2 := filepath.Join(tmpDir, "subdir", "another.invkmod")
	if err := os.MkdirAll(module2, 0o755); err != nil {
		t.Fatalf("Failed to create module2: %v", err)
	}

	// Create a non-module directory
	notModule := filepath.Join(tmpDir, "notamodule")
	if err := os.MkdirAll(notModule, 0o755); err != nil {
		t.Fatalf("Failed to create notamodule: %v", err)
	}

	// Discover modules
	modules := discoverModules([]string{tmpDir})

	if len(modules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(modules))
	}

	// Verify module paths
	foundModule1 := false
	foundModule2 := false
	for _, m := range modules {
		if strings.HasSuffix(m, "mymodule.invkmod") {
			foundModule1 = true
		}
		if strings.HasSuffix(m, "another.invkmod") {
			foundModule2 = true
		}
	}

	if !foundModule1 {
		t.Error("Expected to find mymodule.invkmod")
	}
	if !foundModule2 {
		t.Error("Expected to find another.invkmod")
	}
}

func TestCopyFile(t *testing.T) {
	// Create source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	content := "test content for copy"
	if err := os.WriteFile(srcFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Copy to destination
	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "dest.txt")

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content %q, got %q", content, string(data))
	}
}

func TestCopyDir(t *testing.T) {
	// Create source directory with files
	srcDir := t.TempDir()

	// Create files
	file1 := filepath.Join(srcDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Copy to destination
	dstDir := filepath.Join(t.TempDir(), "dest")

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("Expected file1.txt to exist")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("Expected subdir/file2.txt to exist")
	}

	// Verify content
	data, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatalf("Failed to read file1.txt: %v", err)
	}
	if string(data) != "content1" {
		t.Errorf("Expected content1, got %s", string(data))
	}
}

func TestLayerProvisionerGenerateDockerfile(t *testing.T) {
	cfg := &ContainerProvisionConfig{
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
	cfg := &ContainerProvisionConfig{
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
