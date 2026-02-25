// SPDX-License-Identifier: MPL-2.0

// Package primitivelint implements a go/analysis analyzer that detects
// bare primitive type usage in struct fields, function parameters, and
// return types. It is designed to enforce DDD Value Type conventions
// where named types (e.g., type CommandName string) should be used
// instead of raw string, int, etc.
//
// The analyzer supports an exception mechanism via TOML config file
// and inline //primitivelint:ignore directives for intentional
// primitive usage at exec/OS boundaries, display-only fields, etc.
//
// Additional modes:
//   - --audit-exceptions: report exception patterns that matched zero locations
//   - --check-isvalid: report named non-struct types missing IsValid() method
//   - --check-stringer: report named non-struct types missing String() method
//   - --check-constructors: report exported structs missing NewXxx() constructor
package primitivelint

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Diagnostic category constants for structured JSON output.
// These appear in the "category" field of analysis.Diagnostic
// when using -json mode, enabling agents to filter by finding type.
const (
	CategoryPrimitive          = "primitive"
	CategoryMissingIsValid     = "missing-isvalid"
	CategoryMissingStringer    = "missing-stringer"
	CategoryMissingConstructor = "missing-constructor"
	CategoryStaleException     = "stale-exception"
)

// Flag binding variables for the analyzer's flag set. These are populated
// by the go/analysis framework during flag parsing via BoolVar/StringVar.
// The run() function never reads or mutates these directly — it reads them
// once via newRunConfig() into a local struct.
var (
	configPath        string
	baselinePath      string
	auditExceptions   bool
	checkAll          bool
	checkIsValid      bool
	checkStringer     bool
	checkConstructors bool
)

// Analyzer is the primitivelint analysis pass. Use it with singlechecker
// or multichecker, or via go vet -vettool.
var Analyzer = &analysis.Analyzer{
	Name:     "primitivelint",
	Doc:      "reports bare primitive types where DDD Value Types should be used",
	URL:      "https://github.com/invowk/invowk/tools/primitivelint",
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func init() {
	Analyzer.Flags.StringVar(&configPath, "config", "",
		"path to exceptions TOML config file")
	Analyzer.Flags.StringVar(&baselinePath, "baseline", "",
		"path to baseline TOML file (suppress known findings, report only new ones)")
	Analyzer.Flags.BoolVar(&auditExceptions, "audit-exceptions", false,
		"report exception patterns that matched zero locations (stale entries)")
	Analyzer.Flags.BoolVar(&checkIsValid, "check-isvalid", false,
		"report named non-struct types missing IsValid() (bool, []error) method")
	Analyzer.Flags.BoolVar(&checkStringer, "check-stringer", false,
		"report named non-struct types missing String() string method")
	Analyzer.Flags.BoolVar(&checkConstructors, "check-constructors", false,
		"report exported struct types missing NewXxx() constructor function")
	Analyzer.Flags.BoolVar(&checkAll, "check-all", false,
		"enable all DDD compliance checks (isvalid + stringer + constructors)")
}

// runConfig holds the resolved flag values for a single run() invocation.
// Reading flag bindings into this struct at run() entry ensures run()
// never reads or mutates package-level state directly.
type runConfig struct {
	configPath        string
	baselinePath      string
	auditExceptions   bool
	checkIsValid      bool
	checkStringer     bool
	checkConstructors bool
}

// newRunConfig reads the current flag binding values into a local config
// struct and applies the --check-all expansion. The expansion happens on
// the local struct, never mutating the package-level flag variables.
func newRunConfig() runConfig {
	rc := runConfig{
		configPath:        configPath,
		baselinePath:      baselinePath,
		auditExceptions:   auditExceptions,
		checkIsValid:      checkIsValid,
		checkStringer:     checkStringer,
		checkConstructors: checkConstructors,
	}
	// Expand --check-all into individual supplementary checks.
	// Deliberately excludes --audit-exceptions which is a config
	// maintenance tool with per-package false positives.
	if checkAll {
		rc.checkIsValid = true
		rc.checkStringer = true
		rc.checkConstructors = true
	}
	return rc
}

func run(pass *analysis.Pass) (interface{}, error) {
	rc := newRunConfig()

	cfg, err := loadConfig(rc.configPath)
	if err != nil {
		return nil, err
	}

	bl, err := loadBaseline(rc.baselinePath)
	if err != nil {
		return nil, err
	}

	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Collectors for the supplementary check modes. Built during the
	// same AST traversal as the primary primitive check and evaluated
	// after the traversal completes.
	var (
		namedTypes      []namedTypeInfo      // non-struct named types (for isvalid/stringer)
		methodSeen      map[string]bool      // "TypeName.MethodName" → true
		exportedStructs []exportedStructInfo // exported struct types (for constructors)
		constructorSeen map[string]bool      // "NewTypeName" → true
	)

	if rc.checkIsValid || rc.checkStringer {
		methodSeen = make(map[string]bool)
	}
	if rc.checkConstructors {
		constructorSeen = make(map[string]bool)
	}

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

			// Supplementary: collect named types and exported structs.
			if rc.checkIsValid || rc.checkStringer {
				collectNamedTypes(pass, n, &namedTypes)
			}
			if rc.checkConstructors {
				collectExportedStructs(pass, n, &exportedStructs)
			}

		case *ast.FuncDecl:
			// Primary mode: check func params and returns for primitives.
			inspectFuncDecl(pass, n, cfg, bl)

			// Supplementary: track methods and constructor functions.
			if rc.checkIsValid || rc.checkStringer {
				trackMethods(n, methodSeen)
			}
			if rc.checkConstructors {
				trackConstructors(n, constructorSeen)
			}
		}
	})

	// Post-traversal checks for supplementary modes.
	if rc.checkIsValid {
		reportMissingIsValid(pass, namedTypes, methodSeen, cfg, bl)
	}
	if rc.checkStringer {
		reportMissingStringer(pass, namedTypes, methodSeen, cfg, bl)
	}
	if rc.checkConstructors {
		reportMissingConstructors(pass, exportedStructs, constructorSeen, cfg, bl)
	}
	if rc.auditExceptions {
		reportStaleExceptionsInline(pass, cfg)
	}

	return nil, nil
}

