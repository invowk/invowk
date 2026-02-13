// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"invowk-cli/internal/testutil"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Expected Enabled to be true by default")
	}

	if cfg.ForceRebuild {
		t.Error("Expected ForceRebuild to be false by default")
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

func TestConfigOptions(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Apply(
		WithForceRebuild(true),
		WithEnabled(false),
		WithInvowkBinaryPath("/custom/path"),
		WithCacheDir("/custom/cache"),
	)

	if !cfg.ForceRebuild {
		t.Error("Expected ForceRebuild to be true after WithForceRebuild(true)")
	}

	if cfg.Enabled {
		t.Error("Expected Enabled to be false after WithEnabled(false)")
	}

	if cfg.InvowkBinaryPath != "/custom/path" {
		t.Errorf("Expected InvowkBinaryPath to be /custom/path, got %s", cfg.InvowkBinaryPath)
	}

	if cfg.CacheDir != "/custom/cache" {
		t.Errorf("Expected CacheDir to be /custom/cache, got %s", cfg.CacheDir)
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
	hash1, err := CalculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("CalculateFileHash failed: %v", err)
	}

	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Calculate again - should be the same
	hash2, err := CalculateFileHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("CalculateFileHash failed second time: %v", err)
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
	hash1, err := CalculateDirHash(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirHash failed: %v", err)
	}

	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}

	// Calculate again - should be the same
	hash2, err := CalculateDirHash(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirHash failed second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Expected same hash, got %s and %s", hash1, hash2)
	}
}

func TestDiscoverModules(t *testing.T) {
	// Create a temp directory structure with modules
	tmpDir := t.TempDir()

	// Create some module directories
	module1 := filepath.Join(tmpDir, "mymodule.invowkmod")
	if err := os.MkdirAll(module1, 0o755); err != nil {
		t.Fatalf("Failed to create module1: %v", err)
	}

	module2 := filepath.Join(tmpDir, "subdir", "another.invowkmod")
	if err := os.MkdirAll(module2, 0o755); err != nil {
		t.Fatalf("Failed to create module2: %v", err)
	}

	// Create a non-module directory
	notModule := filepath.Join(tmpDir, "notamodule")
	if err := os.MkdirAll(notModule, 0o755); err != nil {
		t.Fatalf("Failed to create notamodule: %v", err)
	}

	// Discover modules
	modules := DiscoverModules([]string{tmpDir})

	if len(modules) != 2 {
		t.Errorf("Expected 2 modules, got %d", len(modules))
	}

	// Verify module paths
	foundModule1 := false
	foundModule2 := false
	for _, m := range modules {
		if strings.HasSuffix(m, "mymodule.invowkmod") {
			foundModule1 = true
		}
		if strings.HasSuffix(m, "another.invowkmod") {
			foundModule2 = true
		}
	}

	if !foundModule1 {
		t.Error("Expected to find mymodule.invowkmod")
	}
	if !foundModule2 {
		t.Error("Expected to find another.invowkmod")
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

	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
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

	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
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

// --- Error Path Tests ---

func TestCalculateFileHash_NonExistentFile(t *testing.T) {
	_, err := CalculateFileHash("/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestCalculateFileHash_DifferentContent(t *testing.T) {
	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content-alpha"), 0o644); err != nil {
		t.Fatalf("failed to write file1: %v", err)
	}

	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content-beta"), 0o644); err != nil {
		t.Fatalf("failed to write file2: %v", err)
	}

	hash1, err := CalculateFileHash(file1)
	if err != nil {
		t.Fatalf("CalculateFileHash(file1) failed: %v", err)
	}

	hash2, err := CalculateFileHash(file2)
	if err != nil {
		t.Fatalf("CalculateFileHash(file2) failed: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes for different content")
	}
}

func TestCalculateDirHash_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	hash, err := CalculateDirHash(tmpDir)
	if err != nil {
		t.Fatalf("CalculateDirHash failed on empty dir: %v", err)
	}

	if hash == "" {
		t.Error("expected non-empty hash for empty directory")
	}
}

