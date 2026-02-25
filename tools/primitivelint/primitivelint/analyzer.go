// SPDX-License-Identifier: MPL-2.0

// Package primitivelint implements a go/analysis analyzer that detects
// bare primitive type usage in struct fields, function parameters, and
// return types. It is designed to enforce DDD Value Type conventions
// where named types (e.g., type CommandName string) should be used
// instead of raw string, int, etc.
//
// The analyzer supports an exception mechanism via TOML config file
// and inline //primitivelint:ignore directives for intentional
// primitive usage at exec/OS boundaries, display-only fields, etc.
package primitivelint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// configPath holds the path to the TOML exceptions file, set via the
// -config flag.
var configPath string

// Analyzer is the primitivelint analysis pass. Use it with singlechecker
// or multichecker, or via go vet -vettool.
var Analyzer = &analysis.Analyzer{
	Name:     "primitivelint",
	Doc:      "reports bare primitive types where DDD Value Types should be used",
	URL:      "https://github.com/invowk/invowk/tools/primitivelint",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func init() {
	Analyzer.Flags.StringVar(&configPath, "config", "",
		"path to exceptions TOML config file")
}

func run(pass *analysis.Pass) (interface{}, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.GenDecl)(nil),
		(*ast.FuncDecl)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		// Skip test files entirely — test data legitimately uses primitives.
		if isTestFile(pass, n.Pos()) {
			return
		}

		// Skip files matching exclude_paths from config.
		filePath := pass.Fset.Position(n.Pos()).Filename
		if cfg.isExcludedPath(filePath) {
			return
		}

		switch n := n.(type) {
		case *ast.GenDecl:
			inspectStructFields(pass, n, cfg)
		case *ast.FuncDecl:
			inspectFuncDecl(pass, n, cfg)
		}
	})

	return nil, nil
}
