// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestSemanticSpecHistoricalFixturesReplay(t *testing.T) {
	t.Parallel()

	catalog := mustLoadSemanticRuleCatalog(t)
	testdata := analysistest.TestData()

	for _, fixture := range catalog.HistoricalMissFixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			h := newAnalyzerHarness()
			resetFlags(t, h)
			configureHistoricalFixtureReplay(t, testdata, h, fixture)
			assertHistoricalFixtureAnalyzes(t, h.Analyzer, fixture, expectedHistoricalCategoryForFixture(fixture))
		})
	}
}

func configureHistoricalFixtureReplay(t *testing.T, testdata string, h analyzerHarness, fixture string) {
	t.Helper()

	switch {
	case fixture == "constructorvalidates_nocfa_ast":
		setFlag(t, h.Analyzer, "check-constructor-validates", "true")
		setFlag(t, h.Analyzer, "cfg-backend", cfgBackendAST)
	case strings.HasPrefix(fixture, "castvalidation_nocfa_"):
		setFlag(t, h.Analyzer, "check-cast-validation", "true")
		setFlag(t, h.Analyzer, "cfg-backend", cfgBackendAST)
		if fixture == "castvalidation_nocfa_suppression" {
			setFlag(t, h.Analyzer, "config", filepath.Join(testdata, "src", fixture, "goplint.toml"))
			setFlag(t, h.Analyzer, "baseline", filepath.Join(testdata, "src", fixture, "goplint-baseline.toml"))
		}
	default:
		t.Fatalf("no replay configuration for historical fixture %q", fixture)
	}
}

func expectedHistoricalCategoryForFixture(fixture string) string {
	if strings.HasPrefix(fixture, "castvalidation_nocfa_") {
		return CategoryUnvalidatedCast
	}
	if fixture == "constructorvalidates_nocfa_ast" {
		return CategoryMissingConstructorValidate
	}
	return ""
}

func assertHistoricalFixtureAnalyzes(t *testing.T, analyzer *analysis.Analyzer, fixture, expectedCategory string) {
	t.Helper()

	diagnostics, _, results := collectDiagnosticsForPackages(t, analyzer, fixture)
	for _, result := range results {
		if result != nil && result.Err != nil {
			t.Fatalf("fixture %q analysis error: %v", fixture, result.Err)
		}
	}
	if expectedCategory == "" {
		t.Fatalf("fixture %q has no expected historical category mapping", fixture)
	}
	matched := false
	for _, diag := range diagnostics {
		if diag.Category == expectedCategory {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("fixture %q produced no diagnostics in expected category %q", fixture, expectedCategory)
	}
}
