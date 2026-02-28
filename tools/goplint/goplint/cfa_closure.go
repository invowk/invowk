// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// inspectClosureCastsCFA analyzes a FuncLit body for unvalidated casts
// using its own CFG and independent validation scope. This is the CFA
// counterpart to the AST mode's closure skip — instead of ignoring closures
// entirely, CFA mode builds a per-closure CFG and performs the same
// path-reachability analysis.
//
// Each closure gets its own cast index counter (starting from 0) and
// finding IDs include "cfa", "closure", and the closure's prefix
// within the enclosing function for uniqueness. Nested closures use
// compound prefixes (e.g., "0/1" for the second closure inside the first).
func inspectClosureCastsCFA(
	pass *analysis.Pass,
	lit *ast.FuncLit,
	qualEnclosingFunc string,
	closurePrefix string,
	excCfg *ExceptionConfig,
	bl *BaselineConfig,
) {
	if lit.Body == nil {
		return
	}

	// Build CFG for this closure's body.
	closureCFG := buildFuncCFG(lit.Body)
	if closureCFG == nil {
		return
	}

	parentMap := buildParentMap(lit.Body)

	type assignedCast struct {
		varName  string
		typeName string
		pos      ast.Node
		assign   ast.Node
		idx      int
	}
	type unassignedCast struct {
		typeName string
		pos      ast.Node
		idx      int
	}

	var assignedCasts []assignedCast
	var unassignedCasts []unassignedCast
	castIndex := 0
	nestedIndex := 0

	ast.Inspect(lit.Body, func(n ast.Node) bool {
		// Recursively analyze nested closures with their own CFG
		// and independent validation scope.
		if nested, ok := n.(*ast.FuncLit); ok && nested != lit {
			nestedPrefix := closurePrefix + "/" + strconv.Itoa(nestedIndex)
			inspectClosureCastsCFA(pass, nested, qualEnclosingFunc, nestedPrefix, excCfg, bl)
			nestedIndex++
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		tv, ok := pass.TypesInfo.Types[call.Fun]
		if !ok || !tv.IsType() {
			return true
		}

		if len(call.Args) != 1 {
			return true
		}

		targetType := tv.Type
		if !hasValidateMethod(targetType) {
			return true
		}

		srcTV, srcOK := pass.TypesInfo.Types[call.Args[0]]
		if !srcOK {
			return true
		}
		if srcTV.Value != nil {
			return true
		}
		if isErrorMessageExpr(pass, call.Args[0]) {
			return true
		}
		if !isRawPrimitive(srcTV.Type) {
			return true
		}

		targetTypeName := qualifiedTypeName(targetType, pass.Pkg)
		parent := parentMap[call]

		if assign, ok := parent.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if rhs != call {
					continue
				}
				if i < len(assign.Lhs) {
					if ident, ok := assign.Lhs[i].(*ast.Ident); ok && ident.Name != "_" {
						assignedCasts = append(assignedCasts, assignedCast{
							varName:  ident.Name,
							typeName: targetTypeName,
							pos:      call,
							assign:   assign,
							idx:      castIndex,
						})
						castIndex++
						return true
					}
				}
			}
		}

		if isAutoSkipContext(pass, call, parent) {
			return true
		}

		unassignedCasts = append(unassignedCasts, unassignedCast{
			typeName: targetTypeName,
			pos:      call,
			idx:      castIndex,
		})
		castIndex++
		return true
	})

	// Report assigned casts with unvalidated paths.
	for _, ac := range assignedCasts {
		excKey := qualEnclosingFunc + ".cast-validation"
		if excCfg.isExcepted(excKey) {
			continue
		}

		if hasIgnoreAtPos(pass, ac.pos.Pos()) {
			continue
		}

		defBlock, defIdx := findDefiningBlock(closureCFG, ac.assign)
		if defBlock == nil {
			continue
		}

		if !hasPathToReturnWithoutValidate(closureCFG, defBlock, defIdx, ac.varName) {
			continue
		}

		msg := fmt.Sprintf("type conversion to %s from non-constant without Validate() check", ac.typeName)
		findingID := StableFindingID(CategoryUnvalidatedCast, "cfa", "closure", closurePrefix, qualEnclosingFunc, ac.typeName, "assigned", strconv.Itoa(ac.idx))
		if bl.ContainsFinding(CategoryUnvalidatedCast, findingID, msg) {
			continue
		}

		reportDiagnostic(pass, ac.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}

	// Report unassigned casts.
	for _, uc := range unassignedCasts {
		excKey := qualEnclosingFunc + ".cast-validation"
		if excCfg.isExcepted(excKey) {
			continue
		}

		if hasIgnoreAtPos(pass, uc.pos.Pos()) {
			continue
		}

		msg := fmt.Sprintf("type conversion to %s from non-constant without Validate() check", uc.typeName)
		findingID := StableFindingID(CategoryUnvalidatedCast, "cfa", "closure", closurePrefix, qualEnclosingFunc, uc.typeName, "unassigned", strconv.Itoa(uc.idx))
		if bl.ContainsFinding(CategoryUnvalidatedCast, findingID, msg) {
			continue
		}

		reportDiagnostic(pass, uc.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}
}
