// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// inspectUnvalidatedCastsCFA is the CFA-enhanced replacement for
// inspectUnvalidatedCasts. Instead of a name-based heuristic that considers
// any Validate() call in the function body as validating all same-named
// variables, this function uses CFG path reachability to determine whether
// each individual cast has an unvalidated path to a return block.
//
// When --cfa is enabled, this function is called instead of
// inspectUnvalidatedCasts. The two implementations are fully compartmentalized:
// neither imports from the other.
func inspectUnvalidatedCastsCFA(
	pass *analysis.Pass,
	fn *ast.FuncDecl,
	excCfg *ExceptionConfig,
	bl *BaselineConfig,
) {
	if fn.Body == nil {
		return
	}
	if shouldSkipFunc(fn) {
		return
	}

	// Build the qualified function name for exception matching.
	pkgName := packageName(pass.Pkg)
	funcName := fn.Name.Name
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvName := receiverTypeName(fn.Recv.List[0].Type)
		if recvName != "" {
			funcName = recvName + "." + funcName
		}
	}
	qualFuncName := pkgName + "." + funcName

	// Build parent map for auto-skip context detection.
	parentMap := buildParentMap(fn.Body)

	// Build the CFG for path-sensitive analysis.
	funcCFG := buildFuncCFG(fn.Body)
	if funcCFG == nil {
		return
	}

	// Collect casts and closures in a single AST walk.
	type assignedCast struct {
		varName  string
		typeName string
		pos      ast.Node
		assign   ast.Node // the AssignStmt containing this cast
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
	closureIndex := 0

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// When encountering a closure, delegate to CFA closure analysis
		// instead of skipping entirely.
		if lit, ok := n.(*ast.FuncLit); ok {
			inspectClosureCastsCFA(pass, lit, qualFuncName, strconv.Itoa(closureIndex), excCfg, bl)
			closureIndex++
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
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

		// Check if assigned to a named variable.
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

		// Unassigned cast — check auto-skip contexts.
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

	// Report assigned casts where an unvalidated path to return exists.
	for _, ac := range assignedCasts {
		excKey := qualFuncName + ".cast-validation"
		if excCfg.isExcepted(excKey) {
			continue
		}

		// Find the assignment in the CFG.
		defBlock, defIdx := findDefiningBlock(funcCFG, ac.assign)
		if defBlock == nil {
			// Node not found in CFG — likely in dead code. Skip.
			continue
		}

		// Check if there's any path from the cast to a return block
		// that doesn't pass through varName.Validate().
		if !hasPathToReturnWithoutValidate(funcCFG, defBlock, defIdx, ac.varName) {
			continue // all paths validated
		}

		msg := fmt.Sprintf("type conversion to %s from non-constant without Validate() check", ac.typeName)
		findingID := StableFindingID(CategoryUnvalidatedCast, "cfa", qualFuncName, ac.typeName, "assigned", strconv.Itoa(ac.idx))
		if bl.ContainsFinding(CategoryUnvalidatedCast, findingID, msg) {
			continue
		}

		reportDiagnostic(pass, ac.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}

	// Unassigned casts: always report (no variable to track).
	for _, uc := range unassignedCasts {
		excKey := qualFuncName + ".cast-validation"
		if excCfg.isExcepted(excKey) {
			continue
		}

		msg := fmt.Sprintf("type conversion to %s from non-constant without Validate() check", uc.typeName)
		findingID := StableFindingID(CategoryUnvalidatedCast, "cfa", qualFuncName, uc.typeName, "unassigned", strconv.Itoa(uc.idx))
		if bl.ContainsFinding(CategoryUnvalidatedCast, findingID, msg) {
			continue
		}

		reportDiagnostic(pass, uc.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}
}
