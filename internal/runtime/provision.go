// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"invowk-cli/internal/config"
)

// ContainerProvisionConfig holds configuration for auto-provisioning
// invowk resources into containers.
type ContainerProvisionConfig struct {
	// Enabled controls whether auto-provisioning is active
	Enabled bool

	// InvowkBinaryPath is the path to the invowk binary on the host.
	// If empty, os.Executable() will be used.
	InvowkBinaryPath string

	// PacksPaths are paths to pack directories on the host.
	// These are discovered from config search paths and user commands dir.
	PacksPaths []string

	// InvkfilePath is the path to the current invkfile being executed.
	// This is used to determine what needs to be provisioned.
	InvkfilePath string

	// BinaryMountPath is where to place the binary in the container.
	// Default: /invowk/bin
	BinaryMountPath string

	// PacksMountPath is where to place packs in the container.
	// Default: /invowk/packs
	PacksMountPath string

	// CacheDir is where to store cached provisioned images metadata.
	// Default: ~/.cache/invowk/provision
	CacheDir string
}

// DefaultProvisionConfig returns a ContainerProvisionConfig with default values.
func DefaultProvisionConfig() *ContainerProvisionConfig {
	binaryPath, _ := os.Executable()

	// Discover pack paths from user commands dir and config
	var packsPaths []string
	if userDir, err := config.CommandsDir(); err == nil {
		if info, err := os.Stat(userDir); err == nil && info.IsDir() {
			packsPaths = append(packsPaths, userDir)
		}
	}

	cacheDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		cacheDir = filepath.Join(home, ".cache", "invowk", "provision")
	}

	return &ContainerProvisionConfig{
		Enabled:          true,
		InvowkBinaryPath: binaryPath,
		PacksPaths:       packsPaths,
		BinaryMountPath:  "/invowk/bin",
		PacksMountPath:   "/invowk/packs",
		CacheDir:         cacheDir,
	}
}

// ProvisionResult contains the information about a provisioned container image.
type ProvisionResult struct {
	// ImageTag is the tag of the provisioned image to use
	ImageTag string

	// Cleanup is called to clean up temporary resources after the container exits.
	// This may remove temporary build contexts but typically does NOT remove
	// the cached image (for reuse).
	Cleanup func()

	// EnvVars are environment variables to set in the container
	EnvVars map[string]string
}

// calculateFileHash calculates SHA256 hash of a file's contents.
func calculateFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// calculateDirHash calculates a hash of a directory's contents.
// It includes file names, sizes, and modification times for efficiency.
func calculateDirHash(dirPath string) (string, error) {
	h := sha256.New()

	var entries []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible files
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(dirPath, path)
		entry := fmt.Sprintf("%s:%d:%d", relPath, info.Size(), info.ModTime().Unix())
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort for consistent ordering
	sort.Strings(entries)

	for _, entry := range entries {
		h.Write([]byte(entry))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// discoverPacks finds all .invkpack directories in the given paths.
func discoverPacks(paths []string) []string {
	var packs []string
	seen := make(map[string]bool)

	for _, basePath := range paths {
		filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() && strings.HasSuffix(info.Name(), ".invkpack") {
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					packs = append(packs, absPath)
				}
				return filepath.SkipDir // Don't descend into packs
			}
			return nil
		})
	}

	return packs
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
