// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// configSchema is embedded in config.go and available to tests via the same package.

// =============================================================================
// Schema Sync Tests - Phase 3 (T014)
// =============================================================================
// These tests verify Go struct JSON tags match CUE schema field names.
// They catch misalignments at CI time, preventing silent parsing failures.

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
		// used to explicitly forbid certain field names.
		// We detect these by checking if the error message contains "explicit error (_|_ literal)".
		// This distinguishes between:
		// - "explicitly _|_" → skip, not a real field
		// - "constraint evaluation error" → include, valid field
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
	schema := ctx.CompileString(configSchema)
	if schema.Err() != nil {
		t.Fatalf("failed to compile CUE schema: %v", schema.Err())
	}

	return schema, ctx
}

// lookupDefinition looks up a CUE definition by path (e.g., "#Config").
func lookupDefinition(t *testing.T, schema cue.Value, defPath string) cue.Value {
	t.Helper()

	def := schema.LookupPath(cue.ParsePath(defPath))
	if def.Err() != nil {
		t.Fatalf("failed to lookup CUE definition %s: %v", defPath, def.Err())
	}

	return def
}

// TestConfigSchemaSync verifies Config Go struct matches #Config CUE definition.
// T014: Create sync_test.go for Config struct
func TestConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Config"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Config]())

	assertFieldsSync(t, "Config", cueFields, goFields)
}

// TestVirtualShellConfigSchemaSync verifies VirtualShellConfig Go struct matches #VirtualShellConfig CUE definition.
func TestVirtualShellConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#VirtualShellConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[VirtualShellConfig]())

	assertFieldsSync(t, "VirtualShellConfig", cueFields, goFields)
}

// TestUIConfigSchemaSync verifies UIConfig Go struct matches #UIConfig CUE definition.
func TestUIConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#UIConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[UIConfig]())

	assertFieldsSync(t, "UIConfig", cueFields, goFields)
}

// TestContainerConfigSchemaSync verifies ContainerConfig Go struct matches #ContainerConfig CUE definition.
func TestContainerConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#ContainerConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[ContainerConfig]())

	assertFieldsSync(t, "ContainerConfig", cueFields, goFields)
}

// TestAutoProvisionConfigSchemaSync verifies AutoProvisionConfig Go struct matches #AutoProvisionConfig CUE definition.
func TestAutoProvisionConfigSchemaSync(t *testing.T) {
	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#AutoProvisionConfig"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[AutoProvisionConfig]())

	assertFieldsSync(t, "AutoProvisionConfig", cueFields, goFields)
}

// =============================================================================
// Schema Boundary Tests
// =============================================================================
// These tests verify CUE schema constraints (MaxRunes, non-empty, etc.)
// catch invalid values at parse time. Each test validates boundary conditions
// for string length limits and empty string rejections.

// validateCUE compiles CUE test data against the embedded config schema's #Config definition.
// It returns nil if the data is valid, or an error describing why validation failed.
func validateCUE(t *testing.T, cueData string) error {
	t.Helper()

	ctx := cuecontext.New()

	schemaValue := ctx.CompileString(configSchema)
	if schemaValue.Err() != nil {
		t.Fatalf("failed to compile schema: %v", schemaValue.Err())
	}

	userValue := ctx.CompileString(cueData)
	if userValue.Err() != nil {
		return fmt.Errorf("CUE compile error: %w", userValue.Err())
	}

	schemaDef := schemaValue.LookupPath(cue.ParsePath("#Config"))
	if schemaDef.Err() != nil {
		t.Fatalf("failed to lookup #Config: %v", schemaDef.Err())
	}

	unified := schemaDef.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("CUE validation error: %w", err)
	}

	return nil
}

// TestSearchPathsElementConstraints verifies search_paths elements reject empty strings
// and enforce the 4096 rune limit.
func TestSearchPathsElementConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "empty string element rejected",
			cueData: `container_engine: "docker"` + "\n" + `search_paths: [""]`,
			wantErr: true,
		},
		{
			name:    "4096-char element accepted",
			cueData: `container_engine: "docker"` + "\n" + `search_paths: ["` + strings.Repeat("a", 4096) + `"]`,
			wantErr: false,
		},
		{
			name:    "4097-char element rejected",
			cueData: `container_engine: "docker"` + "\n" + `search_paths: ["` + strings.Repeat("a", 4097) + `"]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCUE(t, tt.cueData)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestBinaryPathConstraints verifies container.auto_provision.binary_path rejects empty
// strings and enforces the 4096 rune limit.
func TestBinaryPathConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "empty string rejected",
			cueData: `container: auto_provision: { binary_path: "" }`,
			wantErr: true,
		},
		{
			name:    "4096-char path accepted",
			cueData: `container: auto_provision: { binary_path: "` + strings.Repeat("a", 4096) + `" }`,
			wantErr: false,
		},
		{
			name:    "4097-char path rejected",
			cueData: `container: auto_provision: { binary_path: "` + strings.Repeat("a", 4097) + `" }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCUE(t, tt.cueData)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestModulesPathsElementConstraints verifies container.auto_provision.modules_paths
// elements reject empty strings and enforce the 4096 rune limit.
func TestModulesPathsElementConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "empty string element rejected",
			cueData: `container: auto_provision: { modules_paths: [""] }`,
			wantErr: true,
		},
		{
			name:    "4096-char element accepted",
			cueData: `container: auto_provision: { modules_paths: ["` + strings.Repeat("a", 4096) + `"] }`,
			wantErr: false,
		},
		{
			name:    "4097-char element rejected",
			cueData: `container: auto_provision: { modules_paths: ["` + strings.Repeat("a", 4097) + `"] }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCUE(t, tt.cueData)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestCacheDirConstraints verifies container.auto_provision.cache_dir rejects empty
// strings and enforces the 4096 rune limit.
func TestCacheDirConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "empty string rejected",
			cueData: `container: auto_provision: { cache_dir: "" }`,
			wantErr: true,
		},
		{
			name:    "4096-char path accepted",
			cueData: `container: auto_provision: { cache_dir: "` + strings.Repeat("a", 4096) + `" }`,
			wantErr: false,
		},
		{
			name:    "4097-char path rejected",
			cueData: `container: auto_provision: { cache_dir: "` + strings.Repeat("a", 4097) + `" }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCUE(t, tt.cueData)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestModuleAliasesConstraints verifies module_aliases rejects empty values
// and enforces the 256 rune limit on values.
// Note: CUE pattern constraints on map keys ([string & !=""]:) do not reject
// empty string keys at validation time — empty key validation must be done in Go.
func TestModuleAliasesConstraints(t *testing.T) {
	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "empty value rejected",
			cueData: `container_engine: "docker"` + "\n" + `module_aliases: "com.example.mod": ""`,
			wantErr: true,
		},
		{
			name:    "256-char value accepted",
			cueData: `container_engine: "docker"` + "\n" + `module_aliases: "com.example.mod": "` + strings.Repeat("a", 256) + `"`,
			wantErr: false,
		},
		{
			name:    "257-char value rejected",
			cueData: `container_engine: "docker"` + "\n" + `module_aliases: "com.example.mod": "` + strings.Repeat("a", 257) + `"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCUE(t, tt.cueData)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}
