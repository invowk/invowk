// SPDX-License-Identifier: MPL-2.0

// Package schematest provides shared test helpers for CUE schema ↔ Go struct
// synchronization tests. These helpers are used by sync_test.go in
// pkg/invowkfile, pkg/invowkmod, and internal/config to verify field-level
// alignment between CUE schemas and Go types.
package schematest

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
)

// ExtractCUEFields extracts all field names from a CUE struct definition.
// It returns a map of field names to whether the field is optional.
// Nested struct fields are not included; only top-level fields of the given definition.
func ExtractCUEFields(t *testing.T, val cue.Value) map[string]bool {
	t.Helper()

	fields := make(map[string]bool)

	iter, err := val.Fields(cue.Definitions(false), cue.Optional(true))
	if err != nil {
		t.Fatalf("failed to iterate CUE fields: %v", err)
	}

	for iter.Next() {
		sel := iter.Selector()
		labelType := sel.LabelType()
		if labelType.IsHidden() || sel.IsDefinition() {
			continue
		}

		// Skip fields explicitly set to bottom (_|_) — error constraints used to
		// forbid certain field names. "explicit error (_|_ literal)" distinguishes
		// intentional _|_ from constraint evaluation errors.
		fieldValue := iter.Value()
		if fieldValue.Kind() == cue.BottomKind && fieldValue.Err() != nil {
			errMsg := fieldValue.Err().Error()
			if strings.Contains(errMsg, "explicit error (_|_ literal)") {
				continue
			}
		}

		fieldName := sel.String()
		fieldName = strings.TrimSuffix(fieldName, "?")
		isOptional := iter.IsOptional()
		fields[fieldName] = isOptional
	}

	return fields
}

// ExtractGoJSONTags extracts all JSON field names from a Go struct using reflection.
// It returns a map of JSON tag names to whether the field has "omitempty".
// Fields with json:"-" are excluded.
// Embedded structs are not expanded; only direct fields are returned.
func ExtractGoJSONTags(t *testing.T, typ reflect.Type) map[string]bool {
	t.Helper()

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		t.Fatalf("expected struct type, got %s", typ.Kind())
	}

	fields := make(map[string]bool)

	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}

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

// AssertFieldsSync verifies that CUE schema fields and Go struct JSON tags are in sync.
// It checks:
//  1. Every CUE field has a corresponding Go JSON tag
//  2. Every Go JSON tag has a corresponding CUE field
//  3. Optional/omitempty alignment (warning only, not a failure)
func AssertFieldsSync(t *testing.T, structName string, cueFields, goFields map[string]bool) {
	t.Helper()

	for field, isOptional := range cueFields {
		hasOmitempty, exists := goFields[field]
		if !exists {
			t.Errorf("[%s] CUE field %q not found in Go struct (missing JSON tag)", structName, field)
			continue
		}
		if isOptional && !hasOmitempty {
			t.Logf("[%s] Note: CUE field %q is optional but Go field lacks omitempty tag", structName, field)
		}
	}

	for field := range goFields {
		if _, exists := cueFields[field]; !exists {
			t.Errorf("[%s] Go JSON tag %q not found in CUE schema (missing CUE field)", structName, field)
		}
	}
}

// LookupDefinition looks up a CUE definition by path (e.g., "#Invowkfile").
func LookupDefinition(t *testing.T, schema cue.Value, defPath string) cue.Value {
	t.Helper()

	def := schema.LookupPath(cue.ParsePath(defPath))
	if def.Err() != nil {
		t.Fatalf("failed to lookup CUE definition %s: %v", defPath, def.Err())
	}

	return def
}

// LookupCUEFieldConstraint extracts the constraint value for a specific field
// within a CUE struct definition. Unlike LookupPath, this handles optional
// fields by iterating with cue.Optional(true).
func LookupCUEFieldConstraint(t *testing.T, schema cue.Value, parentPath, fieldName string) cue.Value {
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
