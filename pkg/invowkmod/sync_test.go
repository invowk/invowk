// SPDX-License-Identifier: MPL-2.0

package invowkmod

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
// Used by TestBehavioralSync_* tests at the bottom of this file.
type behavioralSyncCase struct {
	input       string
	goExpect    bool
	cueExpect   bool
	divergeNote string
}

// =============================================================================
// Schema Sync Tests - Phase 3 (T013)
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
	schema := ctx.CompileString(invowkmodSchema)
	if schema.Err() != nil {
		t.Fatalf("failed to compile CUE schema: %v", schema.Err())
	}

	return schema, ctx
}

// lookupDefinition looks up a CUE definition by path (e.g., "#Invowkmod").
func lookupDefinition(t *testing.T, schema cue.Value, defPath string) cue.Value {
	t.Helper()

	def := schema.LookupPath(cue.ParsePath(defPath))
	if def.Err() != nil {
		t.Fatalf("failed to lookup CUE definition %s: %v", defPath, def.Err())
	}

	return def
}

// TestInvowkmodSchemaSync verifies Invowkmod Go struct matches #Invowkmod CUE definition.
// T013: Create sync_test.go for Invowkmod struct
func TestInvowkmodSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#Invowkmod"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[Invowkmod]())

	assertFieldsSync(t, "Invowkmod", cueFields, goFields)
}

// TestModuleRequirementSchemaSync verifies ModuleRequirement Go struct matches #ModuleRequirement CUE definition.
func TestModuleRequirementSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := extractCUEFields(t, lookupDefinition(t, schema, "#ModuleRequirement"))
	goFields := extractGoJSONTags(t, reflect.TypeFor[ModuleRequirement]())

	assertFieldsSync(t, "ModuleRequirement", cueFields, goFields)
}

// =============================================================================
// Constraint Boundary Tests
// =============================================================================
// These tests verify CUE schema constraints reject invalid values at parse time.

// validateCUEInvowkmod is a helper that validates data against the #Invowkmod CUE definition.
// Returns nil if validation succeeds, error if validation fails.
func validateCUEInvowkmod(t *testing.T, cueData string) error {
	t.Helper()

	ctx := cuecontext.New()

	schemaValue := ctx.CompileString(invowkmodSchema)
	if schemaValue.Err() != nil {
		t.Fatalf("failed to compile schema: %v", schemaValue.Err())
	}

	userValue := ctx.CompileString(cueData)
	if userValue.Err() != nil {
		return fmt.Errorf("CUE compile error: %w", userValue.Err())
	}

	schemaDef := schemaValue.LookupPath(cue.ParsePath("#Invowkmod"))
	if schemaDef.Err() != nil {
		t.Fatalf("failed to lookup #Invowkmod: %v", schemaDef.Err())
	}

	unified := schemaDef.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("CUE validation error: %w", err)
	}
	return nil
}

// validateCUEModuleRequirement is a helper that validates data against the #ModuleRequirement CUE definition.
// Returns nil if validation succeeds, error if validation fails.
func validateCUEModuleRequirement(t *testing.T, cueData string) error {
	t.Helper()

	ctx := cuecontext.New()

	schemaValue := ctx.CompileString(invowkmodSchema)
	if schemaValue.Err() != nil {
		t.Fatalf("failed to compile schema: %v", schemaValue.Err())
	}

	userValue := ctx.CompileString(cueData)
	if userValue.Err() != nil {
		return fmt.Errorf("CUE compile error: %w", userValue.Err())
	}

	schemaDef := schemaValue.LookupPath(cue.ParsePath("#ModuleRequirement"))
	if schemaDef.Err() != nil {
		t.Fatalf("failed to lookup #ModuleRequirement: %v", schemaDef.Err())
	}

	unified := schemaDef.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("CUE validation error: %w", err)
	}
	return nil
}

// TestModuleNameLengthConstraint verifies #Invowkmod.module has a 256 rune limit.
func TestModuleNameLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 256 characters should pass (valid RDNS-style name)
	name256 := strings.Repeat("a", 256)
	valid := "module: \"" + name256 + "\"\nversion: \"1.0.0\""
	if err := validateCUEInvowkmod(t, valid); err != nil {
		t.Errorf("256-char module name should be valid, got error: %v", err)
	}

	// 257 characters should fail
	name257 := strings.Repeat("a", 257)
	invalid := "module: \"" + name257 + "\"\nversion: \"1.0.0\""
	if err := validateCUEInvowkmod(t, invalid); err == nil {
		t.Errorf("257-char module name should fail validation, but passed")
	}
}

// TestVersionLengthConstraint verifies #ModuleRequirement.version has a 64 rune limit.
func TestVersionLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 64 characters should pass (starts with digit to match regex)
	version64 := "1" + strings.Repeat("0", 63)
	valid := `{
	git_url: "https://github.com/user/test.invowkmod.git"
	version: "` + version64 + `"
}`
	if err := validateCUEModuleRequirement(t, valid); err != nil {
		t.Errorf("64-char version should be valid, got error: %v", err)
	}

	// 65 characters should fail
	version65 := "1" + strings.Repeat("0", 64)
	invalid := `{
	git_url: "https://github.com/user/test.invowkmod.git"
	version: "` + version65 + `"
}`
	if err := validateCUEModuleRequirement(t, invalid); err == nil {
		t.Errorf("65-char version should fail validation, but passed")
	}
}

