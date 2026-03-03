// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCheckUseBeforeValidateClosureCastCFA(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	h := newAnalyzerHarness()
	setFlag(t, h.Analyzer, "check-cast-validation", "true")
	setFlag(t, h.Analyzer, "check-use-before-validate", "true")
	setFlag(t, h.Analyzer, "ubv-mode", ubvModeOrder)

	runAnalysisTest(t, testdata, h.Analyzer, "cfa_closure_ubv")
}

func TestCheckUseBeforeValidateEscapeInterproceduralSummaryCFA(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	h := newAnalyzerHarness()
	setFlag(t, h.Analyzer, "check-cast-validation", "true")
	setFlag(t, h.Analyzer, "check-use-before-validate", "true")
	setFlag(t, h.Analyzer, "ubv-mode", ubvModeEscape)
	setFlag(t, h.Analyzer, "cfg-backend", cfgBackendSSA)

	runAnalysisTest(t, testdata, h.Analyzer, "use_before_validate_escape")
}

func TestCheckCastValidationCFABackendASTConservative(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	h := newAnalyzerHarness()
	setFlag(t, h.Analyzer, "check-cast-validation", "true")
	setFlag(t, h.Analyzer, "cfg-backend", cfgBackendAST)

	runAnalysisTest(t, testdata, h.Analyzer, "cfa_backend_ast")
}

func TestCheckCastValidationCFABackendSSAHandlesNoReturn(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	h := newAnalyzerHarness()
	setFlag(t, h.Analyzer, "check-cast-validation", "true")
	setFlag(t, h.Analyzer, "cfg-backend", cfgBackendSSA)

	runAnalysisTest(t, testdata, h.Analyzer, "cfa_no_return_terminator")
}
