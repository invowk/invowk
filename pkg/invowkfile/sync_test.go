// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// behavioralSyncCase defines a single input for behavioral equivalence testing.
// Used by TestBehavioralSync_* tests at the bottom of this file.
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

// TestRuntimeConfigSchemaSync verifies RuntimeConfig Go struct matches CUE runtime definitions.
//
// Note: The CUE schema uses a union type (#RuntimeConfig = #RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer)
// while Go uses a single RuntimeConfig struct with all fields. We need to extract the union of all fields
// from the three CUE types. This requires custom merge logic, so it remains a separate test.
func TestRuntimeConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)

	// Extract fields from each runtime type variant
	nativeFields := extractCUEFields(t, lookupDefinition(t, schema, "#RuntimeConfigNative"))
	virtualFields := extractCUEFields(t, lookupDefinition(t, schema, "#RuntimeConfigVirtual"))
	containerFields := extractCUEFields(t, lookupDefinition(t, schema, "#RuntimeConfigContainer"))

	// Merge all CUE fields (the Go struct has the union of all fields)
	// We can't use maps.Copy because we need custom merge logic that OR's the optional flags
	allCUEFields := make(map[string]bool)
	maps.Copy(allCUEFields, nativeFields)
	for field, optional := range virtualFields {
		// If already present from native, use the more lenient (optional = true) value
		if existing, ok := allCUEFields[field]; ok {
			allCUEFields[field] = existing || optional
		} else {
			allCUEFields[field] = optional
		}
	}
	for field, optional := range containerFields {
		if existing, ok := allCUEFields[field]; ok {
			allCUEFields[field] = existing || optional
		} else {
			allCUEFields[field] = optional
		}
	}

	goFields := extractGoJSONTags(t, reflect.TypeFor[RuntimeConfig]())

	assertFieldsSync(t, "RuntimeConfig", allCUEFields, goFields)
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

// TestImageLengthConstraint verifies #RuntimeConfigContainer.image has a 512 rune limit.
// T090: Add boundary tests for image length constraint (512 chars)
func TestImageLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 512 characters should pass
	image512 := strings.Repeat("a", 512)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "` + image512 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("512-char image should be valid, got error: %v", err)
	}

	// 513 characters should fail
	image513 := strings.Repeat("a", 513)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "` + image513 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("513-char image should fail validation, but passed")
	}
}

// TestInterpreterLengthConstraint verifies interpreter fields have a 1024 rune limit.
// T091: Add boundary tests for interpreter length constraint (1024 chars)
func TestInterpreterLengthConstraint(t *testing.T) {
	t.Parallel()

	// Test native interpreter - exactly 1024 characters should pass
	interp1024 := strings.Repeat("a", 1024)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native", interpreter: "` + interp1024 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("1024-char native interpreter should be valid, got error: %v", err)
	}

	// 1025 characters should fail
	interp1025 := strings.Repeat("a", 1025)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native", interpreter: "` + interp1025 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("1025-char native interpreter should fail validation, but passed")
	}

	// Test container interpreter - exactly 1024 characters should pass
	validContainer := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", interpreter: "` + interp1024 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, validContainer); err != nil {
		t.Errorf("1024-char container interpreter should be valid, got error: %v", err)
	}

	// 1025 characters should fail for container interpreter too
	invalidContainer := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", interpreter: "` + interp1025 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalidContainer); err == nil {
		t.Errorf("1025-char container interpreter should fail validation, but passed")
	}
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

