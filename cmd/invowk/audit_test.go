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
