// SPDX-License-Identifier: MPL-2.0

package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/invowk/invowk/internal/testutil/schematest"
)

// behavioralSyncCase defines a single input for behavioral equivalence testing.
type behavioralSyncCase struct {
	input       string
	goExpect    bool
	cueExpect   bool
	divergeNote string
}

// configSchema is embedded in config.go and available to tests via the same package.

// =============================================================================
// Schema Sync Tests - Phase 3 (T014)
// =============================================================================
// These tests verify Go struct JSON tags match CUE schema field names.
// They catch misalignments at CI time, preventing silent parsing failures.

// getCUESchema compiles the embedded config CUE schema and returns the context and compiled value.
func getCUESchema(t *testing.T) (cue.Value, *cue.Context) {
	t.Helper()

	ctx := cuecontext.New()
	schema := ctx.CompileString(configSchema)
	if schema.Err() != nil {
		t.Fatalf("failed to compile CUE schema: %v", schema.Err())
	}

	return schema, ctx
}

// TestConfigSchemaSync verifies Config Go struct matches #Config CUE definition.
// T014: Create sync_test.go for Config struct
func TestConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#Config"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[Config]())

	schematest.AssertFieldsSync(t, "Config", cueFields, goFields)
}

// TestVirtualShellConfigSchemaSync verifies VirtualShellConfig Go struct matches #VirtualShellConfig CUE definition.
func TestVirtualShellConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#VirtualShellConfig"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[VirtualShellConfig]())

	schematest.AssertFieldsSync(t, "VirtualShellConfig", cueFields, goFields)
}

// TestUIConfigSchemaSync verifies UIConfig Go struct matches #UIConfig CUE definition.
func TestUIConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#UIConfig"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[UIConfig]())

	schematest.AssertFieldsSync(t, "UIConfig", cueFields, goFields)
}

// TestLLMConfigSchemaSync verifies LLMConfig Go struct matches #LLMConfig CUE definition.
func TestLLMConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#LLMConfig"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[LLMConfig]())

	schematest.AssertFieldsSync(t, "LLMConfig", cueFields, goFields)
}

// TestLLMAPIConfigSchemaSync verifies LLMAPIConfig Go struct matches #LLMAPIConfig CUE definition.
func TestLLMAPIConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#LLMAPIConfig"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[LLMAPIConfig]())

	schematest.AssertFieldsSync(t, "LLMAPIConfig", cueFields, goFields)
}

// TestContainerConfigSchemaSync verifies ContainerConfig Go struct matches #ContainerConfig CUE definition.
func TestContainerConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#ContainerConfig"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[ContainerConfig]())

	schematest.AssertFieldsSync(t, "ContainerConfig", cueFields, goFields)
}

// TestAutoProvisionConfigSchemaSync verifies AutoProvisionConfig Go struct matches #AutoProvisionConfig CUE definition.
func TestAutoProvisionConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#AutoProvisionConfig"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[AutoProvisionConfig]())

	schematest.AssertFieldsSync(t, "AutoProvisionConfig", cueFields, goFields)
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

// absoluteModulePath creates an OS-native absolute path to a test *.invowkmod entry.
func absoluteModulePath(t *testing.T, elems ...string) string {
	t.Helper()

	parts := append([]string{t.TempDir()}, elems...)
	return filepath.Join(parts...)
}

// TestIncludeEntrySchemaSync verifies IncludeEntry Go struct matches #IncludeEntry CUE definition.
func TestIncludeEntrySchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	cueFields := schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, "#IncludeEntry"))
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[IncludeEntry]())

	schematest.AssertFieldsSync(t, "IncludeEntry", cueFields, goFields)
}