// TestErrorMessagesIncludeCUEPaths verifies error messages include CUE paths.
// T094: Verify error messages include CUE paths in constraint violation tests
func TestErrorMessagesIncludeCUEPaths(t *testing.T) {
	t.Parallel()

	// Create an invalid invowkfile that should produce a path-containing error
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "` + strings.Repeat("a", 600) + `"}]
		platforms: [{name: "linux"}]
	}]
}]`

	err := validateCUE(t, invalid)
	if err == nil {
		t.Fatalf("expected validation error for oversized image")
	}

	// The error message should contain path information
	errStr := err.Error()

	// Check that error contains path-like components (cmds, implementations, runtimes, image)
	// CUE error formatting includes the path to the invalid field
	if !strings.Contains(errStr, "cmds") && !strings.Contains(errStr, "implementations") &&
		!strings.Contains(errStr, "image") && !strings.Contains(errStr, "runtimes") {
		t.Logf("Full error: %s", errStr)
		t.Errorf("error message should contain path information, got: %s", errStr)
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

// TestCustomCheckNameLengthConstraint verifies #CustomCheck.name has a 256 rune limit.
func TestCustomCheckNameLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 256 characters should pass
	name256 := "a" + strings.Repeat("b", 255)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "` + name256 + `"
			check_script: "echo ok"
		}]
	}
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("256-char custom check name should be valid, got error: %v", err)
	}

	// 257 characters should fail
	name257 := "a" + strings.Repeat("b", 256)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "` + name257 + `"
			check_script: "echo ok"
		}]
	}
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("257-char custom check name should fail validation, but passed")
	}
}

// TestExpectedOutputLengthConstraint verifies #CustomCheck.expected_output has a 1000 rune limit.
func TestExpectedOutputLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 1000 characters should pass
	output1000 := strings.Repeat("a", 1000)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "mycheck"
			check_script: "echo ok"
			expected_output: "` + output1000 + `"
		}]
	}
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("1000-char expected_output should be valid, got error: %v", err)
	}

	// 1001 characters should fail
	output1001 := strings.Repeat("a", 1001)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "mycheck"
			check_script: "echo ok"
			expected_output: "` + output1001 + `"
		}]
	}
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("1001-char expected_output should fail validation, but passed")
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

// TestEnvFilesElementConstraints verifies #EnvConfig.files element constraints.
// Elements must be non-empty and at most 4096 runes.
func TestEnvFilesElementConstraints(t *testing.T) {
	t.Parallel()

	// Empty string should fail
	invalidEmpty := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		files: [""]
	}
}]`
	if err := validateCUE(t, invalidEmpty); err == nil {
		t.Errorf("empty env file path should fail validation, but passed")
	}

	// 4096-char path should pass
	path4096 := strings.Repeat("a", 4096)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		files: ["` + path4096 + `"]
	}
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("4096-char env file path should be valid, got error: %v", err)
	}

	// 4097-char path should fail
	path4097 := strings.Repeat("a", 4097)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		files: ["` + path4097 + `"]
	}
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("4097-char env file path should fail validation, but passed")
	}
}

// TestEnvVarsKeyConstraint verifies #EnvConfig.vars keys must match POSIX regex.
// Key pattern: ^[A-Za-z_][A-Za-z0-9_]*$
func TestEnvVarsKeyConstraint(t *testing.T) {
	t.Parallel()

	// Valid POSIX key should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		vars: {
			MY_VAR: "hello"
		}
	}
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("valid POSIX env var key 'MY_VAR' should pass, got error: %v", err)
	}

	// Invalid key starting with digit should fail
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		vars: {
			"123bad": "hello"
		}
	}
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("env var key '123bad' should fail validation, but passed")
	}
}

// TestEnvVarsValueLengthConstraint verifies #EnvConfig.vars values have a 32768 rune limit.
func TestEnvVarsValueLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 32768 characters should pass
	val32768 := strings.Repeat("a", 32768)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		vars: {
			MY_VAR: "` + val32768 + `"
		}
	}
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("32768-char env var value should be valid, got error: %v", err)
	}

	// 32769 characters should fail
	val32769 := strings.Repeat("a", 32769)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		vars: {
			MY_VAR: "` + val32769 + `"
		}
	}
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("32769-char env var value should fail validation, but passed")
	}
}

// TestVolumesElementConstraints verifies #RuntimeConfigContainer.volumes element constraints.
// Elements must be non-empty and at most 4096 runes.
func TestVolumesElementConstraints(t *testing.T) {
	t.Parallel()

	// Empty string should fail
	invalidEmpty := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", volumes: [""]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalidEmpty); err == nil {
		t.Errorf("empty volume string should fail validation, but passed")
	}

	// Valid volume string should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", volumes: ["./data:/data"]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("valid volume string should pass, got error: %v", err)
	}

	// 4097-char volume string should fail
	vol4097 := strings.Repeat("a", 4097)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", volumes: ["` + vol4097 + `"]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("4097-char volume string should fail validation, but passed")
	}
}

// TestPortsElementConstraints verifies #RuntimeConfigContainer.ports element constraints.
// Elements must be non-empty and at most 256 runes.
func TestPortsElementConstraints(t *testing.T) {
	t.Parallel()

	// Empty string should fail
	invalidEmpty := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", ports: [""]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalidEmpty); err == nil {
		t.Errorf("empty port string should fail validation, but passed")
	}

	// Valid port mapping should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", ports: ["8080:80"]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("valid port mapping '8080:80' should pass, got error: %v", err)
	}

	// 257-char port string should fail
	port257 := strings.Repeat("a", 257)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "debian:stable-slim", ports: ["` + port257 + `"]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("257-char port string should fail validation, but passed")
	}
}

