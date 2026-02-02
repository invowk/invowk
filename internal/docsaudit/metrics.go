// SPDX-License-Identifier: MPL-2.0

package docsaudit

// ComputeMetrics calculates coverage and mismatch/severity counts.
func ComputeMetrics(surfaces []UserFacingSurface, findings []Finding) Metrics {
	metrics := Metrics{
		CountsByMismatchType: make(map[MismatchType]int),
		CountsBySeverity:     make(map[Severity]int),
	}

	metrics.TotalSurfaces = len(surfaces)
	covered := 0
	for _, surface := range surfaces {
		if len(surface.DocumentationRefs) > 0 {
			covered++
		}
	}

	if metrics.TotalSurfaces > 0 {
		metrics.CoveragePercentage = (float64(covered) / float64(metrics.TotalSurfaces)) * 100
	}

	for _, mismatchType := range mismatchTypeOrder {
		metrics.CountsByMismatchType[mismatchType] = 0
	}
	for _, severity := range severityOrder {
		metrics.CountsBySeverity[severity] = 0
	}

	for _, finding := range findings {
		if finding.MismatchType != "" {
			metrics.CountsByMismatchType[finding.MismatchType]++
		}
		if finding.Severity != "" {
			metrics.CountsBySeverity[finding.Severity]++
		}
	}

	return metrics
}
