// SPDX-License-Identifier: MPL-2.0

package goplint

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// validateAllStruct records a struct annotated with //goplint:validate-all
// along with its fields that have Validate() methods.
type validateAllStruct struct {
	name            string    // type name (e.g., "Config")
	pos             token.Pos // position of the type declaration
	validatableKeys []string  // field names whose types have Validate()
}

// inspectValidateDelegation checks structs annotated with //goplint:validate-all
// for delegation completeness: every field whose type has Validate() should
// be called in the struct's own Validate() method.
func inspectValidateDelegation(pass *analysis.Pass, cfg *ExceptionConfig, bl *BaselineConfig) {
	pkgName := packageName(pass.Pkg)

	// Phase 1: Collect structs with //goplint:validate-all directive and
	// their validatable fields.
	var targets []validateAllStruct
	for _, file := range pass.Files {
		if isTestFile(pass, file.Pos()) {
			continue
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if !hasValidateAllDirective(gd.Doc, ts.Doc) {
					continue
				}

				// Collect fields whose types have Validate().
				var validatableKeys []string
				for _, field := range st.Fields.List {
					fieldType := pass.TypesInfo.TypeOf(field.Type)
					if fieldType == nil {
						continue
					}
					if !hasValidateMethod(fieldType) {
						continue
					}
					for _, name := range field.Names {
						validatableKeys = append(validatableKeys, name.Name)
					}
				}

				if len(validatableKeys) > 0 {
					targets = append(targets, validateAllStruct{
						name:            ts.Name.Name,
						pos:             ts.Name.Pos(),
						validatableKeys: validatableKeys,
					})
				}
			}
		}
	}

	if len(targets) == 0 {
		return
	}

	// Phase 2: For each target, find its Validate() method and check
	// which fields are delegated.
	for _, target := range targets {
		calledFields := findDelegatedFields(pass, target.name)
		for _, fieldName := range target.validatableKeys {
			if calledFields[fieldName] {
				continue
			}

			qualName := fmt.Sprintf("%s.%s", pkgName, target.name)
			excKey := fmt.Sprintf("%s.%s.validate-delegation", qualName, fieldName)
			if cfg.isExcepted(excKey) {
				continue
			}

			msg := fmt.Sprintf(
				"%s.Validate() does not delegate to field %s which has Validate()",
				qualName, fieldName)
			findingID := StableFindingID(CategoryIncompleteValidateDelegation, qualName, fieldName)
			if bl.ContainsFinding(CategoryIncompleteValidateDelegation, findingID, msg) {
				continue
			}

			reportDiagnostic(pass, target.pos, CategoryIncompleteValidateDelegation, findingID, msg)
		}
	}
}

// findDelegatedFields searches the package for a Validate() method on the
// given type and returns a set of field names that appear in
// `receiver.Field.Validate()` call patterns. Also handles intermediate
// variable assignment: `field := receiver.Field; field.Validate()`.
func findDelegatedFields(pass *analysis.Pass, typeName string) map[string]bool {
	called := make(map[string]bool)

	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Body == nil {
				continue
			}
			recvName := receiverTypeName(fn.Recv.List[0].Type)
			if recvName != typeName || fn.Name.Name != "Validate" {
				continue
			}

			// Get the receiver variable name (e.g., "c" in "func (c *Config)")
			recvVarName := ""
			if len(fn.Recv.List[0].Names) > 0 {
				recvVarName = fn.Recv.List[0].Names[0].Name
			}

			// Pass 1: Direct receiver.Field.Validate() calls.
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Validate" {
					return true
				}
				innerSel, ok := sel.X.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if recvVarName != "" {
					if ident, ok := innerSel.X.(*ast.Ident); ok && ident.Name == recvVarName {
						called[innerSel.Sel.Name] = true
					}
				}
				return true
			})

			// Pass 2: Intermediate variable pattern:
			//   field := receiver.Field
			//   field.Validate()
			fieldAliases := make(map[string]string)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, rhs := range assign.Rhs {
					sel, ok := rhs.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					if recvVarName != "" {
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == recvVarName {
							if i < len(assign.Lhs) {
								if lhsIdent, ok := assign.Lhs[i].(*ast.Ident); ok {
									fieldAliases[lhsIdent.Name] = sel.Sel.Name
								}
							}
						}
					}
				}
				return true
			})
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				callExpr, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
				if !ok || selExpr.Sel.Name != "Validate" {
					return true
				}
				ident, ok := selExpr.X.(*ast.Ident)
				if !ok {
					return true
				}
				if fieldName, ok := fieldAliases[ident.Name]; ok {
					called[fieldName] = true
				}
				return true
			})
		}
	}

	return called
}

// hasValidateAllDirective checks whether a type declaration has the
// //goplint:validate-all directive.
func hasValidateAllDirective(genDoc *ast.CommentGroup, specDoc *ast.CommentGroup) bool {
	return hasDirectiveKey(genDoc, nil, "validate-all") || hasDirectiveKey(specDoc, nil, "validate-all")
}
