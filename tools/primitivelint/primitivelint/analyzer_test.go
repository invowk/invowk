// SPDX-License-Identifier: MPL-2.0

package primitivelint_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/invowk/invowk/tools/primitivelint/primitivelint"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, primitivelint.Analyzer,
		"basic",
		"structs",
		"funcparams",
		"returns",
		"interfaces",
		"typedefs",
		"exceptions",
		"skipfuncs",
		"edgecases",
		"generics",
	)
}
