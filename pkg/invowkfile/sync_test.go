// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// behavioralSyncCase defines a single input for behavioral equivalence testing.
// Used by TestBehavioralSync_* tests across sync_test.go and sync_runtime_test.go.
type behavioralSyncCase struct {
	input       string // The value to test
	goExpect    bool   // true = Go Validate() should return nil
	cueExpect   bool   // true = CUE should accept
	divergeNote string // non-empty = expected CUE/Go divergence, skip agreement check
}

// extractCUEFields extracts all field names from a CUE struct definition.
// It returns a map of field names to whether the field is optional.
// Nested struct fields are not included; only top-level fields of the given definition.
func extractCUEFields(t *testing.T, val cue.Value) map[string]bool {
	t.Helper()

	fields := make(map[string]bool)

	// Iterate over the struct fields
	iter, err := val.Fields(cue.Definitions(false), cue.Optional(true))
	if err != nil {
		t.Fatalf("failed to iterate CUE fields: %v", err)
	}

	for iter.Next() {
		sel := iter.Selector()
		// Skip hidden fields (start with _) and definitions (start with #)
		labelType := sel.LabelType()
		if labelType.IsHidden() || sel.IsDefinition() {
			continue
		}

		// Skip fields that are explicitly set to bottom (_|_) - these are error constraints
		// used to explicitly forbid certain field names (e.g., module, version in Invowkfile).
		// We detect these by checking if the error message contains "explicit error (_|_ literal)".
		// This distinguishes between:
		// - "explicitly _|_" (module?: _|_) → skip, not a real field
		// - "constraint evaluation error" (containerfile with strings.HasPrefix) → include, valid field
		fieldValue := iter.Value()
		if fieldValue.Kind() == cue.BottomKind && fieldValue.Err() != nil {
			errMsg := fieldValue.Err().Error()
			if strings.Contains(errMsg, "explicit error (_|_ literal)") {
				continue
			}
		}

		// The selector string may include the "?" suffix for optional fields
		// We need to strip it to get the actual field name
		fieldName := sel.String()
		fieldName = strings.TrimSuffix(fieldName, "?")
		isOptional := iter.IsOptional()
		fields[fieldName] = isOptional
	}

	return fields
}

// extractGoJSONTags extracts all JSON field names from a Go struct using reflection.
// It returns a map of JSON tag names to whether the field has "omitempty".
// Fields with json:"-" are excluded.
// Embedded structs are not expanded; only direct fields are returned.
func extractGoJSONTags(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()

	// Dereference pointer types
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		t.Fatalf("expected struct type, got %s", typ.Kind())
	}

	fields := make(map[string]bool)

	for field := range typ.Fields() {
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			// No json tag or explicitly excluded
			continue
		}

		// Parse the tag: "name,omitempty" or just "name"
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "" || name == "-" {
			continue
		}

		hasOmitempty := slices.Contains(parts[1:], "omitempty")

		fields[name] = hasOmitempty
	}

	return fields
}

// assertFieldsSync verifies that CUE schema fields and Go struct JSON tags are in sync.
// It checks:
// 1. Every CUE field has a corresponding Go JSON tag
// 2. Every Go JSON tag has a corresponding CUE field
// 3. Optional/omitempty alignment (warning only, not a failure)
func assertFieldsSync(t *testing.T, structName string, cueFields, goFields map[string]bool) {
	t.Helper()

	// Check CUE fields exist in Go struct
	for field, isOptional := range cueFields {
		hasOmitempty, exists := goFields[field]
		if !exists {
			t.Errorf("[%s] CUE field %q not found in Go struct (missing JSON tag)", structName, field)
			continue
		}
		// Warn about optional/omitempty mismatch (not a hard failure)
		if isOptional && !hasOmitempty {
			t.Logf("[%s] Note: CUE field %q is optional but Go field lacks omitempty tag", structName, field)
		}
	}

	// Check Go fields exist in CUE schema
	for field := range goFields {
		if _, exists := cueFields[field]; !exists {
			t.Errorf("[%s] Go JSON tag %q not found in CUE schema (missing CUE field)", structName, field)
		}
	}
}

