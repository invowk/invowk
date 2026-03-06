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
	cfgInterprocEngine string,
	cfgMaxStates int,
	cfgMaxDepth int,
	cfgInconclusivePolicy string,
	cfgWitnessMaxSteps int,
	phaseC cfgPhaseCOptions,
) error {
	if lit.Body == nil {
		return nil
	}

	// Build CFG for this closure's body.
	closureCFG := buildFuncCFGForBackend(pass, lit.Body, cfgBackend)
	if closureCFG == nil {
		return nil
	}
	solver := newInterprocSolver(pass, cfgBackend, cfgInterprocEngine)
	compatTracker := newInterprocCompatTracker(cfgInterprocEngine)
	refiner := newCFGRefinementController(phaseC)
	effectiveBudget := adaptiveBlockVisitBudget(
		closureCFG,
		blockVisitBudget{maxStates: cfgMaxStates, maxDepth: cfgMaxDepth},
	)
	noReturnAliases := collectNoReturnFuncAliasEvents(pass, lit.Body)

	parentMap := buildParentMap(lit.Body)

	// Collect casts using the shared CFA collection logic.
	// Nested closures are analyzed recursively with compound prefixes.
	var nestedErr error
	assignedCasts, unassignedCasts, closureCalls, _ := collectCFACasts(
		pass, lit.Body, parentMap,
		func(nested *ast.FuncLit, nestedIdx int) {
			if nestedErr != nil {
				return
			}
			nestedPrefix := closurePrefix + "/" + strconv.Itoa(nestedIdx)
			nestedErr = inspectClosureCastsCFA(
				pass,
				nested,
				qualEnclosingFunc,
				nestedPrefix,
				excCfg,
				bl,
				checkUBV,
				ubvMode,
				cfgBackend,
				cfgInterprocEngine,
				cfgMaxStates,
				cfgMaxDepth,
				cfgInconclusivePolicy,
				cfgWitnessMaxSteps,
				phaseC,
			)
		},
	)
	if nestedErr != nil {
		return nestedErr
	}

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
			collectCalleeValidatedCalls(pass, lit.Body, stackScopeFromMap(nil)),
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

		originKey := stablePosKey(pass, ac.pos.Pos())
		callChain := []string{qualEnclosingFunc, "closure:" + closurePrefix}
		pathAnchors := map[string]string{
			"witness_cast_pos": originKey,
			"witness_def_block": strconv.FormatInt(
				int64(defBlock.Index),
				10,
			),
		}
		castInput := interprocCastPathInput{
			CFG:             closureCFG,
			DefBlock:        defBlock,
			DefIdx:          defIdx,
			Target:          ac.target,
			TypeName:        ac.typeName,
			OriginKey:       originKey,
			SyncLits:        pathSyncLits,
			SyncCalls:       pathSyncCalls,
			MethodCalls:     pathMethodCalls,
			NoReturnAliases: noReturnAliases,
			MaxStates:       effectiveBudget.maxStates,
			MaxDepth:        effectiveBudget.maxDepth,
			CallChain:       callChain,
			AllowSafe:       phaseC.AllowsSafeResult(),
		}
		pathLegacy := solver.EvaluateCastPathLegacy(castInput)
		pathResult := solver.EvaluateCastPath(castInput)
		pathFindingID := PackageScopedFindingID(
			pass,
			CategoryUnvalidatedCast,
			"cfa",
			"closure",
			closurePrefix,
			qualEnclosingFunc,
			ac.typeName,
			"assigned",
			originKey,
			ac.target.key(),
		)
		pathResult = refiner.Refine(cfgRefinementRequest{
			Pass:          pass,
			Position:      ac.pos.Pos(),
			CFG:           closureCFG,
			Result:        pathResult,
			Category:      CategoryUnvalidatedCast,
			FindingID:     pathFindingID,
			CallChain:     callChain,
			OriginAnchors: pathAnchors,
			SyntheticPath: []int32{defBlock.Index},
			Rerun: func(override cfgRefinementOverride) interprocPathResult {
				next := castInput
				if override.MaxStates > 0 {
					next.MaxStates = override.MaxStates
				}
				if override.MaxDepth > 0 {
					next.MaxDepth = override.MaxDepth
				}
				next.DischargedWitnesses = override.DischargedWitnesses
				next.AllowSafe = override.AllowSafe
				if override.ResolveTargets {
					next.ResolveCFGCalls = true
				}
				refined := solver.EvaluateCastPath(next)
				if override.ResolveTargets &&
					refined.Class == interprocOutcomeInconclusive &&
					refined.Reason == pathOutcomeReasonUnresolvedTarget {
					refined = mergeResolvedTargetRefinement(refined, solver.EvaluateCastPathLegacy(next))
				}
				return refined
			},
		})
		writeRefinementTraceToSink(pass, ac.pos.Pos(), pathResult)
		hasEquivalentUnsafe := pathResult.Class == interprocOutcomeUnsafe
		pathOutcome := pathResult.toPathOutcome()
		pathReason := pathResult.Reason
		pathWitness := pathResult.Witness
		if pathOutcome == pathOutcomeSafe {
			// All paths validated. Check use-before-validate with
			// same-block priority, then cross-block.
			if checkUBV {
				inBlockInput := interprocUBVInBlockInput{
					Target:        ac.target,
					Nodes:         defBlock.Nodes,
					StartIndex:    defIdx + 1,
					Mode:          ubvMode,
					OriginKey:     originKey,
					TypeName:      ac.typeName,
					SyncLits:      ubvSyncLits,
					SyncCalls:     ubvSyncCalls,
					MethodCalls:   ubvMethodCalls,
					DefBlockIndex: defBlock.Index,
					CallChain:     callChain,
				}
				inBlockLegacy := solver.EvaluateUBVInBlockLegacy(inBlockInput)
				inBlockResult := solver.EvaluateUBVInBlock(inBlockInput)
				hasEquivalentUnsafe = hasEquivalentUnsafe || inBlockResult.Class == interprocOutcomeUnsafe
				inBlockFindingID := PackageScopedFindingID(
					pass,
					CategoryUseBeforeValidateSameBlock,
					"cfa",
					"closure",
					closurePrefix,
					qualEnclosingFunc,
					ac.typeName,
					"ubv",
					originKey,
					ac.target.key(),
				)
				inBlockResult = refiner.Refine(cfgRefinementRequest{
					Pass:          pass,
					Position:      ac.pos.Pos(),
					CFG:           closureCFG,
					Result:        inBlockResult,
					Category:      CategoryUseBeforeValidateSameBlock,
					FindingID:     inBlockFindingID,
					CallChain:     callChain,
					OriginAnchors: pathAnchors,
					SyntheticPath: []int32{defBlock.Index},
					Rerun: func(override cfgRefinementOverride) interprocPathResult {
						next := inBlockInput
						if override.RefineRecursion {
							next.SummaryStack = summaryStackWithRecursionFallback(next.SummaryStack)
						}
						return solver.EvaluateUBVInBlock(next)
					},
				})
				writeRefinementTraceToSink(pass, ac.pos.Pos(), inBlockResult)
				inBlockOutcome := inBlockResult.toPathOutcome()
				inBlockReason := inBlockResult.Reason
				switch inBlockOutcome {
				case pathOutcomeUnsafe:
					ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, false)
					ubvID := PackageScopedFindingID(pass,
						CategoryUseBeforeValidateSameBlock,
						"cfa",
						"closure",
						closurePrefix,
						qualEnclosingFunc,
						ac.typeName,
						"ubv",
						originKey,
						ac.target.key(),
					)
					meta := appendPhaseCMeta(map[string]string{
						"ubv_mode":         ubvMode,
						"ubv_scope":        "same-block",
						"witness_cast_pos": originKey,
						"witness_def_block": strconv.FormatInt(
							int64(defBlock.Index),
							10,
						),
					}, inBlockResult)
					reportFindingWithMetaIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUseBeforeValidateSameBlock, ubvID, ubvMsg, meta)
				case pathOutcomeInconclusive:
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
						originKey,
						ac.target.key(),
						string(inBlockReason),
					)
					meta := cfgOutcomeMetaWithWitness(
						cfgBackend,
						effectiveBudget.maxStates,
						effectiveBudget.maxDepth,
						inBlockReason,
						[]int32{defBlock.Index},
						cfgWitnessMaxSteps,
					)
					meta["ubv_mode"] = ubvMode
					meta["ubv_scope"] = "same-block"
					meta["witness_cast_pos"] = originKey
					meta["witness_def_block"] = strconv.FormatInt(int64(defBlock.Index), 10)
					addCFGWitnessCallChainMeta(meta, []string{qualEnclosingFunc, "closure:" + closurePrefix}, cfgWitnessMaxSteps)
					meta = appendPhaseCMeta(meta, inBlockResult)
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
				default:
					crossInput := interprocUBVCrossBlockInput{
						Target:      ac.target,
						DefBlock:    defBlock,
						DefIdx:      defIdx,
						Mode:        ubvMode,
						OriginKey:   originKey,
						TypeName:    ac.typeName,
						SyncLits:    ubvSyncLits,
						SyncCalls:   ubvSyncCalls,
						MethodCalls: ubvMethodCalls,
						MaxStates:   effectiveBudget.maxStates,
						MaxDepth:    effectiveBudget.maxDepth,
						CallChain:   callChain,
					}
					crossLegacy := solver.EvaluateUBVCrossBlockLegacy(crossInput)
					crossResult := solver.EvaluateUBVCrossBlock(crossInput)
					hasEquivalentUnsafe = hasEquivalentUnsafe || crossResult.Class == interprocOutcomeUnsafe
					crossFindingID := PackageScopedFindingID(
						pass,
						CategoryUseBeforeValidateCrossBlock,
						"cfa",
						"closure",
						closurePrefix,
						qualEnclosingFunc,
						ac.typeName,
						"ubv-xblock",
						originKey,
						ac.target.key(),
					)
					crossResult = refiner.Refine(cfgRefinementRequest{
						Pass:          pass,
						Position:      ac.pos.Pos(),
						CFG:           closureCFG,
						Result:        crossResult,
						Category:      CategoryUseBeforeValidateCrossBlock,
						FindingID:     crossFindingID,
						CallChain:     callChain,
						OriginAnchors: pathAnchors,
						SyntheticPath: []int32{defBlock.Index},
						Rerun: func(override cfgRefinementOverride) interprocPathResult {
							next := crossInput
							if override.MaxStates > 0 {
								next.MaxStates = override.MaxStates
							}
							if override.MaxDepth > 0 {
								next.MaxDepth = override.MaxDepth
							}
							next.DischargedWitnesses = override.DischargedWitnesses
							if override.ResolveTargets {
								next.ResolveCFGCalls = true
							}
							if override.RefineRecursion {
								next.SummaryStack = summaryStackWithRecursionFallback(next.SummaryStack)
							}
							refined := solver.EvaluateUBVCrossBlock(next)
							if override.ResolveTargets &&
								refined.Class == interprocOutcomeInconclusive &&
								refined.Reason == pathOutcomeReasonUnresolvedTarget {
								refined = mergeResolvedTargetRefinement(refined, solver.EvaluateUBVCrossBlockLegacy(next))
							}
							return refined
						},
					})
					writeRefinementTraceToSink(pass, ac.pos.Pos(), crossResult)
					compatTracker.Check(
						CategoryUseBeforeValidateCrossBlock,
						crossFindingID,
						crossLegacy,
						crossResult,
						hasEquivalentUnsafe,
					)
					ubvOutcome := crossResult.toPathOutcome()
					ubvReason := crossResult.Reason
					ubvWitness := crossResult.Witness
					if ubvOutcome == pathOutcomeUnsafe {
						ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, true)
						ubvID := PackageScopedFindingID(pass,
							CategoryUseBeforeValidateCrossBlock,
							"cfa",
							"closure",
							closurePrefix,
							qualEnclosingFunc,
							ac.typeName,
							"ubv-xblock",
							originKey,
							ac.target.key(),
						)
						meta := map[string]string{
							"ubv_mode":         ubvMode,
							"ubv_scope":        "cross-block",
							"witness_cast_pos": originKey,
							"witness_def_block": strconv.FormatInt(
								int64(defBlock.Index),
								10,
							),
						}
						addCFGWitnessMeta(meta, ubvWitness, cfgWitnessMaxSteps)
						addCFGWitnessCallChainMeta(meta, []string{qualEnclosingFunc, "closure:" + closurePrefix}, cfgWitnessMaxSteps)
						meta = appendPhaseCMeta(meta, crossResult)
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
							originKey,
							ac.target.key(),
							string(ubvReason),
						)
						meta := cfgOutcomeMetaWithWitness(cfgBackend, effectiveBudget.maxStates, effectiveBudget.maxDepth, ubvReason, ubvWitness, cfgWitnessMaxSteps)
						meta["ubv_mode"] = ubvMode
						meta["ubv_scope"] = "cross-block"
						meta["witness_cast_pos"] = originKey
						meta["witness_def_block"] = strconv.FormatInt(int64(defBlock.Index), 10)
						addCFGWitnessCallChainMeta(meta, []string{qualEnclosingFunc, "closure:" + closurePrefix}, cfgWitnessMaxSteps)
						meta = appendPhaseCMeta(meta, crossResult)
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
				compatTracker.Check(
					CategoryUseBeforeValidateSameBlock,
					inBlockFindingID,
					inBlockLegacy,
					inBlockResult,
					hasEquivalentUnsafe,
				)
			}
			compatTracker.Check(
				CategoryUnvalidatedCast,
				pathFindingID,
				pathLegacy,
				pathResult,
				hasEquivalentUnsafe,
			)
			continue
		}
		compatTracker.Check(
			CategoryUnvalidatedCast,
			pathFindingID,
			pathLegacy,
			pathResult,
			hasEquivalentUnsafe,
		)
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
				originKey,
				ac.target.key(),
			)
			meta := cfgOutcomeMetaWithWitness(cfgBackend, effectiveBudget.maxStates, effectiveBudget.maxDepth, pathReason, pathWitness, cfgWitnessMaxSteps)
			meta["witness_cast_pos"] = originKey
			meta["witness_def_block"] = strconv.FormatInt(int64(defBlock.Index), 10)
			addCFGWitnessCallChainMeta(meta, []string{qualEnclosingFunc, "closure:" + closurePrefix}, cfgWitnessMaxSteps)
			meta = appendPhaseCMeta(meta, pathResult)
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
			originKey,
			ac.target.key(),
		)
		var meta map[string]string
		if pathResult.PhaseC.Enabled {
			meta = appendPhaseCMeta(copyFindingMeta(pathAnchors), pathResult)
			addCFGWitnessMeta(meta, pathWitness, cfgWitnessMaxSteps)
			addCFGWitnessCallChainMeta(meta, []string{qualEnclosingFunc, "closure:" + closurePrefix}, cfgWitnessMaxSteps)
		}
		reportFindingWithMetaIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUnvalidatedCast, findingID, msg, meta)
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

	return compatTracker.Err()
}
