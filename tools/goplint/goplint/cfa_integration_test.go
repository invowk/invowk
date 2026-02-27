// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestCheckCastValidationCFA exercises the --check-cast-validation mode with
// --cfa enabled against the cfa_castvalidation fixture. This verifies
// path-reachability analysis catches cases the AST heuristic misses:
// dead-branch validation, use-before-validate, and conditional validation.
//
// NOT parallel: shares Analyzer.Flags state.
func TestCheckCastValidationCFA(t *testing.T) {
	testdata := analysistest.TestData()
	t.Cleanup(func() { resetFlags(t) })
	setFlag(t, "check-cast-validation", "true")
	setFlag(t, "cfa", "true")

	analysistest.Run(t, testdata, Analyzer, "cfa_castvalidation")
}

// TestCheckCastValidationCFAClosure exercises --check-cast-validation with
// --cfa enabled against the cfa_closure fixture. This verifies that CFA
// mode analyzes closure bodies (which AST mode skips entirely) with
// independent CFGs and validation scopes.
//
// NOT parallel: shares Analyzer.Flags state.
func TestCheckCastValidationCFAClosure(t *testing.T) {
	testdata := analysistest.TestData()
	t.Cleanup(func() { resetFlags(t) })
	setFlag(t, "check-cast-validation", "true")
	setFlag(t, "cfa", "true")

	analysistest.Run(t, testdata, Analyzer, "cfa_closure")
}

// TestCFADoesNotAffectCheckAll verifies that --check-all does not implicitly
// enable --cfa, confirming the opt-in design.
//
// NOT parallel: shares Analyzer.Flags state.
func TestCFADoesNotAffectCheckAll(t *testing.T) {
	t.Cleanup(func() { resetFlags(t) })
	setFlag(t, "check-all", "true")

	rc := newRunConfig()
	if rc.cfa {
		t.Error("expected cfa = false when only --check-all is set")
	}
}
