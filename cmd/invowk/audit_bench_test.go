// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/pkg/types"
)

// BenchmarkAuditRenderJSONReport benchmarks the user-facing JSON renderer for
// deterministic audit reports.
func BenchmarkAuditRenderJSONReport(b *testing.B) {
	report := benchmarkAuditReport()
	var buf bytes.Buffer

	b.ResetTimer()
	for b.Loop() {
		buf.Reset()
		if err := renderAuditJSON(&buf, report, audit.SeverityLow); err != nil {
			b.Fatalf("renderAuditJSON() error = %v", err)
		}
		if buf.Len() == 0 {
			b.Fatal("renderAuditJSON() produced empty output")
		}
	}
}

func benchmarkAuditReport() *audit.Report {
	findings := make([]audit.Finding, 0, 48)
	for i := range 48 {
		findings = append(findings, audit.Finding{
			Code:           audit.FindingCode(fmt.Sprintf("benchmark-finding-%02d", i)),
			Severity:       benchmarkSeverity(i),
			Category:       benchmarkCategory(i),
			SurfaceID:      "com.example.audit.benchmark",
			SurfaceKind:    audit.SurfaceKindLocalModule,
			CheckerName:    "benchmark",
			FilePath:       types.FilesystemPath(fmt.Sprintf("module/scripts/script-%02d.sh", i)),
			Line:           i + 1,
			Title:          fmt.Sprintf("Benchmark finding %02d", i),
			Description:    "Synthetic audit finding used to benchmark JSON rendering.",
			Recommendation: "Keep audit report rendering fast as finding counts grow.",
		})
	}
	return &audit.Report{
		Findings:        findings,
		ScanDuration:    125 * time.Millisecond,
		ModuleCount:     4,
		InvowkfileCount: 4,
		ScriptCount:     48,
	}
}

func benchmarkSeverity(i int) audit.Severity {
	switch i % 4 {
	case 0:
		return audit.SeverityLow
	case 1:
		return audit.SeverityMedium
	case 2:
		return audit.SeverityHigh
	default:
		return audit.SeverityCritical
	}
}

func benchmarkCategory(i int) audit.Category {
	switch i % 4 {
	case 0:
		return audit.CategoryIntegrity
	case 1:
		return audit.CategoryExecution
	case 2:
		return audit.CategoryExfiltration
	default:
		return audit.CategoryTrust
	}
}
