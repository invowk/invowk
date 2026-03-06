// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/token"

	"golang.org/x/tools/go/analysis"
	gocfg "golang.org/x/tools/go/cfg"
)

type cfgRefinementOverride struct {
	MaxStates           int
	MaxDepth            int
	DischargedWitnesses map[string]bool
	AllowSafe           bool
}

type cfgRefinementRequest struct {
	Pass          *analysis.Pass
	Position      token.Pos
	CFG           *gocfg.CFG
	Result        interprocPathResult
	Category      string
	FindingID     string
	CallChain     []string
	OriginAnchors map[string]string
	SyntheticPath []int32
	Rerun         func(cfgRefinementOverride) interprocPathResult
}

type cfgRefinementController struct {
	options cfgPhaseCOptions
	cache   *cfgRefinementCache
}

func newCFGRefinementController(options cfgPhaseCOptions) cfgRefinementController {
	return cfgRefinementController{
		options: options,
		cache:   newCFGRefinementCache(),
	}
}

func (c cfgRefinementController) Refine(request cfgRefinementRequest) interprocPathResult {
	result := request.Result
	if !c.options.Enabled() {
		return result
	}

	record := buildCFGWitnessRecord(
		request.Category,
		request.FindingID,
		request.OriginAnchors,
		result,
		request.CallChain,
		request.SyntheticPath,
	)
	backend := feasibilityBackendForEngine(c.options.FeasibilityEngine)
	if backend == nil {
		return result
	}

	queryBudget := c.options.QueryBudget()
	if queryBudget <= 0 {
		queryBudget = defaultCFGFeasibilityMaxQueries
	}
	iterations := 0
	queries := 0
	discharged := map[string]bool{}
	verdict, reason := c.checkFeasibility(backend, request.Pass, request.Position, request.CFG, record)
	queries++

	for iterations < c.options.IterationBudget() && queries < queryBudget {
		if !shouldAttemptRefinement(result, verdict) {
			break
		}
		if record.WitnessHash == "" || !c.cache.record(record.WitnessHash) {
			break
		}
		if verdict == cfgFeasibilityResultUNSAT {
			discharged[record.WitnessHash] = true
		}
		override := cfgRefinementOverride{
			MaxStates:           refinementMaxStatesForTrigger(result.Reason),
			MaxDepth:            refinementMaxDepthForTrigger(result.Reason),
			DischargedWitnesses: discharged,
			AllowSafe:           c.options.AllowsSafeResult(),
		}
		next := request.Rerun(override)
		iterations++
		if next.Class == interprocOutcomeSafe {
			next.PhaseC = cfgPhaseCResult{
				Enabled:              true,
				FeasibilityEngine:    c.options.FeasibilityEngine,
				FeasibilityResult:    verdict,
				FeasibilityReason:    reason,
				RefinementStatus:     cfgRefinementStatusProvenSafe,
				RefinementIterations: iterations,
				RefinementTrigger:    record.TriggerReason,
				WitnessHash:          record.WitnessHash,
			}
			next.WitnessRecord = record
			return next
		}
		result = next
		record = buildCFGWitnessRecord(
			request.Category,
			request.FindingID,
			request.OriginAnchors,
			result,
			request.CallChain,
			request.SyntheticPath,
		)
		verdict, reason = c.checkFeasibility(backend, request.Pass, request.Position, request.CFG, record)
		queries++
	}

	status := cfgRefinementStatusUnsafe
	if result.Class == interprocOutcomeInconclusive {
		if iterations > 0 {
			status = cfgRefinementStatusInconclusiveRefined
		} else {
			status = cfgRefinementStatusInconclusiveRaw
		}
	}
	result.PhaseC = cfgPhaseCResult{
		Enabled:              true,
		FeasibilityEngine:    c.options.FeasibilityEngine,
		FeasibilityResult:    verdict,
		FeasibilityReason:    reason,
		RefinementStatus:     status,
		RefinementIterations: iterations,
		RefinementTrigger:    record.TriggerReason,
		WitnessHash:          record.WitnessHash,
	}
	result.WitnessRecord = record
	return result
}

func shouldAttemptRefinement(result interprocPathResult, verdict string) bool {
	if result.Class == interprocOutcomeUnsafe {
		return verdict == cfgFeasibilityResultUNSAT
	}
	if result.Class != interprocOutcomeInconclusive {
		return false
	}
	switch result.Reason {
	case pathOutcomeReasonStateBudget, pathOutcomeReasonDepthBudget, pathOutcomeReasonRecursionCycle, pathOutcomeReasonUnresolvedTarget:
		return true
	default:
		return false
	}
}

func (c cfgRefinementController) checkFeasibility(
	backend cfgFeasibilityBackend,
	pass *analysis.Pass,
	pos token.Pos,
	cfg *gocfg.CFG,
	record cfgWitnessRecord,
) (string, string) {
	if backend == nil {
		return "", cfgFeasibilityReasonNone
	}
	return backend.Check(cfgFeasibilityQuery{
		Pass:     pass,
		CFG:      cfg,
		Witness:  record,
		Timeout:  c.options.FeasibilityTimeout,
		Position: pos,
	})
}

func refinementMaxStatesForTrigger(reason pathOutcomeReason) int {
	switch reason {
	case pathOutcomeReasonStateBudget:
		return defaultCFGMaxStates
	default:
		return 0
	}
}

func refinementMaxDepthForTrigger(reason pathOutcomeReason) int {
	switch reason {
	case pathOutcomeReasonDepthBudget:
		return defaultCFGMaxDepth
	default:
		return 0
	}
}
