// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// castTarget identifies the LHS target of a cast assignment.
// It stores a canonical key so equivalent selector forms (for example
// (*cfg).Name and cfg.Name) match consistently.
type castTarget struct {
	displayName string
	targetKey   string
}

func newCastTargetFromName(name string) castTarget {
	return castTarget{
		displayName: name,
		targetKey:   "name:" + name,
	}
}

func castTargetFromExpr(pass *analysis.Pass, expr ast.Expr) (castTarget, bool) {
	key := targetKeyForExpr(pass, expr)
	if key == "" {
		return castTarget{}, false
	}
	display := exprStringKey(expr)
	if ident, ok := stripParensAndStar(expr).(*ast.Ident); ok {
		display = ident.Name
	}
	return castTarget{
		displayName: display,
		targetKey:   key,
	}, true
}

func (t castTarget) key() string {
	return t.targetKey
}

func (t castTarget) matchesExpr(pass *analysis.Pass, expr ast.Expr) bool {
	return t.targetKey != "" && targetKeyForExpr(pass, expr) == t.targetKey
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
	expr = stripParensAndStar(expr)
	switch e := expr.(type) {
	case *ast.Ident:
		if e.Name == "_" {
			return ""
		}
		if obj := objectForIdent(pass, e); obj != nil {
			return objectKey(obj)
		}
		return "name:" + e.Name
	case *ast.SelectorExpr:
		base := targetKeyForExpr(pass, e.X)
		if base == "" {
			return ""
		}
		return base + "." + e.Sel.Name
	case *ast.IndexExpr:
		base := targetKeyForExpr(pass, e.X)
		if base == "" {
			return ""
		}
		index := canonicalIndexExprKey(e.Index)
		if index == "" {
			return ""
		}
		return base + "[" + index + "]"
	case *ast.IndexListExpr:
		base := targetKeyForExpr(pass, e.X)
		if base == "" {
			return ""
		}
		indexes := make([]string, 0, len(e.Indices))
		for _, idx := range e.Indices {
			key := canonicalIndexExprKey(idx)
			if key == "" {
				return ""
			}
			indexes = append(indexes, key)
		}
		return base + "[" + strings.Join(indexes, ",") + "]"
	}

	key := exprStringKey(expr)
	if key == "" {
		return ""
	}
	return "expr:" + key
}

func objectKey(obj types.Object) string {
	if obj == nil {
		return ""
	}
	return fmt.Sprintf("obj:%p", obj)
}

func canonicalIndexExprKey(expr ast.Expr) string {
	return exprStringKey(stripParens(expr))
}

func stripParens(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func stripParensAndStar(expr ast.Expr) ast.Expr {
	for {
		switch e := expr.(type) {
		case *ast.ParenExpr:
			expr = e.X
		case *ast.StarExpr:
			expr = e.X
		default:
			return expr
		}
	}
}

func exprStringKey(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	return types.ExprString(expr)
}
