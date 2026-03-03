// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/token"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestReportInconclusiveFindingPolicy(t *testing.T) {
	t.Parallel()

	const (
		category = CategoryUnvalidatedCastInconclusive
		finding  = "gpl3_test"
		message  = "inconclusive"
	)

	t.Run("off suppresses finding", func(t *testing.T) {
		t.Parallel()

		var diags []analysis.Diagnostic
		pass := &analysis.Pass{
			Report: func(diag analysis.Diagnostic) {
				diags = append(diags, diag)
			},
		}
		reportInconclusiveFindingWithMetaIfNotBaselined(
			pass,
			nil,
			cfgInconclusivePolicyOff,
			token.Pos(1),
			category,
			finding,
			message,
			map[string]string{"cfg_inconclusive_reason": "state-budget"},
		)
		if len(diags) != 0 {
			t.Fatalf("expected no diagnostics with policy off, got %d", len(diags))
		}
	})

	t.Run("warn keeps finding and tags severity metadata", func(t *testing.T) {
		t.Parallel()

		var diags []analysis.Diagnostic
		pass := &analysis.Pass{
			Report: func(diag analysis.Diagnostic) {
				diags = append(diags, diag)
			},
		}
		reportInconclusiveFindingWithMetaIfNotBaselined(
			pass,
			nil,
			cfgInconclusivePolicyWarn,
			token.Pos(1),
			category,
			finding,
			message,
			map[string]string{"cfg_inconclusive_reason": "state-budget"},
		)
		if len(diags) != 1 {
			t.Fatalf("expected one diagnostic with policy warn, got %d", len(diags))
		}
		if got := FindingMetaFromDiagnosticURL(diags[0].URL, "cfg_inconclusive_policy"); got != cfgInconclusivePolicyWarn {
			t.Fatalf("cfg_inconclusive_policy meta = %q, want %q", got, cfgInconclusivePolicyWarn)
		}
		if got := FindingMetaFromDiagnosticURL(diags[0].URL, "cfg_inconclusive_severity"); got != "warning" {
			t.Fatalf("cfg_inconclusive_severity meta = %q, want %q", got, "warning")
		}
	})
}

func TestCFGOutcomeMetaWithWitness(t *testing.T) {
	t.Parallel()

	meta := cfgOutcomeMetaWithWitness(
		cfgBackendSSA,
		5,
		3,
		pathOutcomeReasonDepthBudget,
		[]int32{0, 1, 2, 3},
		2,
	)
	if got := meta["witness_cfg_path"]; got != "0->1" {
		t.Fatalf("witness_cfg_path = %q, want %q", got, "0->1")
	}
	if got := meta["witness_cfg_steps"]; got != "2" {
		t.Fatalf("witness_cfg_steps = %q, want %q", got, "2")
	}
	if got := meta["witness_cfg_truncated"]; got != "true" {
		t.Fatalf("witness_cfg_truncated = %q, want %q", got, "true")
	}
}
