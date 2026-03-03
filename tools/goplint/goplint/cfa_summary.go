// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/ast"
	"go/types"
	"sync"

	"golang.org/x/tools/go/analysis"
	gocfg "golang.org/x/tools/go/cfg"
)

// firstArgCallSummary captures interprocedural call behavior relevant to UBV
// escape-mode checks.
type firstArgCallSummary struct {
	AlwaysValidatesFirstArg         bool
	ValidatesBeforeEscapeOfFirstArg bool
}

type firstArgSummaryEntry struct {
	summary firstArgCallSummary
	ok      bool
}

var firstArgSummaryCache sync.Map // map[string]firstArgSummaryEntry keyed by objectKey(func)

func resetFirstArgSummaryCache() {
	firstArgSummaryCache = sync.Map{}
}

func callHasTargetAsFirstArg(pass *analysis.Pass, call *ast.CallExpr, target castTarget) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}
	return target.matchesExpr(pass, call.Args[0])
}

func calledFunctionObject(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	if pass == nil || pass.TypesInfo == nil || call == nil {
		return nil
	}
	switch fun := stripParens(call.Fun).(type) {
	case *ast.Ident:
		fnObj, _ := objectForIdent(pass, fun).(*types.Func)
		return fnObj
	case *ast.SelectorExpr:
		fnObj, _ := objectForIdent(pass, fun.Sel).(*types.Func)
		return fnObj
	default:
		return nil
	}
}

func callFirstArgSummary(pass *analysis.Pass, call *ast.CallExpr) (firstArgCallSummary, bool) {
	return callFirstArgSummaryWithStack(pass, call, nil)
}

func callFirstArgSummaryWithStack(pass *analysis.Pass, call *ast.CallExpr, stack map[string]bool) (firstArgCallSummary, bool) {
	fnObj := calledFunctionObject(pass, call)
	if fnObj == nil {
		return firstArgCallSummary{}, false
	}
	return firstArgSummaryForFunc(pass, fnObj, stack)
}

func firstArgSummaryForFunc(pass *analysis.Pass, fnObj *types.Func, stack map[string]bool) (firstArgCallSummary, bool) {
	if fnObj == nil {
		return firstArgCallSummary{}, false
	}
	key := objectKey(fnObj)
	if key == "" {
		return firstArgCallSummary{}, false
	}
	if cached, ok := firstArgSummaryCache.Load(key); ok {
		entry := cached.(firstArgSummaryEntry)
		return entry.summary, entry.ok
	}
	if stack == nil {
		stack = make(map[string]bool)
	}
	// Cycles are treated conservatively as unknown: do not claim a guarantee.
	if stack[key] {
		return firstArgCallSummary{}, false
	}
	stack[key] = true
	summary, ok := deriveFirstArgSummary(pass, fnObj, stack)
	delete(stack, key)
	firstArgSummaryCache.Store(key, firstArgSummaryEntry{summary: summary, ok: ok})
	return summary, ok
}