func TestCalculateDirHash_NonExistentDir(t *testing.T) {
	// filepath.Walk silently skips the non-existent root (callback returns nil
	// on error), so CalculateDirHash returns the hash of an empty entry list.
	hash, err := CalculateDirHash("/nonexistent/directory/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still produce a non-empty hash (hash of empty input)
	if hash == "" {
		t.Error("expected non-empty hash even for non-existent directory")
	}

	// Hash should match the empty directory hash (same empty entry list)
	emptyDirHash, err := CalculateDirHash(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hash != emptyDirHash {
		t.Errorf("non-existent dir hash should match empty dir hash, got %q vs %q", hash, emptyDirHash)
	}
}

func TestCopyFile_SourceNotFound(t *testing.T) {
	dstDir := t.TempDir()
	err := CopyFile("/nonexistent/file.txt", filepath.Join(dstDir, "dest.txt"))
	if err == nil {
		t.Fatal("expected error when source file does not exist")
	}

	if !strings.Contains(err.Error(), "failed to open source file") {
		t.Errorf("expected 'failed to open source file' in error, got: %v", err)
	}
}

func TestCopyFile_PreservesPermissions(t *testing.T) {
	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "executable.sh")
	if err := os.WriteFile(srcFile, []byte("#!/bin/sh\necho hello"), 0o755); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	dstDir := t.TempDir()
	dstFile := filepath.Join(dstDir, "copy.sh")

	if err := CopyFile(srcFile, dstFile); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	srcInfo, err := os.Stat(srcFile)
	if err != nil {
		t.Fatalf("failed to stat source: %v", err)
	}

	dstInfo, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("failed to stat destination: %v", err)
	}

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("expected mode %v, got %v", srcInfo.Mode(), dstInfo.Mode())
	}
}

func TestCopyDir_SourceNotFound(t *testing.T) {
	err := CopyDir("/nonexistent/directory", filepath.Join(t.TempDir(), "dest"))
	if err == nil {
		t.Fatal("expected error when source directory does not exist")
	}

	if !strings.Contains(err.Error(), "failed to stat source directory") {
		t.Errorf("expected 'failed to stat source directory' in error, got: %v", err)
	}
}

func TestDiscoverModules_EmptyPaths(t *testing.T) {
	modules := DiscoverModules(nil)
	if len(modules) != 0 {
		t.Errorf("expected 0 modules for nil paths, got %d", len(modules))
	}

	modules = DiscoverModules([]string{})
	if len(modules) != 0 {
		t.Errorf("expected 0 modules for empty paths, got %d", len(modules))
	}
}

func TestDiscoverModules_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()

	modPath := filepath.Join(tmpDir, "test.invowkmod")
	if err := os.MkdirAll(modPath, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}

	// Pass the same path twice
	modules := DiscoverModules([]string{tmpDir, tmpDir})

	if len(modules) != 1 {
		t.Errorf("expected 1 module after deduplication, got %d: %v", len(modules), modules)
	}
}

func TestDiscoverModules_MultiplePaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	mod1 := filepath.Join(dir1, "mod1.invowkmod")
	if err := os.MkdirAll(mod1, 0o755); err != nil {
		t.Fatalf("failed to create mod1: %v", err)
	}

	mod2 := filepath.Join(dir2, "mod2.invowkmod")
	if err := os.MkdirAll(mod2, 0o755); err != nil {
		t.Fatalf("failed to create mod2: %v", err)
	}

	modules := DiscoverModules([]string{dir1, dir2})

	if len(modules) != 2 {
		t.Errorf("expected 2 modules across paths, got %d: %v", len(modules), modules)
	}

	// Verify both are found
	found1, found2 := false, false
	for _, m := range modules {
		if strings.HasSuffix(m, "mod1.invowkmod") {
			found1 = true
		}
		if strings.HasSuffix(m, "mod2.invowkmod") {
			found2 = true
		}
	}

	if !found1 {
		t.Error("expected to find mod1.invowkmod")
	}
	if !found2 {
		t.Error("expected to find mod2.invowkmod")
	}
}
