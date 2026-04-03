// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	lockFileCheckerName = "lockfile"
	// maxLockFileSize matches the CUE guard and the lock file parser DoS protection (5 MiB).
	maxLockFileSize = 5 * 1024 * 1024
)

// LockFileChecker validates lock file integrity: hash mismatches, orphaned or
// missing entries, version checks, and size limits. Only operates on modules
// (standalone invowkfiles have no lock files).
type LockFileChecker struct{}

// NewLockFileChecker creates a LockFileChecker.
func NewLockFileChecker() *LockFileChecker { return &LockFileChecker{} }

// Name returns the checker identifier.
func (c *LockFileChecker) Name() string { return lockFileCheckerName }

// Category returns CategoryIntegrity.
func (c *LockFileChecker) Category() Category { return CategoryIntegrity }

// Check validates lock file integrity for all discovered modules.
func (c *LockFileChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	for _, mod := range sc.Modules() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("lockfile checker cancelled: %w", ctx.Err())
		default:
		}

		if mod.LockFile == nil {
			continue
		}

		findings = append(findings, c.checkSize(mod)...)
		findings = append(findings, c.checkVersion(mod)...)
		findings = append(findings, c.checkHashMismatches(ctx, mod)...)
		findings = append(findings, c.checkOrphanedEntries(mod)...)
		findings = append(findings, c.checkMissingEntries(mod)...)
	}

	return findings, nil
}

func (c *LockFileChecker) checkSize(mod *ScannedModule) []Finding {
	if mod.LockPath == "" {
		return nil
	}
	info, err := os.Stat(string(mod.LockPath))
	if err != nil {
		return nil
	}
	if info.Size() > maxLockFileSize {
		return []Finding{{
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Lock file exceeds size limit",
			Description:    fmt.Sprintf("Lock file is %d bytes, exceeding the %d byte limit — may be crafted for denial-of-service", info.Size(), maxLockFileSize),
			Recommendation: "Inspect the lock file for suspicious content; regenerate with 'invowk module sync'",
		}}
	}
	return nil
}

func (c *LockFileChecker) checkVersion(mod *ScannedModule) []Finding {
	version := mod.LockFile.Version
	if version != invowkmod.LockFileVersionV1 && version != invowkmod.LockFileVersionV2 {
		return []Finding{{
			Severity:       SeverityHigh,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Unknown lock file version",
			Description:    fmt.Sprintf("Lock file version %q is not recognized (expected %s or %s) — may have been crafted", version, invowkmod.LockFileVersionV1, invowkmod.LockFileVersionV2),
			Recommendation: "Regenerate the lock file with 'invowk module sync'",
		}}
	}
	if version == invowkmod.LockFileVersionV1 {
		return []Finding{{
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Lock file uses v1.0 format without content hashes",
			Description:    "Version 1.0 lock files do not include content hashes for tamper detection — upgrade by running 'invowk module sync'",
			Recommendation: "Run 'invowk module sync' to upgrade to v2.0 format with SHA-256 content hashes",
		}}
	}
	return nil
}

func (c *LockFileChecker) checkHashMismatches(ctx context.Context, mod *ScannedModule) []Finding {
	var findings []Finding

	hashes := mod.LockFile.ContentHashes()
	if len(hashes) == 0 {
		return nil
	}

	for _, vendored := range mod.VendoredModules {
		select {
		case <-ctx.Done():
			return findings
		default:
		}

		// Find matching lock entry.
		var matchedKey invowkmod.ModuleRefKey
		var expectedHash invowkmod.ContentHash
		found := false
		for key, hash := range hashes {
			// Match by module ID extracted from the namespace.
			lockMod := mod.LockFile.Modules[key]
			nsID := string(invowkmod.ExtractModuleIDFromNamespace(lockMod.Namespace))
			if nsID == string(vendored.Metadata.Module) {
				matchedKey = key
				expectedHash = hash
				found = true
				break
			}
		}
		if !found {
			continue
		}

		actualHash, err := invowkmod.ComputeModuleHash(string(vendored.Path))
		if err != nil {
			continue
		}

		if actualHash != expectedHash {
			findings = append(findings, Finding{
				Severity:       SeverityCritical,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       types.FilesystemPath(string(vendored.Path)),
				Title:          "Module content hash mismatch",
				Description:    fmt.Sprintf("Vendored module %q has hash %s but lock file expects %s — module may have been tampered with", matchedKey, actualHash, expectedHash),
				Recommendation: "Re-vendor with 'invowk module sync' and verify the module source has not been compromised",
			})
		}
	}

	return findings
}

func (c *LockFileChecker) checkOrphanedEntries(mod *ScannedModule) []Finding {
	var findings []Finding

	// Build set of vendored module IDs.
	vendoredIDs := make(map[string]bool)
	for _, v := range mod.VendoredModules {
		vendoredIDs[string(v.Metadata.Module)] = true
	}

	for key := range mod.LockFile.Modules {
		lockMod := mod.LockFile.Modules[key]
		nsID := string(invowkmod.ExtractModuleIDFromNamespace(lockMod.Namespace))
		if !vendoredIDs[nsID] {
			findings = append(findings, Finding{
				Severity:       SeverityLow,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       mod.LockPath,
				Title:          "Orphaned lock file entry",
				Description:    fmt.Sprintf("Lock file contains entry %q which is not present in vendored modules", key),
				Recommendation: "Run 'invowk module tidy' to remove stale lock file entries",
			})
		}
	}

	return findings
}

func (c *LockFileChecker) checkMissingEntries(mod *ScannedModule) []Finding {
	var findings []Finding

	if mod.Module == nil || mod.Module.Metadata == nil {
		return nil
	}

	// Check that each required module has a lock file entry.
	for _, req := range mod.Module.Metadata.Requires {
		reqKey := string(req.GitURL)
		if req.Path != "" {
			reqKey += "#" + string(req.Path)
		}

		found := false
		for key := range mod.LockFile.Modules {
			if strings.Contains(string(key), string(req.GitURL)) {
				found = true
				break
			}
		}

		if !found {
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       types.FilesystemPath(filepath.Join(string(mod.Path), "invowkmod.cue")),
				Title:          "Required module has no lock file entry",
				Description:    fmt.Sprintf("Required dependency %q has no corresponding entry in the lock file", reqKey),
				Recommendation: "Run 'invowk module sync' to resolve and lock all dependencies",
			})
		}
	}

	return findings
}