// TestDefaultShellNonWhitespaceConstraint verifies default_shell rejects whitespace-only values.
// Pattern: =~"^\\s*\\S.*$"
func TestDefaultShellNonWhitespaceConstraint(t *testing.T) {
	t.Parallel()

	// Whitespace-only should fail
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]
default_shell: " "
`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("whitespace-only default_shell should fail validation, but passed")
	}

	// Valid shell path should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]
default_shell: "/bin/bash"
`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("valid default_shell '/bin/bash' should pass, got error: %v", err)
	}
}

// TestImageNonEmptyConstraint verifies #RuntimeConfigContainer.image rejects empty strings.
// Constraint: !=""
func TestImageNonEmptyConstraint(t *testing.T) {
	t.Parallel()

	// Empty image should fail
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: ""}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("empty image string should fail validation, but passed")
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

// validateStringAgainstCUE validates a single string value against a CUE
// constraint path. Returns nil if CUE accepts the value, error if it rejects.
// The cuePath should point to a CUE definition or field (e.g., "#RuntimeType",
// "#Flag.name"). For optional CUE fields, use lookupCUEFieldConstraint instead.
func validateStringAgainstCUE(t *testing.T, schema cue.Value, ctx *cue.Context, cuePath, value string) error {
	t.Helper()

	constraint := schema.LookupPath(cue.ParsePath(cuePath))
	if constraint.Err() != nil {
		t.Fatalf("CUE path %s not found: %v", cuePath, constraint.Err())
	}
	unified := constraint.Unify(ctx.CompileString(fmt.Sprintf("%q", value)))
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("CUE validation: %w", err)
	}
	return nil
}

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
	for _, tc := range cases {
		label := tc.input
		if len(label) > 30 {
			label = label[:27] + "..."
		}
		if label == "" {
			label = "<empty>"
		}

		t.Run(label, func(t *testing.T) {
			t.Parallel()

			goErr := goValidate(tc.input)
			goAccepts := goErr == nil

			unified := constraint.Unify(ctx.CompileString(fmt.Sprintf("%q", tc.input)))
			cueErr := unified.Validate(cue.Concrete(true))
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
		})
	}
}

// runBehavioralSync runs behavioral equivalence tests for a string-backed DDD type.
// goValidate is the Go Validate() function; cuePath is the CUE constraint path.
func runBehavioralSync(
	t *testing.T, schema cue.Value, ctx *cue.Context,
	cuePath string,
	goValidate func(string) error,
	cases []behavioralSyncCase,
) {
	t.Helper()

	for _, tc := range cases {
		// Use a truncated label for readability
		label := tc.input
		if len(label) > 30 {
			label = label[:27] + "..."
		}
		if label == "" {
			label = "<empty>"
		}

		t.Run(label, func(t *testing.T) {
			t.Parallel()

			goErr := goValidate(tc.input)
			goAccepts := goErr == nil

			cueErr := validateStringAgainstCUE(t, schema, ctx, cuePath, tc.input)
			cueAccepts := cueErr == nil

			// Verify Go matches expectation
			if goAccepts != tc.goExpect {
				t.Errorf("Go Validate() unexpected: got accept=%v, want %v (err=%v)", goAccepts, tc.goExpect, goErr)
			}

			// Verify CUE matches expectation
			if cueAccepts != tc.cueExpect {
				t.Errorf("CUE Validate() unexpected: got accept=%v, want %v (err=%v)", cueAccepts, tc.cueExpect, cueErr)
			}

			// Verify behavioral agreement (unless known divergence)
			if tc.divergeNote == "" && goAccepts != cueAccepts {
				t.Errorf("BEHAVIORAL DRIFT: Go accept=%v, CUE accept=%v for input %q", goAccepts, cueAccepts, tc.input)
			}
			if tc.divergeNote != "" && goAccepts != cueAccepts {
				t.Logf("Expected divergence: %s (Go=%v, CUE=%v)", tc.divergeNote, goAccepts, cueAccepts)
			}
		})
	}
}

// TestBehavioralSync_RuntimeMode verifies Go RuntimeMode.Validate() agrees with
// CUE #RuntimeType disjunction ("native" | "virtual" | "container").
func TestBehavioralSync_RuntimeMode(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#RuntimeType",
		func(s string) error { return RuntimeMode(s).Validate() },
		[]behavioralSyncCase{
			{"native", true, true, ""},
			{"virtual", true, true, ""},
			{"container", true, true, ""},
			{"invalid", false, false, ""},
			{"NATIVE", false, false, ""},
			{"", false, false, ""},
			{" ", false, false, ""},
			{"native ", false, false, ""},
		},
	)
}

