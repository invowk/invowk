// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestRenderAuditJSONSeparatesFindingsAndCompoundThreats(t *testing.T) {
	t.Parallel()

	report := &audit.Report{
		Findings: []audit.Finding{{
			Severity:    audit.SeverityMedium,
			Category:    audit.CategoryTrust,
			CheckerName: "module-metadata",
			Title:       "base finding",
		}},
		Correlated: []audit.Finding{{
			Severity:    audit.SeverityHigh,
			Category:    audit.CategoryTrust,
			CheckerName: "correlator",
			Title:       "compound finding",
		}},
	}

	var buf bytes.Buffer
	if err := renderAuditJSON(&buf, report, audit.SeverityInfo); err != nil {
		t.Fatalf("renderAuditJSON() error = %v", err)
	}

	var got auditJSONOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("len(Findings) = %d, want 1", len(got.Findings))
	}
	if len(got.CompoundThreats) != 1 {
		t.Fatalf("len(CompoundThreats) = %d, want 1", len(got.CompoundThreats))
	}
	if got.Summary.Total != 2 {
		t.Fatalf("Summary.Total = %d, want 2", got.Summary.Total)
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

func TestScanErrorContainsCheckerFindsJoinedLLMFailure(t *testing.T) {
	t.Parallel()

	err := errors.Join(
		&audit.CheckerFailedError{CheckerName: "network", Err: errors.New("network failed")},
		&audit.CheckerFailedError{CheckerName: audit.LLMCheckerName, Err: errors.New("llm failed")},
	)

	if !scanErrorContainsChecker(err, audit.LLMCheckerName) {
		t.Fatal("expected joined LLM checker failure to be detected")
	}
}

func TestScanErrorContainsCheckerIgnoresOtherCheckers(t *testing.T) {
	t.Parallel()

	err := &audit.CheckerFailedError{CheckerName: "network", Err: errors.New("network failed")}

	if scanErrorContainsChecker(err, audit.LLMCheckerName) {
		t.Fatal("did not expect non-LLM checker failure to be detected as LLM")
	}
}
