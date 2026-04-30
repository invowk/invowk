// SPDX-License-Identifier: MPL-2.0

// Package modulecache owns filesystem cache operations for module sync and vendor workflows.
package modulecache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// ModuleCachePathEnv is the environment variable for overriding the default module cache path.
	ModuleCachePathEnv = "INVOWK_MODULES_PATH"

	// DefaultModulesDir is the default subdirectory within ~/.invowk for module cache.
	DefaultModulesDir = "modules"
)

// ErrModuleNotFoundInDir is returned when a directory does not contain
// a .invowkmod subdirectory or an invowkmod.cue file.
var ErrModuleNotFoundInDir = errors.New("no module found")

// DefaultDir returns the default module cache directory.
// It checks INVOWK_MODULES_PATH first, then falls back to ~/.invowk/modules.
func DefaultDir() (types.FilesystemPath, error) {
	return DefaultDirWith(os.Getenv)
}

// DefaultDirWith returns the default module cache directory using getenv.
func DefaultDirWith(getenv func(string) string) (types.FilesystemPath, error) {
	if envPath := getenv(ModuleCachePathEnv); envPath != "" {
		cachePath := types.FilesystemPath(envPath)
		if err := cachePath.Validate(); err != nil {
			return "", fmt.Errorf("module cache path from %s: %w", ModuleCachePathEnv, err)
		}
		return cachePath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	cachePath := types.FilesystemPath(filepath.Join(homeDir, ".invowk", DefaultModulesDir))
	if err := cachePath.Validate(); err != nil {
		return "", fmt.Errorf("module cache path: %w", err)
	}
	return cachePath, nil
}

// LocateModuleInDir finds a module directory inside dir.
func LocateModuleInDir(dir types.FilesystemPath) (types.FilesystemPath, invowkmod.ModuleShortName, error) {
	moduleDir, moduleName, err := findModuleInDir(string(dir))
	return types.FilesystemPath(moduleDir), moduleName, err //goplint:ignore -- result is returned from filesystem traversal
}

// CopyModuleDir recursively copies a module directory, skipping symlinks.
func CopyModuleDir(src, dst types.FilesystemPath) error {
	return copyDir(string(src), string(dst))
}

// findModuleInDir finds a .invowkmod directory or invowkmod.cue in the given directory.
//
//goplint:ignore -- helper inspects transient OS-native cache paths from filesystem traversal.
func findModuleInDir(dir string) (moduleDir string, moduleName invowkmod.ModuleShortName, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), invowkmod.ModuleSuffix) {
			moduleName, err = newModuleShortName(strings.TrimSuffix(entry.Name(), invowkmod.ModuleSuffix))
			if err != nil {
				return "", "", err
			}
			return filepath.Join(dir, entry.Name()), moduleName, nil
		}
	}

	invowkmodPath := filepath.Join(dir, "invowkmod.cue")
	if _, err := os.Stat(invowkmodPath); err == nil {
		dirName := filepath.Base(dir)
		if name, found := strings.CutSuffix(dirName, invowkmod.ModuleSuffix); found {
			moduleName, err = newModuleShortName(name)
		} else {
			moduleName, err = newModuleShortName(dirName)
		}
		if err != nil {
			return "", "", err
		}
		return dir, moduleName, nil
	}

	return "", "", fmt.Errorf("%w in %s (expected .invowkmod directory or invowkmod.cue)", ErrModuleNotFoundInDir, dir)
}

//goplint:ignore -- helper validates module names derived from OS directory entries.
func newModuleShortName(raw string) (invowkmod.ModuleShortName, error) {
	name := invowkmod.ModuleShortName(raw)
	if err := name.Validate(); err != nil {
		return "", err
	}
	return name, nil
}

//goplint:ignore -- helper copies transient OS-native cache paths.
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
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

//goplint:ignore -- helper copies transient OS-native cache paths.
func copyFile(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("lstat %s: %w", src, err)
	}
	if !srcInfo.Mode().IsRegular() {
		return nil
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, srcInfo.Mode())
}