// TestBehavioralSync_PlatformType verifies Go PlatformType.Validate() agrees with
// CUE #PlatformType disjunction ("linux" | "macos" | "windows").
func TestBehavioralSync_PlatformType(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#PlatformType",
		func(s string) error { return PlatformType(s).Validate() },
		[]behavioralSyncCase{
			{"linux", true, true, ""},
			{"macos", true, true, ""},
			{"windows", true, true, ""},
			{"darwin", false, false, ""},
			{"LINUX", false, false, ""},
			{"", false, false, ""},
			{"freebsd", false, false, ""},
		},
	)
}

// TestBehavioralSync_EnvInheritMode verifies Go EnvInheritMode.Validate() agrees with
// CUE #RuntimeConfigBase.env_inherit_mode disjunction ("none" | "allow" | "all").
func TestBehavioralSync_EnvInheritMode(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// env_inherit_mode is an optional field in #RuntimeConfigBase.
	// We use the field-level lookup to extract the constraint.
	runBehavioralSyncField(t, schema, ctx, "#RuntimeConfigBase", "env_inherit_mode",
		func(s string) error { return EnvInheritMode(s).Validate() },
		[]behavioralSyncCase{
			{"none", true, true, ""},
			{"allow", true, true, ""},
			{"all", true, true, ""},
			{"inherit", false, false, ""},
			{"NONE", false, false, ""},
			{"", false, false, ""},
		},
	)
}

// TestBehavioralSync_CapabilityName verifies Go CapabilityName.Validate() agrees with
// CUE #CapabilityName disjunction.
func TestBehavioralSync_CapabilityName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#CapabilityName",
		func(s string) error { return CapabilityName(s).Validate() },
		[]behavioralSyncCase{
			{"local-area-network", true, true, ""},
			{"internet", true, true, ""},
			{"containers", true, true, ""},
			{"tty", true, true, ""},
			{"gpu", false, false, ""},
			{"", false, false, ""},
			{"TTY", false, false, ""},
		},
	)
}

// TestBehavioralSync_FlagType verifies Go FlagType.Validate() agrees with
// CUE #Flag.type disjunction ("string" | "bool" | "int" | "float").
// Note: FlagType("") is valid in Go (defaults to "string") but CUE field type?
// is optional — absent means default. The zero-value divergence is expected.
func TestBehavioralSync_FlagType(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// type is an optional field in #Flag — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#Flag", "type",
		func(s string) error { return FlagType(s).Validate() },
		[]behavioralSyncCase{
			{"string", true, true, ""},
			{"bool", true, true, ""},
			{"int", true, true, ""},
			{"float", true, true, ""},
			{"array", false, false, ""},
			{"STRING", false, false, ""},
			// Go accepts "" (defaults to "string"), CUE rejects "" because it doesn't match the disjunction.
			// This is expected: CUE handles optionality at the field level (type? is omitted), not value level.
			{"", true, false, "Go zero-value defaults to string; CUE uses field optionality"},
		},
	)
}

// TestBehavioralSync_ArgumentType verifies Go ArgumentType.Validate() agrees with
// CUE #Argument.type disjunction ("string" | "int" | "float").
func TestBehavioralSync_ArgumentType(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// type is an optional field in #Argument — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#Argument", "type",
		func(s string) error { return ArgumentType(s).Validate() },
		[]behavioralSyncCase{
			{"string", true, true, ""},
			{"int", true, true, ""},
			{"float", true, true, ""},
			{"bool", false, false, ""},
			{"", true, false, "Go zero-value defaults to string; CUE uses field optionality"},
		},
	)
}

// TestBehavioralSync_FlagName verifies Go FlagName.Validate() agrees with
// CUE #Flag.name constraint (regex + length + non-empty).
func TestBehavioralSync_FlagName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Flag.name",
		func(s string) error { return FlagName(s).Validate() },
		[]behavioralSyncCase{
			{"verbose", true, true, ""},
			{"output-file", true, true, ""},
			{"num_retries", true, true, ""},
			{"a", true, true, ""},
			{"A", true, true, ""},
			{"a1", true, true, ""},
			{"", false, false, ""},
			{"   ", false, false, ""},
			{"123bad", false, false, ""},
			{"-starts-hyphen", false, false, ""},
			{"_starts_underscore", false, false, ""},
			{"a" + strings.Repeat("b", 255), true, true, ""},   // exactly 256 runes
			{"a" + strings.Repeat("b", 256), false, false, ""}, // 257 runes
		},
	)
}

