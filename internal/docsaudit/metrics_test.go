// SPDX-License-Identifier: MPL-2.0

package docsaudit

import "testing"

func TestComputeMetrics(t *testing.T) {
	surfaces := []UserFacingSurface{
		{ID: "s1", DocumentationRefs: []DocReference{{SourceID: "doc"}}},
		{ID: "s2"},
	}
	findings := []Finding{
		{ID: "f1", MismatchType: MismatchTypeOutdated, Severity: SeverityHigh},
		{ID: "f2", MismatchType: MismatchTypeMissing, Severity: SeverityMedium},
	}

	metrics := ComputeMetrics(surfaces, findings)
	if metrics.TotalSurfaces != 2 {
		t.Fatalf("unexpected total surfaces: %d", metrics.TotalSurfaces)
	}
	if metrics.CoveragePercentage <= 0 {
		t.Fatalf("expected coverage percentage to be > 0")
	}
	if metrics.CountsByMismatchType[MismatchTypeOutdated] != 1 {
		t.Fatalf("unexpected mismatch count for outdated")
	}
	if metrics.CountsBySeverity[SeverityHigh] != 1 {
		t.Fatalf("unexpected severity count for high")
	}
}
