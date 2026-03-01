// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

type runNeeds struct {
	needConstructors bool
	needStructFields bool
	needOptionTypes  bool
	needWithFuncs    bool
	needMethods      bool
}

func deriveRunNeeds(rc runConfig) runNeeds {
	return runNeeds{
		needConstructors: rc.checkConstructors ||
			rc.checkConstructorSig ||
			rc.checkFuncOptions ||
			rc.checkImmutability ||
			rc.checkStructValidate ||
			rc.checkConstructorValidates ||
			rc.checkConstructorReturnError,
		needStructFields: rc.checkFuncOptions || rc.checkImmutability,
		needOptionTypes:  rc.checkFuncOptions,
		needWithFuncs:    rc.checkFuncOptions,
		needMethods: rc.checkValidate ||
			rc.checkStringer ||
			rc.checkConstructors ||
			rc.checkStructValidate,
	}
}

type runCollectors struct {
	// non-struct named types (for validate/stringer)
	namedTypes []namedTypeInfo
	// "TypeName.MethodName" -> signature info
	methodSeen map[string]*methodInfo
	// exported struct types (for constructors + structural)
	exportedStructs []exportedStructInfo
	// "NewTypeName" -> details
	constructorDetails map[string]*constructorFuncInfo
	// optionTypeName -> targetStructName
	optionTypes map[string]string
	// targetStructName -> [withFuncInfo, ...]
	withFunctions map[string][]withFuncInfo
	// <pkg-path>.<TypeName> -> true
	constantOnlyTypes map[string]bool
}

func newRunCollectors(rc runConfig, needs runNeeds) runCollectors {
	collectors := runCollectors{}

	// constantOnlyTypes tracks fully-qualified type keys
	// (<pkg-path>.<TypeName>) annotated with //goplint:constant-only.
	// These types have Validate() but are only ever instantiated from
	// compile-time constants, so their constructors are intentionally
	// exempt from --check-constructor-validates and --check-constructor-return-error.
	if rc.checkConstructorValidates || rc.checkConstructorReturnError {
		collectors.constantOnlyTypes = make(map[string]bool)
	}
	if needs.needMethods {
		collectors.methodSeen = make(map[string]*methodInfo)
	}
	if needs.needConstructors {
		collectors.constructorDetails = make(map[string]*constructorFuncInfo)
	}
	if needs.needOptionTypes {
		collectors.optionTypes = make(map[string]string)
	}
	if needs.needWithFuncs {
		collectors.withFunctions = make(map[string][]withFuncInfo)
	}
	return collectors
}