// namedTypeInfo records a non-struct named type definition for
// IsValid/Stringer checking.
type namedTypeInfo struct {
	name     string    // unqualified type name (e.g., "CommandName")
	pos      token.Pos // position for diagnostics
	exported bool      // whether the type name is exported
}

// exportedStructInfo records an exported struct type for constructor checking.
type exportedStructInfo struct {
	name string    // unqualified type name (e.g., "Config")
	pos  token.Pos // position for diagnostics
}

// collectNamedTypes extracts non-struct named type definitions from a
// GenDecl. These are the DDD Value Types themselves (type Foo string)
// that should have IsValid() and String() methods.
//
// Skips type aliases (type X = Y) since they inherit methods from the
// aliased type.
func collectNamedTypes(pass *analysis.Pass, node *ast.GenDecl, out *[]namedTypeInfo) {
	if node.Tok != token.TYPE {
		return
	}

	for _, spec := range node.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		// Skip type aliases — they inherit methods from the target type.
		if ts.Assign.IsValid() {
			continue
		}

		// Skip struct types — they use composite IsValid() delegation.
		if _, isStruct := ts.Type.(*ast.StructType); isStruct {
			continue
		}

		// Skip interface types.
		if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
			continue
		}

		// Only check types backed by primitives (string, int, etc.)
		// to avoid flagging types like func aliases or channel types.
		typeObj := pass.TypesInfo.TypeOf(ts.Type)
		if typeObj == nil {
			continue
		}
		if !isPrimitiveUnderlying(typeObj) {
			continue
		}

		*out = append(*out, namedTypeInfo{
			name:     ts.Name.Name,
			pos:      ts.Name.Pos(),
			exported: ts.Name.IsExported(),
		})
	}
}

// collectExportedStructs extracts exported struct type definitions.
func collectExportedStructs(pass *analysis.Pass, node *ast.GenDecl, out *[]exportedStructInfo) {
	if node.Tok != token.TYPE {
		return
	}

	for _, spec := range node.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		if _, isStruct := ts.Type.(*ast.StructType); !isStruct {
			continue
		}

		// Only exported structs.
		if !ts.Name.IsExported() {
			continue
		}

		*out = append(*out, exportedStructInfo{
			name: ts.Name.Name,
			pos:  ts.Name.Pos(),
		})
	}
}

