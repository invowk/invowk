// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	slashpath "path"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/app/modulecache"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// getCachePath returns the canonical cache path for a module.
//
// Known limitation (L-07): URLs differing only by scheme (e.g., https://github.com/user/repo
// and git@github.com:user/repo) normalize to the same cache path. The content hash check
// provides a backstop when a prior lock file exists, but on first sync (no baseline) a
// stale cached copy from a different scheme could be reused undetected.
func (m *Resolver) getCachePath(gitURL, version, subPath string, moduleID ModuleID) (string, error) {
	canonicalDirName, err := invowkmod.CanonicalModuleDirectoryName(moduleID)
	if err != nil {
		return "", fmt.Errorf("module ID %q for cache path: %w", moduleID, err)
	}

	urlPath := strings.TrimPrefix(gitURL, "https://")
	urlPath = strings.TrimPrefix(urlPath, "git@")
	urlPath = strings.TrimSuffix(urlPath, ".git")
	urlPath = strings.ReplaceAll(urlPath, ":", "/")

	parts := []string{string(m.cacheDir), urlPath, version}
	if subPath != "" {
		path := SubdirectoryPath(subPath)
		if err := path.Validate(); err != nil {
			return "", fmt.Errorf("module cache subpath: %w", err)
		}
		normalized := slashpath.Clean(strings.ReplaceAll(path.String(), "\\", "/"))
		parent := slashpath.Dir(normalized)
		if parent != "." {
			parts = append(parts, strings.Split(parent, "/")...)
		}
	}
	parts = append(parts, canonicalDirName.String())

	return filepath.Join(parts...), nil
}

// cacheModule copies a module to the cache directory and returns its content hash.
// key and version identify the module for error context. When expectedHash is
// non-empty, the module content must hash to it on both paths: an existing
// cache directory is verified before reuse, and a fresh copy is verified after
// copying. A fresh copy that fails verification is removed so unverified
// content never persists in the cache. A ContentHashMismatchError is returned
// on mismatch.
func (m *Resolver) cacheModule(srcDir, dstDir string, key ModuleRefKey, version SemVer, expectedHash ContentHash) (ContentHash, error) {
	_, statErr := os.Stat(dstDir)
	if statErr == nil {
		return m.verifyModuleHash(dstDir, key, version, expectedHash)
	}
	// Only a missing directory proceeds to a fresh copy: any other stat failure
	// must not be mistaken for "not cached", because the fresh-copy path removes
	// the destination on a verification failure.
	if !errors.Is(statErr, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to inspect module cache directory: %w", statErr)
	}

	if expectedHash == "" {
		slog.Warn("caching module without integrity baseline (new module or version change)",
			"dst", dstDir)
	}

	if err := m.copyModuleToCache(srcDir, dstDir); err != nil {
		return "", err
	}

	hash, err := m.verifyModuleHash(dstDir, key, version, expectedHash)
	if errors.Is(err, invowkmod.ErrContentHashMismatch) {
		// dstDir did not exist before this call, so it holds only the copy made here.
		if rmErr := os.RemoveAll(dstDir); rmErr != nil {
			return "", errors.Join(err, fmt.Errorf("failed to remove unverified cache copy %s: %w", dstDir, rmErr))
		}
	}
	return hash, err
}

// copyModuleToCache creates dstDir's parent and copies the module tree into it.
func (m *Resolver) copyModuleToCache(srcDir, dstDir string) error {
	if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	//goplint:ignore -- OS-resolved path from repository checkout.
	srcPath := types.FilesystemPath(srcDir)
	//goplint:ignore -- resolver-managed cache path.
	dstPath := types.FilesystemPath(dstDir)
	return modulecache.CopyModuleDir(srcPath, dstPath)
}

// verifyModuleHash hashes the module tree at dir and, when expectedHash is
// set, checks that it matches. The mismatch error names the cache directory so
// callers can point users at it.
func (m *Resolver) verifyModuleHash(dir string, key ModuleRefKey, version SemVer, expectedHash ContentHash) (ContentHash, error) {
	actual, err := invowkmod.ComputeModuleHash(dir)
	if err != nil {
		return "", fmt.Errorf("failed to hash cached module: %w", err)
	}
	if expectedHash != "" && actual != expectedHash {
		return "", fmt.Errorf("module cache %s: %w", dir, &invowkmod.ContentHashMismatchError{
			ModuleKey: key,
			Version:   version,
			Expected:  expectedHash,
			Actual:    actual,
		})
	}
	return actual, nil
}
