// SPDX-License-Identifier: MPL-2.0

package invowkmod

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
	return GetDefaultCacheDirWith(os.Getenv)
}

// GetDefaultCacheDirWith returns the default module cache directory using the provided
// getenv function. This enables testing without mutating process-global environment state.
func GetDefaultCacheDirWith(getenv func(string) string) (string, error) {
	if envPath := getenv(ModuleCachePathEnv); envPath != "" {
		return envPath, nil
	}

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

	parts := []string{m.cacheDir, urlPath, version}
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

// findModuleInDir finds a .invowkmod directory or invowkmod.cue in the given directory.
// A Git repo is considered a module if:
//   - Repo name ends with .invowkmod suffix, OR
//   - Contains an invowkmod.cue file at the root
func findModuleInDir(dir string) (moduleDir, moduleName string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read directory: %w", err)
	}

	// First, look for .invowkmod directories
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".invowkmod") {
			moduleName = strings.TrimSuffix(entry.Name(), ".invowkmod")
			return filepath.Join(dir, entry.Name()), moduleName, nil
		}
	}

	// Check if this directory IS a module (has invowkmod.cue at root)
	// This supports Git repos with .invowkmod suffix in their name
	invowkmodPath := filepath.Join(dir, "invowkmod.cue")
	if _, err := os.Stat(invowkmodPath); err == nil {
		// Extract module name from directory (for .invowkmod repos)
		dirName := filepath.Base(dir)
		if name, found := strings.CutSuffix(dirName, ".invowkmod"); found {
			moduleName = name
		} else {
			// Fall back to parsing invowkmod.cue to get the module name
			moduleName = dirName
		}
		return dir, moduleName, nil
	}

	return "", "", fmt.Errorf("no module found in %s (expected .invowkmod directory or invowkmod.cue)", dir)
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
