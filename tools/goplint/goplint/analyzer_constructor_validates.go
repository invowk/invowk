// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	gocfg "golang.org/x/tools/go/cfg"
)

// maxTransitiveDepth is the maximum call chain depth for transitive
// factory tracking in --check-constructor-validates. This bounds
// recursion in bodyCallsValidateTransitive() to prevent pathological
// cases while allowing realistic delegation chains (e.g.,
// NewFoo → buildBar → initBaz → baz.Validate()).
const maxTransitiveDepth = 5

// ValidatesTypeFact is an analysis.Fact exported for functions annotated
// with //goplint:validates-type=TypeName. The directive indicates that
// the function validates the named type on behalf of a constructor,
// enabling cross-package constructor-validates tracking.
//
// When goplint processes a package containing a helper function annotated
// with this directive, it exports the fact. Consuming packages can then
// import the fact when checking whether a constructor's call to the helper
// satisfies the Validate() requirement.
type ValidatesTypeFact struct {
	TypeName    string // unqualified type name (e.g., "Server")
	TypePkgPath string // package path for the type name (optional for legacy facts)
}

// AFact implements the analysis.Fact interface marker method.
func (*ValidatesTypeFact) AFact() {}

// String returns a human-readable representation for analysistest fact matching.
func (f *ValidatesTypeFact) String() string {
	return fmt.Sprintf("validates-type(%s)", f.TypeName)
}

// exportValidatesTypeFacts scans a function declaration for the
// //goplint:validates-type=TypeName directive and exports a
// ValidatesTypeFact for the function. This enables cross-package
// tracking in bodyCallsValidateTransitive.
//
// Only free functions (not methods) are supported — the directive is
// intended for standalone helper functions like util.ValidateServer().
func exportValidatesTypeFacts(pass *analysis.Pass, fn *ast.FuncDecl) {
	if fn.Recv != nil {
		return
	}
	directiveType, ok := directiveValue([]*ast.CommentGroup{fn.Doc}, "validates-type")
	if !ok || directiveType == "" {
		return
	}
	typeName, typePkgPath := resolveDirectiveTypeIdentity(pass, fn, directiveType)
	if typeName == "" {
		return
	}
	obj := pass.TypesInfo.Defs[fn.Name]
	if obj == nil {
		return
	}
	pass.ExportObjectFact(obj, &ValidatesTypeFact{
		TypeName:    typeName,
		TypePkgPath: typePkgPath,
	})
}

func resolveDirectiveTypeIdentity(pass *analysis.Pass, fn *ast.FuncDecl, raw string) (typeName, typePkgPath string) {
	directive := strings.TrimSpace(raw)
	if directive == "" {
		return "", ""
	}

	pkgAlias := ""
	if dot := strings.LastIndex(directive, "."); dot > 0 && dot < len(directive)-1 {
		left := directive[:dot]
		right := directive[dot+1:]
		directive = right
		if strings.Contains(left, "/") {
			typePkgPath = left
		} else {
			pkgAlias = left
		}
	}

	typeName = directive
	if typePkgPath == "" {
		typePkgPath = inferDirectiveTypePkgPath(pass, fn, typeName, pkgAlias)
	}
	if typePkgPath == "" && pass != nil && pass.Pkg != nil {
		typePkgPath = pass.Pkg.Path()
	}
	return typeName, typePkgPath
}

func inferDirectiveTypePkgPath(pass *analysis.Pass, fn *ast.FuncDecl, typeName, pkgAlias string) string {
	if pass == nil || pass.TypesInfo == nil || fn == nil {
		return ""
	}
	obj := pass.TypesInfo.Defs[fn.Name]
	if obj == nil {
		return ""
	}
	fnObj, ok := obj.(*types.Func)
	if !ok {
		return ""
	}
	sig, ok := fnObj.Type().(*types.Signature)
	if !ok {
		return ""
	}

	if path := matchNamedTypePkgPath(sig.Params(), typeName, pkgAlias); path != "" {
		return path
	}
	if path := matchNamedTypePkgPath(sig.Results(), typeName, pkgAlias); path != "" {
		return path
	}
	return ""
}

