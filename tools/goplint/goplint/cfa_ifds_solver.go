// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	gocfg "golang.org/x/tools/go/cfg"
)

type interprocSolver struct {
	pass    *analysis.Pass
	backend string
	engine  string
}

func newInterprocSolver(pass *analysis.Pass, backend, engine string) interprocSolver {
	if backend == "" {
		backend = defaultCFGBackend
	}
	return interprocSolver{
		pass:    pass,
		backend: backend,
		engine:  normalizeInterprocEngine(engine),
	}
}

func normalizeInterprocEngine(engine string) string {
	switch engine {
	case cfgInterprocEngineIFDS, cfgInterprocEngineCompare:
		return engine
	default:
		return cfgInterprocEngineLegacy
	}
}

type interprocCastPathInput struct {
	Decl            *ast.FuncDecl
	CFG             *gocfg.CFG
	DefBlock        *gocfg.Block
	DefIdx          int
	Target          castTarget
	TypeName        string
	OriginKey       string
	SyncLits        map[*ast.FuncLit]bool
	SyncCalls       closureVarCallSet
	MethodCalls     methodValueValidateCallSet
	NoReturnAliases noReturnAliasSet
	MaxStates       int
	MaxDepth        int
}

type interprocUBVInBlockInput struct {
	Target      castTarget
	Nodes       []ast.Node
	StartIndex  int
	Mode        string
	OriginKey   string
	TypeName    string
	SyncLits    map[*ast.FuncLit]bool
	SyncCalls   closureVarCallSet
	MethodCalls methodValueValidateCallSet
}

type interprocUBVCrossBlockInput struct {
	Target      castTarget
	DefBlock    *gocfg.Block
	DefIdx      int
	Mode        string
	OriginKey   string
	TypeName    string
	SyncLits    map[*ast.FuncLit]bool
	SyncCalls   closureVarCallSet
	MethodCalls methodValueValidateCallSet
	MaxStates   int
	MaxDepth    int
}

type interprocConstructorPathInput struct {
	Decl              *ast.FuncDecl
	ReturnTypeKey     string
	ReturnTypePkgPath string
	Constructor       string
	ReturnType        string
	MaxStates         int
	MaxDepth          int
}

func (s interprocSolver) EvaluateCastPath(input interprocCastPathInput) interprocPathResult {
	legacy := s.evaluateCastPathLegacy(input)
	if s.engine != cfgInterprocEngineIFDS && s.engine != cfgInterprocEngineCompare {
		return legacy
	}
	ifds := s.evaluateCastPathIFDS(input)
	if s.engine == cfgInterprocEngineCompare {
		// Compare mode reports IFDS semantics while parity checks run in adapters.
		return ifds
	}
	return ifds
}

func (s interprocSolver) EvaluateCastPathLegacy(input interprocCastPathInput) interprocPathResult {
	return s.evaluateCastPathLegacy(input)
}

func (s interprocSolver) EvaluateCastPathIFDS(input interprocCastPathInput) interprocPathResult {
	return s.evaluateCastPathIFDS(input)
}

func (s interprocSolver) EvaluateUBVInBlock(input interprocUBVInBlockInput) interprocPathResult {
	legacy := s.evaluateUBVInBlockLegacy(input)
	if s.engine != cfgInterprocEngineIFDS && s.engine != cfgInterprocEngineCompare {
		return legacy
	}
	ifds := s.evaluateUBVInBlockIFDS(input)
	if s.engine == cfgInterprocEngineCompare {
		return ifds
	}
	return ifds
}

func (s interprocSolver) EvaluateUBVInBlockLegacy(input interprocUBVInBlockInput) interprocPathResult {
	return s.evaluateUBVInBlockLegacy(input)
}

func (s interprocSolver) EvaluateUBVInBlockIFDS(input interprocUBVInBlockInput) interprocPathResult {
	return s.evaluateUBVInBlockIFDS(input)
}

func (s interprocSolver) EvaluateUBVCrossBlock(input interprocUBVCrossBlockInput) interprocPathResult {
	legacy := s.evaluateUBVCrossBlockLegacy(input)
	if s.engine != cfgInterprocEngineIFDS && s.engine != cfgInterprocEngineCompare {
		return legacy
	}
	ifds := s.evaluateUBVCrossBlockIFDS(input)
	if s.engine == cfgInterprocEngineCompare {
		return ifds
	}
	return ifds
}

func (s interprocSolver) EvaluateUBVCrossBlockLegacy(input interprocUBVCrossBlockInput) interprocPathResult {
	return s.evaluateUBVCrossBlockLegacy(input)
}

func (s interprocSolver) EvaluateUBVCrossBlockIFDS(input interprocUBVCrossBlockInput) interprocPathResult {
	return s.evaluateUBVCrossBlockIFDS(input)
}

func (s interprocSolver) EvaluateConstructorPath(input interprocConstructorPathInput) interprocPathResult {
	legacy := s.evaluateConstructorPathLegacy(input)
	if s.engine != cfgInterprocEngineIFDS && s.engine != cfgInterprocEngineCompare {
		return legacy
	}
	ifds := s.evaluateConstructorPathIFDS(input)
	if s.engine == cfgInterprocEngineCompare {
		return ifds
	}
	return ifds
}

func (s interprocSolver) EvaluateConstructorPathLegacy(input interprocConstructorPathInput) interprocPathResult {
	return s.evaluateConstructorPathLegacy(input)
}