// getCUESchema compiles the embedded CUE schema and returns the context and compiled value.
func getCUESchema(t *testing.T) (cue.Value, *cue.Context) {
	t.Helper()

	ctx := cuecontext.New()
	schema := ctx.CompileString(invowkfileSchema)
	if schema.Err() != nil {
		t.Fatalf("failed to compile CUE schema: %v", schema.Err())
	}

	return schema, ctx
}

// lookupDefinition looks up a CUE definition by path (e.g., "#Invowkfile").
func lookupDefinition(t *testing.T, schema cue.Value, defPath string) cue.Value {
	t.Helper()

	def := schema.LookupPath(cue.ParsePath(defPath))
	if def.Err() != nil {
		t.Fatalf("failed to lookup CUE definition %s: %v", defPath, def.Err())
	}

	return def
}

// TestSyncHelpersSmoke verifies the sync test helpers work correctly.
// This is a smoke test - the actual sync tests for each struct are below.
func TestSyncHelpersSmoke(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)

	// Test lookupDefinition
	invowkfileDef := lookupDefinition(t, schema, "#Invowkfile")
	if invowkfileDef.Err() != nil {
		t.Errorf("failed to lookup #Invowkfile: %v", invowkfileDef.Err())
	}

	// Test extractCUEFields on #Invowkfile
	cueFields := extractCUEFields(t, invowkfileDef)
	if len(cueFields) == 0 {
		t.Errorf("extractCUEFields returned no fields for #Invowkfile")
	}
	// Verify known field exists
	if _, ok := cueFields["cmds"]; !ok {
		t.Errorf("expected 'cmds' field in #Invowkfile CUE definition")
	}

	// Test extractGoJSONTags on Invowkfile struct
	goFields := extractGoJSONTags(t, reflect.TypeFor[Invowkfile]())
	if len(goFields) == 0 {
		t.Errorf("extractGoJSONTags returned no fields for Invowkfile")
	}
	// Verify known field exists
	if _, ok := goFields["cmds"]; !ok {
		t.Errorf("expected 'cmds' JSON tag in Invowkfile struct")
	}

	// Test assertFieldsSync doesn't panic (actual sync tests are below)
	// We just verify the helper runs without crashing
	assertFieldsSync(t, "Invowkfile-smoke", cueFields, goFields)
}

// =============================================================================
// Schema Sync Tests - Phase 3 (T007-T012)
// =============================================================================
// These tests verify Go struct JSON tags match CUE schema field names.
// They catch misalignments at CI time, preventing silent parsing failures.

// TestSchemaSync is a table-driven test that verifies each Go struct's JSON tags
// match its corresponding CUE schema definition fields.
func TestSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)

	cases := []struct {
		cueDef string
		goType reflect.Type
	}{
		{"#Invowkfile", reflect.TypeFor[Invowkfile]()},
		{"#Command", reflect.TypeFor[Command]()},
		{"#Implementation", reflect.TypeFor[Implementation]()},
		{"#DependsOn", reflect.TypeFor[DependsOn]()},
		{"#Flag", reflect.TypeFor[Flag]()},
		{"#Argument", reflect.TypeFor[Argument]()},
		{"#EnvConfig", reflect.TypeFor[EnvConfig]()},
		{"#PlatformConfig", reflect.TypeFor[PlatformConfig]()},
		{"#ToolDependency", reflect.TypeFor[ToolDependency]()},
		{"#FilepathDependency", reflect.TypeFor[FilepathDependency]()},
		{"#CapabilityDependency", reflect.TypeFor[CapabilityDependency]()},
		{"#CommandDependency", reflect.TypeFor[CommandDependency]()},
		{"#EnvVarCheck", reflect.TypeFor[EnvVarCheck]()},
		{"#EnvVarDependency", reflect.TypeFor[EnvVarDependency]()},
		{"#CustomCheck", reflect.TypeFor[CustomCheck]()},
		{"#WatchConfig", reflect.TypeFor[WatchConfig]()},
	}

	for _, tc := range cases {
		t.Run(tc.cueDef, func(t *testing.T) {
			t.Parallel()
			cueFields := extractCUEFields(t, lookupDefinition(t, schema, tc.cueDef))
			goFields := extractGoJSONTags(t, tc.goType)
			assertFieldsSync(t, tc.cueDef, cueFields, goFields)
		})
	}
}