func matchNamedTypePkgPath(tuple *types.Tuple, typeName, pkgAlias string) string {
	if tuple == nil || typeName == "" {
		return ""
	}
	for variable := range tuple.Variables() {
		t := variable.Type()
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}
		t = types.Unalias(t)
		named, ok := t.(*types.Named)
		if !ok || named.Obj() == nil || named.Obj().Name() != typeName {
			continue
		}
		pkg := named.Obj().Pkg()
		if pkg == nil {
			continue
		}
		if pkgAlias != "" && pkg.Name() != pkgAlias {
			continue
		}
		return pkg.Path()
	}
	return ""
}

// constructorValidateInfo records a constructor function and whether
// its body calls Validate() on the returned value.
type constructorValidateInfo struct {
	name           string   // function name (e.g., "NewConfig")
	pos            ast.Node // position of the function declaration
	returnTypeName string   // resolved first non-error return type name
	callsValidate  bool     // body contains a .Validate() selector call
}

// inspectConstructorValidates checks whether NewXxx() constructors call
// Validate() on the type they construct. Constructors returning types with
// a Validate() method should call it before returning to enforce invariants.
//
// Types annotated with //goplint:constant-only are exempt — their values
// only come from compile-time constants, so runtime validation is unnecessary.
//
// This is a post-traversal check: it receives the constructorDetails map
// already populated by trackConstructorDetails, then walks the function
// bodies looking for .Validate() calls.
func inspectConstructorValidates(
	pass *analysis.Pass,
	ctors map[string]*constructorFuncInfo,
	constantOnlyTypes map[string]bool,
	cfg *ExceptionConfig,
	bl *BaselineConfig,
	noCFA bool,
) {
	pkgName := packageName(pass.Pkg)

	// Build a set of struct names that have Validate() methods.
	validatableStructs := buildValidatableStructs(pass)

	// Walk all files to find constructor function bodies.
	for _, file := range pass.Files {
		if isTestFile(pass, file.Pos()) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}

			name := fn.Name.Name
			if !strings.HasPrefix(name, "New") || len(name) <= 3 {
				continue
			}

			ctorInfo, exists := ctors[name]
			if !exists {
				continue
			}

			// Skip constructors returning interfaces — they may delegate
			// validation to the concrete implementation.
			if ctorInfo.returnsInterface {
				continue
			}

			retInfo := resolveReturnTypeValidateInfo(pass, fn)
			returnType := retInfo.TypeName
			if returnType == "" {
				returnType = ctorInfo.returnTypeName
			}
			if returnType == "" {
				continue
			}
			returnTypePkg := retInfo.TypePkgName
			if returnTypePkg == "" {
				returnTypePkg = pkgName
			}
			returnTypePkgPath := retInfo.TypePkgPath
			if returnTypePkgPath == "" {
				returnTypePkgPath = pass.Pkg.Path()
			}
			returnTypeKey := retInfo.TypeKey
			if returnTypeKey == "" {
				returnTypeKey = returnTypePkgPath + "." + returnType
			}

			// Check if the return type has Validate(). Try same-package
			// fast path first; fall back to type-checker resolution for
			// cross-package and alias-heavy cases.
			if !retInfo.HasValidate && !(returnTypePkgPath == pass.Pkg.Path() && validatableStructs[returnType]) {
				continue
			}

			// Skip types annotated with //goplint:constant-only — their
			// Validate() is intentionally unwired because all values come
			// from compile-time constants.
			if constantOnlyTypes[returnTypeKey] {
				continue
			}

			// Check whether constructor paths validate the returned type.
			//
			// CFA mode (default): evaluate all return paths directly in CFG.
			// This includes synchronous closures (defer/IIFE) and recognizes
			// transitive helper calls in the current block.
			//
			// AST mode (--no-cfa): use direct/transitive body heuristics.
			if noCFA {
				if bodyCallsValidateOnType(pass, fn.Body, returnTypeKey) ||
					bodyCallsValidateTransitive(pass, fn.Body, returnType, returnTypePkgPath, returnTypeKey, nil, 0) {
					continue
				}
			} else if !constructorHasUnvalidatedReturnPath(pass, fn, returnType, returnTypePkgPath, returnTypeKey) {
				continue
			}

			// Check for ignore directive on the function.
			if hasIgnoreDirective(fn.Doc, nil) {
				continue
			}

			qualName := fmt.Sprintf("%s.%s", pkgName, name)
			excKey := qualName + ".constructor-validate"
			if cfg.isExcepted(excKey) {
				continue
			}

			msg := fmt.Sprintf(
				"constructor %s returns %s.%s which has Validate() but never calls it",
				qualName, returnTypePkg, returnType)
			findingID := StableFindingID(CategoryMissingConstructorValidate, qualName, returnType)
			if bl.ContainsFinding(CategoryMissingConstructorValidate, findingID, msg) {
				continue
			}

			reportDiagnostic(pass, fn.Name.Pos(), CategoryMissingConstructorValidate, findingID, msg)
		}
	}
}

