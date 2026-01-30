// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
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
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Invkfile"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Invkfile]())

	assertFieldsSync(t, "Invkfile", cueFields, goFields)
}

// TestCommandSchemaSync verifies Command Go struct matches #Command CUE definition.
// T008: Add sync test for Command struct
func TestCommandSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Command"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Command]())

	assertFieldsSync(t, "Command", cueFields, goFields)
}

// TestImplementationSchemaSync verifies Implementation Go struct matches #Implementation CUE definition.
// T009: Add sync test for Implementation struct
func TestImplementationSchemaSync(t *testing.T) {
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
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#DependsOn"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[DependsOn]())

	assertFieldsSync(t, "DependsOn", cueFields, goFields)
}

// TestFlagSchemaSync verifies Flag Go struct matches #Flag CUE definition.
// T012: Add sync test for Flag struct
func TestFlagSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Flag"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Flag]())

	assertFieldsSync(t, "Flag", cueFields, goFields)
}

// TestArgumentSchemaSync verifies Argument Go struct matches #Argument CUE definition.
// T012: Add sync test for Argument struct (same task as Flag)
func TestArgumentSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Argument"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Argument]())

	assertFieldsSync(t, "Argument", cueFields, goFields)
}

// TestEnvConfigSchemaSync verifies EnvConfig Go struct matches #EnvConfig CUE definition.
func TestEnvConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#EnvConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[EnvConfig]())

	assertFieldsSync(t, "EnvConfig", cueFields, goFields)
}

// TestPlatformConfigSchemaSync verifies PlatformConfig Go struct matches #PlatformConfig CUE definition.
func TestPlatformConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#PlatformConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[PlatformConfig]())

	assertFieldsSync(t, "PlatformConfig", cueFields, goFields)
}

// TestToolDependencySchemaSync verifies ToolDependency Go struct matches #ToolDependency CUE definition.
func TestToolDependencySchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#ToolDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[ToolDependency]())

	assertFieldsSync(t, "ToolDependency", cueFields, goFields)
}

// TestFilepathDependencySchemaSync verifies FilepathDependency Go struct matches #FilepathDependency CUE definition.
func TestFilepathDependencySchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#FilepathDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[FilepathDependency]())

	assertFieldsSync(t, "FilepathDependency", cueFields, goFields)
}

// TestCapabilityDependencySchemaSync verifies CapabilityDependency Go struct matches #CapabilityDependency CUE definition.
func TestCapabilityDependencySchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#CapabilityDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[CapabilityDependency]())

	assertFieldsSync(t, "CapabilityDependency", cueFields, goFields)
}

// TestCommandDependencySchemaSync verifies CommandDependency Go struct matches #CommandDependency CUE definition.
func TestCommandDependencySchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#CommandDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[CommandDependency]())

	assertFieldsSync(t, "CommandDependency", cueFields, goFields)
}

// TestEnvVarCheckSchemaSync verifies EnvVarCheck Go struct matches #EnvVarCheck CUE definition.
func TestEnvVarCheckSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#EnvVarCheck"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[EnvVarCheck]())

	assertFieldsSync(t, "EnvVarCheck", cueFields, goFields)
}

// TestEnvVarDependencySchemaSync verifies EnvVarDependency Go struct matches #EnvVarDependency CUE definition.
func TestEnvVarDependencySchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#EnvVarDependency"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[EnvVarDependency]())

	assertFieldsSync(t, "EnvVarDependency", cueFields, goFields)
}

// TestCustomCheckSchemaSync verifies CustomCheck Go struct matches #CustomCheck CUE definition.
func TestCustomCheckSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#CustomCheck"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[CustomCheck]())

	assertFieldsSync(t, "CustomCheck", cueFields, goFields)
}