// TestBehavioralSync_ArgumentName verifies Go ArgumentName.Validate() agrees with
// CUE #Argument.name constraint (regex + length + non-empty).
func TestBehavioralSync_ArgumentName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Argument.name",
		func(s string) error { return ArgumentName(s).Validate() },
		[]behavioralSyncCase{
			{"file", true, true, ""},
			{"output-dir", true, true, ""},
			{"source_path", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"123bad", false, false, ""},
			{"-flag", false, false, ""},
			{"a" + strings.Repeat("b", 255), true, true, ""},
			{"a" + strings.Repeat("b", 256), false, false, ""},
		},
	)
}

// TestBehavioralSync_CommandName verifies Go CommandName.Validate() agrees with
// CUE #Command.name constraint (regex + length + non-empty).
// FINDING: Go CommandName.Validate() only checks non-empty/non-whitespace.
// CUE enforces regex (^[a-zA-Z][a-zA-Z0-9_ -]*$) and MaxRunes(256).
// Go is MORE LENIENT than CUE — it accepts values CUE rejects.
// CUE is the primary validator at parse time; Go Validate() is a secondary guard.
func TestBehavioralSync_CommandName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Command.name",
		func(s string) error { return CommandName(s).Validate() },
		[]behavioralSyncCase{
			{"build", true, true, ""},
			{"test unit", true, true, ""},
			{"deploy-prod", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"   ", false, false, ""},
			// Go accepts these because it only checks non-whitespace; CUE enforces regex
			{"123bad", true, false, "Go only checks non-whitespace; CUE enforces regex ^[a-zA-Z]..."},
			{"-starts-hyphen", true, false, "Go only checks non-whitespace; CUE enforces regex ^[a-zA-Z]..."},
			{"a" + strings.Repeat("b", 255), true, true, ""},
			// Go accepts over-length because it doesn't check MaxRunes; CUE enforces MaxRunes(256)
			{"a" + strings.Repeat("b", 256), true, false, "Go has no length check; CUE enforces MaxRunes(256)"},
		},
	)
}

// TestBehavioralSync_DurationString verifies Go DurationString.Validate() agrees with
// CUE #DurationString constraint (regex + length).
// Note: Go uses time.ParseDuration() which is strictly more powerful than CUE's regex.
// CUE regex: ^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$
// Go: time.ParseDuration + positive check
func TestBehavioralSync_DurationString(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#DurationString",
		func(s string) error { return DurationString(s).Validate() },
		[]behavioralSyncCase{
			{"30s", true, true, ""},
			{"5m", true, true, ""},
			{"1h30m", true, true, ""},
			{"500ms", true, true, ""},
			{"1h", true, true, ""},
			{"100ns", true, true, ""},
			// Go accepts "" (no duration = use default), CUE regex rejects ""
			{"", true, false, "Go zero-value means no duration; CUE regex requires content"},
			{"abc", false, false, ""},
			// Go rejects negative durations; CUE regex doesn't match negative sign
			{"-5s", false, false, ""},
			// Go rejects "0s" (non-positive); CUE regex accepts "0s" format
			{"0s", false, true, "Go rejects zero duration (must be positive); CUE only checks format"},
		},
	)
}

// TestBehavioralSync_ContainerImage verifies Go ContainerImage.Validate() agrees with
// CUE #RuntimeConfigContainer.image constraint (non-empty + length).
// Note: ContainerImage("") is valid in Go (no image = use containerfile),
// but CUE field image?: string & !="" means empty is rejected at the CUE level.
// The CUE optionality handles the "no image" case (field is absent, not empty).
func TestBehavioralSync_ContainerImage(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// image is an optional field in #RuntimeConfigContainer — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#RuntimeConfigContainer", "image",
		func(s string) error { return ContainerImage(s).Validate() },
		[]behavioralSyncCase{
			{"debian:stable-slim", true, true, ""},
			{"golang:1.26", true, true, ""},
			{"myregistry.io/myimage:latest", true, true, ""},
			// Go accepts "" (containerfile will be used), CUE rejects "" (!="")
			{"", true, false, "Go zero-value means no image; CUE uses field optionality with !=\"\""},
			// Whitespace-only: Go rejects (TrimSpace check), CUE accepts (!="" passes for whitespace)
			{"   ", false, true, "Go checks TrimSpace; CUE !=\"\" only rejects literal empty string"},
			{strings.Repeat("a", 512), true, true, ""},
			// Go ContainerImage.Validate() only checks whitespace; length check is in
			// ValidateContainerImage() (a separate function). CUE enforces MaxRunes(512).
			{strings.Repeat("a", 513), true, false, "Go Validate() has no length check; CUE enforces MaxRunes(512)"},
		},
	)
}
