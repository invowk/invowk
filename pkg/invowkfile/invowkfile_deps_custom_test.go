// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Custom Check Dependencies
// ============================================================================

// TestGenerateCUE_WithRootLevelDependsOn_CustomChecks verifies GenerateCUE handles custom_checks at root level
func TestGenerateCUE_WithRootLevelDependsOn_CustomChecks(t *testing.T) {
	t.Parallel()

	expectedCode := 0
	inv := &Invowkfile{
		DependsOn: &DependsOn{
			CustomChecks: []CustomCheckDependency{
				{
					Name:         "check-version",
					CheckScript:  "sh --version",
					ExpectedCode: &expectedCode,
				},
				{
					Alternatives: []CustomCheck{
						{Name: "alt1", CheckScript: "echo 1"},
						{Name: "alt2", CheckScript: "echo 2"},
					},
				},
			},
		},
		Commands: []Command{
			{
				Name: "hello",
				Implementations: []Implementation{
					{
						Script:    "echo hello",
						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	result := GenerateCUE(inv)

	// Verify custom_checks section is present at root level
	if !strings.Contains(result, "custom_checks:") {
		t.Error("GenerateCUE should include 'custom_checks:' section at root level")
	}
	if !strings.Contains(result, `"check-version"`) {
		t.Error("GenerateCUE should include 'check-version' custom check name")
	}
	if !strings.Contains(result, `"sh --version"`) {
		t.Error("GenerateCUE should include 'sh --version' check_script")
	}
	if !strings.Contains(result, "expected_code: 0") {
		t.Error("GenerateCUE should include 'expected_code: 0'")
	}
	if !strings.Contains(result, "alternatives:") {
		t.Error("GenerateCUE should include alternatives for custom checks")
	}
}

// TestParse_RootLevelDependsOn_CustomChecks verifies custom_checks parsing at root level
func TestParse_RootLevelDependsOn_CustomChecks(t *testing.T) {
	t.Parallel()

	cueContent := `
depends_on: {
	custom_checks: [
		{
			name: "version-check"
			check_script: "sh --version"
			expected_code: 0
		},
		{
			alternatives: [
				{name: "bash-check", check_script: "bash --version"},
				{name: "sh-check", check_script: "sh --version"}
			]
		}
	]
}

cmds: [
	{
		name: "hello"
		implementations: [
			{
				script: "echo hello"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	if parsed.DependsOn == nil {
		t.Fatal("Invowkfile.DependsOn should not be nil")
	}

	if len(parsed.DependsOn.CustomChecks) != 2 {
		t.Fatalf("Expected 2 custom checks, got %d", len(parsed.DependsOn.CustomChecks))
	}

	// First check is direct (not alternatives)
	check1 := parsed.DependsOn.CustomChecks[0]
	if check1.IsAlternatives() {
		t.Error("First custom check should not be alternatives format")
	}
	if check1.Name != "version-check" {
		t.Errorf("First check name = %q, want %q", check1.Name, "version-check")
	}
	if check1.CheckScript != "sh --version" {
		t.Errorf("First check script = %q, want %q", check1.CheckScript, "sh --version")
	}

	// Second check uses alternatives
	check2 := parsed.DependsOn.CustomChecks[1]
	if !check2.IsAlternatives() {
		t.Error("Second custom check should be alternatives format")
	}
	if len(check2.Alternatives) != 2 {
		t.Fatalf("Expected 2 alternatives in second check, got %d", len(check2.Alternatives))
	}
}

// TestParse_RootLevelDependsOn_CommandDeps verifies command dependencies parsing at root level
func TestParse_RootLevelDependsOn_CommandDeps(t *testing.T) {
	t.Parallel()

	cueContent := `
depends_on: {
	cmds: [
		{alternatives: ["test setup"]},
		{alternatives: ["test init", "test bootstrap"]}
	]
}

cmds: [
	{
		name: "hello"
		implementations: [
			{
				script: "echo hello"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`
	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	if parsed.DependsOn == nil {
		t.Fatal("Invowkfile.DependsOn should not be nil")
	}

	if len(parsed.DependsOn.Commands) != 2 {
		t.Fatalf("Expected 2 command dependencies, got %d", len(parsed.DependsOn.Commands))
	}

	// First command dependency has one alternative
	if len(parsed.DependsOn.Commands[0].Alternatives) != 1 {
		t.Errorf("Expected 1 alternative in first command dep, got %d", len(parsed.DependsOn.Commands[0].Alternatives))
	}
	if parsed.DependsOn.Commands[0].Alternatives[0] != "test setup" {
		t.Errorf("First command dep alternative = %q, want %q", parsed.DependsOn.Commands[0].Alternatives[0], "test setup")
	}

	// Second command dependency has two alternatives
	if len(parsed.DependsOn.Commands[1].Alternatives) != 2 {
		t.Errorf("Expected 2 alternatives in second command dep, got %d", len(parsed.DependsOn.Commands[1].Alternatives))
	}
}

func TestParseCustomChecks_ValidCheckScript(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
depends_on: {
	custom_checks: [
		{name: "check-docker", check_script: "docker --version"},
		{name: "version-check", check_script: "echo v1.0.0", expected_output: "^v[0-9]+\\.[0-9]+"},
	]
}
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() should accept valid custom_checks: %v", err)
	}

	if parsed.DependsOn == nil || len(parsed.DependsOn.CustomChecks) != 2 {
		t.Errorf("Expected 2 custom_checks, got %v", parsed.DependsOn)
	}
}

func TestParseCustomChecks_RejectsLongCheckScript(t *testing.T) {
	t.Parallel()

	// Create a check_script that exceeds MaxScriptLength (10MB).
	// Since DefaultMaxCUEFileSize is 5MB, the file size guard triggers first.
	// This is correct behavior - the file size guard prevents OOM attacks.
	longScript := strings.Repeat("echo test; ", MaxScriptLength/11+1)

	cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
depends_on: {
	custom_checks: [
		{name: "check", check_script: %q},
	]
}
`, longScript)

	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject oversized file")
	}
	// The file is >10MB, so it exceeds DefaultMaxCUEFileSize (5MB) first
	if err != nil && !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("Expected error about file size limit, got: %v", err)
	}
}

func TestParseCustomChecks_RejectsDangerousExpectedOutput(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
depends_on: {
	custom_checks: [
		{name: "check", check_script: "echo test", expected_output: "(a+)+"},
	]
}
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject dangerous expected_output regex pattern")
	}
	if err != nil && !strings.Contains(err.Error(), "nested quantifiers") {
		t.Errorf("Expected error about nested quantifiers, got: %v", err)
	}
}

func TestParseCustomChecks_CommandLevel(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			custom_checks: [
				{name: "check-docker", check_script: "docker --version"},
			]
		}
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() should accept valid command-level custom_checks: %v", err)
	}

	if parsed.Commands[0].DependsOn == nil || len(parsed.Commands[0].DependsOn.CustomChecks) != 1 {
		t.Errorf("Expected 1 command-level custom_check")
	}
}

func TestParseCustomChecks_CommandLevelRejectsDangerousPattern(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		depends_on: {
			custom_checks: [
				{name: "check", check_script: "echo test", expected_output: "(a+)+"},
			]
		}
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject dangerous expected_output in command-level custom_checks")
	}
	if err != nil && !strings.Contains(err.Error(), "command 'test'") {
		t.Errorf("Error should mention command name, got: %v", err)
	}
}

func TestParseCustomChecks_ImplementationLevel(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
				depends_on: {
					custom_checks: [
						{name: "check-docker", check_script: "docker --version"},
					]
				}
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() should accept valid implementation-level custom_checks: %v", err)
	}

	if parsed.Commands[0].Implementations[0].DependsOn == nil ||
		len(parsed.Commands[0].Implementations[0].DependsOn.CustomChecks) != 1 {
		t.Errorf("Expected 1 implementation-level custom_check")
	}
}

func TestParseCustomChecks_ImplementationLevelRejectsLongCheckScript(t *testing.T) {
	t.Parallel()

	// Create a check_script that exceeds MaxScriptLength (10MB).
	// Since DefaultMaxCUEFileSize is 5MB, the file size guard triggers first.
	longScript := strings.Repeat("echo test; ", MaxScriptLength/11+1)

	cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "test"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
				depends_on: {
					custom_checks: [
						{name: "check", check_script: %q},
					]
				}
			}
		]
	}
]
`, longScript)

	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject oversized file")
	}
	// The file is >10MB, so it exceeds DefaultMaxCUEFileSize (5MB) first
	if err != nil && !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("Expected error about file size limit, got: %v", err)
	}
}