func deriveFirstArgSummary(pass *analysis.Pass, fnObj *types.Func, stack map[string]bool) (firstArgCallSummary, bool) {
	fnDecl := findFuncDeclForObject(pass, fnObj)
	if fnDecl == nil || fnDecl.Body == nil {
		return firstArgCallSummary{}, false
	}
	target, ok := functionFirstParamTarget(pass, fnDecl)
	if !ok {
		return firstArgCallSummary{}, false
	}

	parentMap := buildParentMap(fnDecl.Body)
	_, _, closureCalls, methodValueCalls := collectCFACasts(pass, fnDecl.Body, parentMap, func(*ast.FuncLit, int) {})

	pathSyncLits := collectSynchronousClosureLits(fnDecl.Body)
	pathSyncCalls := collectSynchronousClosureVarCalls(closureCalls)
	ubvSyncLits := collectUBVClosureLits(fnDecl.Body)
	ubvSyncCalls := collectUBVClosureVarCalls(closureCalls)

	methodCalls := collectMethodValueValidateCallSet(methodValueCalls)
	methodCalls = mergeMethodValueValidateCallSets(methodCalls, collectFirstArgValidatedCalls(pass, fnDecl.Body, stack))

	cfg := buildFuncCFGForPass(pass, fnDecl.Body)
	entry := cfgEntryBlock(cfg)
	if entry == nil {
		return firstArgCallSummary{}, false
	}

	noReturnAliases := collectNoReturnFuncAliasEvents(pass, fnDecl.Body)
	alwaysValidates := !hasPathToReturnWithoutValidate(
		pass,
		cfg,
		entry,
		-1,
		target,
		pathSyncLits,
		pathSyncCalls,
		methodCalls,
		noReturnAliases,
	)
	escapesBeforeValidate := hasUseBeforeValidateInBlockModeWithSummaryStack(
		pass,
		entry.Nodes,
		0,
		target,
		ubvSyncLits,
		ubvSyncCalls,
		methodCalls,
		ubvModeEscape,
		stack,
	)
	if !escapesBeforeValidate {
		escapesBeforeValidate = hasUseBeforeValidateCrossBlockModeWithSummaryStack(
			pass,
			entry,
			-1,
			target,
			ubvSyncLits,
			ubvSyncCalls,
			methodCalls,
			ubvModeEscape,
			defaultCFGMaxStates,
			defaultCFGMaxDepth,
			stack,
		)
	}

	return firstArgCallSummary{
		AlwaysValidatesFirstArg:         alwaysValidates,
		ValidatesBeforeEscapeOfFirstArg: alwaysValidates && !escapesBeforeValidate,
	}, true
}

func cfgEntryBlock(cfg *gocfg.CFG) *gocfg.Block {
	if cfg == nil || len(cfg.Blocks) == 0 {
		return nil
	}
	return cfg.Blocks[0]
}

// collectFirstArgValidatedCalls identifies call sites in body where the callee
// guarantees that its first argument is validated before any escape/use.
// Returned entries are represented as call->receiver mappings so existing
// validate matching can reuse the same method-call path.
func collectFirstArgValidatedCalls(pass *analysis.Pass, body *ast.BlockStmt, stack map[string]bool) methodValueValidateCallSet {
	if pass == nil || body == nil {
		return nil
	}
	var out methodValueValidateCallSet
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		summary, ok := callFirstArgSummaryWithStack(pass, call, stack)
		if !ok || !summary.ValidatesBeforeEscapeOfFirstArg {
			return true
		}
		if out == nil {
			out = make(methodValueValidateCallSet)
		}
		out[call] = call.Args[0]
		return true
	})
	return out
}

func findFuncDeclForObject(pass *analysis.Pass, fnObj *types.Func) *ast.FuncDecl {
	if pass == nil || pass.TypesInfo == nil || fnObj == nil {
		return nil
	}
	wantKey := objectKey(fnObj)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fnDecl, ok := decl.(*ast.FuncDecl)
			if !ok || fnDecl.Name == nil {
				continue
			}
			obj := pass.TypesInfo.Defs[fnDecl.Name]
			if obj == nil {
				continue
			}
			if obj == fnObj {
				return fnDecl
			}
			if wantKey != "" && objectKey(obj) == wantKey {
				return fnDecl
			}
		}
	}
	return nil
}

func functionFirstParamTarget(pass *analysis.Pass, fnDecl *ast.FuncDecl) (castTarget, bool) {
	if fnDecl == nil || fnDecl.Type == nil || fnDecl.Type.Params == nil || len(fnDecl.Type.Params.List) == 0 {
		return castTarget{}, false
	}
	first := fnDecl.Type.Params.List[0]
	if len(first.Names) == 0 {
		return castTarget{}, false
	}
	if pass != nil {
		if target, ok := castTargetFromExpr(pass, first.Names[0]); ok {
			return target, true
		}
	}
	return newCastTargetFromName(first.Names[0].Name), true
}
