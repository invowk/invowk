// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
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

		modPath := string(mod.Path)
		findings = append(findings, c.scanDir(ctx, modPath, mod.SurfaceID)...)
	}

	return findings, nil
}

func (c *SymlinkChecker) scanDir(ctx context.Context, modPath, surfaceID string) []Finding {
	var findings []Finding

	err := filepath.WalkDir(modPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip inaccessible entries — return the error to let WalkDir
			// decide whether to skip (file) or abort (directory).
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check for symlinks via entry type.
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}

		relPath, relErr := filepath.Rel(modPath, path)
		if relErr != nil {
			// Cannot compute relative path — skip this entry rather than
			// silently discarding the error.
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		findings = append(findings, Finding{
			Severity:       SeverityHigh,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       types.FilesystemPath(path),
			Title:          "Symlink found in module directory",
			Description:    fmt.Sprintf("Symlink %q in module — may reference content outside module boundary (SC-05)", relPath),
			Recommendation: "Remove symlinks from module directories; copy files instead",
		})

		// Check where the symlink points.
		findings = append(findings, c.checkSymlinkTarget(path, modPath, surfaceID)...)

		// Check for symlink chains.
		findings = append(findings, c.checkSymlinkChain(path, surfaceID)...)

		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		// Walk error is not critical — emit a diagnostic finding so incomplete
		// scans are visible, then return partial findings.
		findings = append(findings, Finding{
			Severity:       SeverityLow,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       types.FilesystemPath(modPath),
			Title:          "Module directory walk incomplete",
			Description:    fmt.Sprintf("Walk error during symlink scan: %v — some entries may not have been checked", err),
			Recommendation: "Verify directory permissions; re-run the audit after fixing access issues",
		})
	}

	return findings
}

func (c *SymlinkChecker) checkSymlinkTarget(path, modPath, surfaceID string) []Finding {
	target, err := os.Readlink(path)
	if err != nil {
		return []Finding{{
			Severity:       SeverityMedium,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       types.FilesystemPath(path),
			Title:          "Symlink target could not be read — boundary check bypassed",
			Description:    fmt.Sprintf("Failed to read symlink target: %v — the boundary escape check cannot run, so the symlink's safety is unknown", err),
			Recommendation: "Verify file permissions and symlink integrity; an unreadable symlink may mask a boundary escape",
		}}
	}

	// Resolve to absolute path for boundary check.
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	target = filepath.Clean(target)

	// Check if target is outside module boundary.
	rel, err := filepath.Rel(modPath, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return []Finding{{
			Severity:       SeverityCritical,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       types.FilesystemPath(path),
			Title:          "Symlink points outside module boundary",
			Description:    fmt.Sprintf("Symlink target %q escapes the module directory", target),
			Recommendation: "Remove the symlink; if the target file is needed, copy it into the module",
		}}
	}

	// Check for dangling symlink.
	if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
		return []Finding{{
			Severity:       SeverityLow,
			Category:       CategoryPathTraversal,
			SurfaceID:      surfaceID,
			CheckerName:    symlinkCheckerName,
			FilePath:       types.FilesystemPath(path),
			Title:          "Dangling symlink in module directory",
			Description:    fmt.Sprintf("Symlink target %q does not exist", target),
			Recommendation: "Remove the dangling symlink or restore the target file",
		}}
	}

	return nil
}

func (c *SymlinkChecker) checkSymlinkChain(path, surfaceID string) []Finding {
	current := path
	// Follow up to maxSymlinkChainDepth-1 hops: the initial path (already a
	// confirmed symlink) counts as hop 1, so we follow maxSymlinkChainDepth-1
	// additional hops to fire the finding at exactly maxSymlinkChainDepth total.
	for range maxSymlinkChainDepth - 1 {
		target, err := os.Readlink(current)
		if err != nil {
			return nil // Not a symlink — chain ends.
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}

		// Check if the target is itself a symlink.
		info, lstatErr := os.Lstat(target)
		if lstatErr != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil // Chain ends at a regular file.
		}
		current = target
	}

	// If we got here, the chain is too deep.
	return []Finding{{
		Severity:       SeverityMedium,
		Category:       CategoryPathTraversal,
		SurfaceID:      surfaceID,
		CheckerName:    symlinkCheckerName,
		FilePath:       types.FilesystemPath(path),
		Title:          "Symlink chain detected",
		Description:    fmt.Sprintf("Symlink chain reaches %d links — may obscure the final target", maxSymlinkChainDepth),
		Recommendation: "Replace the symlink chain with a direct reference to the target file",
	}}
}
