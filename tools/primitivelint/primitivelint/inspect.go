// SPDX-License-Identifier: MPL-2.0

package primitivelint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// inspectStructFields checks all named struct types in the file for fields
// using bare primitive types. Findings present in the baseline are suppressed.
func inspectStructFields(pass *analysis.Pass, node *ast.GenDecl, cfg *ExceptionConfig, bl *BaselineConfig) {
	if node.Tok != token.TYPE {
		return
	}

	pkgName := packageName(pass.Pkg)

	for _, spec := range node.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			// Not a struct — could be a type definition like "type X string".
			// These are the DDD types themselves; skip.
			continue
		}

		structName := pkgName + "." + ts.Name.Name

		for _, field := range st.Fields.List {
			if hasIgnoreDirective(field.Doc, field.Comment) {
				continue
			}

			fieldType := pass.TypesInfo.TypeOf(field.Type)
			if fieldType == nil {
				continue
			}

			if !isPrimitive(fieldType) {
				continue
			}

			typeName := primitiveTypeName(fieldType)
			if cfg.isSkippedType(typeName) {
				continue
			}

			for _, name := range field.Names {
				qualName := fmt.Sprintf("%s.%s", structName, name.Name)
				if cfg.isExcepted(qualName) {
					continue
				}

				msg := fmt.Sprintf("struct field %s uses primitive type %s", qualName, typeName)
				if bl.Contains(CategoryPrimitive, msg) {
					continue
				}

				pass.Report(analysis.Diagnostic{
					Pos:      name.Pos(),
					Category: CategoryPrimitive,
					Message:  msg,
				})
			}

			// Anonymous/embedded fields (no names)
			if len(field.Names) == 0 {
				qualName := fmt.Sprintf("%s.(embedded)", structName)
				if cfg.isExcepted(qualName) {
					continue
				}

				msg := fmt.Sprintf("struct field %s uses primitive type %s", qualName, typeName)
				if bl.Contains(CategoryPrimitive, msg) {
					continue
				}

				pass.Report(analysis.Diagnostic{
					Pos:      field.Pos(),
					Category: CategoryPrimitive,
					Message:  msg,
				})
			}
		}
	}
}

// inspectFuncDecl checks function/method parameters and return types for
// bare primitive types. Findings present in the baseline are suppressed.
func inspectFuncDecl(pass *analysis.Pass, fn *ast.FuncDecl, cfg *ExceptionConfig, bl *BaselineConfig) {
	if shouldSkipFunc(fn) {
		return
	}

	if hasIgnoreDirective(fn.Doc, nil) {
		return
	}

	pkgName := packageName(pass.Pkg)
	funcName := fn.Name.Name

	// If it's a method, prefix with the receiver type name.
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recvName := receiverTypeName(fn.Recv.List[0].Type)
		if recvName != "" {
			funcName = recvName + "." + funcName
		}
	}

	// Prefix with package name for exception matching.
	funcName = pkgName + "." + funcName

	// Check parameters
	if fn.Type.Params != nil {
		inspectFieldList(pass, fn.Type.Params, funcName, "parameter", cfg, bl)
	}

	// Check return types — skip for well-known interface methods
	// (String, Error, GoString, MarshalText) whose return types are
	// dictated by the interface contract.
	if fn.Type.Results != nil && !isInterfaceMethodReturn(fn) {
		inspectReturnTypes(pass, fn.Type.Results, funcName, cfg, bl)
	}
}

// inspectFieldList checks a function's parameter list for primitive types.
// Findings present in the baseline are suppressed.
func inspectFieldList(pass *analysis.Pass, fields *ast.FieldList, funcName, kind string, cfg *ExceptionConfig, bl *BaselineConfig) {
	for _, field := range fields.List {
		if hasIgnoreDirective(field.Doc, field.Comment) {
			continue
		}

		fieldType := pass.TypesInfo.TypeOf(field.Type)
		if fieldType == nil {
			continue
		}

		if !isPrimitive(fieldType) {
			continue
		}

		typeName := primitiveTypeName(fieldType)
		if cfg.isSkippedType(typeName) {
			continue
		}

		for _, name := range field.Names {
			qualName := fmt.Sprintf("%s.%s", funcName, name.Name)
			if cfg.isExcepted(qualName) {
				continue
			}

			msg := fmt.Sprintf("%s %q of %s uses primitive type %s", kind, name.Name, funcName, typeName)
			if bl.Contains(CategoryPrimitive, msg) {
				continue
			}

			pass.Report(analysis.Diagnostic{
				Pos:      name.Pos(),
				Category: CategoryPrimitive,
				Message:  msg,
			})
		}

		// Unnamed parameters (e.g., func(string))
		if len(field.Names) == 0 {
			qualName := fmt.Sprintf("%s.(unnamed)", funcName)
			if cfg.isExcepted(qualName) {
				continue
			}

			msg := fmt.Sprintf("unnamed %s of %s uses primitive type %s", kind, funcName, typeName)
			if bl.Contains(CategoryPrimitive, msg) {
				continue
			}

			pass.Report(analysis.Diagnostic{
				Pos:      field.Pos(),
				Category: CategoryPrimitive,
				Message:  msg,
			})
		}
	}
}

