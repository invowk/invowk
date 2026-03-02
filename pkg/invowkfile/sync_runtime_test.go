// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"maps"
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// Schema Sync Tests — Runtime/Config Domain
// =============================================================================

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
// Constraint Boundary Tests — Runtime/Config Domain
// =============================================================================

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
// Behavioral Sync Tests — Runtime/Config Domain
// =============================================================================

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
			// Both Go and CUE reject >512 chars. Go Validate() now includes length,
			// injection, and format checks (merged from ValidateContainerImage).
			{strings.Repeat("a", 513), false, false, ""},
		},
	)
}

// TestBehavioralSync_EnvVarName verifies Go EnvVarName.Validate() agrees with
// CUE #EnvVarCheck.name constraint (=~"^[A-Za-z_][A-Za-z0-9_]*$").
func TestBehavioralSync_EnvVarName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#EnvVarCheck", "name",
		func(s string) error { return EnvVarName(s).Validate() },
		[]behavioralSyncCase{
			{"HOME", true, true, ""},
			{"_private", true, true, ""},
			{"PATH", true, true, ""},
			{"MY_VAR_123", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"123BAD", false, false, ""},
			{"-invalid", false, false, ""},
			{"has space", false, false, ""},
			{"has-hyphen", false, false, ""},
		},
	)
}

// TestBehavioralSync_DotenvFilePath verifies Go DotenvFilePath.Validate() agrees with
// CUE #EnvConfig.files element constraint (!="" & strings.MaxRunes(4096)).
func TestBehavioralSync_DotenvFilePath(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncListElement(t, schema, ctx, "#EnvConfig", "files",
		func(s string) error { return DotenvFilePath(s).Validate() },
		[]behavioralSyncCase{
			{".env", true, true, ""},
			{".env.local", true, true, ""},
			{"path/to/.env", true, true, ""},
			{".env?", true, true, ""},
			{"", false, false, ""},
		},
	)
}

// TestBehavioralSync_VolumeMountSpec verifies Go VolumeMountSpec.Validate() agrees with
// CUE #RuntimeConfigContainer.volumes element constraint (!="" & strings.MaxRunes(4096)).
func TestBehavioralSync_VolumeMountSpec(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncListElement(t, schema, ctx, "#RuntimeConfigContainer", "volumes",
		func(s string) error { return VolumeMountSpec(s).Validate() },
		[]behavioralSyncCase{
			{"./data:/data", true, true, ""},
			{"/tmp:/tmp:ro", true, true, ""},
			{"", false, false, ""},
			// Go requires ':' separator; CUE only requires non-empty
			{"no-colon", false, true, "Go requires host:container format; CUE only checks non-empty"},
		},
	)
}

// TestBehavioralSync_PortMappingSpec verifies Go PortMappingSpec.Validate() agrees with
// CUE #RuntimeConfigContainer.ports element constraint (!="" & strings.MaxRunes(256)).
func TestBehavioralSync_PortMappingSpec(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncListElement(t, schema, ctx, "#RuntimeConfigContainer", "ports",
		func(s string) error { return PortMappingSpec(s).Validate() },
		[]behavioralSyncCase{
			{"8080:80", true, true, ""},
			{"3000:3000", true, true, ""},
			{"", false, false, ""},
			// Go requires ':' separator; CUE only requires non-empty
			{"no-colon", false, true, "Go requires host:container format; CUE only checks non-empty"},
		},
	)
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

// TestBehavioralSync_GlobPattern verifies Go GlobPattern.Validate() agrees with
// CUE #WatchConfig.patterns element constraint (!="" & strings.MaxRunes(4096)).
func TestBehavioralSync_GlobPattern(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncListElement(t, schema, ctx, "#WatchConfig", "patterns",
		func(s string) error { return GlobPattern(s).Validate() },
		[]behavioralSyncCase{
			{"**/*.go", true, true, ""},
			{"src/**", true, true, ""},
			{"*.ts", true, true, ""},
			{"", false, false, ""},
		},
	)
}

// TestBehavioralSync_BinaryName verifies Go BinaryName.Validate() agrees with
// CUE #ToolDependency.alternatives element constraint (=~"^[a-zA-Z0-9][a-zA-Z0-9._+-]*$").
func TestBehavioralSync_BinaryName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncListElement(t, schema, ctx, "#ToolDependency", "alternatives",
		func(s string) error { return BinaryName(s).Validate() },
		[]behavioralSyncCase{
			{"git", true, true, ""},
			{"python3.11", true, true, ""},
			{"g++", true, true, ""},
			{"my-tool", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"/usr/bin/git", false, false, ""},
			// Go BinaryName.Validate() only checks non-empty + no path separators.
			// CUE regex is stricter: must start with alphanumeric, only [a-zA-Z0-9._+-].
			{".hidden", true, false, "Go allows dot-start; CUE regex requires alphanumeric start"},
			{"-flag", true, false, "Go allows hyphen-start; CUE regex requires alphanumeric start"},
			{"has space", true, false, "Go allows spaces; CUE regex does not include space"},
		},
	)
}

// TestBehavioralSync_CheckName verifies Go CheckName.Validate() agrees with
// CUE #CustomCheck.name constraint (!="" & strings.MaxRunes(256)).
func TestBehavioralSync_CheckName(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#CustomCheck", "name",
		func(s string) error { return CheckName(s).Validate() },
		[]behavioralSyncCase{
			{"check-ports", true, true, ""},
			{"validate", true, true, ""},
			{"a", true, true, ""},
			{"", false, false, ""},
			{"   ", false, true, "Go rejects whitespace-only; CUE !=\"\" only rejects literal empty"},
		},
	)
}

// TestBehavioralSync_ScriptContent verifies Go ScriptContent.Validate() agrees with
// CUE #CustomCheck.check_script constraint (!="" & strings.MaxRunes(10485760)).
func TestBehavioralSync_ScriptContent(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#CustomCheck", "check_script",
		func(s string) error { return ScriptContent(s).Validate() },
		[]behavioralSyncCase{
			{"echo hello", true, true, ""},
			{"#!/bin/bash\nset -e\necho ok", true, true, ""},
			{"a", true, true, ""},
			// Go accepts "" (zero=valid for ScriptContent), CUE rejects "" (!="")
			{"", true, false, "Go zero-value is valid (no script); CUE !=\"\" rejects empty"},
			{"   ", false, true, "Go rejects whitespace-only; CUE !=\"\" only rejects literal empty"},
		},
	)
}

// TestBehavioralSync_ContainerfilePath verifies Go ContainerfilePath.Validate() agrees with
// CUE #RuntimeConfigContainer.containerfile constraint (strings.MaxRunes(4096) & =~"^[^/]" & !~"\\.\\.", optional).
func TestBehavioralSync_ContainerfilePath(t *testing.T) {
	t.Parallel()
	schema, ctx := getCUESchema(t)

	runBehavioralSyncField(t, schema, ctx, "#RuntimeConfigContainer", "containerfile",
		func(s string) error { return ContainerfilePath(s).Validate() },
		[]behavioralSyncCase{
			{"Dockerfile", true, true, ""},
			{"build/Containerfile", true, true, ""},
			{"my.Dockerfile", true, true, ""},
			// Go accepts "" (no containerfile), CUE rejects as optional field
			{"", true, false, "Go zero-value means no containerfile; CUE uses field optionality"},
			{"   ", false, true, "Go rejects whitespace-only; CUE regex accepts non-slash start"},
		},
	)
}
