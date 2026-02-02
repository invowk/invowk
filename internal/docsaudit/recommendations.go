// SPDX-License-Identifier: MPL-2.0

package docsaudit

// ApplyRecommendations assigns recommendations to findings lacking one.
func ApplyRecommendations(findings []Finding) []Finding {
	if len(findings) == 0 {
		return findings
	}

	updated := append([]Finding(nil), findings...)
	for i, finding := range updated {
		if finding.Recommendation != "" {
			continue
		}
		updated[i].Recommendation = RecommendationForFinding(finding)
	}

	return updated
}

// RecommendationForFinding maps a finding to a default remediation action.
func RecommendationForFinding(finding Finding) string {
	switch finding.MismatchType {
	case MismatchTypeMissing:
		return "Add documentation for the feature or mark it as undocumented."
	case MismatchTypeOutdated:
		return "Update or remove outdated documentation, or restore the referenced feature."
	case MismatchTypeIncorrect:
		return "Correct the documentation to match current behavior."
	case MismatchTypeInconsistent:
		return "Consolidate conflicting documentation and align to a single source of truth."
	default:
		return "Review the documentation and align it with current behavior."
	}
}