// TestIncludesEntryConstraints verifies #IncludeEntry path rejects empty strings,
// enforces the 4096 rune limit, and only accepts paths ending with .invowkmod.
func TestIncludesEntryConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "empty path rejected",
			cueData: `includes: [{path: ""}]`,
			wantErr: true,
		},
		{
			name:    "path not ending with invowkmod rejected",
			cueData: `includes: [{path: "/some/random/path"}]`,
			wantErr: true,
		},
		{
			name:    "invowkfile.cue path rejected",
			cueData: `includes: [{path: "/home/user/invowkfile.cue"}]`,
			wantErr: true,
		},
		{
			name:    "invowkfile path rejected",
			cueData: `includes: [{path: "/home/user/invowkfile"}]`,
			wantErr: true,
		},
		{
			name:    "invowkmod path accepted",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod"}]`,
			wantErr: false,
		},
		{
			name:    "path over 4096 chars rejected",
			cueData: `includes: [{path: "/` + strings.Repeat("a", 4090) + `.invowkmod"}]`,
			wantErr: true,
		},
		{
			name:    "alias on invowkmod accepted",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod", alias: "my-alias"}]`,
			wantErr: false,
		},
		{
			name:    "empty alias rejected",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod", alias: ""}]`,
			wantErr: true,
		},
		{
			name:    "alias starting with digit rejected",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod", alias: "1bad"}]`,
			wantErr: true,
		},
		{
			name:    "alias containing space rejected",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod", alias: "bad alias"}]`,
			wantErr: true,
		},
		{
			name:    "container alias starting with digit rejected",
			cueData: `container: {auto_provision: {includes: [{path: "/home/user/mymod.invowkmod", alias: "1bad"}]}}`,
			wantErr: true,
		},
		{
			name:    "container alias containing space rejected",
			cueData: `container: {auto_provision: {includes: [{path: "/home/user/mymod.invowkmod", alias: "bad alias"}]}}`,
			wantErr: true,
		},
		{
			name:    "alias over 256 chars rejected",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod", alias: "` + strings.Repeat("a", 257) + `"}]`,
			wantErr: true,
		},
		{
			name:    "alias at 256 chars accepted",
			cueData: `includes: [{path: "/home/user/mymod.invowkmod", alias: "` + strings.Repeat("a", 256) + `"}]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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

func TestLLMSchemaConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "provider accepted",
			cueData: `llm: {provider: "codex"}`,
			wantErr: false,
		},
		{
			name:    "unknown provider rejected",
			cueData: `llm: {provider: "llama-file"}`,
			wantErr: true,
		},
		{
			name:    "negative concurrency rejected",
			cueData: `llm: {concurrency: -1}`,
			wantErr: true,
		},
		{
			name:    "valid api key env accepted",
			cueData: `llm: {api: {api_key_env: "OPENAI_API_KEY"}}`,
			wantErr: false,
		},
		{
			name:    "invalid api key env rejected",
			cueData: `llm: {api: {api_key_env: "1_BAD"}}`,
			wantErr: true,
		},
		{
			name:    "empty api model rejected",
			cueData: `llm: {api: {model: ""}}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateCUE(t, tt.cueData)
			if tt.wantErr && err == nil {
				t.Fatal("validateCUE() succeeded, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateCUE() error = %v, want nil", err)
			}
		})
	}
}

