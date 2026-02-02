// SPDX-License-Identifier: MPL-2.0

package docsaudit

// ApplySeverity assigns severities to findings lacking one.
func ApplySeverity(findings []Finding) []Finding {
	if len(findings) == 0 {
		return findings
	}

	updated := append([]Finding(nil), findings...)
	for i, finding := range updated {
		if finding.Severity != "" {
			continue
		}
		updated[i].Severity = SeverityForFinding(finding)
	}

	return updated
}

// SeverityForFinding maps a finding to a severity based on mismatch type.
func SeverityForFinding(finding Finding) Severity {
	switch finding.MismatchType {
	case MismatchTypeOutdated, MismatchTypeIncorrect:
		return SeverityHigh
	case MismatchTypeMissing, MismatchTypeInconsistent:
		return SeverityMedium
	default:
		return SeverityLow
	}
}
