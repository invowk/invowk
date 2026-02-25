// SPDX-License-Identifier: MPL-2.0

package provision

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

// CalculateDirHash calculates a hash of a directory's contents.
// It includes file names, sizes, and modification times for efficiency.
func CalculateDirHash(dirPath string) (string, error) {
	h := sha256.New()

	var entries []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // Intentionally skip inaccessible files to continue walking
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

// DiscoverModules finds all .invowkmod directories in the given paths.
func DiscoverModules(paths []types.FilesystemPath) []string {
	var modules []string
	seen := make(map[string]bool)

	for _, basePath := range paths {
		_ = filepath.Walk(string(basePath), func(path string, info os.FileInfo, err error) error { // Walk never returns error with this callback
			if err != nil {
				return nil //nolint:nilerr // Intentionally skip errors to continue walking
			}
			if info.IsDir() && strings.HasSuffix(info.Name(), ".invowkmod") {
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

// CopyFile copies a file from src to dst.
func CopyFile(src, dst string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = srcFile.Close() }() // Read-only file; close error non-critical

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

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
