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
	cfgInterprocEngine string,
	cfgMaxStates int,
	cfgMaxDepth int,
	cfgInconclusivePolicy string,
	cfgWitnessMaxSteps int,
	phaseC cfgPhaseCOptions,
	cfgAliasMode string,
	ssaRes *ssaResult,
) error {
	if fn.Body == nil {
		return nil
	}
	if shouldSkipFunc(fn) {
		return nil
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
		return nil
	}
	solver := newInterprocSolver(pass, cfgBackend, cfgInterprocEngine)
	compatTracker := newInterprocCompatTracker(cfgInterprocEngine)
	refiner := newCFGRefinementController(phaseC)
	effectiveBudget := adaptiveBlockVisitBudget(
		funcCFG,
		blockVisitBudget{maxStates: cfgMaxStates, maxDepth: cfgMaxDepth},
	)
	noReturnAliases := collectNoReturnFuncAliasEvents(pass, fn.Body)

	// Collect casts using the shared CFA collection logic.
	// Closures are delegated to inspectClosureCastsCFA.
	var closureErr error
	assignedCasts, unassignedCasts, closureCalls, _ := collectCFACasts(
		pass, fn.Body, parentMap,
		func(lit *ast.FuncLit, closureIdx int) {
			if closureErr != nil {
				return
			}
			closureErr = inspectClosureCastsCFA(
				pass,
				lit,
				qualFuncName,
				strconv.Itoa(closureIdx),
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
				cfgAliasMode,
				ssaRes,
			)
		},
	)
	if closureErr != nil {
		return closureErr
	}

	// Enrich cast targets with SSA-derived alias sets when alias mode is active.
	if cfgAliasMode == cfgAliasModeSSA {
		enrichAssignedCastsWithSSA(pass, ssaRes, fn, assignedCasts)
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
		pathSyncLits = collectSynchronousClosureLits(fn.Body)
		pathSyncCalls = collectSynchronousClosureVarCalls(closureCalls)
		pathMethodCalls = mergeMethodValueValidateCallSets(
			collectMethodValueValidateCalls(pass, fn.Body),
			collectCalleeValidatedCalls(pass, fn.Body, stackScopeFromMap(nil)),
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

		originKey := stablePosKey(pass, ac.pos.Pos())
		callChain := []string{qualFuncName}
		pathAnchors := map[string]string{
			"witness_cast_pos": originKey,
			"witness_def_block": strconv.FormatInt(
				int64(defBlock.Index),
				10,
			),
		}
		castInput := interprocCastPathInput{
			Decl:            fn,
			CFG:             funcCFG,
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
			qualFuncName,
			ac.typeName,
			"assigned",
			originKey,
			ac.target.key(),
		)
		pathResult = refiner.Refine(cfgRefinementRequest{
			Pass:          pass,
			Position:      ac.pos.Pos(),
			CFG:           funcCFG,
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
			// All paths DO have validate. Check use-before-validate with
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
					qualFuncName,
					ac.typeName,
					"ubv",
					originKey,
					ac.target.key(),
				)
				inBlockResult = refiner.Refine(cfgRefinementRequest{
					Pass:          pass,
					Position:      ac.pos.Pos(),
					CFG:           funcCFG,
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
						qualFuncName,
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
						qualFuncName,
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
					addCFGWitnessCallChainMeta(meta, []string{qualFuncName}, cfgWitnessMaxSteps)
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
						qualFuncName,
						ac.typeName,
						"ubv-xblock",
						originKey,
						ac.target.key(),
					)
					crossResult = refiner.Refine(cfgRefinementRequest{
						Pass:          pass,
						Position:      ac.pos.Pos(),
						CFG:           funcCFG,
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
						// Cross-block UBV: the variable is used in a successor
						// block before any block on that path calls Validate().
						ubvMsg := useBeforeValidateMessage(ac.target.displayName, ac.typeName, true)
						ubvID := PackageScopedFindingID(pass,
							CategoryUseBeforeValidateCrossBlock,
							"cfa",
							qualFuncName,
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
						addCFGWitnessCallChainMeta(meta, []string{qualFuncName}, cfgWitnessMaxSteps)
						meta = appendPhaseCMeta(meta, crossResult)
						reportFindingWithMetaIfNotBaselined(
							pass,
							bl,
							ac.pos.Pos(),
							CategoryUseBeforeValidateCrossBlock,
							ubvID,
							ubvMsg,
							meta,
						)
					} else if ubvOutcome == pathOutcomeInconclusive {
						ubvMsg := useBeforeValidateInconclusiveMessage(ac.target.displayName, ac.typeName)
						ubvID := PackageScopedFindingID(pass,
							CategoryUseBeforeValidateInconclusive,
							"cfa",
							qualFuncName,
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
						addCFGWitnessCallChainMeta(meta, []string{qualFuncName}, cfgWitnessMaxSteps)
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
			continue // all paths validated
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
				qualFuncName,
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
			addCFGWitnessCallChainMeta(meta, []string{qualFuncName}, cfgWitnessMaxSteps)
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
			qualFuncName,
			ac.typeName,
			"assigned",
			originKey,
			ac.target.key(),
		)
		var meta map[string]string
		if pathResult.PhaseC.Enabled {
			meta = appendPhaseCMeta(copyFindingMeta(pathAnchors), pathResult)
			addCFGWitnessMeta(meta, pathWitness, cfgWitnessMaxSteps)
			addCFGWitnessCallChainMeta(meta, []string{qualFuncName}, cfgWitnessMaxSteps)
		}
		reportFindingWithMetaIfNotBaselined(pass, bl, ac.pos.Pos(), CategoryUnvalidatedCast, findingID, msg, meta)
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

	return compatTracker.Err()
}
