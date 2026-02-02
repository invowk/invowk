// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ModuleCachePathEnv is the environment variable for overriding the default module cache path.
	ModuleCachePathEnv = "INVOWK_MODULES_PATH"

	// DefaultModulesDir is the default subdirectory within ~/.invowk for module cache.
	DefaultModulesDir = "modules"
)

// GetDefaultCacheDir returns the default module cache directory.
// It checks INVOWK_MODULES_PATH environment variable first, then falls back to ~/.invowk/modules.
func GetDefaultCacheDir() (string, error) {
	// Check environment variable first
	if envPath := os.Getenv(ModuleCachePathEnv); envPath != "" {
		return envPath, nil
	}

	// Fall back to ~/.invowk/modules
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".invowk", DefaultModulesDir), nil
}

// getCachePath returns the cache path for a module.
func (m *Resolver) getCachePath(gitURL, version, subPath string) string {
	// Convert git URL to path-safe format
	// e.g., "https://github.com/user/repo.git" -> "github.com/user/repo"
	urlPath := strings.TrimPrefix(gitURL, "https://")
	urlPath = strings.TrimPrefix(urlPath, "git@")
	urlPath = strings.TrimSuffix(urlPath, ".git")
	urlPath = strings.ReplaceAll(urlPath, ":", "/")

	parts := []string{m.CacheDir, urlPath, version}
	if subPath != "" {
		parts = append(parts, subPath)
	}

	return filepath.Join(parts...)
}

// cacheModule copies a module to the cache directory.
func (m *Resolver) cacheModule(srcDir, dstDir string) error {
	// Check if already cached
	if _, err := os.Stat(dstDir); err == nil {
		return nil // Already cached
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Copy the module directory
	return copyDir(srcDir, dstDir)
}

// findModuleInDir finds a .invkmod directory or invkmod.cue in the given directory.
// A Git repo is considered a module if:
//   - Repo name ends with .invkmod suffix, OR
//   - Contains an invkmod.cue file at the root
func findModuleInDir(dir string) (moduleDir, moduleName string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read directory: %w", err)
	}

	// First, look for .invkmod directories
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invkmod") {
			moduleName = strings.TrimSuffix(entry.Name(), ".invkmod")
			return filepath.Join(dir, entry.Name()), moduleName, nil
		}
	}

	// Check if this directory IS a module (has invkmod.cue at root)
	// This supports Git repos with .invkmod suffix in their name
	invkmodPath := filepath.Join(dir, "invkmod.cue")
	if _, err := os.Stat(invkmodPath); err == nil {
		// Extract module name from directory (for .invkmod repos)
		dirName := filepath.Base(dir)
		if name, found := strings.CutSuffix(dirName, ".invkmod"); found {
			moduleName = name
		} else {
			// Fall back to parsing invkmod.cue to get the module name
			moduleName = dirName
		}
		return dir, moduleName, nil
	}

	return "", "", fmt.Errorf("no module found in %s (expected .invkmod directory or invkmod.cue)", dir)
}

// copyDir recursively copies a directory, skipping symlinks for security.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if mkdirErr := os.MkdirAll(dst, srcInfo.Mode()); mkdirErr != nil {
		return mkdirErr
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip symlinks to prevent directory traversal attacks
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}

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

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, srcInfo.Mode())
}
