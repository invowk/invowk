// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// inspectIsValidUsage walks a function body to detect two misuse patterns
// of IsValid() calls on DDD Value Types:
//
//  1. unused-isvalid-result: The (bool, []error) return is completely discarded
//     (bare expression statement or both returns assigned to _).
//
//  2. truncated-isvalid-errors: The []error return from IsValid() is later
//     indexed with [0], discarding subsequent errors. The correct pattern is
//     errors.Join(errs...).
func inspectIsValidUsage(pass *analysis.Pass, fn *ast.FuncDecl, cfg *ExceptionConfig, bl *BaselineConfig) {
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

	// Build a parent map for detecting expression statements and blank
	// assignments containing IsValid() calls.
	parentMap := buildParentMap(fn.Body)

	// errSliceVars tracks variable names assigned from the []error return
	// of IsValid() calls. Used to detect errs[0] truncation patterns.
	errSliceVars := make(map[string]token.Pos) // varName → position of the IsValid() call

	// Phase 1: Walk the function body looking for IsValid() calls.
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Skip closure bodies — separate validation scope.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for x.IsValid() pattern.
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "IsValid" {
			return true
		}

		// Verify the receiver type actually has IsValid() (bool, []error).
		recvType := pass.TypesInfo.TypeOf(sel.X)
		if recvType == nil || !hasIsValidMethod(recvType) {
			return true
		}

		parent := parentMap[call]

		// Check 1: Result completely discarded as a bare expression statement.
		// Pattern: x.IsValid() as a standalone statement.
		if _, isExprStmt := parent.(*ast.ExprStmt); isExprStmt {
			reportIsValidUsageFinding(pass, call.Pos(), CategoryUnusedIsValidResult,
				qualFuncName, cfg, bl,
				"IsValid() result discarded — both bool and []error are unused")
			return true
		}

		// Check 2: Both returns assigned to blank identifiers.
		// Pattern: _, _ = x.IsValid()
		if assign, isAssign := parent.(*ast.AssignStmt); isAssign {
			if allBlankLHS(assign, call) {
				reportIsValidUsageFinding(pass, call.Pos(), CategoryUnusedIsValidResult,
					qualFuncName, cfg, bl,
					"IsValid() result discarded — both bool and []error are unused")
				return true
			}

			// Track the []error variable name for truncation detection.
			// In `ok, errs := x.IsValid()`, the second LHS is the error slice.
			trackErrSliceVar(assign, call, errSliceVars)
		}

		return true
	})

	// Phase 2: Look for errs[0] truncation patterns on tracked variables.
	if len(errSliceVars) == 0 {
		return
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Skip closure bodies.
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		indexExpr, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}

		// Check if X is an identifier matching a tracked error slice variable.
		ident, ok := indexExpr.X.(*ast.Ident)
		if !ok {
			return true
		}

		if _, tracked := errSliceVars[ident.Name]; !tracked {
			return true
		}

		// Check if the index is the literal 0.
		lit, ok := indexExpr.Index.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT || lit.Value != "0" {
			return true
		}

		reportIsValidUsageFinding(pass, indexExpr.Pos(), CategoryTruncatedIsValidErrs,
			qualFuncName, cfg, bl,
			fmt.Sprintf("IsValid() errors truncated via %s[0] — use errors.Join(%s...) instead",
				ident.Name, ident.Name))

		return true
	})
}

// allBlankLHS reports whether all LHS identifiers in the assignment that
// correspond to the given IsValid() call RHS are blank identifiers (_).
// This detects `_, _ = x.IsValid()` patterns.
func allBlankLHS(assign *ast.AssignStmt, call *ast.CallExpr) bool {
	// Find which RHS position this call occupies.
	rhsIdx := -1
	for i, rhs := range assign.Rhs {
		if rhs == call {
			rhsIdx = i
			break
		}
	}
	if rhsIdx < 0 {
		return false
	}

	// For multi-value returns assigned via a single RHS expression,
	// the LHS has multiple entries and the RHS has one entry.
	if len(assign.Rhs) == 1 && len(assign.Lhs) >= 2 {
		for _, lhs := range assign.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if !ok || ident.Name != "_" {
				return false
			}
		}
		return true
	}

	// Single-value assignment: just check the corresponding LHS.
	if rhsIdx < len(assign.Lhs) {
		ident, ok := assign.Lhs[rhsIdx].(*ast.Ident)
		return ok && ident.Name == "_"
	}

	return false
}

// trackErrSliceVar records the variable name assigned to the []error return
// from an IsValid() call. In an assignment like `ok, errs := x.IsValid()`,
// the second LHS entry (index 1) is the error slice variable.
func trackErrSliceVar(assign *ast.AssignStmt, call *ast.CallExpr, errVars map[string]token.Pos) {
	// Multi-value return: len(RHS) == 1, len(LHS) >= 2.
	if len(assign.Rhs) != 1 || assign.Rhs[0] != call {
		return
	}
	if len(assign.Lhs) < 2 {
		return
	}

	// The second LHS is the []error variable.
	errIdent, ok := assign.Lhs[1].(*ast.Ident)
	if !ok || errIdent.Name == "_" {
		return
	}

	errVars[errIdent.Name] = call.Pos()
}

// reportIsValidUsageFinding emits a diagnostic for an IsValid() usage issue,
// respecting exception patterns and baseline suppression.
func reportIsValidUsageFinding(
	pass *analysis.Pass,
	pos token.Pos,
	category string,
	qualFuncName string,
	cfg *ExceptionConfig,
	bl *BaselineConfig,
	message string,
) {
	excKey := qualFuncName + ".isvalid-usage"
	if cfg.isExcepted(excKey) {
		return
	}

	findingID := StableFindingID(category, qualFuncName, message)
	if bl.ContainsFinding(category, findingID, message) {
		return
	}

	reportDiagnostic(pass, pos, category, findingID, message)
}
