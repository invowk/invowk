// SPDX-License-Identifier: MPL-2.0

package audit

import "fmt"

type (
	// CorrelationRule defines a compound threat pattern that emerges when findings
	// from different checkers appear in the same surface. The correlator applies
	// these rules after all individual checkers complete.
	CorrelationRule struct {
		// Name identifies this rule (e.g., "credential-exfiltration").
		Name string
		// Description explains the compound threat.
		Description string
		// RequiredCheckers lists the two checker names that must both have
		// findings in the same surface for this rule to fire.
		RequiredCheckers [2]string
		// RequiredCategories optionally restricts matching to findings that
		// have both the checker name AND the specific category. When non-empty,
		// each element pairs with the same-index RequiredCheckers entry. This
		// enables rules that correlate two different categories from the same
		// checker (e.g., script+execution vs script+path-traversal).
		RequiredCategories [2]Category
		// ResultSeverity is the severity assigned to the compound finding.
		ResultSeverity Severity
		// ResultCategory is the category assigned to the compound finding.
		ResultCategory Category
		// ResultTitle is the title for the compound finding.
		ResultTitle string
		// ResultRecommendation is the fix suggestion for the compound finding.
		ResultRecommendation string
	}

	// Correlator detects compound threats by cross-referencing findings from
	// different checkers. It groups findings by surface ID and applies correlation
	// rules plus severity escalation logic.
	Correlator struct {
		rules []CorrelationRule
	}
)

// NewCorrelator creates a Correlator with the given rules. It validates that
// same-checker rules (RequiredCheckers[0] == RequiredCheckers[1]) always have
// RequiredCategories set to prevent degenerate self-correlation.
func NewCorrelator(rules []CorrelationRule) (*Correlator, error) {
	for i := range rules {
		r := &rules[i]
		if r.RequiredCheckers[0] == r.RequiredCheckers[1] && (r.RequiredCategories[0] == "" || r.RequiredCategories[1] == "") {
			return nil, fmt.Errorf("correlation rule %q has same checker on both sides without RequiredCategories", r.Name)
		}
	}
	return &Correlator{rules: rules}, nil
}

// Correlate examines findings grouped by surface ID and produces compound
// findings when correlation rules match. Also applies escalation rules:
//   - Medium + Medium in same surface → High
//   - High + any in same surface → Critical
//   - 3+ distinct categories in same surface → Critical
func (c *Correlator) Correlate(findings []Finding) []Finding {
	// Group findings by surface.
	bySurface := make(map[string][]Finding)
	for i := range findings {
		sid := findings[i].SurfaceID
		if sid == "" {
			sid = string(findings[i].FilePath)
		}
		bySurface[sid] = append(bySurface[sid], findings[i])
	}

	var correlated []Finding

	for surfaceID, surfaceFindings := range bySurface {
		// Apply named correlation rules.
		correlated = append(correlated, c.applyRules(surfaceID, surfaceFindings)...)

		// Apply severity escalation rules.
		correlated = append(correlated, c.applyEscalation(surfaceID, surfaceFindings)...)
	}

	return correlated
}

func (c *Correlator) applyRules(surfaceID string, findings []Finding) []Finding {
	var result []Finding

	// Build checker presence map and checker+category presence map for this surface.
	checkers := make(map[string]bool)
	checkerCategories := make(map[string]map[Category]bool)
	for i := range findings {
		name := findings[i].CheckerName
		checkers[name] = true
		if checkerCategories[name] == nil {
			checkerCategories[name] = make(map[Category]bool)
		}
		checkerCategories[name][findings[i].Category] = true
	}

	for ri := range c.rules {
		rule := &c.rules[ri]
		if !checkers[rule.RequiredCheckers[0]] || !checkers[rule.RequiredCheckers[1]] {
			continue
		}

		// When RequiredCategories is set, require the checker+category combination
		// rather than just the checker name. This supports same-checker rules that
		// correlate two different categories (e.g., script+execution vs script+path-traversal).
		if rule.RequiredCategories[0] != "" {
			if !checkerCategories[rule.RequiredCheckers[0]][rule.RequiredCategories[0]] ||
				!checkerCategories[rule.RequiredCheckers[1]][rule.RequiredCategories[1]] {
				continue
			}
		}

		// Collect the titles of findings from the two required checkers.
		var escalatedFrom []string
		for i := range findings {
			if findings[i].CheckerName == rule.RequiredCheckers[0] || findings[i].CheckerName == rule.RequiredCheckers[1] {
				escalatedFrom = append(escalatedFrom, findings[i].Title)
			}
		}

		result = append(result, Finding{
			Severity:       rule.ResultSeverity,
			Category:       rule.ResultCategory,
			SurfaceID:      surfaceID,
			CheckerName:    "correlator",
			Title:          rule.ResultTitle,
			Description:    rule.Description,
			Recommendation: rule.ResultRecommendation,
			EscalatedFrom:  escalatedFrom,
		})
	}

	return result
}

