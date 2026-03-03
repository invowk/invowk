// SPDX-License-Identifier: MPL-2.0

package goplint

import "strconv"

type pathOutcome int

const (
	pathOutcomeSafe pathOutcome = iota
	pathOutcomeUnsafe
	pathOutcomeInconclusive
)

type pathOutcomeReason string

const (
	pathOutcomeReasonNone             pathOutcomeReason = ""
	pathOutcomeReasonStateBudget      pathOutcomeReason = "state-budget"
	pathOutcomeReasonDepthBudget      pathOutcomeReason = "depth-budget"
	pathOutcomeReasonRecursionCycle   pathOutcomeReason = "recursion-cycle"
	pathOutcomeReasonUnresolvedTarget pathOutcomeReason = "unresolved-target"
)

func cfgOutcomeMeta(
	backend string,
	maxStates int,
	maxDepth int,
	reason pathOutcomeReason,
) map[string]string {
	meta := map[string]string{
		"cfg_backend":       backend,
		"cfg_budget_states": strconv.Itoa(maxStates),
		"cfg_budget_depth":  strconv.Itoa(maxDepth),
	}
	if reason != pathOutcomeReasonNone {
		meta["cfg_inconclusive_reason"] = string(reason)
	}
	return meta
}

