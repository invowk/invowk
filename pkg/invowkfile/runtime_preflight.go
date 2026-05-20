// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"strconv"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
)

const runtimePreflightValidatorName ValidatorName = "runtime-preflight"

var nonContainerRuntimeFields = map[string]struct{}{
	"containerfile":   {},
	"depends_on":      {},
	"enable_host_ssh": {},
	"image":           {},
	"persistent":      {},
	"ports":           {},
	"volumes":         {},
}

//goplint:ignore -- CUE parser boundary consumes raw bytes and filename display text.
func runtimeSchemaPreflightValidationErrors(data []byte, path string) ValidationErrors {
	file, err := parser.ParseFile(path, data)
	if err != nil {
		return runtimePreflightParseFallback()
	}
	cmds := fieldList(fieldStruct(file.Decls), "cmds")
	var errs ValidationErrors
	for cmdIdx, cmd := range cmds {
		impls := fieldList(cmd, "implementations")
		for implIdx, impl := range impls {
			runtimes := fieldList(impl, "runtimes")
			for runtimeIdx, runtime := range runtimes {
				errs = append(errs, validateRuntimePreflight(runtime, runtimeFieldPath(cmdIdx, implIdx, runtimeIdx))...)
			}
		}
	}
	return errs
}

//goplint:ignore -- AST preflight helper builds display-only validation paths from parsed CUE syntax.
func validateRuntimePreflight(runtime *ast.StructLit, path string) ValidationErrors {
	if runtime == nil {
		return nil
	}
	name, ok := fieldString(runtime, "name")
	if !ok {
		return nil
	}
	switch name {
	case string(RuntimeNative):
		return validateNonContainerRuntimePreflight(runtime, path, RuntimeNative)
	case string(RuntimeVirtual):
		return validateNonContainerRuntimePreflight(runtime, path, RuntimeVirtual)
	case string(RuntimeContainer):
		return validateContainerRuntimePreflight(runtime, path)
	default:
		return nil
	}
}

//goplint:ignore -- AST preflight helper builds display-only validation paths from parsed CUE syntax.
func validateNonContainerRuntimePreflight(runtime *ast.StructLit, path string, _ RuntimeMode) ValidationErrors {
	var errs ValidationErrors
	for field := range nonContainerRuntimeFields {
		if hasField(runtime, field) {
			errs = append(errs, runtimePreflightError(
				path+"."+field,
				field+" is only valid for container runtime",
			))
		}
	}
	return errs
}

//goplint:ignore -- AST preflight helper builds display-only validation paths from parsed CUE syntax.
func validateContainerRuntimePreflight(runtime *ast.StructLit, path string) ValidationErrors {
	hasImage := hasField(runtime, "image")
	hasContainerfile := hasField(runtime, "containerfile")
	switch {
	case hasImage && hasContainerfile:
		return ValidationErrors{runtimePreflightError(
			path+".image",
			"image and containerfile are mutually exclusive; choose exactly one container source",
		)}
	case !hasImage && !hasContainerfile:
		return ValidationErrors{runtimePreflightError(
			path,
			"container runtime requires either image or containerfile",
		)}
	default:
		return nil
	}
}

func runtimePreflightParseFallback() ValidationErrors {
	return nil
}

//goplint:ignore -- validation error fields are display-only diagnostics assembled from parser-owned state.
func runtimePreflightError(field, message string) ValidationError {
	return ValidationError{
		Validator: runtimePreflightValidatorName,
		Field:     field,
		Message:   message,
		Severity:  SeverityError,
	}
}

//goplint:ignore -- indices format a display-only CUE field path.
func runtimeFieldPath(cmdIdx, implIdx, runtimeIdx int) string {
	return fmt.Sprintf("cmds[%d].implementations[%d].runtimes[%d]", cmdIdx, implIdx, runtimeIdx)
}

func fieldStruct(decls []ast.Decl) *ast.StructLit {
	return &ast.StructLit{Elts: decls}
}

//goplint:ignore -- CUE AST helper accepts literal field names from this validator.
func fieldList(parent *ast.StructLit, name string) []*ast.StructLit {
	expr, ok := fieldExpr(parent, name)
	if !ok {
		return nil
	}
	list, ok := expr.(*ast.ListLit)
	if !ok {
		return nil
	}
	items := make([]*ast.StructLit, 0, len(list.Elts))
	for _, item := range list.Elts {
		if st, ok := item.(*ast.StructLit); ok {
			items = append(items, st)
		}
	}
	return items
}

//goplint:ignore -- CUE AST helper accepts literal field names from this validator.
func fieldString(parent *ast.StructLit, name string) (string, bool) {
	expr, ok := fieldExpr(parent, name)
	if !ok {
		return "", false
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

//goplint:ignore -- CUE AST helper accepts literal field names from this validator.
func hasField(parent *ast.StructLit, name string) bool {
	_, ok := fieldExpr(parent, name)
	return ok
}

//goplint:ignore -- CUE AST helper accepts literal field names from this validator.
func fieldExpr(parent *ast.StructLit, name string) (ast.Expr, bool) {
	if parent == nil {
		return nil, false
	}
	for _, decl := range parent.Elts {
		field, ok := decl.(*ast.Field)
		if !ok {
			continue
		}
		label, _, err := ast.LabelName(field.Label)
		if err != nil || label != name {
			continue
		}
		return field.Value, true
	}
	return nil, false
}
