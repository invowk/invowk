// SPDX-License-Identifier: MPL-2.0

// primitivelint reports bare primitive types (string, int, float64, etc.)
// in struct fields, function parameters, and return types where DDD Value
// Types should be used instead.
//
// Usage:
//
//	primitivelint [-config=exceptions.toml] [-json] ./...
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/invowk/invowk/tools/primitivelint/primitivelint"
)

func main() {
	singlechecker.Main(primitivelint.Analyzer)
}