// =============================================================================
// Constraint Boundary Tests - Phase 5 (T090-T094)
// =============================================================================
// These tests verify CUE schema constraints reject invalid values at parse time.

// validateCUE is a helper that attempts to validate data against the CUE schema.
// Returns nil if validation succeeds, error if validation fails.
func validateCUE(t *testing.T, cueData string) error {
	t.Helper()

	ctx := cuecontext.New()

	// Compile schema
	schemaValue := ctx.CompileString(invowkfileSchema)
	if schemaValue.Err() != nil {
		t.Fatalf("failed to compile schema: %v", schemaValue.Err())
	}

	// Compile user data
	userValue := ctx.CompileString(cueData)
	if userValue.Err() != nil {
		return fmt.Errorf("CUE compile error: %w", userValue.Err())
	}

	// Get the #Invowkfile definition
	schemaDef := schemaValue.LookupPath(cue.ParsePath("#Invowkfile"))
	if schemaDef.Err() != nil {
		t.Fatalf("failed to lookup #Invowkfile: %v", schemaDef.Err())
	}

	// Unify and validate
	unified := schemaDef.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("CUE validation error: %w", err)
	}
	return nil
}

// TestDefaultValueLengthConstraint verifies default_value fields have a 4096 rune limit.
// T092: Add boundary tests for default_value length constraint (4096 chars)
func TestDefaultValueLengthConstraint(t *testing.T) {
	t.Parallel()

	// Test flag default_value - exactly 4096 characters should pass
	val4096 := strings.Repeat("a", 4096)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	flags: [{
		name: "myflag"
		description: "A test flag"
		default_value: "` + val4096 + `"
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("4096-char flag default_value should be valid, got error: %v", err)
	}

	// 4097 characters should fail
	val4097 := strings.Repeat("a", 4097)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	flags: [{
		name: "myflag"
		description: "A test flag"
		default_value: "` + val4097 + `"
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("4097-char flag default_value should fail validation, but passed")
	}

	// Test argument default_value - exactly 4096 characters should pass
	validArg := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	args: [{
		name: "myarg"
		description: "A test argument"
		default_value: "` + val4096 + `"
	}]
}]`
	if err := validateCUE(t, validArg); err != nil {
		t.Errorf("4096-char argument default_value should be valid, got error: %v", err)
	}

	// 4097 characters should fail for arguments too
	invalidArg := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	args: [{
		name: "myarg"
		description: "A test argument"
		default_value: "` + val4097 + `"
	}]
}]`
	if err := validateCUE(t, invalidArg); err == nil {
		t.Errorf("4097-char argument default_value should fail validation, but passed")
	}
}

