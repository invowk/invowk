// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"

	"github.com/invowk/invowk/internal/testutil/schematest"
)

// =============================================================================
// Schema Sync Tests — Runtime/Config Domain
// =============================================================================

// TestRuntimeConfigSchemaSync verifies RuntimeConfig Go struct matches CUE runtime definitions.
//
// Note: The CUE schema uses a union type (#RuntimeConfig = #RuntimeConfigNative | #RuntimeConfigVirtualSh | #RuntimeConfigVirtualLua | #RuntimeConfigContainer)
// while Go uses a single RuntimeConfig struct with all fields. We need to extract the union of all fields
// from the runtime variants, including the container source variants. This requires custom merge logic, so it
// remains a separate test.
func TestRuntimeConfigSchemaSync(t *testing.T) {
	t.Parallel()

	schema, _ := getCUESchema(t)
	allCUEFields := runtimeConfigCUEFields(t, schema)
	goFields := schematest.ExtractGoJSONTags(t, reflect.TypeFor[RuntimeConfig]())

	schematest.AssertFieldsSync(t, "RuntimeConfig", allCUEFields, goFields)
}

func runtimeConfigCUEFields(t *testing.T, schema cue.Value) map[string]bool {
	t.Helper()

	allFields := extractRuntimeConfigFields(t, schema, "#RuntimeConfigNative")
	for _, definition := range []string{"#RuntimeConfigVirtualSh", "#RuntimeConfigVirtualLua"} {
		mergeRuntimeConfigFields(allFields, extractRuntimeConfigFields(t, schema, definition), false)
	}
	for _, definition := range []string{"#RuntimeConfigContainerWithImage", "#RuntimeConfigContainerWithContainerfile"} {
		mergeRuntimeConfigFields(allFields, extractRuntimeConfigFields(t, schema, definition), true)
	}
	return allFields
}

func extractRuntimeConfigFields(t *testing.T, schema cue.Value, definition string) map[string]bool {
	t.Helper()

	return schematest.ExtractCUEFields(t, schematest.LookupDefinition(t, schema, definition))
}

func mergeRuntimeConfigFields(dst, src map[string]bool, forceContainerSourcesOptional bool) {
	for field, optional := range src {
		if forceContainerSourcesOptional && isContainerSourceField(field) {
			optional = true
		}
		if existing, ok := dst[field]; ok {
			dst[field] = existing || optional
			continue
		}
		dst[field] = optional
	}
}

func isContainerSourceField(field string) bool {
	return field == "image" || field == "containerfile"
}

func TestRuntimeConfigContainerSourceVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime string
		wantErr bool
	}{
		{
			name:    "image source",
			runtime: `{name: "container", image: "debian:stable-slim"}`,
		},
		{
			name:    "containerfile source",
			runtime: `{name: "container", containerfile: "Containerfile"}`,
		},
		{
			name:    "missing source",
			runtime: `{name: "container"}`,
			wantErr: true,
		},
		{
			name:    "duplicated source",
			runtime: `{name: "container", image: "debian:stable-slim", containerfile: "Containerfile"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
		runtimes: [` + tt.runtime + `]
		platforms: [{name: "linux"}]
	}]
}]`
			err := validateCUE(t, data)
			if tt.wantErr && err == nil {
				t.Fatal("validateCUE() error = nil, want source variant error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateCUE() error = %v, want nil", err)
			}
		})
	}
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
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "container", image: "` + image513 + `"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalid) == nil {
		t.Errorf("513-char image should fail validation, but passed")
	}
}

