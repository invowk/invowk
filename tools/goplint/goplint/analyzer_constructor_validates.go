// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
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
	TypeName string // unqualified type name (e.g., "Server")
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
	typeName, ok := directiveValue([]*ast.CommentGroup{fn.Doc}, "validates-type")
	if !ok || typeName == "" {
		return
	}
	obj := pass.TypesInfo.Defs[fn.Name]
	if obj == nil {
		return
	}
	pass.ExportObjectFact(obj, &ValidatesTypeFact{TypeName: typeName})
}

// constructorValidateInfo records a constructor function and whether
// its body calls Validate() on the returned value.
type constructorValidateInfo struct {
	name           string    // function name (e.g., "NewConfig")
	pos            ast.Node  // position of the function declaration
	returnTypeName string    // resolved first non-error return type name
	callsValidate  bool      // body contains a .Validate() selector call
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

			returnType := ctorInfo.returnTypeName
			if returnType == "" {
				continue
			}
			// Check if the return type has Validate(). First try same-package
			// types (fast path), then check cross-package types via the
			// constructor's type signature. Also resolve the return type's
			// package name for diagnostics (reused below).
			var returnTypePkg string
			if validatableStructs[returnType] {
				returnTypePkg = pkgName
			} else {
				hasValidate, retPkg := resolveReturnTypeValidateInfo(pass, fn)
				if !hasValidate {
					continue
				}
				returnTypePkg = retPkg
			}

			// Skip types annotated with //goplint:constant-only — their
			// Validate() is intentionally unwired because all values come
			// from compile-time constants.
			if constantOnlyTypes[returnType] {
				continue
			}

			// Check if the constructor body calls .Validate() on the return type.
			// This is receiver-type-aware: cfg.Validate() on a Config param does
			// not satisfy the check when the constructor returns *Server.
			// Also checks transitively through private factory calls.
			if bodyCallsValidateOnType(pass, fn.Body, returnType) {
				continue
			}
			if bodyCallsValidateTransitive(pass, fn.Body, returnType, nil, 0) {
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
func bodyCallsValidateOnType(pass *analysis.Pass, body *ast.BlockStmt, returnTypeName string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "Validate" {
			return true
		}

		// Resolve the type of the receiver expression (the X in X.Validate()).
		receiverType := pass.TypesInfo.TypeOf(sel.X)
		if receiverType == nil {
			return true
		}

		// Dereference pointers: *Server → Server.
		if ptr, ok := receiverType.(*types.Pointer); ok {
			receiverType = ptr.Elem()
		}

		// Resolve aliases.
		receiverType = types.Unalias(receiverType)

		// Check if the receiver's type name matches the constructor's return type.
		if named, ok := receiverType.(*types.Named); ok {
			if named.Obj().Name() == returnTypeName {
				found = true
				return false
			}
		}

		return true
	})
	return found
}

// methodCallTarget identifies a method call on the constructor's return type
// for transitive validation tracking.
type methodCallTarget struct {
	typeName   string
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
	var bareFuncCallees []string
	var methodCallees []methodCallTarget

	crossPkgValidates := false

	ast.Inspect(body, func(n ast.Node) bool {
		if crossPkgValidates {
			return false
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
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
				if pass.ImportObjectFact(fn, &fact) && fact.TypeName == returnTypeName {
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
								if pass.ImportObjectFact(callee, &fact) && fact.TypeName == returnTypeName {
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
			if named.Obj().Name() == returnTypeName && named.Obj().Pkg() == pass.Pkg {
				methodCallees = append(methodCallees, methodCallTarget{
					typeName:   returnTypeName,
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
		if bodyCallsValidateOnType(pass, calleeBody, returnTypeName) {
			return true
		}
		// Recurse into the callee's body.
		if bodyCallsValidateTransitive(pass, calleeBody, returnTypeName, visited, depth+1) {
			return true
		}
	}

	// Follow method callees on the return type.
	for _, mc := range methodCallees {
		visitKey := mc.typeName + "." + mc.methodName
		if visited[visitKey] {
			continue
		}
		visited[visitKey] = true

		methodBody, _ := findMethodBody(pass, mc.typeName, mc.methodName)
		if methodBody == nil {
			continue
		}
		if bodyCallsValidateOnType(pass, methodBody, returnTypeName) {
			return true
		}
		if bodyCallsValidateTransitive(pass, methodBody, returnTypeName, visited, depth+1) {
			return true
		}
	}

	return false
}

// resolveReturnTypeValidateInfo resolves the constructor's first non-error
// return type via the type checker and checks if it has a Validate() method.
// Returns (hasValidate, returnTypePkgName) where returnTypePkgName is the
// resolved package name of the return type (for cross-package diagnostics).
func resolveReturnTypeValidateInfo(pass *analysis.Pass, fn *ast.FuncDecl) (bool, string) {
	obj := pass.TypesInfo.Defs[fn.Name]
	if obj == nil {
		return false, ""
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Results().Len() == 0 {
		return false, ""
	}
	retType := sig.Results().At(0).Type()
	if ptr, ok := retType.(*types.Pointer); ok {
		retType = ptr.Elem()
	}
	retType = types.Unalias(retType)
	retPkg := ""
	if named, ok := retType.(*types.Named); ok && named.Obj().Pkg() != nil {
		retPkg = packageName(named.Obj().Pkg())
	}
	return hasValidateMethod(retType), retPkg
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
