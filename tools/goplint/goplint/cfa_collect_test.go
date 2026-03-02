// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/ast"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestValidateMethodReceiverFromExpr(t *testing.T) {
	t.Parallel()

	src := `package testpkg
type Val string
func (v Val) Validate() error { return nil }
func (v Val) Setup() {}

type Holder struct {
	Validate func() error
}

func use() {
	var x Val
	var h Holder
	_ = x.Validate
	_ = Val.Validate
	_ = h.Validate
	_ = x.Setup
}`

	pass, file := buildTypedPassFromSource(t, src)
	useFn := findFuncDecl(t, file, "use")
	selectorMethodVal := findSelectorExprInFunc(t, useFn, "x", "Validate")
	selectorMethodExpr := findSelectorExprInFunc(t, useFn, "Val", "Validate")
	selectorField := findSelectorExprInFunc(t, useFn, "h", "Validate")
	selectorOther := findSelectorExprInFunc(t, useFn, "x", "Setup")

	t.Run("guards", func(t *testing.T) {
		t.Parallel()

		if recv, isMethodExpr, ok := validateMethodReceiverFromExpr(nil, selectorMethodVal); recv != nil || isMethodExpr || ok {
			t.Fatalf("validateMethodReceiverFromExpr(nil pass) = (%v, %v, %v), want (nil, false, false)", recv, isMethodExpr, ok)
		}
		if recv, isMethodExpr, ok := validateMethodReceiverFromExpr(pass, nil); recv != nil || isMethodExpr || ok {
			t.Fatalf("validateMethodReceiverFromExpr(nil expr) = (%v, %v, %v), want (nil, false, false)", recv, isMethodExpr, ok)
		}
	})

	t.Run("method value", func(t *testing.T) {
		t.Parallel()

		receiver, isMethodExpr, ok := validateMethodReceiverFromExpr(pass, selectorMethodVal)
		if !ok || isMethodExpr {
			t.Fatalf("validateMethodReceiverFromExpr(method value) = (_, %v, %v), want (_, false, true)", isMethodExpr, ok)
		}
		if got := types.ExprString(receiver); got != "x" {
			t.Fatalf("receiver = %q, want %q", got, "x")
		}
	})

	t.Run("method expression", func(t *testing.T) {
		t.Parallel()

		receiver, isMethodExpr, ok := validateMethodReceiverFromExpr(pass, selectorMethodExpr)
		if !ok || !isMethodExpr {
			t.Fatalf("validateMethodReceiverFromExpr(method expr) = (_, %v, %v), want (_, true, true)", isMethodExpr, ok)
		}
		if got := types.ExprString(receiver); got != "Val" {
			t.Fatalf("receiver = %q, want %q", got, "Val")
		}
	})

	t.Run("field selection defaults to false", func(t *testing.T) {
		t.Parallel()

		if recv, isMethodExpr, ok := validateMethodReceiverFromExpr(pass, selectorField); recv != nil || isMethodExpr || ok {
			t.Fatalf("validateMethodReceiverFromExpr(field) = (%v, %v, %v), want (nil, false, false)", recv, isMethodExpr, ok)
		}
	})

	t.Run("non validate selector", func(t *testing.T) {
		t.Parallel()

		if recv, isMethodExpr, ok := validateMethodReceiverFromExpr(pass, selectorOther); recv != nil || isMethodExpr || ok {
			t.Fatalf("validateMethodReceiverFromExpr(non-validate) = (%v, %v, %v), want (nil, false, false)", recv, isMethodExpr, ok)
		}
	})

	t.Run("fallback without selections", func(t *testing.T) {
		t.Parallel()

		cloned := clonePassTypesInfo(pass)
		delete(cloned.TypesInfo.Selections, selectorMethodVal)
		delete(cloned.TypesInfo.Selections, selectorField)

		receiver, isMethodExpr, ok := validateMethodReceiverFromExpr(cloned, selectorMethodVal)
		if !ok || isMethodExpr {
			t.Fatalf("validateMethodReceiverFromExpr(fallback method value) = (_, %v, %v), want (_, false, true)", isMethodExpr, ok)
		}
		if got := types.ExprString(receiver); got != "x" {
			t.Fatalf("receiver = %q, want %q", got, "x")
		}

		if recv, fallbackMethodExpr, fallbackOK := validateMethodReceiverFromExpr(cloned, selectorField); recv != nil || fallbackMethodExpr || fallbackOK {
			t.Fatalf("validateMethodReceiverFromExpr(fallback field) = (%v, %v, %v), want (nil, false, false)", recv, fallbackMethodExpr, fallbackOK)
		}
	})
}

func clonePassTypesInfo(pass *analysis.Pass) *analysis.Pass {
	clone := &analysis.Pass{
		Fset:      pass.Fset,
		Files:     pass.Files,
		Pkg:       pass.Pkg,
		TypesInfo: &types.Info{},
	}

	clone.TypesInfo.Types = make(map[ast.Expr]types.TypeAndValue, len(pass.TypesInfo.Types))
	for expr, tv := range pass.TypesInfo.Types {
		clone.TypesInfo.Types[expr] = tv
	}

	clone.TypesInfo.Defs = make(map[*ast.Ident]types.Object, len(pass.TypesInfo.Defs))
	for ident, obj := range pass.TypesInfo.Defs {
		clone.TypesInfo.Defs[ident] = obj
	}

	clone.TypesInfo.Uses = make(map[*ast.Ident]types.Object, len(pass.TypesInfo.Uses))
	for ident, obj := range pass.TypesInfo.Uses {
		clone.TypesInfo.Uses[ident] = obj
	}

	clone.TypesInfo.Selections = make(map[*ast.SelectorExpr]*types.Selection, len(pass.TypesInfo.Selections))
	for sel, selection := range pass.TypesInfo.Selections {
		clone.TypesInfo.Selections[sel] = selection
	}

	return clone
}

func findSelectorExprInFunc(t *testing.T, fn *ast.FuncDecl, xName, selName string) *ast.SelectorExpr {
	t.Helper()

	var found *ast.SelectorExpr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != selName {
			return true
		}
		ident, ok := stripParens(sel.X).(*ast.Ident)
		if !ok || ident.Name != xName {
			return true
		}
		found = sel
		return false
	})
	if found == nil {
		t.Fatalf("selector %s.%s not found", xName, selName)
	}
	return found
}
