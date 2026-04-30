// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/internal/app/modulecache"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// getCachePath returns the cache path for a module.
//
// Known limitation (L-07): URLs differing only by scheme (e.g., https://github.com/user/repo
// and git@github.com:user/repo) normalize to the same cache path. The content hash check
// provides a backstop when a prior lock file exists, but on first sync (no baseline) a
// stale cached copy from a different scheme could be reused undetected.
func (m *Resolver) getCachePath(gitURL, version, subPath string) string {
	urlPath := strings.TrimPrefix(gitURL, "https://")
	urlPath = strings.TrimPrefix(urlPath, "git@")
	urlPath = strings.TrimSuffix(urlPath, ".git")
	urlPath = strings.ReplaceAll(urlPath, ":", "/")

	parts := []string{string(m.cacheDir), urlPath, version}
	if subPath != "" {
		parts = append(parts, subPath)
	}

	return filepath.Join(parts...)
}

// cacheModule copies a module to the cache directory and returns its content hash.
// If the destination already exists and expectedHash is non-empty, the cached content
// is verified against the expected hash. A ContentHashMismatchError is returned on mismatch.
func (m *Resolver) cacheModule(srcDir, dstDir string, expectedHash ContentHash) (ContentHash, error) {
	if _, err := os.Stat(dstDir); err == nil {
		actualHash, hashErr := invowkmod.ComputeModuleHash(dstDir)
		if hashErr != nil {
			return "", fmt.Errorf("failed to hash cached module: %w", hashErr)
		}
		if expectedHash != "" && actualHash != expectedHash {
			return "", &invowkmod.ContentHashMismatchError{
				Expected: expectedHash,
				Actual:   actualHash,
			}
		}
		return actualHash, nil
	}

	if expectedHash == "" {
		slog.Warn("caching module without integrity baseline (first sync)",
			"dst", dstDir)
	}

	if err := os.MkdirAll(filepath.Dir(dstDir), 0o755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	srcPath := types.FilesystemPath(srcDir) //goplint:ignore -- OS-resolved path from repository checkout
	dstPath := types.FilesystemPath(dstDir) //goplint:ignore -- resolver-managed cache path
	if err := modulecache.CopyModuleDir(srcPath, dstPath); err != nil {
		return "", err
	}

	return invowkmod.ComputeModuleHash(dstDir)
}