// TestDescriptionNonEmptyWithContentConstraint verifies description fields reject empty/whitespace strings.
// T093: Add non-empty-with-content validation tests for Command.description
func TestDescriptionNonEmptyWithContentConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		description string
		shouldPass  bool
	}{
		{"valid description", "A valid description", true},
		{"description with leading space", "  Valid description", true},
		{"description with trailing space", "Valid description  ", true},
		{"single word", "Valid", true},
		{"empty string", "", false},
		{"whitespace only - single space", " ", false},
		{"whitespace only - multiple spaces", "   ", false},
		{"whitespace only - tab", "\t", false},
		{"whitespace only - newline", "\n", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Escape special characters for CUE string literal
			escapedDesc := strings.ReplaceAll(tc.description, "\\", "\\\\")
			escapedDesc = strings.ReplaceAll(escapedDesc, "\"", "\\\"")
			escapedDesc = strings.ReplaceAll(escapedDesc, "\n", "\\n")
			escapedDesc = strings.ReplaceAll(escapedDesc, "\t", "\\t")

			cueData := `
cmds: [{
	name: "test"
	description: "` + escapedDesc + `"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
			err := validateCUE(t, cueData)
			if tc.shouldPass && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tc.shouldPass && err == nil {
				t.Errorf("expected invalid, but validation passed")
			}
		})
	}
}

// =============================================================================
// Constraint Boundary Tests - Phase 5 (Extended)
// =============================================================================
// These tests verify additional CUE schema constraints at their exact boundaries.

// TestCommandNameLengthConstraint verifies #Command.name has a 256 rune limit.
func TestCommandNameLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 256 characters should pass (must match ^[a-zA-Z][a-zA-Z0-9_ -]*$)
	name256 := "a" + strings.Repeat("b", 255)
	valid := `
cmds: [{
	name: "` + name256 + `"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("256-char command name should be valid, got error: %v", err)
	}

	// 257 characters should fail
	name257 := "a" + strings.Repeat("b", 256)
	invalid := `
cmds: [{
	name: "` + name257 + `"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("257-char command name should fail validation, but passed")
	}
}

// TestCommandDescriptionLengthConstraint verifies #Command.description has a 10240 rune limit.
func TestCommandDescriptionLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 10240 characters should pass (must match ^\\s*\\S.*$)
	desc10240 := "A" + strings.Repeat("a", 10239)
	valid := `
cmds: [{
	name: "test"
	description: "` + desc10240 + `"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("10240-char description should be valid, got error: %v", err)
	}

	// 10241 characters should fail
	desc10241 := "A" + strings.Repeat("a", 10240)
	invalid := `
cmds: [{
	name: "test"
	description: "` + desc10241 + `"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("10241-char description should fail validation, but passed")
	}
}

// TestArgumentValidationLengthConstraint verifies #Argument.validation has a 1000 rune limit.
func TestArgumentValidationLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 1000 characters should pass
	validation1000 := strings.Repeat("a", 1000)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	args: [{
		name: "myarg"
		description: "A test argument"
		validation: "` + validation1000 + `"
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("1000-char argument validation should be valid, got error: %v", err)
	}

	// 1001 characters should fail
	validation1001 := strings.Repeat("a", 1001)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	args: [{
		name: "myarg"
		description: "A test argument"
		validation: "` + validation1001 + `"
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("1001-char argument validation should fail validation, but passed")
	}
}

// TestFlagValidationLengthConstraint verifies #Flag.validation has a 1000 rune limit.
func TestFlagValidationLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 1000 characters should pass
	validation1000 := strings.Repeat("a", 1000)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	flags: [{
		name: "myflag"
		description: "A test flag"
		validation: "` + validation1000 + `"
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("1000-char flag validation should be valid, got error: %v", err)
	}

	// 1001 characters should fail
	validation1001 := strings.Repeat("a", 1001)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	flags: [{
		name: "myflag"
		description: "A test flag"
		validation: "` + validation1001 + `"
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("1001-char flag validation should fail validation, but passed")
	}
}

// =============================================================================
// Behavioral Sync Tests — CUE Oracle
// =============================================================================
// These tests verify that Go Validate() methods and CUE schema constraints
// produce the same accept/reject verdict on identical inputs. This catches
// behavioral drift that structural sync tests and individual boundary tests
// cannot detect — for example, a regex difference of one character between
// CUE and Go would pass both test suites independently.

// lookupCUEFieldConstraint extracts the constraint value for a specific field
// within a CUE struct definition. Unlike LookupPath, this handles optional
// fields by iterating with cue.Optional(true).
func lookupCUEFieldConstraint(t *testing.T, schema cue.Value, parentPath, fieldName string) cue.Value {
	t.Helper()

	parent := schema.LookupPath(cue.ParsePath(parentPath))
	if parent.Err() != nil {
		t.Fatalf("CUE parent %s not found: %v", parentPath, parent.Err())
	}

	iter, err := parent.Fields(cue.Optional(true))
	if err != nil {
		t.Fatalf("failed to iterate fields of %s: %v", parentPath, err)
	}

	for iter.Next() {
		sel := iter.Selector()
		name := strings.TrimSuffix(sel.String(), "?")
		if name == fieldName {
			return iter.Value()
		}
	}

	t.Fatalf("CUE field %s not found in %s", fieldName, parentPath)
	return cue.Value{} // unreachable
}

// subtestLabel returns a truncated, human-readable label for subtest names.
func subtestLabel(input string) string {
	if len(input) > 30 {
		return input[:27] + "..."
	}
	if input == "" {
		return "<empty>"
	}
	return input
}

// assertBehavioralSync checks Go and CUE accept/reject verdicts against expectations
// and verifies behavioral agreement (or logs expected divergence).
func assertBehavioralSync(t *testing.T, tc behavioralSyncCase, goErr, cueErr error) {
	t.Helper()

	goAccepts := goErr == nil
	cueAccepts := cueErr == nil

	if goAccepts != tc.goExpect {
		t.Errorf("Go Validate() unexpected: got accept=%v, want %v (err=%v)", goAccepts, tc.goExpect, goErr)
	}
	if cueAccepts != tc.cueExpect {
		t.Errorf("CUE Validate() unexpected: got accept=%v, want %v (err=%v)", cueAccepts, tc.cueExpect, cueErr)
	}
	if tc.divergeNote == "" && goAccepts != cueAccepts {
		t.Errorf("BEHAVIORAL DRIFT: Go accept=%v, CUE accept=%v for input %q", goAccepts, cueAccepts, tc.input)
	}
	if tc.divergeNote != "" && goAccepts != cueAccepts {
		t.Logf("Expected divergence: %s (Go=%v, CUE=%v)", tc.divergeNote, goAccepts, cueAccepts)
	}
}

// runBehavioralSyncCore runs behavioral equivalence subtests against a pre-resolved
// CUE constraint. Both runBehavioralSync and runBehavioralSyncField delegate here.
//
// Subtests run serially: CUE Value.Unify() and Context.CompileString() mutate
// internal state and are not safe for concurrent use. Parent tests already run
// in parallel, so test-function-level parallelism is preserved.
// cueExprForScalar formats a test input as a quoted CUE string literal.
func cueExprForScalar(input string) string { return fmt.Sprintf("%q", input) }

// cueExprForListElement wraps a test input in a single-element CUE list,
// matching list-typed constraints like [...string & !=""].
func cueExprForListElement(input string) string { return fmt.Sprintf("[%q]", input) }

func runBehavioralSyncCore(
	t *testing.T, ctx *cue.Context,
	constraint cue.Value,
	goValidate func(string) error,
	formatCUEExpr func(string) string,
	cases []behavioralSyncCase,
) {
	t.Helper()

	for _, tc := range cases {
		t.Run(subtestLabel(tc.input), func(t *testing.T) {
			goErr := goValidate(tc.input)

			unified := constraint.Unify(ctx.CompileString(formatCUEExpr(tc.input)))
			cueErr := unified.Validate(cue.Concrete(true))

			assertBehavioralSync(t, tc, goErr, cueErr)
		})
	}
}

// runBehavioralSyncField runs behavioral equivalence tests using field-level CUE
// constraint lookup. This handles optional CUE fields that LookupPath cannot find.
func runBehavioralSyncField(
	t *testing.T, schema cue.Value, ctx *cue.Context,
	parentPath, fieldName string,
	goValidate func(string) error,
	cases []behavioralSyncCase,
) {
	t.Helper()

	constraint := lookupCUEFieldConstraint(t, schema, parentPath, fieldName)
	runBehavioralSyncCore(t, ctx, constraint, goValidate, cueExprForScalar, cases)
}

// runBehavioralSyncListElement runs behavioral equivalence tests for a type
// that maps to a CUE list element constraint (e.g., [...string & !=""]).
// Wraps each test value in a single-element list before unifying with the
// list constraint, then checks element 0 for validity.
func runBehavioralSyncListElement(
	t *testing.T, schema cue.Value, ctx *cue.Context,
	parentPath, fieldName string,
	goValidate func(string) error,
	cases []behavioralSyncCase,
) {
	t.Helper()

	constraint := lookupCUEFieldConstraint(t, schema, parentPath, fieldName)
	runBehavioralSyncCore(t, ctx, constraint, goValidate, cueExprForListElement, cases)
}

// runBehavioralSync runs behavioral equivalence tests for a string-backed DDD type.
// cuePath is the CUE constraint path (e.g., "#RuntimeType", "#Flag.name").
func runBehavioralSync(
	t *testing.T, schema cue.Value, ctx *cue.Context,
	cuePath string,
	goValidate func(string) error,
	cases []behavioralSyncCase,
) {
	t.Helper()

	constraint := schema.LookupPath(cue.ParsePath(cuePath))
	if constraint.Err() != nil {
		t.Fatalf("CUE path %s not found: %v", cuePath, constraint.Err())
	}
	runBehavioralSyncCore(t, ctx, constraint, goValidate, cueExprForScalar, cases)
}
