// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

// cfaAssignedCast records a type conversion assigned to a named variable,
// along with its containing AssignStmt for CFG lookup.
type cfaAssignedCast struct {
	target    castTarget
	typeName  string
	pos       ast.Node
	assign    ast.Node // AssignStmt or ValueSpec containing this cast
	castIndex int
}

// cfaUnassignedCast records a type conversion not assigned to a named
// variable (e.g., return, function argument, blank identifier).
type cfaUnassignedCast struct {
	typeName  string
	pos       ast.Node
	castIndex int
}

// cfaClosureHandler is called when the cast-collection walk encounters a
// FuncLit. Implementations decide how to handle the closure (e.g.,
// delegate to inspectClosureCastsCFA or recurse for nested closures).
// Returning false from the outer walk callback prevents descent into
// the closure body.
type cfaClosureHandler func(lit *ast.FuncLit, closureIdx int)

// collectCFACasts walks a function or closure body and classifies type
// conversions from raw primitives to DDD Value Types into assigned and
// unassigned casts. Executable closures (IIFEs, go/defer closure calls)
// are delegated to the provided handler rather than being analyzed inline.
// Non-executable literals (for example, detached func values) are skipped.
//
// This is the shared cast-collection logic used by both
// inspectUnvalidatedCastsCFA (outer functions) and
// inspectClosureCastsCFA (closure bodies). The walk root is always
// body (*ast.BlockStmt), so the parent *ast.FuncLit is never visited.
func collectCFACasts(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	parentMap map[ast.Node]ast.Node,
	onClosure cfaClosureHandler,
) ([]cfaAssignedCast, []cfaUnassignedCast) {
	var assignedCasts []cfaAssignedCast
	var unassignedCasts []cfaUnassignedCast
	castIndex := 0
	closureIndex := 0
	closureVarBindings := collectClosureVarBindings(pass, body)
	analyzedClosures := make(map[*ast.FuncLit]bool)

	analyzeClosure := func(lit *ast.FuncLit) {
		if lit == nil || analyzedClosures[lit] {
			return
		}
		analyzedClosures[lit] = true
		onClosure(lit, closureIndex)
		closureIndex++
	}

	ast.Inspect(body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			if isExecutableClosureLiteral(lit, parentMap) {
				analyzeClosure(lit)
			}
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if lit, ok := executableClosureVarCall(pass, call, closureVarBindings); ok {
			analyzeClosure(lit)
		}

		// Not a type conversion — skip.
		tv, ok := pass.TypesInfo.Types[call.Fun]
		if !ok || !tv.IsType() {
			return true
		}

		if len(call.Args) != 1 {
			return true
		}

		// Target must have Validate() — i.e., it's a DDD Value Type.
		targetType := tv.Type
		if !hasValidateMethod(targetType) {
			return true
		}

		// Source must be a raw primitive from a runtime expression.
		srcTV, srcOK := pass.TypesInfo.Types[call.Args[0]]
		if !srcOK {
			return true
		}
		if srcTV.Value != nil {
			return true // constant expression — skip
		}
		if isErrorMessageExpr(pass, call.Args[0]) {
			return true // error-message source — skip
		}
		if !isRawPrimitive(srcTV.Type) {
			return true // named-to-named cast — skip
		}

		targetTypeName := qualifiedTypeName(targetType, pass.Pkg)
		parent := parentMap[call]

		target, assignNode, assigned := resolveCastAssignmentTarget(pass, call, parentMap)
		if assigned {
			assignedCasts = append(assignedCasts, cfaAssignedCast{
				target:    target,
				typeName:  targetTypeName,
				pos:       call,
				assign:    assignNode,
				castIndex: castIndex,
			})
			castIndex++
			return true
		}

		// Unassigned cast — check auto-skip contexts.
		if isAutoSkipContext(pass, call, parent, parentMap) {
			return true
		}

		unassignedCasts = append(unassignedCasts, cfaUnassignedCast{
			typeName:  targetTypeName,
			pos:       call,
			castIndex: castIndex,
		})
		castIndex++
		return true
	})

	return assignedCasts, unassignedCasts
}

// isExecutableClosureLiteral reports whether lit is directly invoked in-place:
// func() { ... }(), go func() { ... }(), or defer func() { ... }().
func isExecutableClosureLiteral(lit *ast.FuncLit, parentMap map[ast.Node]ast.Node) bool {
	if lit == nil || parentMap == nil {
		return false
	}
	call, ok := closureLiteralCall(lit, parentMap)
	if !ok {
		return false
	}
	return stripParens(call.Fun) == lit
}

// closureLiteralCall returns the CallExpr that invokes lit, allowing any number
// of parenthesized wrappers between the literal and the call expression.
func closureLiteralCall(lit *ast.FuncLit, parentMap map[ast.Node]ast.Node) (*ast.CallExpr, bool) {
	if lit == nil || parentMap == nil {
		return nil, false
	}
	current := ast.Node(lit)
	for {
		parent := parentMap[current]
		if parent == nil {
			return nil, false
		}
		if paren, ok := parent.(*ast.ParenExpr); ok && paren.X == current {
			current = paren
			continue
		}
		call, ok := parent.(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		return call, true
	}
}

func collectClosureVarBindings(pass *analysis.Pass, body *ast.BlockStmt) map[string]*ast.FuncLit {
	if pass == nil || pass.TypesInfo == nil || body == nil {
		return nil
	}
	bindings := make(map[string]*ast.FuncLit)

	recordBinding := func(lhs *ast.Ident, rhs ast.Expr) {
		if lhs == nil || lhs.Name == "_" {
			return
		}
		lit, ok := exprFuncLit(rhs)
		if !ok {
			return
		}
		obj := objectForIdent(pass, lhs)
		if obj == nil {
			return
		}
		if _, isVar := obj.(*types.Var); !isVar {
			return
		}
		bindings[objectKey(obj)] = lit
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range node.Rhs {
				if i >= len(node.Lhs) {
					break
				}
				lhsIdent, ok := stripParens(node.Lhs[i]).(*ast.Ident)
				if !ok {
					continue
				}
				recordBinding(lhsIdent, rhs)
			}
		case *ast.ValueSpec:
			for i, rhs := range node.Values {
				if i >= len(node.Names) {
					break
				}
				recordBinding(node.Names[i], rhs)
			}
		}
		return true
	})

	return bindings
}

func exprFuncLit(expr ast.Expr) (*ast.FuncLit, bool) {
	if expr == nil {
		return nil, false
	}
	lit, ok := stripParens(expr).(*ast.FuncLit)
	return lit, ok
}

// executableClosureVarCall reports whether call invokes a local variable bound
// to a function literal (for example, f := func() { ... }; f()).
func executableClosureVarCall(
	pass *analysis.Pass,
	call *ast.CallExpr,
	bindings map[string]*ast.FuncLit,
) (*ast.FuncLit, bool) {
	if pass == nil || pass.TypesInfo == nil || call == nil || len(bindings) == 0 {
		return nil, false
	}
	funIdent, ok := stripParens(call.Fun).(*ast.Ident)
	if !ok {
		return nil, false
	}
	obj := objectForIdent(pass, funIdent)
	if obj == nil {
		return nil, false
	}
	lit, ok := bindings[objectKey(obj)]
	return lit, ok
}
