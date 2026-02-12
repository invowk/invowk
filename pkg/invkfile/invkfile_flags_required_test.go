// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Enhanced Flags Feature - Required and Short alias handling
// ============================================================================

func TestParseFlags_RequiredFlag(t *testing.T) {
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
		flags: [
			{name: "myflag", description: "Test flag", required: true},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flag := inv.Commands[0].Flags[0]
	if !flag.Required {
		t.Errorf("Flag.Required = false, want true")
	}
}

func TestParseFlagsValidation_RequiredWithDefaultValue(t *testing.T) {
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
		flags: [
			{name: "myflag", description: "Test flag", required: true, default_value: "foo"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject flag that is both required and has default_value")
	}
	if err != nil && !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "default_value") {
		t.Errorf("Error message should mention required and default_value conflict, got: %v", err)
	}
}

func TestParseFlags_ShortAlias(t *testing.T) {
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
		flags: [
			{name: "verbose", description: "Enable verbose output", short: "x"},
			{name: "quiet", description: "Quiet mode", short: "q"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	inv, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	flags := inv.Commands[0].Flags
	if flags[0].Short != "x" {
		t.Errorf("Flag[0].Short = %q, want %q", flags[0].Short, "x")
	}
	if flags[1].Short != "q" {
		t.Errorf("Flag[1].Short = %q, want %q", flags[1].Short, "q")
	}
}

func TestParseFlagsValidation_InvalidShortAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		short string
	}{
		{"multiple chars", "ab"},
		{"digit", "1"},
		{"special char", "-"},
		{"empty string is valid", ""}, // Should NOT cause error - empty means no short alias
	}

	for _, tt := range tests {
		if tt.short == "" {
			continue // Skip empty - it's valid
		}
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
		flags: [
			{name: "myflag", description: "Test flag", short: "%s"},
		]
	}
]
`, tt.short)

			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if err := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); err != nil {
				t.Fatalf("Failed to write invkfile: %v", err)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Errorf("Parse() should reject invalid short alias %q", tt.short)
			}
		})
	}
}

func TestParseFlagsValidation_DuplicateShortAlias(t *testing.T) {
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
		flags: [
			{name: "verbose", description: "Enable verbose output", short: "x"},
			{name: "version", description: "Show version", short: "x"},
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Errorf("Parse() should reject duplicate short alias")
	}
	if err != nil && !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "short") {
		t.Errorf("Error message should mention duplicate short alias, got: %v", err)
	}
}

// ============================================================================
// Tests for Reserved Flag Names and Short Aliases
// ============================================================================

func TestValidateFlags_ReservedInvkEnvFileName(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "invk-env-file", description: "This should fail - reserved flag name"}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Fatal("Parse() should fail for reserved flag name 'invk-env-file', got nil error")
	}

	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention 'reserved', got: %v", err)
	}
}

func TestValidateFlags_ReservedShortAliasE(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "environment", short: "e", description: "This should fail - reserved short alias"}
		]
	}
]
`
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invkfile: %v", writeErr)
	}

	_, err := Parse(invkfilePath)
	if err == nil {
		t.Fatal("Parse() should fail for reserved short alias 'e', got nil error")
	}

	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention 'reserved', got: %v", err)
	}
	if !strings.Contains(err.Error(), "invk-env-file") {
		t.Errorf("Error should mention 'invk-env-file', got: %v", err)
	}
}

func TestValidateFlags_ReservedFlagNames(t *testing.T) {
	t.Parallel()

	reservedNames := []string{
		"invk-env-inherit-mode", "invk-env-inherit-allow", "invk-env-inherit-deny",
		"invk-workdir", "help", "invk-runtime", "invk-from", "invk-force-rebuild",
		"version", "invk-verbose", "invk-config", "invk-interactive",
	}

	for _, name := range reservedNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "%s", description: "This should fail - reserved flag name"}
		]
	}
]
`, name)
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkfile: %v", writeErr)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Fatalf("Parse() should fail for reserved flag name %q, got nil error", name)
			}

			if !strings.Contains(err.Error(), "reserved") {
				t.Errorf("Error should mention 'reserved', got: %v", err)
			}
		})
	}
}

func TestValidateFlags_ReservedShortAliases(t *testing.T) {
	t.Parallel()

	reservedShorts := []struct {
		short    string
		longFlag string
	}{
		{"w", "invk-workdir"},
		{"h", "help"},
		{"r", "invk-runtime"},
		{"v", "invk-verbose"},
		{"i", "invk-interactive"},
		{"c", "invk-config"},
		{"f", "invk-from"},
	}

	for _, tc := range reservedShorts {
		t.Run(tc.short, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "myflag", short: "%s", description: "This should fail - reserved short alias"}
		]
	}
]
`, tc.short)
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkfile: %v", writeErr)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Fatalf("Parse() should fail for reserved short alias %q (--"+tc.longFlag+"), got nil error", tc.short)
			}

			if !strings.Contains(err.Error(), "reserved") {
				t.Errorf("Error should mention 'reserved', got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.longFlag) {
				t.Errorf("Error should mention the long flag '--%s', got: %v", tc.longFlag, err)
			}
		})
	}
}

func TestValidateFlags_InvkPrefixReserved(t *testing.T) {
	t.Parallel()

	// Any flag starting with "invk-" should be rejected, even if not an existing system flag.
	prefixedNames := []string{
		"invk-foobar",
		"invk-custom",
		"invk-my-flag",
	}

	for _, name := range prefixedNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "%s", description: "This should fail - invk- prefix is reserved"}
		]
	}
]
`, name)
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkfile: %v", writeErr)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Fatalf("Parse() should fail for invk- prefixed flag name %q, got nil error", name)
			}

			if !strings.Contains(err.Error(), "invk-") {
				t.Errorf("Error should mention 'invk-' prefix, got: %v", err)
			}
		})
	}
}

func TestValidateFlags_InvowkPrefixReserved(t *testing.T) {
	t.Parallel()

	// Any flag starting with "invowk-" should be rejected.
	prefixedNames := []string{
		"invowk-foobar",
		"invowk-custom",
	}

	for _, name := range prefixedNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "%s", description: "This should fail - invowk- prefix is reserved"}
		]
	}
]
`, name)
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkfile: %v", writeErr)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Fatalf("Parse() should fail for invowk- prefixed flag name %q, got nil error", name)
			}

			if !strings.Contains(err.Error(), "invowk-") {
				t.Errorf("Error should mention 'invowk-' prefix, got: %v", err)
			}
		})
	}
}

func TestValidateFlags_IPrefixReserved(t *testing.T) {
	t.Parallel()

	// Any flag starting with "i-" should be rejected.
	prefixedNames := []string{
		"i-foobar",
		"i-custom",
	}

	for _, name := range prefixedNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cueContent := fmt.Sprintf(`
cmds: [
	{
		name: "deploy"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		flags: [
			{name: "%s", description: "This should fail - i- prefix is reserved"}
		]
	}
]
`, name)
			tmpDir := t.TempDir()
			invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
			if writeErr := os.WriteFile(invkfilePath, []byte(cueContent), 0o644); writeErr != nil {
				t.Fatalf("Failed to write invkfile: %v", writeErr)
			}

			_, err := Parse(invkfilePath)
			if err == nil {
				t.Fatalf("Parse() should fail for i- prefixed flag name %q, got nil error", name)
			}

			if !strings.Contains(err.Error(), "i-") {
				t.Errorf("Error should mention 'i-' prefix, got: %v", err)
			}
		})
	}
}
