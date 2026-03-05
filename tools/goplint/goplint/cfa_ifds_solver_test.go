// SPDX-License-Identifier: MPL-2.0

package goplint

import "testing"

func TestNormalizeInterprocEngine(t *testing.T) {
	t.Parallel()

	if got := normalizeInterprocEngine(cfgInterprocEngineIFDS); got != cfgInterprocEngineIFDS {
		t.Fatalf("normalizeInterprocEngine(ifds) = %q, want %q", got, cfgInterprocEngineIFDS)
	}
	if got := normalizeInterprocEngine("unknown"); got != cfgInterprocEngineLegacy {
		t.Fatalf("normalizeInterprocEngine(unknown) = %q, want %q", got, cfgInterprocEngineLegacy)
	}
}

func TestInterprocSolverCastPathNilDefinitionIsInconclusive(t *testing.T) {
	t.Parallel()

	legacySolver := newInterprocSolver(nil, cfgBackendSSA, cfgInterprocEngineLegacy)
	ifdsSolver := newInterprocSolver(nil, cfgBackendSSA, cfgInterprocEngineIFDS)
	input := interprocCastPathInput{
		TypeName:  "pkg.Type",
		OriginKey: "cast-pos",
	}

	legacy := legacySolver.EvaluateCastPathLegacy(input)
	if legacy.Class != interprocOutcomeInconclusive {
		t.Fatalf("legacy class = %q, want %q", legacy.Class, interprocOutcomeInconclusive)
	}
	if legacy.Reason != pathOutcomeReasonUnresolvedTarget {
		t.Fatalf("legacy reason = %q, want %q", legacy.Reason, pathOutcomeReasonUnresolvedTarget)
	}

	ifds := ifdsSolver.EvaluateCastPathIFDS(input)
	if ifds.Class != legacy.Class || ifds.Reason != legacy.Reason {
		t.Fatalf("ifds result = (%q,%q), want (%q,%q)", ifds.Class, ifds.Reason, legacy.Class, legacy.Reason)
	}
	if ifds.FactFamily != ifdsFactFamilyCastNeedsValidate {
		t.Fatalf("ifds fact family = %q, want %q", ifds.FactFamily, ifdsFactFamilyCastNeedsValidate)
	}
}

func TestInterprocSolverUBVCrossBlockNilDefinitionIsInconclusive(t *testing.T) {
	t.Parallel()

	solver := newInterprocSolver(nil, cfgBackendSSA, cfgInterprocEngineIFDS)
	result := solver.EvaluateUBVCrossBlock(interprocUBVCrossBlockInput{
		Mode:      ubvModeEscape,
		OriginKey: "cast-pos",
		TypeName:  "pkg.Type",
	})
	if result.Class != interprocOutcomeInconclusive {
		t.Fatalf("class = %q, want %q", result.Class, interprocOutcomeInconclusive)
	}
	if result.Reason != pathOutcomeReasonUnresolvedTarget {
		t.Fatalf("reason = %q, want %q", result.Reason, pathOutcomeReasonUnresolvedTarget)
	}
}
