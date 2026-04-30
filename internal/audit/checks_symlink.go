// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
)

const (
	symlinkCheckerName = "symlink"
	// maxSymlinkChainDepth prevents infinite symlink chain traversal.
	maxSymlinkChainDepth = 10
)

// SymlinkChecker walks module directories to detect symlinks that may
// reference content outside the module boundary. Only operates on modules
// (standalone invowkfiles don't have module boundaries to escape).
type SymlinkChecker struct{}

// NewSymlinkChecker creates a SymlinkChecker.
func NewSymlinkChecker() *SymlinkChecker { return &SymlinkChecker{} }

// Name returns the checker identifier.
func (c *SymlinkChecker) Name() string { return symlinkCheckerName }

// Category returns CategoryPathTraversal.
func (c *SymlinkChecker) Category() Category { return CategoryPathTraversal }

// Check walks all module directories checking for symlinks.
func (c *SymlinkChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	for _, mod := range sc.Modules() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("symlink checker cancelled: %w", ctx.Err())
		default:
		}

		findings = append(findings, c.findingsFromModule(mod)...)
	}

	return findings, nil
}

func (c *SymlinkChecker) findingsFromModule(mod *ScannedModule) []Finding {
	var findings []Finding

	for _, symlink := range mod.Symlinks {
		findings = append(findings, Finding{
			Severity:       SeverityHigh,
			Category:       CategoryPathTraversal,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       symlink.Path,
			Title:          "Symlink found in module directory",
			Description:    fmt.Sprintf("Symlink %q in module — may reference content outside module boundary (SC-05)", symlink.RelPath),
			Recommendation: "Remove symlinks from module directories; copy files instead",
		})

		findings = append(findings, c.checkSymlinkTarget(symlink, mod.SurfaceID)...)

		if symlink.ChainTooDeep {
			findings = append(findings, c.checkSymlinkChain(symlink, mod.SurfaceID)...)
		}
	}

	if mod.SymlinkScanErr != nil {
		// Walk error is not critical — emit a diagnostic finding so incomplete
		// scans are visible, then return partial findings.
		findings = append(findings, Finding{
			Severity:       SeverityLow,
			Category:       CategoryPathTraversal,
			SurfaceID:      mod.SurfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       mod.Path,
			Title:          "Module directory walk incomplete",
			Description:    fmt.Sprintf("Walk error during symlink scan: %v — some entries may not have been checked", mod.SymlinkScanErr),
			Recommendation: "Verify directory permissions; re-run the audit after fixing access issues",
		})
	}

	return findings
}

func (c *SymlinkChecker) checkSymlinkTarget(symlink SymlinkRef, surfaceID string) []Finding {
	if symlink.ReadErr != nil {
		return []Finding{{
			Severity:       SeverityMedium,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       symlink.Path,
			Title:          "Symlink target could not be read — boundary check bypassed",
			Description:    fmt.Sprintf("Failed to read symlink target: %v — the boundary escape check cannot run, so the symlink's safety is unknown", symlink.ReadErr),
			Recommendation: "Verify file permissions and symlink integrity; an unreadable symlink may mask a boundary escape",
		}}
	}

	if symlink.EscapesRoot {
		return []Finding{{
			Severity:       SeverityCritical,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       symlink.Path,
			Title:          "Symlink points outside module boundary",
			Description:    fmt.Sprintf("Symlink target %q escapes the module directory", symlink.Target),
			Recommendation: "Remove the symlink; if the target file is needed, copy it into the module",
		}}
	}

	if symlink.Dangling {
		return []Finding{{
			Severity:       SeverityLow,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       symlink.Path,
			Title:          "Dangling symlink in module directory",
			Description:    fmt.Sprintf("Symlink target %q does not exist", symlink.Target),
			Recommendation: "Remove the dangling symlink or restore the target file",
		}}
	}

	return nil
}

func (c *SymlinkChecker) checkSymlinkChain(symlink SymlinkRef, surfaceID string) []Finding {
	return []Finding{{
		Severity:       SeverityMedium,
		Category:       CategoryPathTraversal,
		SurfaceID:      surfaceID,
		CheckerName:    symlinkCheckerName,
		FilePath:       symlink.Path,
		Title:          "Symlink chain detected",
		Description:    fmt.Sprintf("Symlink chain reaches %d links — may obscure the final target", maxSymlinkChainDepth),
		Recommendation: "Replace the symlink chain with a direct reference to the target file",
	}}
}