func (c *Correlator) applyEscalation(surfaceID string, findings []Finding) []Finding {
	var result []Finding

	// Count severities and distinct categories in a single pass.
	categories := make(map[Category]bool)
	var mediumCount, highCount int
	for i := range findings {
		categories[findings[i].Category] = true
		switch findings[i].Severity {
		case SeverityInfo, SeverityLow:
			// Low and info findings do not contribute to escalation.
		case SeverityMedium:
			mediumCount++
		case SeverityHigh:
			highCount++
		case SeverityCritical:
			// If any individual finding is already Critical, escalation adds no
			// new severity information. The compound-threat context is still
			// visible through individual findings.
			return nil
		}
	}

	// Rule: 3+ distinct categories → Critical.
	if len(categories) >= 3 {
		result = append(result, Finding{
			Severity:       SeverityCritical,
			Category:       CategoryTrust,
			SurfaceID:      surfaceID,
			CheckerName:    "correlator",
			Title:          "Multiple security concern categories detected",
			Description:    fmt.Sprintf("%d distinct categories of security findings in the same surface suggest a coordinated threat or severely compromised module", len(categories)),
			Recommendation: "Review this module thoroughly before use; consider isolating it via the container runtime",
		})
		return result
	}

	// Rule: High + any → Critical.
	if highCount > 0 && len(findings) > 1 {
		result = append(result, Finding{
			Severity:       SeverityCritical,
			Category:       CategoryTrust,
			SurfaceID:      surfaceID,
			CheckerName:    "correlator",
			Title:          "High-severity finding combined with other issues",
			Description:    fmt.Sprintf("A high-severity finding plus %d other finding(s) in the same surface elevates the overall risk", len(findings)-1),
			Recommendation: "Investigate each finding individually; the combination may indicate an active threat",
		})
		return result
	}

	// Rule: Medium + Medium → High.
	if mediumCount >= 2 {
		result = append(result, Finding{
			Severity:       SeverityHigh,
			Category:       CategoryTrust,
			SurfaceID:      surfaceID,
			CheckerName:    "correlator",
			Title:          "Multiple medium-severity findings compound risk",
			Description:    fmt.Sprintf("%d medium-severity findings in the same surface compound the overall risk", mediumCount),
			Recommendation: "Address each finding individually to reduce the compound risk",
		})
		return result
	}

	return result
}

// DefaultRules returns the 5 named correlation rules from the threat model.
// Four cross-checker rules plus one same-checker category-pair rule.
func DefaultRules() []CorrelationRule {
	return []CorrelationRule{
		{
			Name:                 "credential-exfiltration",
			Description:          "Module accesses sensitive environment variables and has network access — potential credential exfiltration",
			RequiredCheckers:     [2]string{envCheckerName, networkCheckerName},
			ResultSeverity:       SeverityCritical,
			ResultCategory:       CategoryExfiltration,
			ResultTitle:          "Potential credential exfiltration",
			ResultRecommendation: "Restrict env_inherit_mode to 'none' or 'allow' and audit all network access in this module",
		},
		{
			Name:                 "path-symlink-escape",
			Description:          "Path traversal combined with external symlink target allows escaping the module boundary",
			RequiredCheckers:     [2]string{scriptCheckerName, symlinkCheckerName},
			ResultSeverity:       SeverityCritical,
			ResultCategory:       CategoryPathTraversal,
			ResultTitle:          "Combined path traversal and symlink escape",
			ResultRecommendation: "Remove symlinks from module directories and ensure all script paths are relative within the module",
		},
		{
			Name:                 "obfuscated-exfiltration",
			Description:          "Script contains obfuscation patterns alongside network access — likely deliberate evasion",
			RequiredCheckers:     [2]string{scriptCheckerName, networkCheckerName},
			ResultSeverity:       SeverityCritical,
			ResultCategory:       CategoryExfiltration,
			ResultTitle:          "Obfuscated network access detected",
			ResultRecommendation: "Decode and review all obfuscated content; do not use this module until the obfuscation is explained",
		},
		{
			Name:                 "trust-chain-weakness",
			Description:          "Deep dependency chain with unverified modules increases supply-chain attack surface",
			RequiredCheckers:     [2]string{moduleMetadataCheckerName, lockFileCheckerName},
			ResultSeverity:       SeverityHigh,
			ResultCategory:       CategoryTrust,
			ResultTitle:          "Trust chain weakness — deep deps with missing integrity",
			ResultRecommendation: "Run 'invowk module sync' to update lock file hashes; review dependency chain depth",
		},
		{
			Name:                 "interpreter-traversal",
			Description:          "Unusual interpreter combined with path traversal in the same module",
			RequiredCheckers:     [2]string{scriptCheckerName, scriptCheckerName},
			RequiredCategories:   [2]Category{CategoryExecution, CategoryPathTraversal},
			ResultSeverity:       SeverityCritical,
			ResultCategory:       CategoryExecution,
			ResultTitle:          "Unusual interpreter with path traversal",
			ResultRecommendation: "Audit the interpreter configuration and verify script paths stay within module boundary",
		},
	}
}