// bodyCallsValidateOnType walks a function body looking for a .Validate()
// selector call where the receiver's type matches the constructor's return type.
// This avoids the false-negative pattern where cfg.Validate() (on a Config
// parameter) satisfies the heuristic even though the returned Server is never
// validated.
//
// Accepted patterns:
//   - Direct: s := &Server{...}; s.Validate()
//   - Delegated: s, err := helperNewServer(); s.Validate()
//   - Any .Validate() call on an expression whose resolved type matches returnTypeName
//
// Note: this does not handle deferred closures or IIFEs — it is a quick
// pre-check before CFA. The full path-sensitive analysis in
// constructorHasUnvalidatedReturnPath handles those cases.
func bodyCallsValidateOnType(pass *analysis.Pass, body *ast.BlockStmt, returnTypeKey string) bool {
	methodCalls := collectMethodValueValidateCalls(pass, body)
	return containsValidateOnReceiver(pass, body, typeKeyMatcher(returnTypeKey), nil, nil, methodCalls)
}

// methodCallTarget identifies a method call on the constructor's return type
// for transitive validation tracking.
type methodCallTarget struct {
	typeName   string
	typeKey    string
	methodName string
}

// bodyCallsValidateTransitive checks if any private function or method called
// from body transitively calls Validate() on the given return type. Uses
// pass.TypesInfo to resolve callee identities. Bounds recursion depth
// to maxTransitiveDepth to prevent pathological cases. The visited map
// prevents cycles (re-visiting the same function/method); depth tracks the
// actual call chain depth independently.
//
// This function follows two kinds of callees:
//  1. Same-package bare function calls (e.g., helper()) — via *ast.Ident
//  2. Method calls on variables whose type matches returnTypeName
//     (e.g., s.Setup() where s is *Server) — via *ast.SelectorExpr
func bodyCallsValidateTransitive(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	returnTypeName string,
	returnTypePkgPath string,
	returnTypeKey string,
	visited map[string]bool,
	depth int,
) bool {
	if visited == nil {
		visited = make(map[string]bool)
	}

	// Bound recursion by call chain depth, not visit count.
	if depth >= maxTransitiveDepth {
		return false
	}

	// Collect bare function call identifiers AND method calls on the return type.
	// Calls in conditionally-evaluated contexts are ignored to avoid treating
	// partial/dead-branch delegation as whole-function coverage.
	var bareFuncCallees []string
	var methodCallees []methodCallTarget

	crossPkgValidates := false
	parentMap := buildParentMap(body)

	ast.Inspect(body, func(n ast.Node) bool {
		if crossPkgValidates {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if isConditionallyEvaluated(call, parentMap) {
			return true
		}

		switch fun := call.Fun.(type) {
		case *ast.Ident:
			// Bare function calls — same-package or cross-package with fact.
			obj := pass.TypesInfo.Uses[fun]
			if obj == nil {
				return true
			}
			fn, ok := obj.(*types.Func)
			if !ok {
				return true
			}
			if fn.Pkg() != pass.Pkg {
				// Cross-package: check for validates-type fact.
				var fact ValidatesTypeFact
				if pass.ImportObjectFact(fn, &fact) && factMatchesReturnType(fact, returnTypeName, returnTypePkgPath) {
					crossPkgValidates = true
					return false
				}
				return true
			}
			bareFuncCallees = append(bareFuncCallees, fun.Name)

		case *ast.SelectorExpr:
			// Skip Validate() itself — already handled by bodyCallsValidateOnType.
			if fun.Sel.Name == "Validate" {
				return true
			}

			// Check if this is a cross-package function call (pkg.Func pattern)
			// with a validates-type fact.
			if ident, ok := fun.X.(*ast.Ident); ok {
				obj := pass.TypesInfo.Uses[ident]
				if obj != nil {
					if _, isPkgName := obj.(*types.PkgName); isPkgName {
						// This is a qualified call: pkg.Func(...)
						selObj := pass.TypesInfo.Uses[fun.Sel]
						if selObj != nil {
							if callee, ok := selObj.(*types.Func); ok {
								var fact ValidatesTypeFact
								if pass.ImportObjectFact(callee, &fact) && factMatchesReturnType(fact, returnTypeName, returnTypePkgPath) {
									crossPkgValidates = true
									return false
								}
							}
						}
						return true
					}
				}
			}

			// Method calls on variables whose type matches the return type.
			receiverType := pass.TypesInfo.TypeOf(fun.X)
			if receiverType == nil {
				return true
			}
			// Dereference pointers: *Server → Server.
			if ptr, ok := receiverType.(*types.Pointer); ok {
				receiverType = ptr.Elem()
			}
			receiverType = types.Unalias(receiverType)
			named, ok := receiverType.(*types.Named)
			if !ok {
				return true
			}
			if typeIdentityKey(receiverType) == returnTypeKey && named.Obj().Pkg() == pass.Pkg {
				methodCallees = append(methodCallees, methodCallTarget{
					typeName:   named.Obj().Name(),
					typeKey:    returnTypeKey,
					methodName: fun.Sel.Name,
				})
			}
		}
		return true
	})

	// Cross-package fact match found — the callee validates the return type.
	if crossPkgValidates {
		return true
	}

	// Follow bare function callees.
	for _, calleeName := range bareFuncCallees {
		if visited[calleeName] {
			continue
		}
		visited[calleeName] = true

		calleeBody := findFuncBody(pass, calleeName)
		if calleeBody == nil {
			continue
		}
		if helperBodyAlwaysValidatesType(pass, calleeBody, returnTypeKey) {
			return true
		}
		// Recurse into the callee's body.
		if bodyCallsValidateTransitive(pass, calleeBody, returnTypeName, returnTypePkgPath, returnTypeKey, visited, depth+1) {
			return true
		}
	}

	// Follow method callees on the return type.
	for _, mc := range methodCallees {
		visitKey := mc.typeKey + "." + mc.methodName
		if visited[visitKey] {
			continue
		}
		visited[visitKey] = true

		methodBody, _ := findMethodBody(pass, mc.typeName, mc.methodName)
		if methodBody == nil {
			continue
		}
		if helperBodyAlwaysValidatesType(pass, methodBody, returnTypeKey) {
			return true
		}
		if bodyCallsValidateTransitive(pass, methodBody, returnTypeName, returnTypePkgPath, returnTypeKey, visited, depth+1) {
			return true
		}
	}

	return false
}

type returnTypeValidateInfo struct {
	HasValidate bool
	TypeName    string
	TypePkgName string
	TypePkgPath string
	TypeKey     string
}

// resolveReturnTypeValidateInfo resolves the constructor's first non-error
// return type via the type checker and checks if it has a Validate() method.
func resolveReturnTypeValidateInfo(pass *analysis.Pass, fn *ast.FuncDecl) returnTypeValidateInfo {
	obj := pass.TypesInfo.Defs[fn.Name]
	if obj == nil {
		return returnTypeValidateInfo{}
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Results().Len() == 0 {
		return returnTypeValidateInfo{}
	}

	var retType types.Type
	for resultVar := range sig.Results().Variables() {
		candidate := resultVar.Type()
		if !isErrorType(candidate) {
			retType = candidate
			break
		}
	}
	if retType == nil {
		retType = sig.Results().At(0).Type()
	}
	if ptr, ok := retType.(*types.Pointer); ok {
		retType = ptr.Elem()
	}
	retType = types.Unalias(retType)

	info := returnTypeValidateInfo{
		HasValidate: hasValidateMethod(retType),
		TypeKey:     typeIdentityKey(retType),
	}
	if named, ok := retType.(*types.Named); ok {
		info.TypeName = named.Obj().Name()
		if pkg := named.Obj().Pkg(); pkg != nil {
			info.TypePkgName = packageName(pkg)
			info.TypePkgPath = pkg.Path()
		}
	}
	return info
}

func factMatchesReturnType(fact ValidatesTypeFact, returnTypeName, returnTypePkgPath string) bool {
	if fact.TypeName != returnTypeName {
		return false
	}
	// Legacy facts only had TypeName. Accept them for compatibility.
	if fact.TypePkgPath == "" {
		return true
	}
	return fact.TypePkgPath == returnTypePkgPath
}

func typeIdentityKey(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	t = types.Unalias(t)
	return types.TypeString(t, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}

// findFuncBody searches the package for a non-method function with the given
// name and returns its body. Returns nil if not found.
func findFuncBody(pass *analysis.Pass, funcName string) *ast.BlockStmt {
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			if fn.Name.Name == funcName {
				return fn.Body
			}
		}
	}
	return nil
}

// constructorHasUnvalidatedReturnPath builds a CFG for the constructor body
// and checks whether any path from entry to a return block lacks a .Validate()
// call on the return type. Unlike the cast-validation CFA which starts from a
// specific cast site, this starts from the function entry (block 0) and checks
// all return paths.
//
// Returns true if any return path lacks Validate() on the return type.
func constructorHasUnvalidatedReturnPath(
	pass *analysis.Pass,
	fn *ast.FuncDecl,
	returnTypeName string,
	returnTypePkgPath string,
	returnTypeKey string,
) bool {
	funcCFG := buildFuncCFGForPass(pass, fn.Body)
	if funcCFG == nil || len(funcCFG.Blocks) == 0 {
		return false
	}
	parentMap := buildParentMap(fn.Body)
	_, _, closureCalls, methodValueCalls := collectCFACasts(
		pass,
		fn.Body,
		parentMap,
		func(*ast.FuncLit, int) {},
	)
	syncLits := collectSynchronousClosureLits(fn.Body)
	syncCalls := collectSynchronousClosureVarCalls(closureCalls)
	methodCalls := collectMethodValueValidateCallSet(methodValueCalls)
	bareReturnIncludesTarget := constructorBareReturnIncludesType(pass, fn, returnTypeKey)

	// DFS from the entry block (index 0).
	visited := make(map[int32]bool)
	return dfsConstructorUnvalidated(
		pass,
		funcCFG.Blocks[0:1],
		returnTypeName,
		returnTypePkgPath,
		returnTypeKey,
		bareReturnIncludesTarget,
		visited,
		syncLits,
		syncCalls,
		methodCalls,
	)
}

// dfsConstructorUnvalidated recursively checks whether any path through the
// given CFG blocks reaches a return block without encountering a .Validate()
// call on the constructor's return type. Delegates to the shared
// dfsUnvalidatedBlocks engine with a type-identity matcher.
func dfsConstructorUnvalidated(
	pass *analysis.Pass,
	blocks []*gocfg.Block,
	returnTypeName string,
	returnTypePkgPath string,
	returnTypeKey string,
	bareReturnIncludesTarget bool,
	visited map[int32]bool,
	syncLits map[*ast.FuncLit]bool,
	syncCalls closureVarCallSet,
	methodCalls methodValueValidateCallSet,
) bool {
	matcher := typeKeyMatcher(returnTypeKey)
	checker := func(block *gocfg.Block) bool {
		if blockTerminatesWithoutReturn(pass, block) {
			return true
		}
		// Return blocks that do not return the constructor target type
		// (for example, early `return nil, err`) are irrelevant for
		// constructor-validates path checks.
		if len(block.Succs) == 0 && !blockReturnsTargetType(pass, block, returnTypeKey, bareReturnIncludesTarget) {
			return true
		}
		for _, node := range block.Nodes {
			if containsValidateOnReceiver(pass, node, matcher, syncLits, syncCalls, methodCalls) {
				return true
			}
			if stmt, ok := node.(ast.Stmt); ok {
				// Consider transitive helper validation for this statement.
				// Wrapping in a one-statement block preserves the existing
				// transitive walker behavior while keeping block-level path
				// sensitivity in the outer CFG DFS.
				stmtBody := &ast.BlockStmt{List: []ast.Stmt{stmt}}
				if bodyCallsValidateTransitive(pass, stmtBody, returnTypeName, returnTypePkgPath, returnTypeKey, nil, 0) {
					return true
				}
			}
		}
		return false
	}
	return dfsUnvalidatedBlocks(blocks, visited, checker)
}

func blockReturnsTargetType(pass *analysis.Pass, block *gocfg.Block, returnTypeKey string, bareReturnIncludesTarget bool) bool {
	for _, node := range block.Nodes {
		ret, ok := node.(*ast.ReturnStmt)
		if !ok {
			continue
		}
		if returnStmtReturnsType(pass, ret, returnTypeKey, bareReturnIncludesTarget) {
			return true
		}
	}
	return false
}

func returnStmtReturnsType(pass *analysis.Pass, ret *ast.ReturnStmt, returnTypeKey string, bareReturnIncludesTarget bool) bool {
	if ret == nil {
		return false
	}
	if len(ret.Results) == 0 {
		return bareReturnIncludesTarget
	}
	for _, expr := range ret.Results {
		if exprReturnsType(pass, expr, returnTypeKey) {
			return true
		}
	}
	return false
}

func constructorBareReturnIncludesType(pass *analysis.Pass, fn *ast.FuncDecl, returnTypeKey string) bool {
	if pass == nil || pass.TypesInfo == nil || fn == nil {
		return false
	}
	obj := pass.TypesInfo.Defs[fn.Name]
	if obj == nil {
		return false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Results() == nil || sig.Results().Len() == 0 {
		return false
	}

	hasNamedResults := false
	for resultVar := range sig.Results().Variables() {
		if resultVar.Name() != "" {
			hasNamedResults = true
		}
		if typeIdentityKey(resultVar.Type()) == returnTypeKey && hasNamedResults {
			return true
		}
	}
	return false
}

func helperBodyAlwaysValidatesType(pass *analysis.Pass, body *ast.BlockStmt, returnTypeKey string) bool {
	if body == nil {
		return false
	}
	cfg := buildFuncCFGForPass(pass, body)
	if cfg == nil || len(cfg.Blocks) == 0 {
		return false
	}
	parentMap := buildParentMap(body)
	_, _, closureCalls, methodValueCalls := collectCFACasts(
		pass,
		body,
		parentMap,
		func(*ast.FuncLit, int) {},
	)
	syncLits := collectSynchronousClosureLits(body)
	syncCalls := collectSynchronousClosureVarCalls(closureCalls)
	methodCalls := collectMethodValueValidateCallSet(methodValueCalls)
	matcher := typeKeyMatcher(returnTypeKey)
	checker := func(block *gocfg.Block) bool {
		if blockTerminatesWithoutReturn(pass, block) {
			return true
		}
		for _, node := range block.Nodes {
			if containsValidateOnReceiver(pass, node, matcher, syncLits, syncCalls, methodCalls) {
				return true
			}
		}
		return false
	}
	visited := make(map[int32]bool)
	return !dfsUnvalidatedBlocks(cfg.Blocks[0:1], visited, checker)
}

func exprReturnsType(pass *analysis.Pass, expr ast.Expr, returnTypeKey string) bool {
	if pass == nil || pass.TypesInfo == nil || expr == nil {
		return false
	}
	exprType := pass.TypesInfo.TypeOf(expr)
	if exprType == nil {
		return false
	}
	exprType = types.Unalias(exprType)
	if tuple, ok := exprType.(*types.Tuple); ok {
		for variable := range tuple.Variables() {
			if typeIdentityKey(variable.Type()) == returnTypeKey {
				return true
			}
		}
		return false
	}
	return typeIdentityKey(exprType) == returnTypeKey
}
