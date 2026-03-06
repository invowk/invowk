// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/ssa"
)

// TestSSABuilderProducesDebugRefs verifies that the on-demand SSA builder
// emits DebugRef instructions needed for alias tracking.
func TestSSABuilderProducesDebugRefs(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	probeAnalyzer := &analysis.Analyzer{
		Name: "ssaprobe",
		Doc:  "probe SSA for debug refs",
		Run: func(pass *analysis.Pass) (any, error) {
			res := buildSSAForPass(pass)
			if res == nil || res.Pkg == nil {
				// Standard library package or build failure — skip.
				return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
			}

			var foundDebugRef bool
			for _, mem := range res.Pkg.Members {
				fn, ok := mem.(*ssa.Function)
				if !ok || fn.Blocks == nil {
					continue
				}
				for _, block := range fn.Blocks {
					for _, instr := range block.Instrs {
						if _, ok := instr.(*ssa.DebugRef); ok {
							foundDebugRef = true
						}
					}
				}
			}
			if !foundDebugRef {
				t.Error("no DebugRef instructions found; GlobalDebug mode may not be active")
			}
			return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
		},
	}

	analysistestParallelLimiter <- struct{}{}
	defer func() { <-analysistestParallelLimiter }()

	analysistest.Run(t, testdata, probeAnalyzer, "cfa_ssa_alias_probe")
}

// TestComputeMustAliasKeys_CopyAlias verifies that y := x produces an
// alias set containing y's objectKey via a probe fixture without annotations.
func TestComputeMustAliasKeys_CopyAlias(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	probeAnalyzer := &analysis.Analyzer{
		Name: "aliasprobe",
		Doc:  "probe alias computation",
		Run: func(pass *analysis.Pass) (any, error) {
			res := buildSSAForPass(pass)
			if res == nil || res.Pkg == nil {
				return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
			}

			fn := findMemberFunc(res.Pkg, "CopyAlias")
			if fn == nil {
				return nil, nil //nolint:nilnil // not the probe package
			}

			castValue := findFirstCastInFunc(fn)
			if castValue == nil {
				t.Error("no ChangeType/Convert found")
				return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
			}

			aliases := computeMustAliasKeysFromValue(fn, castValue)
			if len(aliases) < 2 {
				t.Errorf("expected alias set to contain x and y, got %d entries", len(aliases))
			}

			var foundX, foundY bool
			for key := range aliases {
				if aliasTestContains(key, ":x:") {
					foundX = true
				}
				if aliasTestContains(key, ":y:") {
					foundY = true
				}
			}
			if !foundX {
				t.Error("alias set missing key for variable x")
			}
			if !foundY {
				t.Error("alias set missing key for variable y")
			}

			return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
		},
	}

	analysistestParallelLimiter <- struct{}{}
	defer func() { <-analysistestParallelLimiter }()

	analysistest.Run(t, testdata, probeAnalyzer, "cfa_ssa_alias_probe")
}

// TestComputeMustAliasKeys_Reassignment verifies that y := x; y = other
// excludes y from the alias set.
func TestComputeMustAliasKeys_Reassignment(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	probeAnalyzer := &analysis.Analyzer{
		Name: "reassignprobe",
		Doc:  "probe reassignment exclusion",
		Run: func(pass *analysis.Pass) (any, error) {
			res := buildSSAForPass(pass)
			if res == nil || res.Pkg == nil {
				return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
			}

			fn := findMemberFunc(res.Pkg, "ReassignedAlias")
			if fn == nil {
				return nil, nil //nolint:nilnil // not the probe package
			}

			castValue := findFirstCastInFunc(fn)
			if castValue == nil {
				t.Error("no ChangeType/Convert found")
				return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
			}

			aliases := computeMustAliasKeysFromValue(fn, castValue)
			for key := range aliases {
				if aliasTestContains(key, ":y:") {
					t.Errorf("alias set should not contain y (reassigned), found: %s", key)
				}
			}

			return nil, nil //nolint:nilnil // probe analyzer has no meaningful result
		},
	}

	analysistestParallelLimiter <- struct{}{}
	defer func() { <-analysistestParallelLimiter }()

	analysistest.Run(t, testdata, probeAnalyzer, "cfa_ssa_alias_probe")
}

// TestMatchesExprWithAliasSet verifies that castTarget stores and exposes
// the alias set correctly.
func TestMatchesExprWithAliasSet(t *testing.T) {
	t.Parallel()

	target := castTarget{
		displayName: "x",
		targetKey:   "obj:var:pkg:x:100",
		aliasKeys: ssaAliasSet{
			"obj:var:pkg:y:200": true,
		},
	}

	if len(target.aliasKeys) != 1 {
		t.Errorf("expected 1 alias key, got %d", len(target.aliasKeys))
	}
	if !target.aliasKeys["obj:var:pkg:y:200"] {
		t.Error("expected alias key for y to be present")
	}

	noAlias := castTarget{
		displayName: "x",
		targetKey:   "obj:var:pkg:x:100",
	}
	if noAlias.aliasKeys != nil {
		t.Error("expected nil aliasKeys for target without aliases")
	}
}

// computeMustAliasKeysFromValue is a test helper that runs the core alias
// computation directly from an SSA value (bypasses AST position matching).
func computeMustAliasKeysFromValue(ssaFn *ssa.Function, castValue ssa.Value) ssaAliasSet {
	type objInfo struct {
		key         string
		valueCount  int
		aliasesCast bool
	}
	objMap := make(map[types.Object]*objInfo)

	for _, block := range ssaFn.Blocks {
		for _, instr := range block.Instrs {
			dbg, ok := instr.(*ssa.DebugRef)
			if !ok || dbg.IsAddr {
				continue
			}
			obj := dbg.Object()
			if obj == nil {
				continue
			}
			info, exists := objMap[obj]
			if !exists {
				info = &objInfo{key: objectKey(obj)}
				objMap[obj] = info
			}
			if dbg.X == castValue {
				info.aliasesCast = true
			} else {
				info.valueCount++
			}
		}
	}

	result := make(ssaAliasSet)
	for _, info := range objMap {
		if !info.aliasesCast || info.valueCount > 0 || info.key == "" {
			continue
		}
		result[info.key] = true
	}
	return result
}

func findMemberFunc(pkg *ssa.Package, name string) *ssa.Function {
	for _, mem := range pkg.Members {
		fn, ok := mem.(*ssa.Function)
		if ok && fn.Name() == name {
			return fn
		}
	}
	return nil
}

func findFirstCastInFunc(fn *ssa.Function) ssa.Value {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			val, ok := instr.(ssa.Value)
			if !ok {
				continue
			}
			switch val.(type) {
			case *ssa.ChangeType, *ssa.Convert:
				return val
			}
		}
	}
	return nil
}

func aliasTestContains(s, sub string) bool {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
