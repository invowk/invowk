// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

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
		// used to explicitly forbid certain field names (e.g., module, version in Invkfile).
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

	for i := range typ.NumField() {
		field := typ.Field(i)

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
	schema := ctx.CompileString(invkfileSchema)
	if schema.Err() != nil {
		t.Fatalf("failed to compile CUE schema: %v", schema.Err())
	}

	return schema, ctx
}

// lookupDefinition looks up a CUE definition by path (e.g., "#Invkfile").
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
	invkfileDef := lookupDefinition(t, schema, "#Invkfile")
	if invkfileDef.Err() != nil {
		t.Errorf("failed to lookup #Invkfile: %v", invkfileDef.Err())
	}

	// Test extractCUEFields on #Invkfile
	cueFields := extractCUEFields(t, invkfileDef)
	if len(cueFields) == 0 {
		t.Errorf("extractCUEFields returned no fields for #Invkfile")
	}
	// Verify known field exists
	if _, ok := cueFields["cmds"]; !ok {
		t.Errorf("expected 'cmds' field in #Invkfile CUE definition")
	}

	// Test extractGoJSONTags on Invkfile struct
	goFields := extractGoJSONTags(t, reflect.TypeFor[Invkfile]())
	if len(goFields) == 0 {
		t.Errorf("extractGoJSONTags returned no fields for Invkfile")
	}
	// Verify known field exists
	if _, ok := goFields["cmds"]; !ok {
		t.Errorf("expected 'cmds' JSON tag in Invkfile struct")
	}

	// Test assertFieldsSync doesn't panic (actual sync tests are below)
	// We just verify the helper runs without crashing
	assertFieldsSync(t, "Invkfile-smoke", cueFields, goFields)
}

// =============================================================================
// Schema Sync Tests - Phase 3 (T007-T012)
// =============================================================================
// These tests verify Go struct JSON tags match CUE schema field names.
// They catch misalignments at CI time, preventing silent parsing failures.

// TestInvkfileSchemaSync verifies Invkfile Go struct matches #Invkfile CUE definition.
// T007: Add sync test for Invkfile struct
func TestInvkfileSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Invkfile"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Invkfile]())

	assertFieldsSync(t, "Invkfile", cueFields, goFields)
}

// TestCommandSchemaSync verifies Command Go struct matches #Command CUE definition.
// T008: Add sync test for Command struct
func TestCommandSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Command"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Command]())

	assertFieldsSync(t, "Command", cueFields, goFields)
}

// TestImplementationSchemaSync verifies Implementation Go struct matches #Implementation CUE definition.
// T009: Add sync test for Implementation struct
func TestImplementationSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Implementation"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Implementation]())

	assertFieldsSync(t, "Implementation", cueFields, goFields)
}

// TestRuntimeConfigSchemaSync verifies RuntimeConfig Go struct matches CUE runtime definitions.
// T010: Add sync test for RuntimeConfig struct
//
// Note: The CUE schema uses a union type (#RuntimeConfig = #RuntimeConfigNative | #RuntimeConfigVirtual | #RuntimeConfigContainer)
// while Go uses a single RuntimeConfig struct with all fields. We need to extract the union of all fields
// from the three CUE types.
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
	for field, optional := range nativeFields {
		allCUEFields[field] = optional //nolint:modernize // Custom merge logic needed - can't use maps.Copy
	}
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

// TestDependsOnSchemaSync verifies DependsOn Go struct matches #DependsOn CUE definition.
// T011: Add sync test for DependsOn struct
func TestDependsOnSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#DependsOn"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[DependsOn]())

	assertFieldsSync(t, "DependsOn", cueFields, goFields)
}

// TestFlagSchemaSync verifies Flag Go struct matches #Flag CUE definition.
// T012: Add sync test for Flag struct
func TestFlagSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Flag"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Flag]())

	assertFieldsSync(t, "Flag", cueFields, goFields)
}

// TestArgumentSchemaSync verifies Argument Go struct matches #Argument CUE definition.
// T012: Add sync test for Argument struct (same task as Flag)
func TestArgumentSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Argument"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Argument]())

	assertFieldsSync(t, "Argument", cueFields, goFields)
}

// TestEnvConfigSchemaSync verifies EnvConfig Go struct matches #EnvConfig CUE definition.
func TestEnvConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#EnvConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[EnvConfig]())

	assertFieldsSync(t, "EnvConfig", cueFields, goFields)
}

// TestPlatformConfigSchemaSync verifies PlatformConfig Go struct matches #PlatformConfig CUE definition.
func TestPlatformConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#PlatformConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[PlatformConfig]())

	assertFieldsSync(t, "PlatformConfig", cueFields, goFields)
}

// TestToolDependencySchemaSync verifies ToolDependency Go struct matches #ToolDependency CUE definition.
func TestToolDependencySchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#ToolDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[ToolDependency]())

	assertFieldsSync(t, "ToolDependency", cueFields, goFields)
}

// TestFilepathDependencySchemaSync verifies FilepathDependency Go struct matches #FilepathDependency CUE definition.
func TestFilepathDependencySchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#FilepathDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[FilepathDependency]())

	assertFieldsSync(t, "FilepathDependency", cueFields, goFields)
}

// TestCapabilityDependencySchemaSync verifies CapabilityDependency Go struct matches #CapabilityDependency CUE definition.
func TestCapabilityDependencySchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#CapabilityDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[CapabilityDependency]())

	assertFieldsSync(t, "CapabilityDependency", cueFields, goFields)
}

// TestCommandDependencySchemaSync verifies CommandDependency Go struct matches #CommandDependency CUE definition.
func TestCommandDependencySchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#CommandDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[CommandDependency]())

	assertFieldsSync(t, "CommandDependency", cueFields, goFields)
}

// TestEnvVarCheckSchemaSync verifies EnvVarCheck Go struct matches #EnvVarCheck CUE definition.
func TestEnvVarCheckSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#EnvVarCheck"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[EnvVarCheck]())

	assertFieldsSync(t, "EnvVarCheck", cueFields, goFields)
}

// TestEnvVarDependencySchemaSync verifies EnvVarDependency Go struct matches #EnvVarDependency CUE definition.
func TestEnvVarDependencySchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#EnvVarDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[EnvVarDependency]())

	assertFieldsSync(t, "EnvVarDependency", cueFields, goFields)
}

// TestCustomCheckSchemaSync verifies CustomCheck Go struct matches #CustomCheck CUE definition.
func TestCustomCheckSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#CustomCheck"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[CustomCheck]())

	assertFieldsSync(t, "CustomCheck", cueFields, goFields)
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
	schemaValue := ctx.CompileString(invkfileSchema)
	if schemaValue.Err() != nil {
		t.Fatalf("failed to compile schema: %v", schemaValue.Err())
	}

	// Compile user data
	userValue := ctx.CompileString(cueData)
	if userValue.Err() != nil {
		return fmt.Errorf("CUE compile error: %w", userValue.Err())
	}

	// Get the #Invkfile definition
	schemaDef := schemaValue.LookupPath(cue.ParsePath("#Invkfile"))
	if schemaDef.Err() != nil {
		t.Fatalf("failed to lookup #Invkfile: %v", schemaDef.Err())
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

	// Create an invalid invkfile that should produce a path-containing error
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "container", image: "` + strings.Repeat("a", 600) + `"}]
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
	}]
}]`
	if err := validateCUE(t, invalid); err == nil {
		t.Errorf("empty image string should fail validation, but passed")
	}
}
