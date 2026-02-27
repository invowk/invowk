// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

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
// This is a post-traversal check: it receives the constructorDetails map
// already populated by trackConstructorDetails, then walks the function
// bodies looking for .Validate() calls.
func inspectConstructorValidates(
	pass *analysis.Pass,
	ctors map[string]*constructorFuncInfo,
	cfg *ExceptionConfig,
	bl *BaselineConfig,
) {
	pkgName := packageName(pass.Pkg)

	// Build a set of struct names that have Validate() methods.
	// We check this using the type checker to find method sets.
	validatableStructs := make(map[string]bool)
	for _, obj := range pass.TypesInfo.Defs {
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if hasValidateMethod(named) {
			validatableStructs[obj.Name()] = true
		}
	}

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
			if returnType == "" || !validatableStructs[returnType] {
				continue
			}

			// Check if the constructor body calls .Validate() on the return type.
			// This is receiver-type-aware: cfg.Validate() on a Config param does
			// not satisfy the check when the constructor returns *Server.
			if bodyCallsValidateOnType(pass, fn.Body, returnType) {
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
				qualName, pkgName, returnType)
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
