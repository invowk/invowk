// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

// CalculateFileHash calculates SHA256 hash of a file's contents.
func CalculateFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }() // Read-only file; close error non-critical

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// CalculateDirHash calculates a hash of a directory's copied contents.
// It includes normalized relative file names and file bytes, matching CopyDir's
// regular-file-only boundary.
// Returns an error if dirPath itself is a symlink (SC-05 defense-in-depth).
func CalculateDirHash(dirPath string) (string, error) {
	// Check if dirPath itself is a symlink — a symlink-to-directory would
	// produce an empty or incorrect hash because WalkDir uses Lstat on the
	// root and reports it as non-directory (SC-05 residual fix).
	rootInfo, err := os.Lstat(dirPath)
	if err != nil {
		return "", fmt.Errorf("lstat %s: %w", dirPath, err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("CalculateDirHash: %q is a symbolic link, not a directory", dirPath)
	}
	root, err := os.OpenRoot(dirPath)
	if err != nil {
		return "", fmt.Errorf("open root %s: %w", dirPath, err)
	}
	defer func() {
		if root.Close() != nil {
			return
		}
	}() // Read-only root handle; close error non-critical

	h := sha256.New()

	var entries []string
	// Use WalkDir and skip symlinks to ensure the hash is computed only over
	// files that CopyDir would actually copy (SC-05 consistency).
	err = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Intentionally skip inaccessible files to continue walking
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil // Skip directories and symlinks/devices.
		}

		if _, infoErr := d.Info(); infoErr != nil {
			return nil //nolint:nilerr // Skip unreadable entries.
		}

		relPath, _ := filepath.Rel(dirPath, path)
		contentFile, openErr := root.Open(relPath)
		if openErr != nil {
			return nil //nolint:nilerr // Skip unreadable entries consistently with CopyDir.
		}
		content, readErr := io.ReadAll(contentFile)
		closeErr := contentFile.Close()
		if readErr != nil {
			return nil //nolint:nilerr // Skip unreadable entries consistently with CopyDir.
		}
		if closeErr != nil {
			return nil //nolint:nilerr // Skip entries whose content handle failed to close.
		}
		entry := relPath + "\x00" + string(content)
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking directory %s: %w", dirPath, err)
	}

	// Sort for consistent ordering
	sort.Strings(entries)

	for _, entry := range entries {
		h.Write([]byte(entry))
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// DiscoverModules finds all .invowkmod directories in the given paths.
func DiscoverModules(paths []types.FilesystemPath) []string {
	var modules []string
	seen := make(map[string]bool)

	for _, basePath := range paths {
		// Use WalkDir and skip symlinked directories to prevent a symlink named
		// *.invowkmod from being discovered as a module (SC-05 consistency).
		_ = filepath.WalkDir(string(basePath), func(path string, d fs.DirEntry, err error) error { //nolint:errcheck // Walk callback returns nil for all errors to continue walking
			if err != nil {
				return nil //nolint:nilerr // Intentionally skip errors to continue walking
			}
			// Skip non-directory entries, including symlinks-to-dirs which
			// WalkDir reports with d.IsDir()=false (SC-05 consistency).
			if !d.IsDir() {
				return nil
			}
			if strings.HasSuffix(d.Name(), ".invowkmod") {
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					modules = append(modules, absPath)
				}
				return filepath.SkipDir // Don't descend into modules
			}
			return nil
		})
	}

	return modules
}

// CopyFile copies a regular file from src to dst. Uses os.Lstat as a
// defense-in-depth layer to skip non-regular files (symlinks, devices),
// preventing TOCTOU races between the caller's directory-level check and
// the actual file read (SC-05). This mirrors the safe pattern in
// pkg/invowkmod/resolver_cache.go:copyFile.
func CopyFile(src, dst string) (err error) {
	// Defense-in-depth: Lstat to detect symlinks without following them.
	// Each layer validates its own invariants rather than trusting the caller.
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("failed to lstat source file: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		return nil // Skip symlinks, devices, etc.
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }() // Read-only file; close error non-critical

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close destination file: %w", closeErr)
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// CopyDir recursively copies a directory from src to dst.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip symlinks to prevent directory traversal attacks during
		// container provisioning (SC-05). Matches the safe pattern in
		// pkg/invowkmod/resolver_cache.go:copyDir.
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}

		if entry.IsDir() {
			if err := CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