func runWithState(pass *analysis.Pass, state *flagState) (any, error) {
	rc := newRunConfigForState(state)

	cfg, bl, err := loadRunInputs(state, rc)
	if err != nil {
		return nil, err
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	needs := deriveRunNeeds(rc)
	collectors := newRunCollectors(rc, needs)

	runTraversal(pass, insp, rc, cfg, bl, needs, &collectors)
	runPostTraversalChecks(pass, state, rc, cfg, bl, &collectors)

	return nil, nil
}

func loadRunInputs(state *flagState, rc runConfig) (*ExceptionConfig, *BaselineConfig, error) {
	cfg, err := loadConfigCached(state, rc.configPath, rc.configPathExplicit)
	if err != nil {
		return nil, nil, err
	}

	bl, err := loadBaselineCached(state, rc.baselinePath, rc.baselinePathExplicit)
	if err != nil {
		return nil, nil, err
	}

	return cfg, bl, nil
}

func runTraversal(
	pass *analysis.Pass,
	insp *inspector.Inspector,
	rc runConfig,
	cfg *ExceptionConfig,
	bl *BaselineConfig,
	needs runNeeds,
	collectors *runCollectors,
) {
	nodeFilter := []ast.Node{
		(*ast.GenDecl)(nil),
		(*ast.FuncDecl)(nil),
	}

	insp.Preorder(nodeFilter, func(n ast.Node) {
		// Skip test files entirely — test data legitimately uses primitives.
		if isTestFile(pass, n.Pos()) {
			return
		}

		// Skip files matching exclude_paths from config.
		filePath := pass.Fset.Position(n.Pos()).Filename
		if cfg.isExcludedPath(filePath) {
			return
		}

		switch n := n.(type) {
		case *ast.GenDecl:
			// Primary mode: check struct fields for primitives.
			inspectStructFields(pass, n, cfg, bl)

			// Supplementary: collect named types.
			if rc.checkValidate || rc.checkStringer {
				collectNamedTypes(pass, n, &collectors.namedTypes)
			}

			// Collect exported structs — use field-enriched version when
			// structural checks need field metadata.
			if needs.needConstructors {
				if needs.needStructFields {
					collectExportedStructsWithFields(pass, n, &collectors.exportedStructs)
				} else {
					collectExportedStructs(pass, n, &collectors.exportedStructs)
				}
			}

			// Structural: collect option type definitions.
			if needs.needOptionTypes {
				collectOptionTypes(pass, n, collectors.optionTypes)
			}

			// Collect types with //goplint:constant-only directive.
			if collectors.constantOnlyTypes != nil {
				collectConstantOnlyTypes(pass.Pkg.Path(), n, collectors.constantOnlyTypes)
			}

		case *ast.FuncDecl:
			// Always export validates-type facts for cross-package
			// constructor-validates tracking (analysis.Fact propagation).
			exportValidatesTypeFacts(pass, n)

			// Primary mode: check func params and returns for primitives.
			inspectFuncDecl(pass, n, cfg, bl)

			// Supplementary: track methods for validate/stringer and error detection.
			if needs.needMethods {
				trackMethods(pass, n, collectors.methodSeen)
			}

			// Track constructors with return type and param details.
			if collectors.constructorDetails != nil {
				trackConstructorDetails(pass, n, collectors.constructorDetails)
			}

			// Structural: track WithXxx option functions.
			if needs.needWithFuncs {
				trackWithFunctions(pass, n, collectors.optionTypes, collectors.withFunctions)
			}

			// Cast validation: detect unvalidated type conversions to DDD types.
			// CFA (default) uses path-reachability analysis; --no-cfa falls
			// back to AST name-based heuristic.
			if rc.checkCastValidation {
				if rc.noCFA {
					inspectUnvalidatedCasts(pass, n, cfg, bl)
				} else {
					inspectUnvalidatedCastsCFA(pass, n, cfg, bl, rc.checkUseBeforeValidate, rc.checkUseBeforeValidateCross)
				}
			}

			// Validate usage: detect discarded Validate() results.
			if rc.checkValidateUsage {
				inspectValidateUsage(pass, n, cfg, bl)
			}

			// Constructor error usage: detect blanked error returns.
			if rc.checkConstructorErrUsage {
				inspectConstructorErrorUsage(pass, n, cfg, bl)
			}
		}
	})
}

func runPostTraversalChecks(
	pass *analysis.Pass,
	state *flagState,
	rc runConfig,
	cfg *ExceptionConfig,
	bl *BaselineConfig,
	collectors *runCollectors,
) {
	// Post-traversal checks for supplementary modes.
	if rc.checkValidate {
		reportMissingValidate(pass, collectors.namedTypes, collectors.methodSeen, cfg, bl)
	}
	if rc.checkStringer {
		reportMissingStringer(pass, collectors.namedTypes, collectors.methodSeen, cfg, bl)
	}
	if rc.checkConstructors {
		reportMissingConstructors(pass, collectors.exportedStructs, collectors.constructorDetails, collectors.methodSeen, cfg, bl)
	}

	// Structural checks — all require constructorDetails.
	if rc.checkConstructorSig {
		reportWrongConstructorSig(pass, collectors.exportedStructs, collectors.constructorDetails, cfg, bl)
	}
	if rc.checkFuncOptions {
		reportMissingFuncOptions(pass, collectors.exportedStructs, collectors.constructorDetails, collectors.optionTypes, collectors.withFunctions, cfg, bl)
	}
	if rc.checkImmutability {
		reportMissingImmutability(pass, collectors.exportedStructs, collectors.constructorDetails, cfg, bl)
	}
	if rc.checkStructValidate {
		reportMissingStructValidate(pass, collectors.exportedStructs, collectors.constructorDetails, collectors.methodSeen, cfg, bl)
	}
	if rc.checkConstructorValidates {
		inspectConstructorValidates(pass, collectors.constructorDetails, collectors.constantOnlyTypes, cfg, bl, rc.noCFA)
	}

	// Constructor return error — constructors for validatable types should return error.
	if rc.checkConstructorReturnError {
		inspectConstructorReturnError(pass, collectors.constructorDetails, collectors.constantOnlyTypes, cfg, bl)
	}

	// Validate delegation — opt-in via //goplint:validate-all.
	if rc.checkValidateDelegation {
		inspectValidateDelegation(pass, cfg, bl)
	}

	// Nonzero field checks — cross-package via analysis.Fact.
	if rc.checkNonZero {
		inspectNonZero(pass, cfg, bl)
	}

	if rc.auditExceptions {
		reportStaleExceptionsInline(pass, cfg)
	}

	if rc.auditReviewDates {
		reportOverdueExceptions(pass, cfg, state)
	}

	// Suggest validate-all: advisory mode for structs that may benefit from the directive.
	// NOT included in --check-all — this is purely advisory.
	if rc.suggestValidateAll {
		inspectSuggestValidateAll(pass, cfg, bl)
	}

	// Enum sync: compare Go Validate() switch cases against CUE disjunctions.
	if rc.checkEnumSync {
		inspectEnumSync(pass, cfg, bl)
	}
}
