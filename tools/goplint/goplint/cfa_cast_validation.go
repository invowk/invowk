// SPDX-License-Identifier: MPL-2.0

package goplint

import (
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
	checkUBV bool,
	ubvMode string,
	cfgBackend string,
	cfgMaxStates int,
	cfgMaxDepth int,
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
	funcCFG := buildFuncCFGForBackend(pass, fn.Body, cfgBackend)
	if funcCFG == nil {
		return
	}
	noReturnAliases := collectNoReturnFuncAliasEvents(pass, fn.Body)

	// Collect casts using the shared CFA collection logic.
	// Closures are delegated to inspectClosureCastsCFA.
	assignedCasts, unassignedCasts, closureCalls, _ := collectCFACasts(
		pass, fn.Body, parentMap,
		func(lit *ast.FuncLit, closureIdx int) {
			inspectClosureCastsCFA(pass, lit, qualFuncName, strconv.Itoa(closureIdx), excCfg, bl, checkUBV, ubvMode, cfgBackend, cfgMaxStates, cfgMaxDepth)
		},
	)

	// Collect closure classifications lazily — only needed when assigned casts exist.
	// Path validation includes deferred closures + IIFEs; UBV ordering uses only IIFEs.
	var pathSyncLits map[*ast.FuncLit]bool
	var ubvSyncLits map[*ast.FuncLit]bool
	var pathSyncCalls closureVarCallSet
	var ubvSyncCalls closureVarCallSet
	var pathMethodCalls methodValueValidateCallSet
	var ubvMethodCalls methodValueValidateCallSet
	if len(assignedCasts) > 0 {
		pathSyncLits = collectSynchronousClosureLits(fn.Body)
		pathSyncCalls = collectSynchronousClosureVarCalls(closureCalls)
		pathMethodCalls = mergeMethodValueValidateCallSets(
			collectMethodValueValidateCalls(pass, fn.Body),
			collectFirstArgValidatedCalls(pass, fn.Body, nil),
		)
		if checkUBV {
			ubvSyncLits = collectUBVClosureLits(fn.Body)
			ubvSyncCalls = collectUBVClosureVarCalls(closureCalls)
			ubvMethodCalls = pathMethodCalls
		}
	}

	// Report assigned casts where an unvalidated path to return exists.
	for _, ac := range assignedCasts {
		excKey := qualFuncName + ".cast-validation"
		if excCfg.isExcepted(excKey) {
			continue
		}

		// Check inline //goplint:ignore directive on the cast statement.
		if hasIgnoreAtPos(pass, ac.pos.Pos()) {
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
		if !hasPathToReturnWithoutValidate(pass, funcCFG, defBlock, defIdx, ac.target, pathSyncLits, pathSyncCalls, pathMethodCalls, noReturnAliases) {
			// All paths DO have validate. Check use-before-validate with
			// same-block priority, then cross-block.
			if checkUBV {
				if hasUseBeforeValidateInBlockMode(pass, defBlock.Nodes, defIdx+1, ac.target, ubvSyncLits, ubvSyncCalls, ubvMethodCalls, ubvMode) {
					ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, false)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateSameBlock,
						"cfa",
						qualFuncName,
						ac.typeName,
						"ubv",
						stablePosKey(pass, ac.pos.Pos()),
						ac.target.key(),
					)
					reportFindingWithMetaIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUseBeforeValidateSameBlock, ubvID, ubvMsg, map[string]string{
						"ubv_mode":         ubvMode,
						"ubv_scope":        "same-block",
						"witness_cast_pos": stablePosKey(pass, ac.pos.Pos()),
						"witness_def_block": strconv.FormatInt(
							int64(defBlock.Index),
							10,
						),
					})
				} else if hasUseBeforeValidateCrossBlockMode(pass, defBlock, defIdx, ac.target, ubvSyncLits, ubvSyncCalls, ubvMethodCalls, ubvMode, cfgMaxStates, cfgMaxDepth) {
					// Cross-block UBV: the variable is used in a successor
					// block before any block on that path calls Validate().
					ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, true)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateCrossBlock,
						"cfa",
						qualFuncName,
						ac.typeName,
						"ubv-xblock",
						stablePosKey(pass, ac.pos.Pos()),
						ac.target.key(),
					)
					reportFindingWithMetaIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUseBeforeValidateCrossBlock, ubvID, ubvMsg, map[string]string{
						"ubv_mode":         ubvMode,
						"ubv_scope":        "cross-block",
						"witness_cast_pos": stablePosKey(pass, ac.pos.Pos()),
						"witness_def_block": strconv.FormatInt(
							int64(defBlock.Index),
							10,
						),
					})
				}
			}
			continue // all paths validated
		}

		msg := unvalidatedCastMessage(ac.typeName)
		findingID := PackageScopedFindingID(pass,
			CategoryUnvalidatedCast,
			"cfa",
			qualFuncName,
			ac.typeName,
			"assigned",
			stablePosKey(pass, ac.pos.Pos()),
			ac.target.key(),
		)
		reportFindingIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}

	// Unassigned casts: always report (no variable to track).
	for _, uc := range unassignedCasts {
		excKey := qualFuncName + ".cast-validation"
		if excCfg.isExcepted(excKey) {
			continue
		}

		// Check inline //goplint:ignore directive on the cast expression.
		if hasIgnoreAtPos(pass, uc.pos.Pos()) {
			continue
		}

		msg := unvalidatedCastMessage(uc.typeName)
		findingID := PackageScopedFindingID(pass,
			CategoryUnvalidatedCast,
			"cfa",
			qualFuncName,
			uc.typeName,
			"unassigned",
			stablePosKey(pass, uc.pos.Pos()),
		)
		reportFindingIfNotBaselined(pass, bl, uc.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}
}