// TestBinaryPathConstraints verifies container.auto_provision.binary_path rejects empty
// strings and enforces the 4096 rune limit.
func TestBinaryPathConstraints(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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

// TestAutoProvisionIncludesConstraints verifies container.auto_provision.includes
// uses the same #IncludeEntry schema (modules-only paths).
func TestAutoProvisionIncludesConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cueData string
		wantErr bool
	}{
		{
			name:    "invowkmod path accepted",
			cueData: `container: auto_provision: { includes: [{path: "/home/user/mymod.invowkmod"}] }`,
			wantErr: false,
		},
		{
			name:    "invowkfile path rejected",
			cueData: `container: auto_provision: { includes: [{path: "/home/user/invowkfile.cue"}] }`,
			wantErr: true,
		},
		{
			name:    "empty path rejected",
			cueData: `container: auto_provision: { includes: [{path: ""}] }`,
			wantErr: true,
		},
		{
			name:    "alias accepted",
			cueData: `container: auto_provision: { includes: [{path: "/home/user/mymod.invowkmod", alias: "my-alias"}] }`,
			wantErr: false,
		},
		{
			name:    "inherit_includes boolean accepted",
			cueData: `container: auto_provision: { inherit_includes: false }`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()

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
			t.Parallel()

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

// TestValidateIncludes verifies the Go-level validation for includes constraints
// that CUE cannot express (path uniqueness, alias uniqueness, short-name collision).
func TestValidateIncludes(t *testing.T) {
	t.Parallel()

	moduleWithAliasPath := absoluteModulePath(t, "path", "to", "mymod.invowkmod")
	moduleWithoutAliasPath := absoluteModulePath(t, "path", "to", "mymod.invowkmod")
	mod1Path := absoluteModulePath(t, "path", "to", "mod1.invowkmod")
	mod2Path := absoluteModulePath(t, "path", "to", "mod2.invowkmod")
	fooPath := absoluteModulePath(t, "path", "to", "foo.invowkmod")
	barPath := absoluteModulePath(t, "path", "to", "bar.invowkmod")
	fooAPath := absoluteModulePath(t, "path", "a", "foo.invowkmod")
	fooBPath := absoluteModulePath(t, "path", "b", "foo.invowkmod")
	duplicatePath := absoluteModulePath(t, "path", "to", "mymod.invowkmod")
	duplicatePathTrailing := duplicatePath + string(filepath.Separator)

	tests := []struct {
		name     string
		includes []IncludeEntry
		wantErr  bool
	}{
		{
			name:     "empty includes valid",
			includes: nil,
			wantErr:  false,
		},
		{
			name: "module with alias valid",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(moduleWithAliasPath), Alias: "my-alias"},
			},
			wantErr: false,
		},
		{
			name: "module without alias valid",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(moduleWithoutAliasPath)},
			},
			wantErr: false,
		},
		{
			name: "duplicate alias rejected",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(mod1Path), Alias: "same-alias"},
				{Path: ModuleIncludePath(mod2Path), Alias: "same-alias"},
			},
			wantErr: true,
		},
		{
			name: "different aliases accepted",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(mod1Path), Alias: "alias-one"},
				{Path: ModuleIncludePath(mod2Path), Alias: "alias-two"},
			},
			wantErr: false,
		},
		{
			name: "two modules different short names no aliases accepted",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(fooPath)},
				{Path: ModuleIncludePath(barPath)},
			},
			wantErr: false,
		},
		{
			name: "two modules same short name no aliases rejected",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(fooAPath)},
				{Path: ModuleIncludePath(fooBPath)},
			},
			wantErr: true,
		},
		{
			name: "two modules same short name unique aliases accepted",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(fooAPath), Alias: "foo-a"},
				{Path: ModuleIncludePath(fooBPath), Alias: "foo-b"},
			},
			wantErr: false,
		},
		{
			name: "two modules same short name only one has alias rejected",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(fooAPath), Alias: "foo-a"},
				{Path: ModuleIncludePath(fooBPath)},
			},
			wantErr: true,
		},
		{
			name: "relative path rejected",
			includes: []IncludeEntry{
				{Path: "relative/path/mymod.invowkmod"},
			},
			wantErr: true,
		},
		{
			name: "dot-relative path rejected",
			includes: []IncludeEntry{
				{Path: "./local/mymod.invowkmod"},
			},
			wantErr: true,
		},
		{
			name: "duplicate path rejected",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(duplicatePath)},
				{Path: ModuleIncludePath(duplicatePath)},
			},
			wantErr: true,
		},
		{
			name: "duplicate path with trailing slash rejected",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(duplicatePath)},
				{Path: ModuleIncludePath(duplicatePathTrailing)},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateIncludes(IncludeCollectionRoot, tt.includes)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateIncludes_OSNativeAbsolutePathSemantics verifies the intended
// platform-dependent behavior for path absoluteness:
// - OS-native absolute paths are accepted on all platforms
// - Unix-style absolute literals are rejected on Windows
func TestValidateIncludes_OSNativeAbsolutePathSemantics(t *testing.T) {
	t.Parallel()

	nativeAbsolutePath := absoluteModulePath(t, "native", "mymod.invowkmod")
	if err := validateIncludes(IncludeCollectionRoot, []IncludeEntry{{Path: ModuleIncludePath(nativeAbsolutePath)}}); err != nil {
		t.Fatalf("expected OS-native absolute path to be valid, got: %v", err)
	}

	unixStyleAbsolute := "/tmp/mymod.invowkmod"
	err := validateIncludes(IncludeCollectionRoot, []IncludeEntry{{Path: ModuleIncludePath(unixStyleAbsolute)}})
	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("expected Unix-style absolute path %q to be rejected on Windows", unixStyleAbsolute)
		}
		if !errors.Is(err, ErrInvalidIncludeEntry) {
			t.Fatalf("expected ErrInvalidIncludeEntry, got: %v", err)
		}
		return
	}

	if err != nil {
		t.Fatalf("expected Unix-style absolute path %q to be valid on %s, got: %v", unixStyleAbsolute, runtime.GOOS, err)
	}
}

