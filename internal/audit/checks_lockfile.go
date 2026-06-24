// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	lockFileCheckerName = "lockfile"
)

type (
	// VendoredHashEvaluator evaluates one vendored module against lock-file hash metadata.
	VendoredHashEvaluator interface {
		EvaluateVendoredModuleHash(lock *invowkmod.LockFile, module *invowkmod.Module) invowkmod.VendoredHashEvaluation
	}

	// LockFileCheckerOption configures lock-file checker dependencies.
	LockFileCheckerOption func(*LockFileChecker)

	// LockFileChecker validates lock file integrity: hash mismatches, orphaned or
	// missing entries, version checks, and size limits. Only operates on modules
	// (standalone invowkfiles have no lock files).
	LockFileChecker struct {
		hashEvaluator VendoredHashEvaluator
	}

	vendoredHashEvaluatorFunc func(*invowkmod.LockFile, *invowkmod.Module) invowkmod.VendoredHashEvaluation
)

func (f vendoredHashEvaluatorFunc) EvaluateVendoredModuleHash(lock *invowkmod.LockFile, module *invowkmod.Module) invowkmod.VendoredHashEvaluation {
	return f(lock, module)
}

// NewLockFileChecker creates a LockFileChecker.
func NewLockFileChecker(opts ...LockFileCheckerOption) *LockFileChecker {
	checker := &LockFileChecker{
		hashEvaluator: vendoredHashEvaluatorFunc(invowkmod.EvaluateVendoredModuleHash),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(checker)
		}
	}
	return checker
}

// WithHashEvaluator sets the hash evaluator used by the lock-file checker.
func WithHashEvaluator(evaluator VendoredHashEvaluator) LockFileCheckerOption {
	return func(checker *LockFileChecker) {
		checker.hashEvaluator = evaluator
	}
}

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
				Code:           codeLockfilePresentUnparseable,
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
					Code:           codeLockfileDependenciesNoLock,
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
					Code:           codeLockfileVendoredNoLock,
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
		hashFindings, err := c.checkHashMismatches(ctx, mod)
		findings = append(findings, hashFindings...)
		if err != nil {
			return findings, err
		}
		findings = append(findings, c.checkOrphanedEntries(mod)...)
		findings = append(findings, c.checkMissingEntries(mod)...)
	}

	return findings, nil
}

func (c *LockFileChecker) checkSize(mod *ScannedModule) []Finding {
	if mod.LockPath == "" {
		return nil
	}
	if mod.LockFileStatErr != nil {
		return []Finding{{
			Code:           codeLockfileSizeUnknown,
			Severity:       SeverityLow,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Lock file size could not be verified",
			Description:    fmt.Sprintf("Failed to stat lock file %s: %v — size guard bypassed", mod.LockPath, mod.LockFileStatErr),
			Recommendation: "Verify file permissions; re-run the audit after fixing access issues",
		}}
	}
	if mod.LockFileSize > invowkmod.LockFileSizeLimit {
		return []Finding{{
			Code:           codeLockfileSizeLimit,
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Lock file exceeds size limit",
			Description:    fmt.Sprintf("Lock file is %d bytes, exceeding the %d byte limit — may be crafted for denial-of-service", mod.LockFileSize, invowkmod.LockFileSizeLimit),
			Recommendation: "Inspect the lock file for suspicious content; regenerate with 'invowk module sync'",
		}}
	}
	return nil
}

