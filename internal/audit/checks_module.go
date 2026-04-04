// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/invowk/invowk/pkg/types"
)

const (
	moduleMetadataCheckerName = "module-metadata"
	// maxDependencyFanOut is the threshold for flagging wide dependency fan-out.
	// This measures the number of direct dependencies a single module declares,
	// NOT the graph depth of transitive dependency chains.
	maxDependencyFanOut = 5
	// typosquatLevenshteinThreshold flags module IDs within this edit distance.
	typosquatLevenshteinThreshold = 3
)

// unpinnedVersions is the set of version constraints considered effectively
// unpinned — they accept any version and provide no supply-chain protection.
var unpinnedVersions = map[string]bool{
	"":          true,
	"*":         true,
	">=0.0.0":   true,
	">0":        true,
	">=0":       true,
	">0.0.0":    true,
	">=0.0.0-0": true,
	"*.*":       true,
	"*.*.*":     true,
}

// ModuleMetadataChecker analyzes module dependency chains, namespace collisions,
// global module trust, undeclared transitive dependencies, and version pinning.
// Only operates on modules (standalone invowkfiles have no module metadata).
type ModuleMetadataChecker struct{}

// NewModuleMetadataChecker creates a ModuleMetadataChecker.
func NewModuleMetadataChecker() *ModuleMetadataChecker { return &ModuleMetadataChecker{} }

// Name returns the checker identifier.
func (c *ModuleMetadataChecker) Name() string { return moduleMetadataCheckerName }

// Category returns CategoryTrust.
func (c *ModuleMetadataChecker) Category() Category { return CategoryTrust }

// Check analyzes module metadata for security concerns.
func (c *ModuleMetadataChecker) Check(ctx context.Context, sc *ScanContext) ([]Finding, error) {
	var findings []Finding

	// Collect all module IDs for typosquatting detection.
	var moduleIDs []string
	for _, mod := range sc.Modules() {
		if mod.Module != nil && mod.Module.Metadata != nil {
			moduleIDs = append(moduleIDs, string(mod.Module.Metadata.Module))
		}
	}

	for _, mod := range sc.Modules() {
		select {
		case <-ctx.Done():
			return findings, fmt.Errorf("module metadata checker cancelled: %w", ctx.Err())
		default:
		}

		findings = append(findings, c.checkInvowkfileParseFailure(mod)...)
		findings = append(findings, c.checkGlobalTrust(mod)...)
		findings = append(findings, c.checkDependencyFanOut(mod)...)
		findings = append(findings, c.checkTyposquatting(mod, moduleIDs)...)
		findings = append(findings, c.checkVersionPinning(mod)...)
		findings = append(findings, c.checkUndeclaredTransitive(mod)...)
	}

	return findings, nil
}

func (c *ModuleMetadataChecker) checkGlobalTrust(mod *ScannedModule) []Finding {
	if !mod.IsGlobal {
		return nil
	}

	return []Finding{{
		Severity:       SeverityInfo,
		Category:       CategoryTrust,
		SurfaceID:      mod.SurfaceID,
		CheckerName:    moduleMetadataCheckerName,
		FilePath:       mod.Path,
		Title:          "Global module has no content hash verification",
		Description:    fmt.Sprintf("Module %q is installed globally (~/.invowk/cmds/) with no cryptographic integrity verification", mod.SurfaceID),
		Recommendation: "Consider vendoring this module into your project with 'invowk module vendor' for hash-verified integrity",
	}}
}

// checkInvowkfileParseFailure flags modules where the invowkfile exists on disk
// but failed to parse. A corrupt or malformed invowkfile means script-based
// checkers (content analysis, env scanning, network detection) cannot inspect
// the module's commands, creating a blind spot in the audit.
func (c *ModuleMetadataChecker) checkInvowkfileParseFailure(mod *ScannedModule) []Finding {
	if mod.InvowkfileParseErr == nil {
		return nil
	}

	return []Finding{{
		Severity:       SeverityMedium,
		Category:       CategoryTrust,
		SurfaceID:      mod.SurfaceID,
		CheckerName:    moduleMetadataCheckerName,
		FilePath:       types.FilesystemPath(filepath.Join(string(mod.Path), "invowkfile.cue")), //goplint:ignore -- filepath.Join from validated module path
		Title:          "Module invowkfile failed to parse",
		Description:    fmt.Sprintf("Invowkfile exists but could not be parsed: %v — script content cannot be audited", mod.InvowkfileParseErr),
		Recommendation: "Inspect the invowkfile for syntax errors or deliberate corruption; a module with an unparseable invowkfile evades content-based security checks",
	}}
}

func (c *ModuleMetadataChecker) checkDependencyFanOut(mod *ScannedModule) []Finding {
	if mod.Module == nil || mod.Module.Metadata == nil {
		return nil
	}

	requires := mod.Module.Metadata.Requires
	if len(requires) <= maxDependencyFanOut {
		return nil
	}

	return []Finding{{
		Severity:       SeverityMedium,
		Category:       CategoryTrust,
		SurfaceID:      mod.SurfaceID,
		CheckerName:    moduleMetadataCheckerName,
		FilePath:       types.FilesystemPath(filepath.Join(string(mod.Path), "invowkmod.cue")),
		Title:          "Wide dependency fan-out",
		Description:    fmt.Sprintf("Module has %d direct dependencies (threshold: %d) — wide fan-out increases supply-chain attack surface", len(requires), maxDependencyFanOut),
		Recommendation: "Review whether all dependencies are necessary; consider consolidating or inlining rarely-used modules",
	}}
}

