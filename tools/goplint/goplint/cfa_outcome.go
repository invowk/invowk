// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"strconv"
	"strings"
)

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

func cfgOutcomeMetaWithWitness(
	backend string,
	maxStates int,
	maxDepth int,
	reason pathOutcomeReason,
	witness []int32,
	maxWitnessSteps int,
) map[string]string {
	meta := cfgOutcomeMeta(backend, maxStates, maxDepth, reason)
	addCFGWitnessMeta(meta, witness, maxWitnessSteps)
	return meta
}

func addCFGWitnessMeta(meta map[string]string, witness []int32, maxWitnessSteps int) {
	if len(meta) == 0 || len(witness) == 0 || maxWitnessSteps == 0 {
		return
	}
	limit := len(witness)
	if maxWitnessSteps > 0 && limit > maxWitnessSteps {
		limit = maxWitnessSteps
	}
	steps := make([]string, 0, limit)
	for _, idx := range witness[:limit] {
		steps = append(steps, strconv.FormatInt(int64(idx), 10))
	}
	meta["witness_cfg_path"] = strings.Join(steps, "->")
	meta["witness_cfg_steps"] = strconv.Itoa(limit)
	if len(witness) > limit {
		meta["witness_cfg_truncated"] = "true"
	}
}
