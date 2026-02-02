// SPDX-License-Identifier: MPL-2.0

package docsaudit

import "testing"

func TestSeverityAndRecommendations(t *testing.T) {
	findings := []Finding{
		{ID: "f1", MismatchType: MismatchTypeOutdated},
		{ID: "f2", MismatchType: MismatchTypeMissing},
	}

	withSeverity := ApplySeverity(findings)
	if withSeverity[0].Severity != SeverityHigh {
		t.Fatalf("expected high severity for outdated")
	}
	if withSeverity[1].Severity != SeverityMedium {
		t.Fatalf("expected medium severity for missing")
	}

	withRecs := ApplyRecommendations(withSeverity)
	if withRecs[0].Recommendation == "" {
		t.Fatalf("expected recommendation for finding")
	}
}