// TestInterpreterLengthConstraint verifies script interpreter fields have a 1024 rune limit.
// T091: Add boundary tests for interpreter length constraint (1024 chars)
func TestInterpreterLengthConstraint(t *testing.T) {
	t.Parallel()

	// Test script interpreter - exactly 1024 characters should pass
	interp1024 := strings.Repeat("a", 1024)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello", interpreter: "` + interp1024 + `"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Errorf("1024-char script interpreter should be valid, got error: %v", err)
	}

	// 1025 characters should fail
	interp1025 := strings.Repeat("a", 1025)
	invalid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello", interpreter: "` + interp1025 + `"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalid) == nil {
		t.Errorf("1025-char script interpreter should fail validation, but passed")
	}

	// Test custom-check script interpreter length on the same shared script source.
	validCheck := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
		depends_on: {custom_checks: [{name: "check", script: {content: "echo ok", interpreter: "` + interp1024 + `"}}]}
	}]
}]`
	if err := validateCUE(t, validCheck); err != nil {
		t.Errorf("1024-char custom-check interpreter should be valid, got error: %v", err)
	}

	invalidCheck := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
		depends_on: {custom_checks: [{name: "check", script: {content: "echo ok", interpreter: "` + interp1025 + `"}}]}
	}]
}]`
	if validateCUE(t, invalidCheck) == nil {
		t.Errorf("1025-char custom-check interpreter should fail validation, but passed")
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		files: [""]
	}
}]`
	if validateCUE(t, invalidEmpty) == nil {
		t.Errorf("empty env file path should fail validation, but passed")
	}

	// 4096-char path should pass
	path4096 := strings.Repeat("a", 4096)
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		files: ["` + path4097 + `"]
	}
}]`
	if validateCUE(t, invalid) == nil {
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
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		vars: {
			"123bad": "hello"
		}
	}
}]`
	if validateCUE(t, invalid) == nil {
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
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	env: {
		vars: {
			MY_VAR: "` + val32769 + `"
		}
	}
}]`
	if validateCUE(t, invalid) == nil {
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
		script: {content: "echo hello"}
		runtimes: [{name: "container", image: "debian:stable-slim", volumes: [""]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalidEmpty) == nil {
		t.Errorf("empty volume string should fail validation, but passed")
	}

	// Valid volume string should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "container", image: "debian:stable-slim", volumes: ["` + vol4097 + `"]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalid) == nil {
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
		script: {content: "echo hello"}
		runtimes: [{name: "container", image: "debian:stable-slim", ports: [""]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalidEmpty) == nil {
		t.Errorf("empty port string should fail validation, but passed")
	}

	// Valid port mapping should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "container", image: "debian:stable-slim", ports: ["` + port257 + `"]}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalid) == nil {
		t.Errorf("257-char port string should fail validation, but passed")
	}
}

func TestPersistentContainerConfigConstraints(t *testing.T) {
	t.Parallel()

	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
		runtimes: [{
			name: "container"
			image: "debian:stable-slim"
			persistent: {
				create_if_missing: true
				name: "existing_dev"
			}
		}]
		platforms: [{name: "linux"}]
	}]
}]`
	if err := validateCUE(t, valid); err != nil {
		t.Fatalf("valid persistent config should pass, got error: %v", err)
	}

	invalidName := strings.Replace(valid, `name: "existing_dev"`, `name: "ExistingDev"`, 1)
	if validateCUE(t, invalidName) == nil {
		t.Fatal("uppercase persistent container name should fail validation")
	}

	invalidRuntime := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
		runtimes: [{
			name: "native"
			persistent: {create_if_missing: true}
		}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalidRuntime) == nil {
		t.Fatal("persistent config on non-container runtime should fail validation")
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "` + name256 + `"
			script: {content: "echo ok"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "` + name257 + `"
			script: {content: "echo ok"}
		}]
	}
}]`
	if validateCUE(t, invalid) == nil {
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "mycheck"
			script: {content: "echo ok"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
	depends_on: {
		custom_checks: [{
			name: "mycheck"
			script: {content: "echo ok"}
			expected_output: "` + output1001 + `"
		}]
	}
}]`
	if validateCUE(t, invalid) == nil {
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
		script: {content: "echo hello"}
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}]
	}]
}]
default_shell: " "
`
	if validateCUE(t, invalid) == nil {
		t.Errorf("whitespace-only default_shell should fail validation, but passed")
	}

	// Valid shell path should pass
	valid := `
cmds: [{
	name: "test"
	implementations: [{
		script: {content: "echo hello"}
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
		script: {content: "echo hello"}
		runtimes: [{name: "container", image: ""}]
		platforms: [{name: "linux"}]
	}]
}]`
	if validateCUE(t, invalid) == nil {
		t.Errorf("empty image string should fail validation, but passed")
	}
}