// inspectReturnTypes checks a function's return types for primitive types.
// Findings present in the baseline are suppressed.
func inspectReturnTypes(pass *analysis.Pass, results *ast.FieldList, funcName string, cfg *ExceptionConfig, bl *BaselineConfig) {
	for i, field := range results.List {
		if hasIgnoreDirective(field.Doc, field.Comment) {
			continue
		}

		fieldType := pass.TypesInfo.TypeOf(field.Type)
		if fieldType == nil {
			continue
		}

		if !isPrimitive(fieldType) {
			continue
		}

		typeName := primitiveTypeName(fieldType)
		if cfg.isSkippedType(typeName) {
			continue
		}

		// Named return values
		for _, name := range field.Names {
			qualName := fmt.Sprintf("%s.return.%s", funcName, name.Name)
			if cfg.isExcepted(qualName) {
				continue
			}

			msg := fmt.Sprintf("return value %q of %s uses primitive type %s", name.Name, funcName, typeName)
			if bl.Contains(CategoryPrimitive, msg) {
				continue
			}

			pass.Report(analysis.Diagnostic{
				Pos:      name.Pos(),
				Category: CategoryPrimitive,
				Message:  msg,
			})
		}

		// Unnamed return values
		if len(field.Names) == 0 {
			qualName := fmt.Sprintf("%s.return.%d", funcName, i)
			if cfg.isExcepted(qualName) {
				continue
			}

			msg := fmt.Sprintf("return value of %s uses primitive type %s", funcName, typeName)
			if bl.Contains(CategoryPrimitive, msg) {
				continue
			}

			pass.Report(analysis.Diagnostic{
				Pos:      field.Pos(),
				Category: CategoryPrimitive,
				Message:  msg,
			})
		}
	}
}

// shouldSkipFunc returns true for functions that should not be analyzed:
// init, main, test functions, and generated code.
func shouldSkipFunc(fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	switch {
	case name == "init" || name == "main":
		return true
	case strings.HasPrefix(name, "Test"):
		return true
	case strings.HasPrefix(name, "Benchmark"):
		return true
	case strings.HasPrefix(name, "Fuzz"):
		return true
	case strings.HasPrefix(name, "Example"):
		return true
	default:
		return false
	}
}

// isInterfaceMethodReturn returns true if the function is a method whose
// return type is dictated by a well-known interface contract. These methods
// MUST return string by the interface definition and cannot use named types.
//
// Skipped patterns:
//   - String() string (fmt.Stringer)
//   - Error() string (error interface)
//   - GoString() string (fmt.GoStringer)
//   - MarshalText() ([]byte, error) (encoding.TextMarshaler)
func isInterfaceMethodReturn(fn *ast.FuncDecl) bool {
	if fn.Recv == nil {
		return false
	}
	name := fn.Name.Name
	return name == "String" || name == "Error" || name == "GoString" || name == "MarshalText"
}

// receiverTypeName extracts the type name from a method receiver expression.
// Handles both value receivers (T) and pointer receivers (*T).
func receiverTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		// Recurse to handle both *T (simple pointer receiver) and
		// *T[P] (pointer to generic receiver).
		return receiverTypeName(e.X)
	case *ast.IndexExpr:
		// Generic type: T[P]
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.IndexListExpr:
		// Generic type: T[P1, P2]
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// hasIgnoreDirective checks whether a field/func has a //plint:ignore,
// //primitivelint:ignore, or //nolint:primitivelint directive.
func hasIgnoreDirective(doc *ast.CommentGroup, lineComment *ast.CommentGroup) bool {
	return hasCommentDirective(doc, lineComment, "plint:ignore", "primitivelint:ignore", "nolint:primitivelint")
}

// hasInternalDirective checks whether a struct field has a //plint:internal
// directive, indicating the field is internal state that should not be
// initialized via functional options (excluded from WithXxx() checks).
func hasInternalDirective(doc *ast.CommentGroup, lineComment *ast.CommentGroup) bool {
	return hasCommentDirective(doc, lineComment, "plint:internal")
}

// hasCommentDirective checks whether any of the given directive substrings
// appear in the doc or line comment groups.
func hasCommentDirective(doc *ast.CommentGroup, lineComment *ast.CommentGroup, directives ...string) bool {
	for _, cg := range []*ast.CommentGroup{doc, lineComment} {
		if cg == nil {
			continue
		}
		for _, c := range cg.List {
			text := strings.TrimSpace(c.Text)
			for _, d := range directives {
				if strings.Contains(text, d) {
					return true
				}
			}
		}
	}
	return false
}

// isTestFile returns true if the filename ends with _test.go.
func isTestFile(pass *analysis.Pass, pos token.Pos) bool {
	file := pass.Fset.Position(pos).Filename
	return strings.HasSuffix(file, "_test.go")
}

// packageName extracts the last segment of a package path for qualified names.
func packageName(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	path := pkg.Path()
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
