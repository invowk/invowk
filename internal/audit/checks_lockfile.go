// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkmod"
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

		// Flag lock files that exist on disk but failed to parse — they would
		// otherwise appear as absent, masking an integrity issue.
		if mod.LockFileParseErr != nil {
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       mod.LockPath,
				Title:          "Lock file present but unparseable",
				Description:    fmt.Sprintf("Lock file exists but failed to parse: %v", mod.LockFileParseErr),
				Recommendation: "Regenerate the lock file with 'invowk module sync'; inspect for corruption if the error persists",
			})
			continue // Do not run integrity checks against a corrupt lock file.
		}

		if mod.LockFile == nil {
			// Flag modules with declared dependencies but no lock file — dependency
			// integrity cannot be verified without one.
			if mod.Module != nil && mod.Module.Metadata != nil && len(mod.Module.Metadata.Requires) > 0 {
				findings = append(findings, Finding{
					Severity:       SeverityHigh,
					Category:       CategoryIntegrity,
					SurfaceID:      mod.SurfaceID,
					CheckerName:    lockFileCheckerName,
					FilePath:       fspath.JoinStr(mod.Path, "invowkmod.cue"),
					Title:          "Module has dependencies but no lock file",
					Description:    fmt.Sprintf("Module declares %d dependencies in requires but has no lock file — dependency integrity cannot be verified", len(mod.Module.Metadata.Requires)),
					Recommendation: "Run 'invowk module sync' to generate a lock file with SHA-256 content hashes",
				})
			}

			// Flag modules with vendored content but no lock file — manually placed
			// or stale vendored modules bypass integrity verification entirely.
			if len(mod.VendoredModules) > 0 {
				findings = append(findings, Finding{
					Severity:       SeverityMedium,
					Category:       CategoryIntegrity,
					SurfaceID:      mod.SurfaceID,
					CheckerName:    lockFileCheckerName,
					FilePath:       fspath.JoinStr(mod.Path, "invowk_modules"),
					Title:          "Vendored modules present without lock file",
					Description:    fmt.Sprintf("Module has %d vendored modules in invowk_modules/ but no lock file — content hashes cannot be verified, allowing undetected tampering", len(mod.VendoredModules)),
					Recommendation: "Run 'invowk module sync' to generate a lock file, or remove stale vendored modules",
				})
			}

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
		return []Finding{{
			Severity:       SeverityLow,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Lock file size could not be verified",
			Description:    fmt.Sprintf("Failed to stat lock file %s: %v — size guard bypassed", mod.LockPath, err),
			Recommendation: "Verify file permissions; re-run the audit after fixing access issues",
		}}
	}
	if info.Size() > maxLockFileSize {
		return []Finding{{
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
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
		var findings []Finding
		findings = append(findings, Finding{
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Lock file uses v1.0 format without content hashes",
			Description:    "Version 1.0 lock files do not include content hashes for tamper detection — upgrade by running 'invowk module sync'",
			Recommendation: "Run 'invowk module sync' to upgrade to v2.0 format with SHA-256 content hashes",
		})
		// Escalate when vendored modules exist: V1 provides zero hash verification.
		if len(mod.VendoredModules) > 0 {
			findings = append(findings, Finding{
				Severity:       SeverityHigh,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       mod.LockPath,
				Title:          "Vendored modules cannot be verified — V1 lock file has no content hashes",
				Description:    fmt.Sprintf("%d vendored module(s) have no tamper detection because the V1.0 lock file lacks content hashes", len(mod.VendoredModules)),
				Recommendation: "Run 'invowk module sync' to upgrade to v2.0 format with SHA-256 content hashes",
			})
		}
		return findings
	}
	return nil
}

func (c *LockFileChecker) checkHashMismatches(ctx context.Context, mod *ScannedModule) []Finding {
	var findings []Finding

	hashes := mod.LockFile.ContentHashes()
	if len(hashes) == 0 {
		return nil
	}

	// Pre-build moduleID → (key, hash) lookup to detect ambiguous entries
	// where multiple lock entries share the same module ID.
	type lockEntry struct {
		key  invowkmod.ModuleRefKey
		hash invowkmod.ContentHash
	}
	lockByID := make(map[string][]lockEntry)
	for key, hash := range hashes {
		lockMod := mod.LockFile.Modules[key]
		nsID := string(invowkmod.ExtractModuleIDFromNamespace(lockMod.Namespace))
		lockByID[nsID] = append(lockByID[nsID], lockEntry{key: key, hash: hash})
	}

	// Flag ambiguous lock entries (multiple entries for same module ID).
	for id, entries := range lockByID {
		if len(entries) > 1 {
			var keys []string
			for _, e := range entries {
				keys = append(keys, string(e.key))
			}
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       mod.LockPath,
				Title:          "Ambiguous lock file entries for same module",
				Description:    fmt.Sprintf("Module ID %q has %d lock entries (%s) — only the first would be verified, allowing a crafted duplicate to evade detection", id, len(entries), strings.Join(keys, ", ")),
				Recommendation: "Ensure each module ID has exactly one lock file entry; run 'invowk module sync' to regenerate",
			})
		}
	}

	for _, vendored := range mod.VendoredModules {
		select {
		case <-ctx.Done():
			return findings
		default:
		}

		vendoredID := string(vendored.Metadata.Module)
		entries := lockByID[vendoredID]
		if len(entries) == 0 {
			// Vendored module has no matching lock entry — flag it (M3).
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendored.Path,
				Title:          "Vendored module has no lock file entry",
				Description:    fmt.Sprintf("Vendored module %q exists in invowk_modules/ but has no corresponding lock file entry — its content cannot be hash-verified", vendoredID),
				Recommendation: "Run 'invowk module sync' to add the module to the lock file, or remove it from invowk_modules/",
			})
			continue
		}

		// Use first entry for hash verification (ambiguity already flagged above).
		matchedKey := entries[0].key
		expectedHash := entries[0].hash

		actualHash, err := invowkmod.ComputeModuleHash(string(vendored.Path))
		if err != nil {
			findings = append(findings, Finding{
				Severity:       SeverityHigh,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendored.Path,
				Title:          "Vendored module hash could not be computed",
				Description:    fmt.Sprintf("Hash computation failed for vendored module %q: %v — integrity verification incomplete", vendoredID, err),
				Recommendation: "Verify directory permissions and contents; a module that cannot be hashed may be tampered with",
			})
			continue
		}

		if actualHash != expectedHash {
			findings = append(findings, Finding{
				Severity:       SeverityCritical,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendored.Path,
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

	const maxOrphanFindings = 10
	orphanCount := 0
	for key := range mod.LockFile.Modules {
		lockMod := mod.LockFile.Modules[key]
		nsID := string(invowkmod.ExtractModuleIDFromNamespace(lockMod.Namespace))
		if !vendoredIDs[nsID] {
			orphanCount++
			if orphanCount <= maxOrphanFindings {
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
	}

	// Collapse excessive orphan findings into a summary.
	if orphanCount > maxOrphanFindings {
		findings = append(findings, Finding{
			Severity:       SeverityLow,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Additional orphaned lock file entries",
			Description:    fmt.Sprintf("%d additional orphaned entries not shown (total: %d)", orphanCount-maxOrphanFindings, orphanCount),
			Recommendation: "Run 'invowk module tidy' to remove all stale lock file entries",
		})
	}

	return findings
}

func (c *LockFileChecker) checkMissingEntries(mod *ScannedModule) []Finding {
	var findings []Finding

	if mod.Module == nil || mod.Module.Metadata == nil {
		return nil
	}

	// Build an index of lock file keys for O(1) lookup.
	lockKeys := make(map[string]bool, len(mod.LockFile.Modules))
	for key := range mod.LockFile.Modules {
		lockKeys[string(key)] = true
	}

	// Check that each required module has an exact lock file entry.
	// Normalize path separators to forward slashes for cross-platform consistency —
	// lock files always use forward slashes, but SubdirectoryPath may contain
	// native backslashes on Windows (H4 fix).
	for _, req := range mod.Module.Metadata.Requires {
		reqKey := string(req.GitURL)
		if req.Path != "" {
			reqKey += "#" + filepath.ToSlash(string(req.Path))
		}

		if !lockKeys[reqKey] {
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       fspath.JoinStr(mod.Path, "invowkmod.cue"),
				Title:          "Required module has no lock file entry",
				Description:    fmt.Sprintf("Required dependency %q has no corresponding entry in the lock file", reqKey),
				Recommendation: "Run 'invowk module sync' to resolve and lock all dependencies",
			})
		}
	}

	return findings
}