// TestValidateAutoProvisionIncludes verifies that the same validation rules
// apply to container.auto_provision.includes entries.
func TestValidateAutoProvisionIncludes(t *testing.T) {
	t.Parallel()

	modulePath := absoluteModulePath(t, "path", "to", "mymod.invowkmod")
	fooAPath := absoluteModulePath(t, "a", "foo.invowkmod")
	fooBPath := absoluteModulePath(t, "b", "foo.invowkmod")

	tests := []struct {
		name     string
		includes []IncludeEntry
		wantErr  bool
	}{
		{
			name:     "empty includes valid",
			includes: nil,
			wantErr:  false,
		},
		{
			name: "module accepted",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(modulePath)},
			},
			wantErr: false,
		},
		{
			name: "same short name collision rejected",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(fooAPath)},
				{Path: ModuleIncludePath(fooBPath)},
			},
			wantErr: true,
		},
		{
			name: "same short name with aliases accepted",
			includes: []IncludeEntry{
				{Path: ModuleIncludePath(fooAPath), Alias: "foo-a"},
				{Path: ModuleIncludePath(fooBPath), Alias: "foo-b"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateIncludes(IncludeCollectionAutoProvision, tt.includes)
			if tt.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// =============================================================================
// Behavioral Sync Tests — CUE Oracle
// =============================================================================
// These tests verify that Go Validate() methods and CUE schema constraints
// produce the same accept/reject verdict on identical inputs.

// runBehavioralSyncField runs behavioral equivalence tests using field-level CUE
// constraint lookup for optional fields.
func runBehavioralSyncField(
	t *testing.T, schema cue.Value, ctx *cue.Context,
	parentPath, fieldName string,
	goValidate func(string) error,
	cases []behavioralSyncCase,
) {
	t.Helper()

	constraint := schematest.LookupCUEFieldConstraint(t, schema, parentPath, fieldName)
	for _, tc := range cases {
		label := tc.input
		if len(label) > 30 {
			label = label[:27] + "..."
		}
		if label == "" {
			label = "<empty>"
		}

		t.Run(label, func(t *testing.T) {
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

//nolint:tparallel // CUE Value.Unify() and Context.CompileString() mutate internal state; subtests must be serial.
func TestBehavioralSync_ContainerEngine(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// container_engine is an optional field in #Config — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#Config", "container_engine",
		func(s string) error { return ContainerEngine(s).Validate() },
		[]behavioralSyncCase{
			{"podman", true, true, ""},
			{"docker", true, true, ""},
			{"invalid", false, false, ""},
			{"PODMAN", false, false, ""},
			// Config types reject empty (unlike invowkfile types which accept empty for defaults).
			// Both Go and CUE reject empty — agreement on rejection.
			{"", false, false, ""},
		},
	)
}

//nolint:tparallel // CUE Value.Unify() and Context.CompileString() mutate internal state; subtests must be serial.
func TestBehavioralSync_ConfigRuntimeMode(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// default_runtime is an optional field in #Config — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#Config", "default_runtime",
		func(s string) error { return RuntimeMode(s).Validate() },
		[]behavioralSyncCase{
			{"native", true, true, ""},
			{"virtual", true, true, ""},
			{"container", true, true, ""},
			{"invalid", false, false, ""},
			{"NATIVE", false, false, ""},
			{"", false, false, ""},
		},
	)
}

//nolint:tparallel // CUE Value.Unify() and Context.CompileString() mutate internal state; subtests must be serial.
func TestBehavioralSync_ColorScheme(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	// color_scheme is an optional field in #UIConfig — use field-level lookup
	runBehavioralSyncField(t, schema, ctx, "#UIConfig", "color_scheme",
		func(s string) error { return ColorScheme(s).Validate() },
		[]behavioralSyncCase{
			{"auto", true, true, ""},
			{"dark", true, true, ""},
			{"light", true, true, ""},
			{"invalid", false, false, ""},
			{"AUTO", false, false, ""},
			{"", false, false, ""},
		},
	)
}