func (s interprocSolver) EvaluateConstructorPathIFDS(input interprocConstructorPathInput) interprocPathResult {
	return s.evaluateConstructorPathIFDS(input)
}

func (s interprocSolver) String() string {
	return fmt.Sprintf("interproc-solver(engine=%s, backend=%s)", s.engine, s.backend)
}

func (s interprocSolver) evaluateCastPathLegacy(input interprocCastPathInput) interprocPathResult {
	outcome, reason, witness := hasPathToReturnWithoutValidateOutcomeWithWitness(
		s.pass,
		input.CFG,
		input.DefBlock,
		input.DefIdx,
		input.Target,
		input.SyncLits,
		input.SyncCalls,
		input.MethodCalls,
		input.NoReturnAliases,
		input.MaxStates,
		input.MaxDepth,
	)
	result := interprocPathResultFromOutcome(outcome, reason, witness)
	fact := ifdsCastNeedsValidateFact{
		OriginKey: input.OriginKey,
		TargetKey: input.Target.key(),
		TypeKey:   input.TypeName,
	}
	result.FactFamily = fact.Family()
	result.FactKey = fact.Key()
	result.EdgeFunctionTag = edgeTagFromPathResult(result, ubvModeOrder)
	return result
}

func (s interprocSolver) evaluateCastPathIFDS(input interprocCastPathInput) interprocPathResult {
	_ = buildInterprocSupergraphForFunc(s.pass, input.Decl, s.backend)
	return s.evaluateCastPathLegacy(input)
}

func (s interprocSolver) evaluateUBVInBlockLegacy(input interprocUBVInBlockInput) interprocPathResult {
	outcome, reason := hasUseBeforeValidateInBlockOutcomeModeWithSummaryStack(
		s.pass,
		input.Nodes,
		input.StartIndex,
		input.Target,
		input.SyncLits,
		input.SyncCalls,
		input.MethodCalls,
		input.Mode,
		nil,
	)
	result := interprocPathResultFromOutcome(outcome, reason, nil)
	fact := ifdsUBVNeedsValidateBeforeUseFact{
		OriginKey: input.OriginKey,
		TargetKey: input.Target.key(),
		TypeKey:   input.TypeName,
		Mode:      input.Mode,
	}
	result.FactFamily = fact.Family()
	result.FactKey = fact.Key()
	result.EdgeFunctionTag = edgeTagFromPathResult(result, input.Mode)
	return result
}

func (s interprocSolver) evaluateUBVInBlockIFDS(input interprocUBVInBlockInput) interprocPathResult {
	return s.evaluateUBVInBlockLegacy(input)
}

func (s interprocSolver) evaluateUBVCrossBlockLegacy(input interprocUBVCrossBlockInput) interprocPathResult {
	outcome, reason, witness := hasUseBeforeValidateCrossBlockOutcomeModeWithWitness(
		s.pass,
		input.DefBlock,
		input.DefIdx,
		input.Target,
		input.SyncLits,
		input.SyncCalls,
		input.MethodCalls,
		input.Mode,
		input.MaxStates,
		input.MaxDepth,
	)
	result := interprocPathResultFromOutcome(outcome, reason, witness)
	fact := ifdsUBVNeedsValidateBeforeUseFact{
		OriginKey: input.OriginKey,
		TargetKey: input.Target.key(),
		TypeKey:   input.TypeName,
		Mode:      input.Mode,
	}
	result.FactFamily = fact.Family()
	result.FactKey = fact.Key()
	result.EdgeFunctionTag = edgeTagFromPathResult(result, input.Mode)
	return result
}

func (s interprocSolver) evaluateUBVCrossBlockIFDS(input interprocUBVCrossBlockInput) interprocPathResult {
	return s.evaluateUBVCrossBlockLegacy(input)
}

func (s interprocSolver) evaluateConstructorPathLegacy(input interprocConstructorPathInput) interprocPathResult {
	outcome, reason, witness := constructorReturnPathOutcomeWithWitness(
		s.pass,
		input.Decl,
		input.ReturnType,
		input.ReturnTypePkgPath,
		input.ReturnTypeKey,
		s.backend,
		input.MaxStates,
		input.MaxDepth,
	)
	result := interprocPathResultFromOutcome(outcome, reason, witness)
	fact := ifdsCtorReturnNeedsValidateFact{
		ConstructorKey: input.Constructor,
		ReturnTypeKey:  input.ReturnTypeKey,
	}
	result.FactFamily = fact.Family()
	result.FactKey = fact.Key()
	result.EdgeFunctionTag = edgeTagFromPathResult(result, ubvModeOrder)
	return result
}

func (s interprocSolver) evaluateConstructorPathIFDS(input interprocConstructorPathInput) interprocPathResult {
	_ = buildInterprocSupergraphForFunc(s.pass, input.Decl, s.backend)
	return s.evaluateConstructorPathLegacy(input)
}

func edgeTagFromPathResult(result interprocPathResult, ubvMode string) ideEdgeFuncTag {
	switch result.Class {
	case interprocOutcomeSafe:
		return ideEdgeFuncValidate
	case interprocOutcomeUnsafe:
		if ubvMode == ubvModeEscape {
			return ideEdgeFuncEscape
		}
		if result.FactFamily == ifdsFactFamilyUBVNeedsValidateBefore {
			return ideEdgeFuncConsume
		}
		return ideEdgeFuncIdentity
	default:
		return ideEdgeFuncIdentity
	}
}