// trackMethods records method names keyed by receiver type for the
// IsValid/Stringer checks.
func trackMethods(fn *ast.FuncDecl, seen map[string]bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return
	}
	recvName := receiverTypeName(fn.Recv.List[0].Type)
	if recvName == "" {
		return
	}
	seen[recvName+"."+fn.Name.Name] = true
}

// trackConstructors records function names that follow the NewXxx pattern.
func trackConstructors(fn *ast.FuncDecl, seen map[string]bool) {
	if fn.Recv != nil {
		return // methods are not constructors
	}
	name := fn.Name.Name
	if strings.HasPrefix(name, "New") && len(name) > 3 {
		seen[name] = true
	}
}

// reportMissingIsValid reports named non-struct types that lack an
// IsValid() method. For unexported types, also checks for isValid()
// (lowercase), matching the project convention.
func reportMissingIsValid(pass *analysis.Pass, types []namedTypeInfo, methods map[string]bool, cfg *ExceptionConfig, bl *BaselineConfig) {
	pkgName := packageName(pass.Pkg)
	for _, t := range types {
		// Check for IsValid (exported) or isValid (unexported convention).
		if methods[t.name+".IsValid"] || (!t.exported && methods[t.name+".isValid"]) {
			continue
		}

		qualName := fmt.Sprintf("%s.%s", pkgName, t.name)
		if cfg.isExcepted(qualName + ".IsValid") {
			continue
		}

		msg := fmt.Sprintf("named type %s has no IsValid() method", qualName)
		if bl.Contains(CategoryMissingIsValid, msg) {
			continue
		}

		pass.Report(analysis.Diagnostic{
			Pos:      t.pos,
			Category: CategoryMissingIsValid,
			Message:  msg,
		})
	}
}

// reportMissingStringer reports named non-struct types that lack a
// String() method. The String() method name is always capitalized
// regardless of type visibility (it implements fmt.Stringer).
func reportMissingStringer(pass *analysis.Pass, types []namedTypeInfo, methods map[string]bool, cfg *ExceptionConfig, bl *BaselineConfig) {
	pkgName := packageName(pass.Pkg)
	for _, t := range types {
		if methods[t.name+".String"] {
			continue
		}

		qualName := fmt.Sprintf("%s.%s", pkgName, t.name)
		if cfg.isExcepted(qualName + ".String") {
			continue
		}

		msg := fmt.Sprintf("named type %s has no String() method", qualName)
		if bl.Contains(CategoryMissingStringer, msg) {
			continue
		}

		pass.Report(analysis.Diagnostic{
			Pos:      t.pos,
			Category: CategoryMissingStringer,
			Message:  msg,
		})
	}
}

// reportMissingConstructors reports exported struct types that lack a
// NewXxx() constructor function in the same package.
func reportMissingConstructors(pass *analysis.Pass, structs []exportedStructInfo, ctors map[string]bool, cfg *ExceptionConfig, bl *BaselineConfig) {
	pkgName := packageName(pass.Pkg)
	for _, s := range structs {
		ctorName := "New" + s.name
		if ctors[ctorName] {
			continue
		}

		qualName := fmt.Sprintf("%s.%s", pkgName, s.name)
		if cfg.isExcepted(qualName + ".constructor") {
			continue
		}

		msg := fmt.Sprintf("exported struct %s has no %s() constructor", qualName, ctorName)
		if bl.Contains(CategoryMissingConstructor, msg) {
			continue
		}

		pass.Report(analysis.Diagnostic{
			Pos:      s.pos,
			Category: CategoryMissingConstructor,
			Message:  msg,
		})
	}
}

// reportStaleExceptionsInline reports stale exceptions via pass.Reportf.
// Since go/analysis runs per-package, this reports exceptions that matched
// zero locations within the current package only. For cross-package auditing,
// pipe the output through sort -u.
func reportStaleExceptionsInline(pass *analysis.Pass, cfg *ExceptionConfig) {
	stale := cfg.staleExceptions()
	if len(stale) == 0 || len(pass.Files) == 0 {
		return
	}

	pos := pass.Files[0].Package

	for _, idx := range stale {
		exc := cfg.Exceptions[idx]
		pass.Report(analysis.Diagnostic{
			Pos:      pos,
			Category: CategoryStaleException,
			Message: fmt.Sprintf(
				"stale exception: pattern %q matched no diagnostics (reason: %s)",
				exc.Pattern, exc.Reason),
		})
	}
}
