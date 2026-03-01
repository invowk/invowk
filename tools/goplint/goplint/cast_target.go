// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// castTarget identifies the LHS target of a cast assignment.
// For identifiers it tracks type-checker object identity; for non-identifier
// targets (for example selector assignments like cfg.Name = T(raw)) it falls
// back to expression-key matching.
type castTarget struct {
	displayName string
	obj         types.Object
	exprKey     string
}

func newCastTargetFromName(name string) castTarget {
	return castTarget{
		displayName: name,
		exprKey:     name,
	}
}

func castTargetFromExpr(pass *analysis.Pass, expr ast.Expr) (castTarget, bool) {
	if expr == nil {
		return castTarget{}, false
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Name == "_" {
			return castTarget{}, false
		}
		return castTarget{
			displayName: ident.Name,
			obj:         objectForIdent(pass, ident),
			exprKey:     exprStringKey(expr),
		}, true
	}
	key := exprStringKey(expr)
	if key == "" {
		return castTarget{}, false
	}
	return castTarget{
		displayName: key,
		exprKey:     key,
	}, true
}

func (t castTarget) key() string {
	if t.obj != nil {
		return fmt.Sprintf("obj:%p", t.obj)
	}
	if t.exprKey != "" {
		return "expr:" + t.exprKey
	}
	return "name:" + t.displayName
}

func (t castTarget) matchesExpr(pass *analysis.Pass, expr ast.Expr) bool {
	if expr == nil {
		return false
	}
	if t.obj != nil {
		ident, ok := expr.(*ast.Ident)
		if !ok {
			return false
		}
		return objectForIdent(pass, ident) == t.obj
	}
	return t.exprKey != "" && exprStringKey(expr) == t.exprKey
}

func objectForIdent(pass *analysis.Pass, ident *ast.Ident) types.Object {
	if pass == nil || pass.TypesInfo == nil || ident == nil {
		return nil
	}
	if obj := pass.TypesInfo.Uses[ident]; obj != nil {
		return obj
	}
	if obj := pass.TypesInfo.Defs[ident]; obj != nil {
		return obj
	}
	return nil
}

func targetKeyForExpr(pass *analysis.Pass, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if ident, ok := expr.(*ast.Ident); ok {
		if ident.Name == "_" {
			return ""
		}
		if obj := objectForIdent(pass, ident); obj != nil {
			return fmt.Sprintf("obj:%p", obj)
		}
	}
	key := exprStringKey(expr)
	if key == "" {
		return ""
	}
	return "expr:" + key
}

func exprStringKey(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	return types.ExprString(expr)
}
