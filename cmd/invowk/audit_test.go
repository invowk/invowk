// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"testing"

	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/pkg/types"
)

func TestConvertFindingsPreservesCheckerName(t *testing.T) {
	t.Parallel()

	got := convertFindings([]audit.Finding{
		{
			Severity:    audit.SeverityHigh,
			Category:    audit.CategoryIntegrity,
			SurfaceID:   "SC-01",
			CheckerName: "lockfile",
			FilePath:    types.FilesystemPath("invowk.lock.cue"),
			Title:       "hash mismatch",
		},
	})

	if len(got) != 1 {
		t.Fatalf("convertFindings() returned %d findings, want 1", len(got))
	}
	if got[0].CheckerName != auditCheckerName("lockfile") {
		t.Errorf("CheckerName = %q, want lockfile", got[0].CheckerName)
	}
}

func TestConvertFindingsPreservesSurfaceKind(t *testing.T) {
	t.Parallel()

	got := convertFindings([]audit.Finding{
		{
			Severity:    audit.SeverityHigh,
			Category:    audit.CategoryIntegrity,
			SurfaceID:   "SC-01",
			SurfaceKind: audit.SurfaceKindVendoredModule,
			CheckerName: "lockfile",
			FilePath:    types.FilesystemPath("invowk.lock.cue"),
			Title:       "hash mismatch",
		},
	})

	if len(got) != 1 {
		t.Fatalf("convertFindings() returned %d findings, want 1", len(got))
	}
	if got[0].SurfaceKind != audit.SurfaceKindVendoredModule {
		t.Errorf("SurfaceKind = %q, want %q", got[0].SurfaceKind, audit.SurfaceKindVendoredModule)
	}
}

func TestConvertDiagnosticsUsesCLIDTO(t *testing.T) {
	t.Parallel()

	diag, err := audit.NewDiagnostic("warning", "module-skipped", "skipped invalid module")
	if err != nil {
		t.Fatalf("NewDiagnostic() error = %v", err)
	}

	got := convertDiagnostics([]audit.Diagnostic{diag})
	if len(got) != 1 {
		t.Fatalf("convertDiagnostics() returned %d diagnostics, want 1", len(got))
	}
	if got[0].Severity != "warning" || got[0].Code != "module-skipped" || got[0].Message != "skipped invalid module" {
		t.Fatalf("diagnostic DTO = %#v", got[0])
	}
}