// TestAliasLengthConstraint verifies #ModuleRequirement.alias has a 256 rune limit.
func TestAliasLengthConstraint(t *testing.T) {
	t.Parallel()

	// Exactly 256 characters should pass (valid alias matching regex)
	alias256 := strings.Repeat("a", 256)
	valid := `{
	git_url: "https://github.com/user/test.invowkmod.git"
	version: "^1.0.0"
	alias: "` + alias256 + `"
}`
	if err := validateCUEModuleRequirement(t, valid); err != nil {
		t.Errorf("256-char alias should be valid, got error: %v", err)
	}

	// 257 characters should fail
	alias257 := strings.Repeat("a", 257)
	invalid := `{
	git_url: "https://github.com/user/test.invowkmod.git"
	version: "^1.0.0"
	alias: "` + alias257 + `"
}`
	if err := validateCUEModuleRequirement(t, invalid); err == nil {
		t.Errorf("257-char alias should fail validation, but passed")
	}
}

// TestInvowkmodVersionConstraint verifies #Invowkmod.version accepts valid semver and rejects invalid formats.
func TestInvowkmodVersionConstraint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		version    string
		shouldPass bool
	}{
		// Valid semver versions
		{"valid zero version", "0.0.0", true},
		{"valid basic", "1.0.0", true},
		{"valid with all segments", "2.1.3", true},
		{"valid large numbers", "10.20.30", true},
		{"valid pre-release alpha", "1.0.0-alpha", true},
		{"valid pre-release alpha.1", "1.0.0-alpha.1", true},
		{"valid pre-release rc.2", "1.0.0-rc.2", true},
		{"valid pre-release beta on minor", "0.1.0-beta", true},
		{"valid pre-release x.y.z", "1.0.0-x.y.z", true},
		{"valid pre-release alpha1", "1.0.0-alpha1", true},
		{"valid large version numbers", "999.999.999", true},
		{"valid pre-release with hyphen", "1.0.0-pre-release", true},

		// Invalid versions
		{"invalid old format two segments", "1.0", false},
		{"invalid only major", "1", false},
		{"invalid v prefix", "v1.0.0", false},
		{"invalid leading zero major", "01.0.0", false},
		{"invalid leading zero minor", "1.01.0", false},
		{"invalid leading zero patch", "1.0.01", false},
		{"invalid build metadata", "1.0.0+build", false},
		{"invalid empty pre-release", "1.0.0-", false},
		{"invalid trailing dot in pre-release", "1.0.0-alpha.", false},
		{"invalid empty string", "", false},
		{"invalid four segments", "1.0.0.0", false},
		{"invalid non-numeric", "abc", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cueData := fmt.Sprintf("module: \"test.module\"\nversion: %q\n", tc.version)
			_, err := ParseInvowkmodBytes([]byte(cueData), "test.cue")
			if tc.shouldPass && err != nil {
				t.Errorf("version %q should be valid, got error: %v", tc.version, err)
			}
			if !tc.shouldPass && err == nil {
				t.Errorf("version %q should be invalid, but validation passed", tc.version)
			}
		})
	}
}

// TestInvowkmodVersionLengthConstraint verifies #Invowkmod.version has a 64-rune MaxRunes limit.
func TestInvowkmodVersionLengthConstraint(t *testing.T) {
	t.Parallel()

	// Build a valid semver base and pad to exact lengths.
	// "1.0.0-" is 6 chars, so 58 more chars reach exactly 64.
	base := "1.0.0-"
	pad58 := strings.Repeat("a", 58)
	pad59 := strings.Repeat("a", 59)

	version64 := base + pad58
	version65 := base + pad59

	// Sanity check lengths
	if len(version64) != 64 {
		t.Fatalf("expected 64-char version, got %d", len(version64))
	}
	if len(version65) != 65 {
		t.Fatalf("expected 65-char version, got %d", len(version65))
	}

	// Exactly 64 characters should pass
	cueData64 := fmt.Sprintf("module: \"test.module\"\nversion: %q\n", version64)
	if _, err := ParseInvowkmodBytes([]byte(cueData64), "test.cue"); err != nil {
		t.Errorf("64-char version should be valid, got error: %v", err)
	}

	// 65 characters should fail
	cueData65 := fmt.Sprintf("module: \"test.module\"\nversion: %q\n", version65)
	if _, err := ParseInvowkmodBytes([]byte(cueData65), "test.cue"); err == nil {
		t.Errorf("65-char version should fail validation, but passed")
	}
}

