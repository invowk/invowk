// SPDX-License-Identifier: EPL-2.0

package runtime

import (
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

	if cfg.PacksMountPath != "/invowk/packs" {
		t.Errorf("Expected PacksMountPath to be /invowk/packs, got %s", cfg.PacksMountPath)
	}
}

func TestCalculateFileHash(t *testing.T) {
	// Create a temp file with known content
	tmpFile, err := os.CreateTemp("", "test-hash-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "test content for hashing"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

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
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
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

func TestDiscoverPacks(t *testing.T) {
	// Create a temp directory structure with packs
	tmpDir := t.TempDir()

	// Create some pack directories
	pack1 := filepath.Join(tmpDir, "mypack.invkpack")
	if err := os.MkdirAll(pack1, 0755); err != nil {
		t.Fatalf("Failed to create pack1: %v", err)
	}

	pack2 := filepath.Join(tmpDir, "subdir", "another.invkpack")
	if err := os.MkdirAll(pack2, 0755); err != nil {
		t.Fatalf("Failed to create pack2: %v", err)
	}

	// Create a non-pack directory
	notPack := filepath.Join(tmpDir, "notapack")
	if err := os.MkdirAll(notPack, 0755); err != nil {
		t.Fatalf("Failed to create notapack: %v", err)
	}

	// Discover packs
	packs := discoverPacks([]string{tmpDir})

	if len(packs) != 2 {
		t.Errorf("Expected 2 packs, got %d", len(packs))
	}

	// Verify pack paths
	foundPack1 := false
	foundPack2 := false
	for _, p := range packs {
		if strings.HasSuffix(p, "mypack.invkpack") {
			foundPack1 = true
		}
		if strings.HasSuffix(p, "another.invkpack") {
			foundPack2 = true
		}
	}

	if !foundPack1 {
		t.Error("Expected to find mypack.invkpack")
	}
	if !foundPack2 {
		t.Error("Expected to find another.invkpack")
	}
}

func TestCopyFile(t *testing.T) {
	// Create source file
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "source.txt")
	content := "test content for copy"
	if err := os.WriteFile(srcFile, []byte(content), 0644); err != nil {
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
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
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
		PacksMountPath:   "/invowk/packs",
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	dockerfile := provisioner.generateDockerfile("alpine:latest")

	// Verify Dockerfile content
	if !strings.Contains(dockerfile, "FROM alpine:latest") {
		t.Error("Expected FROM alpine:latest")
	}

	if !strings.Contains(dockerfile, "COPY invowk /invowk/bin/invowk") {
		t.Error("Expected COPY invowk")
	}

	if !strings.Contains(dockerfile, "COPY packs/ /invowk/packs/") {
		t.Error("Expected COPY packs/")
	}

	if !strings.Contains(dockerfile, "ENV PATH=\"/invowk/bin:$PATH\"") {
		t.Error("Expected PATH env var")
	}

	if !strings.Contains(dockerfile, "ENV INVOWK_PACK_PATH=\"/invowk/packs\"") {
		t.Error("Expected INVOWK_PACK_PATH env var")
	}
}

func TestLayerProvisionerBuildEnvVars(t *testing.T) {
	cfg := &ContainerProvisionConfig{
		Enabled:          true,
		InvowkBinaryPath: "/usr/bin/invowk",
		BinaryMountPath:  "/invowk/bin",
		PacksMountPath:   "/invowk/packs",
	}

	provisioner := &LayerProvisioner{
		config: cfg,
	}

	envVars := provisioner.buildEnvVars()

	if envVars["INVOWK_PACK_PATH"] != "/invowk/packs" {
		t.Errorf("Expected INVOWK_PACK_PATH=/invowk/packs, got %s", envVars["INVOWK_PACK_PATH"])
	}

	if !strings.Contains(envVars["PATH"], "/invowk/bin") {
		t.Errorf("Expected PATH to contain /invowk/bin, got %s", envVars["PATH"])
	}
}