func (c *LockFileChecker) checkVersion(mod *ScannedModule) []Finding {
	version := mod.LockFile.Version
	if version != invowkmod.LockFileVersionV1 && version != invowkmod.LockFileVersionV2 {
		return []Finding{{
			Code:           codeLockfileUnknownVersion,
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
			Code:           codeLockfileV1NoHashes,
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
				Code:           codeLockfileV1VendoredNoHashes,
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

func (c *LockFileChecker) checkHashMismatches(ctx context.Context, mod *ScannedModule) ([]Finding, error) {
	var findings []Finding

	hashes := mod.LockFile.ContentHashes()
	if len(hashes) == 0 {
		return nil, nil
	}

	// Flag ambiguous lock entries using the same lock/hash policy as vendored
	// module verification.
	for _, ambiguity := range invowkmod.FindAmbiguousLockedModuleEntries(mod.LockFile) {
		findings = append(findings, Finding{
			Code:           codeLockfileAmbiguousModule,
			Severity:       SeverityMedium,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Ambiguous lock file entries for same module",
			Description:    fmt.Sprintf("Module ID %q has %d lock entries (%s) — hash verification is ambiguous until the duplicate identity is resolved", ambiguity.ModuleID, len(ambiguity.LockKeys), moduleRefKeysList(ambiguity.LockKeys)),
			Recommendation: "Ensure each module ID has exactly one lock file entry; run 'invowk module sync' to regenerate",
		})
	}

	pathByVendoredID := vendoredPathByModuleID(mod.VendoredModules)
	for _, vendored := range mod.VendoredModules {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("lockfile hash check cancelled: %w", ctx.Err())
		default:
		}

		evaluation := c.hashEvaluator.EvaluateVendoredModuleHash(mod.LockFile, vendored)
		vendoredID := evaluation.ModuleID
		vendoredPath := pathByVendoredID[vendoredID]
		if vendoredPath == "" {
			vendoredPath = mod.Path
		}
		switch evaluation.Status {
		case invowkmod.VendoredHashMatched:
			continue
		case invowkmod.VendoredHashAmbiguous:
			// Already reported above from the lock-file-wide ambiguity scan.
			continue
		case invowkmod.VendoredHashMissing:
			// Vendored module has no matching lock entry — flag it (M3).
			findings = append(findings, Finding{
				Code:           codeLockfileVendoredMissingEntry,
				Severity:       SeverityMedium,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendoredPath,
				Title:          "Vendored module has no lock file entry",
				Description:    fmt.Sprintf("Vendored module %q exists in invowk_modules/ but has no corresponding lock file entry — its content cannot be hash-verified", vendoredID),
				Recommendation: "Run 'invowk module sync' to add the module to the lock file, or remove it from invowk_modules/",
			})
			continue
		case invowkmod.VendoredHashUnavailable:
			findings = append(findings, Finding{
				Code:           codeLockfileVendoredHashUnavailable,
				Severity:       SeverityHigh,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendoredPath,
				Title:          "Vendored module hash could not be computed",
				Description:    fmt.Sprintf("Hash computation failed for vendored module %q: %v — integrity verification incomplete", vendoredID, evaluation.Err),
				Recommendation: "Verify directory permissions and contents; a module that cannot be hashed may be tampered with",
			})
			continue
		case invowkmod.VendoredHashMismatch:
			findings = append(findings, Finding{
				Code:           codeLockfileContentHashMismatch,
				Severity:       SeverityCritical,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendoredPath,
				Title:          "Module content hash mismatch",
				Description:    fmt.Sprintf("Vendored module %q has hash %s but lock file expects %s — module may have been tampered with", evaluation.ModuleKey, evaluation.Actual, evaluation.Expected),
				Recommendation: "Re-vendor with 'invowk module sync' and verify the module source has not been compromised",
			})
		default:
			findings = append(findings, Finding{
				Code:           codeLockfileVendoredHashUnknown,
				Severity:       SeverityHigh,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       vendoredPath,
				Title:          "Vendored module hash status is unknown",
				Description:    fmt.Sprintf("Vendored module %q returned unknown integrity status %q", vendoredID, evaluation.Status),
				Recommendation: "Run 'invowk module sync' to regenerate lock metadata before using vendored modules",
			})
		}
	}

	return findings, nil
}

//goplint:ignore -- display-only lockfile key list for audit finding descriptions.
func moduleRefKeysList(keys []invowkmod.ModuleRefKey) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, string(key))
	}
	return strings.Join(parts, ", ")
}

func vendoredPathByModuleID(vendoredModules []*invowkmod.Module) map[invowkmod.ModuleID]types.FilesystemPath {
	paths := make(map[invowkmod.ModuleID]types.FilesystemPath, len(vendoredModules))
	for _, vendored := range vendoredModules {
		if vendored == nil || vendored.Metadata == nil {
			continue
		}
		moduleID := vendored.Metadata.Module
		if moduleID == "" || paths[moduleID] != "" {
			continue
		}
		paths[moduleID] = vendored.Path
	}
	return paths
}

func (c *LockFileChecker) checkOrphanedEntries(mod *ScannedModule) []Finding {
	var findings []Finding

	const maxOrphanFindings = 10
	orphaned := invowkmod.OrphanedLockedModuleEntries(mod.Module.Metadata.Requires, mod.LockFile)
	for i, key := range orphaned {
		if i < maxOrphanFindings {
			findings = append(findings, Finding{
				Code:           codeLockfileOrphanEntry,
				Severity:       SeverityLow,
				Category:       CategoryIntegrity,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    lockFileCheckerName,
				FilePath:       mod.LockPath,
				Title:          "Orphaned lock file entry",
				Description:    fmt.Sprintf("Lock file contains entry %q which is not declared in module requirements", key),
				Recommendation: "Run 'invowk module tidy' to remove stale lock file entries",
			})
		}
	}

	// Collapse excessive orphan findings into a summary.
	if len(orphaned) > maxOrphanFindings {
		findings = append(findings, Finding{
			Code:           codeLockfileAdditionalOrphans,
			Severity:       SeverityLow,
			Category:       CategoryIntegrity,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    lockFileCheckerName,
			FilePath:       mod.LockPath,
			Title:          "Additional orphaned lock file entries",
			Description:    fmt.Sprintf("%d additional orphaned entries not shown (total: %d)", len(orphaned)-maxOrphanFindings, len(orphaned)),
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

	// Check that each required module has an exact lock file entry.
	for _, reqKey := range invowkmod.MissingLockedModuleRequirementKeys(mod.Module.Metadata.Requires, mod.LockFile) {
		findings = append(findings, Finding{
			Code:           codeLockfileRequiredMissingEntry,
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

	return findings
}
