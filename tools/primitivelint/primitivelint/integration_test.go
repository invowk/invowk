// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzerWithConfig exercises the full analyzer pipeline with a
// TOML config file loaded, verifying that exception patterns and
// skip_types correctly suppress findings.
//
// NOT parallel: mutates the global configPath variable.
func TestAnalyzerWithConfig(t *testing.T) {
	testdata := analysistest.TestData()

	// Set the config path to the test TOML file inside the fixture package.
	origConfig := configPath
	configPath = filepath.Join(testdata, "src", "configexceptions", "primitivelint.toml")
	t.Cleanup(func() { configPath = origConfig })

	analysistest.Run(t, testdata, Analyzer, "configexceptions")
}

// TestAnalyzerWithRealExceptionsToml loads the real project exceptions.toml
// and runs against the basic fixture to verify:
// 1. The real config file parses without error.
// 2. It doesn't accidentally suppress basic findings (the basic fixture
//    has no patterns matching the real exceptions).
//
// NOT parallel: mutates the global configPath variable.
func TestAnalyzerWithRealExceptionsToml(t *testing.T) {
	testdata := analysistest.TestData()

	// The real exceptions.toml is two directories up from the primitivelint/ package.
	realConfig := filepath.Join(testdata, "..", "..", "exceptions.toml")

	origConfig := configPath
	configPath = realConfig
	t.Cleanup(func() { configPath = origConfig })

	// The basic fixture should produce the same diagnostics regardless of
	// the real config, because none of its types match real exception patterns.
	analysistest.Run(t, testdata, Analyzer, "basic")
}