// TestPathRegexConstraints verifies #ModuleRequirement.path rejects absolute paths and path traversal.
func TestPathRegexConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		shouldPass bool
	}{
		{"valid relative path", "subdir/module", true},
		{"valid single segment", "mymodule", true},
		{"absolute path rejected", "/absolute/path", false},
		{"path traversal rejected", "sub/../escape", false},
		{"path traversal at start rejected", "../escape", false},
		{"double dot in middle rejected", "a/../../etc", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cueData := `{
	git_url: "https://github.com/user/test.invowkmod.git"
	version: "^1.0.0"
	path: "` + tc.path + `"
}`
			err := validateCUEModuleRequirement(t, cueData)
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
// Behavioral Sync Tests — CUE Oracle
// =============================================================================
// These tests verify that Go Validate() methods and CUE schema constraints
// produce the same accept/reject verdict on identical inputs.

// validateStringAgainstCUE validates a single string value against a CUE
// constraint path. Returns nil if CUE accepts the value, error if it rejects.
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
// within a CUE struct definition, including optional fields.
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
// constraint lookup for optional fields.
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
func runBehavioralSync(
	t *testing.T, schema cue.Value, ctx *cue.Context,
	cuePath string,
	goValidate func(string) error,
	cases []behavioralSyncCase,
) {
	t.Helper()

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

			cueErr := validateStringAgainstCUE(t, schema, ctx, cuePath, tc.input)
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

// TestBehavioralSync_ModuleID verifies Go ModuleID.Validate() agrees with
// CUE #Invowkmod.module constraint (RDNS regex + length).
func TestBehavioralSync_ModuleID(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Invowkmod.module",
		func(s string) error { return ModuleID(s).Validate() },
		[]behavioralSyncCase{
			{"io.invowk.sample", true, true, ""},
			{"com.example.mytools", true, true, ""},
			{"simple", true, true, ""},
			{"a", true, true, ""},
			{"a.b", true, true, ""},
			{"a1.b2", true, true, ""},
			{"", false, false, ""},
			{"123", false, false, ""},
			{".bad", false, false, ""},
			{"a.", false, false, ""},
			{"a..b", false, false, ""},
			{"a.1b", false, false, ""},
			{strings.Repeat("a", 256), true, true, ""},
			{strings.Repeat("a", 257), false, false, ""},
		},
	)
}

// TestBehavioralSync_SemVer verifies Go SemVer.Validate() agrees with
// CUE #Invowkmod.version constraint (strict semver regex).
func TestBehavioralSync_SemVer(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#Invowkmod.version",
		func(s string) error { return SemVer(s).Validate() },
		[]behavioralSyncCase{
			{"1.0.0", true, true, ""},
			{"0.1.0", true, true, ""},
			{"10.20.30", true, true, ""},
			{"1.0.0-alpha.1", true, true, ""},
			{"1.0.0-beta", true, true, ""},
			{"1.0.0-rc.1", true, true, ""},
			{"", false, false, ""},
			// Go's semver library is lenient: accepts "v" prefix, incomplete versions, leading zeros.
			// CUE enforces strict format via regex. Document as expected divergences.
			{"v1.0.0", true, false, "Go semver library strips v prefix; CUE regex rejects it"},
			{"01.0.0", true, false, "Go semver library accepts leading zeros; CUE regex rejects them"},
			{"1.0", true, false, "Go semver library accepts incomplete version; CUE regex requires MAJOR.MINOR.PATCH"},
			{"abc", false, false, ""},
		},
	)
}

// TestBehavioralSync_GitURL verifies Go GitURL.Validate() agrees with
// CUE #ModuleRequirement.git_url constraint (prefix regex + length).
func TestBehavioralSync_GitURL(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSync(t, schema, ctx, "#ModuleRequirement.git_url",
		func(s string) error { return GitURL(s).Validate() },
		[]behavioralSyncCase{
			{"https://github.com/user/repo.git", true, true, ""},
			{"git@github.com:user/repo.git", true, true, ""},
			{"ssh://git@host/repo", true, true, ""},
			{"https://gitlab.com/group/project.invowkmod.git", true, true, ""},
			{"", false, false, ""},
			{"ftp://bad.com/repo", false, false, ""},
			{"http://not-https.com/repo", false, false, ""},
			{"just-a-string", false, false, ""},
		},
	)
}

// TestBehavioralSync_SubdirectoryPath verifies Go SubdirectoryPath.Validate() agrees with
// CUE #ModuleRequirement.path constraint (relative path, no traversal).
// Note: Go performs additional security validation (path normalization via slashpath.Clean)
// that CUE's regex cannot express.
func TestBehavioralSync_SubdirectoryPath(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// path is an optional field in #ModuleRequirement — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#ModuleRequirement", "path",
		func(s string) error { return SubdirectoryPath(s).Validate() },
		[]behavioralSyncCase{
			{"subdir", true, true, ""},
			{"a/b/c", true, true, ""},
			{"module-name", true, true, ""},
			// Go accepts "" (repo root), CUE path? is optional so absent means root.
			// But if path is present in CUE, the field has constraints including =~"^[^/]".
			// An explicit empty string would not match the regex, so CUE rejects "".
			{"", true, false, "Go zero-value means repo root; CUE uses field optionality"},
			{"/absolute", false, false, ""},
			{"../traversal", false, false, ""},
		},
	)
}
