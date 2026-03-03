// SPDX-License-Identifier: MPL-2.0

package goplint

import (
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
	checkUBV bool,
	ubvMode string,
	cfgBackend string,
	cfgMaxStates int,
	cfgMaxDepth int,
	cfgInconclusivePolicy string,
	cfgWitnessMaxSteps int,
) {
	if lit.Body == nil {
		return
	}

	// Build CFG for this closure's body.
	closureCFG := buildFuncCFGForBackend(pass, lit.Body, cfgBackend)
	if closureCFG == nil {
		return
	}
	noReturnAliases := collectNoReturnFuncAliasEvents(pass, lit.Body)

	parentMap := buildParentMap(lit.Body)

	// Collect casts using the shared CFA collection logic.
	// Nested closures are analyzed recursively with compound prefixes.
	assignedCasts, unassignedCasts, closureCalls, _ := collectCFACasts(
		pass, lit.Body, parentMap,
		func(nested *ast.FuncLit, nestedIdx int) {
			nestedPrefix := closurePrefix + "/" + strconv.Itoa(nestedIdx)
			inspectClosureCastsCFA(
				pass,
				nested,
				qualEnclosingFunc,
				nestedPrefix,
				excCfg,
				bl,
				checkUBV,
				ubvMode,
				cfgBackend,
				cfgMaxStates,
				cfgMaxDepth,
				cfgInconclusivePolicy,
				cfgWitnessMaxSteps,
			)
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
		pathSyncLits = collectSynchronousClosureLits(lit.Body)
		pathSyncCalls = collectSynchronousClosureVarCalls(closureCalls)
		pathMethodCalls = mergeMethodValueValidateCallSets(
			collectMethodValueValidateCalls(pass, lit.Body),
			collectFirstArgValidatedCalls(pass, lit.Body, nil),
		)
		if checkUBV {
			ubvSyncLits = collectUBVClosureLits(lit.Body)
			ubvSyncCalls = collectUBVClosureVarCalls(closureCalls)
			ubvMethodCalls = pathMethodCalls
		}
	}

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

		pathOutcome, pathReason, pathWitness := hasPathToReturnWithoutValidateOutcomeWithWitness(
			pass,
			closureCFG,
			defBlock,
			defIdx,
			ac.target,
			pathSyncLits,
			pathSyncCalls,
			pathMethodCalls,
			noReturnAliases,
			cfgMaxStates,
			cfgMaxDepth,
		)
		if pathOutcome == pathOutcomeSafe {
			// All paths validated. Check use-before-validate with
			// same-block priority, then cross-block.
			if checkUBV {
				inBlockOutcome, inBlockReason := hasUseBeforeValidateInBlockOutcomeModeWithSummaryStack(
					pass,
					defBlock.Nodes,
					defIdx+1,
					ac.target,
					ubvSyncLits,
					ubvSyncCalls,
					ubvMethodCalls,
					ubvMode,
					nil,
				)
				if inBlockOutcome == pathOutcomeUnsafe {
					ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, false)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateSameBlock,
						"cfa",
						"closure",
						closurePrefix,
						qualEnclosingFunc,
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
				} else if inBlockOutcome == pathOutcomeInconclusive {
					ubvMsg := useBeforeValidateInconclusiveMessage(ac.target.displayName, ac.typeName)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateInconclusive,
						"cfa",
						"closure",
						closurePrefix,
						qualEnclosingFunc,
						ac.typeName,
						"ubv-inconclusive",
						"same-block",
						stablePosKey(pass, ac.pos.Pos()),
						ac.target.key(),
						string(inBlockReason),
					)
					meta := cfgOutcomeMetaWithWitness(
						cfgBackend,
						cfgMaxStates,
						cfgMaxDepth,
						inBlockReason,
						[]int32{defBlock.Index},
						cfgWitnessMaxSteps,
					)
					meta["ubv_mode"] = ubvMode
					meta["ubv_scope"] = "same-block"
					meta["witness_cast_pos"] = stablePosKey(pass, ac.pos.Pos())
					meta["witness_def_block"] = strconv.FormatInt(int64(defBlock.Index), 10)
					reportInconclusiveFindingWithMetaIfNotBaselined(
						pass,
						bl,
						cfgInconclusivePolicy,
						ac.pos.Pos(),
						CategoryUseBeforeValidateInconclusive,
						ubvID,
						ubvMsg,
						meta,
					)
				} else if ubvOutcome, ubvReason, ubvWitness := hasUseBeforeValidateCrossBlockOutcomeModeWithWitness(
					pass,
					defBlock,
					defIdx,
					ac.target,
					ubvSyncLits,
					ubvSyncCalls,
					ubvMethodCalls,
					ubvMode,
					cfgMaxStates,
					cfgMaxDepth,
				); ubvOutcome == pathOutcomeUnsafe {
					ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, true)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateCrossBlock,
						"cfa",
						"closure",
						closurePrefix,
						qualEnclosingFunc,
						ac.typeName,
						"ubv-xblock",
						stablePosKey(pass, ac.pos.Pos()),
						ac.target.key(),
					)
					meta := map[string]string{
						"ubv_mode":         ubvMode,
						"ubv_scope":        "cross-block",
						"witness_cast_pos": stablePosKey(pass, ac.pos.Pos()),
						"witness_def_block": strconv.FormatInt(
							int64(defBlock.Index),
							10,
						),
					}
					addCFGWitnessMeta(meta, ubvWitness, cfgWitnessMaxSteps)
					reportFindingWithMetaIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUseBeforeValidateCrossBlock, ubvID, ubvMsg, meta)
				} else if ubvOutcome == pathOutcomeInconclusive {
					ubvMsg := useBeforeValidateInconclusiveMessage(ac.target.displayName, ac.typeName)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateInconclusive,
						"cfa",
						"closure",
						closurePrefix,
						qualEnclosingFunc,
						ac.typeName,
						"ubv-inconclusive",
						"cross-block",
						stablePosKey(pass, ac.pos.Pos()),
						ac.target.key(),
						string(ubvReason),
					)
					meta := cfgOutcomeMetaWithWitness(cfgBackend, cfgMaxStates, cfgMaxDepth, ubvReason, ubvWitness, cfgWitnessMaxSteps)
					meta["ubv_mode"] = ubvMode
					meta["ubv_scope"] = "cross-block"
					meta["witness_cast_pos"] = stablePosKey(pass, ac.pos.Pos())
					meta["witness_def_block"] = strconv.FormatInt(int64(defBlock.Index), 10)
					reportInconclusiveFindingWithMetaIfNotBaselined(
						pass,
						bl,
						cfgInconclusivePolicy,
						ac.pos.Pos(),
						CategoryUseBeforeValidateInconclusive,
						ubvID,
						ubvMsg,
						meta,
					)
				}
			}
			continue
		}
		if pathOutcome == pathOutcomeInconclusive {
			msg := unvalidatedCastInconclusiveMessage(ac.typeName)
			findingID := PackageScopedFindingID(pass,
				CategoryUnvalidatedCastInconclusive,
				"cfa",
				"closure",
				closurePrefix,
				qualEnclosingFunc,
				ac.typeName,
				"assigned",
				"inconclusive",
				string(pathReason),
				stablePosKey(pass, ac.pos.Pos()),
				ac.target.key(),
			)
			meta := cfgOutcomeMetaWithWitness(cfgBackend, cfgMaxStates, cfgMaxDepth, pathReason, pathWitness, cfgWitnessMaxSteps)
			meta["witness_cast_pos"] = stablePosKey(pass, ac.pos.Pos())
			meta["witness_def_block"] = strconv.FormatInt(int64(defBlock.Index), 10)
			reportInconclusiveFindingWithMetaIfNotBaselined(
				pass,
				bl,
				cfgInconclusivePolicy,
				ac.pos.Pos(),
				CategoryUnvalidatedCastInconclusive,
				findingID,
				msg,
				meta,
			)
			continue
		}

		msg := unvalidatedCastMessage(ac.typeName)
		findingID := PackageScopedFindingID(pass,
			CategoryUnvalidatedCast,
			"cfa",
			"closure",
			closurePrefix,
			qualEnclosingFunc,
			ac.typeName,
			"assigned",
			stablePosKey(pass, ac.pos.Pos()),
			ac.target.key(),
		)
		reportFindingIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
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

		msg := unvalidatedCastMessage(uc.typeName)
		findingID := PackageScopedFindingID(pass,
			CategoryUnvalidatedCast,
			"cfa",
			"closure",
			closurePrefix,
			qualEnclosingFunc,
			uc.typeName,
			"unassigned",
			stablePosKey(pass, uc.pos.Pos()),
		)
		reportFindingIfNotBaselined(pass, bl, uc.pos.Pos(), CategoryUnvalidatedCast, findingID, msg)
	}
}