func (c *ModuleMetadataChecker) checkTyposquatting(mod *ScannedModule, allIDs []string) []Finding {
	if mod.Module == nil || mod.Module.Metadata == nil {
		return nil
	}

	var findings []Finding
	thisID := string(mod.Module.Metadata.Module)

	for _, otherID := range allIDs {
		if thisID == otherID {
			continue
		}
		// Only emit the finding when thisID < otherID (lexicographic ordering)
		// to break symmetry — without this guard, each similar pair produces
		// two symmetric findings (A→B and B→A).
		if thisID > otherID {
			continue
		}
		dist := levenshtein(thisID, otherID)
		if dist > 0 && dist <= typosquatLevenshteinThreshold {
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryTrust,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    moduleMetadataCheckerName,
				FilePath:       mod.Path,
				Title:          "Module ID similar to another module",
				Description:    fmt.Sprintf("Module %q has Levenshtein distance %d from %q — possible typosquatting", thisID, dist, otherID),
				Recommendation: "Verify the module identity; ensure the module ID is correct and not a typosquatting attempt",
			})
		}
	}

	return findings
}

func (c *ModuleMetadataChecker) checkVersionPinning(mod *ScannedModule) []Finding {
	if mod.Module == nil || mod.Module.Metadata == nil {
		return nil
	}

	var findings []Finding
	for _, req := range mod.Module.Metadata.Requires {
		version := string(req.Version)
		if unpinnedVersions[version] {
			findings = append(findings, Finding{
				Severity:       SeverityLow,
				Category:       CategoryTrust,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    moduleMetadataCheckerName,
				FilePath:       types.FilesystemPath(filepath.Join(string(mod.Path), "invowkmod.cue")),
				Title:          "Module dependency has no version constraint",
				Description:    fmt.Sprintf("Dependency %q uses version constraint %q — any version will be accepted", req.GitURL, version),
				Recommendation: "Pin to a specific version range (e.g., ^1.0.0 or ~2.0.0)",
			})
		}
	}

	return findings
}

func (c *ModuleMetadataChecker) checkUndeclaredTransitive(mod *ScannedModule) []Finding {
	if mod.Module == nil || mod.Module.Metadata == nil || len(mod.VendoredModules) == 0 {
		return nil
	}

	// Build set of declared dependency Git URLs for transitive-dep detection
	// and for matching vendored modules by URL.
	declared := make(map[string]bool)
	for _, req := range mod.Module.Metadata.Requires {
		declared[string(req.GitURL)] = true
	}

	// Check each vendored module's own requires for undeclared transitive deps.
	var findings []Finding
	for _, vendored := range mod.VendoredModules {
		if vendored.Metadata == nil {
			continue
		}
		for _, transReq := range vendored.Metadata.Requires {
			if !declared[string(transReq.GitURL)] {
				findings = append(findings, Finding{
					Severity:       SeverityMedium,
					Category:       CategoryTrust,
					SurfaceID:      mod.SurfaceID,
					CheckerName:    moduleMetadataCheckerName,
					FilePath:       types.FilesystemPath(filepath.Join(string(mod.Path), "invowkmod.cue")),
					Title:          "Transitive dependency not declared in root invowkmod.cue",
					Description:    fmt.Sprintf("Module %q requires %q which is not declared in the root requires — violates explicit-only dep model", vendored.Metadata.Module, transReq.GitURL),
					Recommendation: "Run 'invowk module tidy' to add missing transitive dependencies",
				})
			}
		}
	}

	// Second scan: verify each vendored module itself is declared in requires.
	// Match by alias (the canonical module name) or by Git URL equality.
	for _, vendored := range mod.VendoredModules {
		if vendored.Metadata == nil {
			continue
		}
		vendoredID := string(vendored.Metadata.Module)
		found := false
		for _, req := range mod.Module.Metadata.Requires {
			if string(req.Alias) == vendoredID || string(req.GitURL) == vendoredID {
				found = true
				break
			}
		}
		if !found {
			findings = append(findings, Finding{
				Severity:       SeverityMedium,
				Category:       CategoryTrust,
				SurfaceID:      mod.SurfaceID,
				CheckerName:    moduleMetadataCheckerName,
				FilePath:       types.FilesystemPath(filepath.Join(string(mod.Path), "invowkmod.cue")),
				Title:          "Vendored module not declared in requires",
				Description:    fmt.Sprintf("Vendored module %q exists in invowk_modules/ but has no matching entry in requires — it may have been manually placed or left from a removed dependency", vendoredID),
				Recommendation: "Either add the module to requires in invowkmod.cue or remove it from invowk_modules/",
			})
		}
	}

	return findings
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}

	// Use two rows for space efficiency.
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}
