// SPDX-License-Identifier: MPL-2.0

package invkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnv_CommandLevelFiles(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
		env: {
			files: [".env", "config/app.env", ".env.local?"]
		}
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

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if cmd.Env == nil {
		t.Fatalf("Expected cmd.Env to be non-nil")
	}
	if len(cmd.Env.Files) != 3 {
		t.Fatalf("Expected 3 env.files, got %d", len(cmd.Env.Files))
	}

	expectedFiles := []string{".env", "config/app.env", ".env.local?"}
	for i, expected := range expectedFiles {
		if cmd.Env.Files[i] != expected {
			t.Errorf("Env.Files[%d] = %q, want %q", i, cmd.Env.Files[i], expected)
		}
	}
}

func TestParseEnv_ImplementationLevelFiles(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
				env: {
					files: ["impl.env", "secrets.env?"]
				}
			}
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

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	impl := inv.Commands[0].Implementations[0]
	if impl.Env == nil {
		t.Fatalf("Expected impl.Env to be non-nil")
	}
	if len(impl.Env.Files) != 2 {
		t.Fatalf("Expected 2 env.files, got %d", len(impl.Env.Files))
	}

	expectedFiles := []string{"impl.env", "secrets.env?"}
	for i, expected := range expectedFiles {
		if impl.Env.Files[i] != expected {
			t.Errorf("Env.Files[%d] = %q, want %q", i, impl.Env.Files[i], expected)
		}
	}
}

func TestParseEnv_BothLevels(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
				env: {
					files: ["impl.env"]
				}
			}
		]
		env: {
			files: ["cmd.env"]
		}
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

	cmd := inv.Commands[0]
	impl := cmd.Implementations[0]

	if cmd.Env == nil || len(cmd.Env.Files) != 1 || cmd.Env.Files[0] != "cmd.env" {
		t.Errorf("Command Env.Files = %v, want [cmd.env]", cmd.Env.GetFiles())
	}

	if impl.Env == nil || len(impl.Env.Files) != 1 || impl.Env.Files[0] != "impl.env" {
		t.Errorf("Implementation Env.Files = %v, want [impl.env]", impl.Env.GetFiles())
	}
}

func TestParseEnv_EmptyFiles(t *testing.T) {
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
		env: {
			files: []
		}
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

	cmd := inv.Commands[0]
	if cmd.Env != nil && len(cmd.Env.Files) != 0 {
		t.Errorf("Expected 0 env.files, got %d", len(cmd.Env.Files))
	}
}

func TestParseEnv_NoField(t *testing.T) {
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

	cmd := inv.Commands[0]
	if cmd.Env != nil {
		t.Errorf("Expected nil Env when field is omitted, got %+v", cmd.Env)
	}

	impl := cmd.Implementations[0]
	if impl.Env != nil {
		t.Errorf("Expected nil Env when field is omitted, got %+v", impl.Env)
	}
}

func TestParseEnv_WithVars(t *testing.T) {
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
				env: {
					vars: {
						IMPL_VAR: "impl_value"
					}
				}
			}
		]
		env: {
			files: [".env"]
			vars: {
				CMD_VAR: "cmd_value"
				DEBUG: "true"
			}
		}
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

	cmd := inv.Commands[0]
	if cmd.Env == nil {
		t.Fatalf("Expected cmd.Env to be non-nil")
	}
	if len(cmd.Env.Files) != 1 || cmd.Env.Files[0] != ".env" {
		t.Errorf("Command Env.Files = %v, want [\".env\"]", cmd.Env.Files)
	}
	if len(cmd.Env.Vars) != 2 {
		t.Fatalf("Expected 2 command env vars, got %d", len(cmd.Env.Vars))
	}
	if cmd.Env.Vars["CMD_VAR"] != "cmd_value" {
		t.Errorf("CMD_VAR = %q, want %q", cmd.Env.Vars["CMD_VAR"], "cmd_value")
	}
	if cmd.Env.Vars["DEBUG"] != "true" {
		t.Errorf("DEBUG = %q, want %q", cmd.Env.Vars["DEBUG"], "true")
	}

	impl := cmd.Implementations[0]
	if impl.Env == nil {
		t.Fatalf("Expected impl.Env to be non-nil")
	}
	if len(impl.Env.Vars) != 1 {
		t.Fatalf("Expected 1 implementation env var, got %d", len(impl.Env.Vars))
	}
	if impl.Env.Vars["IMPL_VAR"] != "impl_value" {
		t.Errorf("IMPL_VAR = %q, want %q", impl.Env.Vars["IMPL_VAR"], "impl_value")
	}
}

func TestGenerateCUE_WithEnv(t *testing.T) {
	t.Parallel()

	inv := &Invkfile{
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",

						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
						Env: &EnvConfig{
							Files: []string{"impl.env", "secrets.env?"},
						},
					},
				},
				Env: &EnvConfig{
					Files: []string{".env", "config/app.env"},
				},
			},
		},
	}

	cue := GenerateCUE(inv)

	// Check command-level env with files
	if !strings.Contains(cue, `env: {`) {
		t.Errorf("GenerateCUE() should include env block, got:\n%s", cue)
	}
	if !strings.Contains(cue, `files: [".env", "config/app.env"]`) {
		t.Errorf("GenerateCUE() should include command-level env.files, got:\n%s", cue)
	}

	// Check implementation-level env with files
	if !strings.Contains(cue, `files: ["impl.env", "secrets.env?"]`) {
		t.Errorf("GenerateCUE() should include implementation-level env.files, got:\n%s", cue)
	}
}

func TestGenerateCUE_EnvRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Invkfile{
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",

						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
						Env: &EnvConfig{
							Files: []string{"impl.env"},
						},
					},
				},
				Env: &EnvConfig{
					Files: []string{".env", "config/app.env?"},
				},
			},
		},
	}

	// Generate CUE
	cue := GenerateCUE(original)

	// Write to temp file
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cue), 0o644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	// Parse it back
	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify command-level env.files
	if parsed.Commands[0].Env == nil {
		t.Fatalf("Expected non-nil Env after roundtrip")
	}
	if len(parsed.Commands[0].Env.Files) != 2 {
		t.Fatalf("Expected 2 command env.files, got %d", len(parsed.Commands[0].Env.Files))
	}
	if parsed.Commands[0].Env.Files[0] != ".env" {
		t.Errorf("Command Env.Files[0] = %q, want %q", parsed.Commands[0].Env.Files[0], ".env")
	}
	if parsed.Commands[0].Env.Files[1] != "config/app.env?" {
		t.Errorf("Command Env.Files[1] = %q, want %q", parsed.Commands[0].Env.Files[1], "config/app.env?")
	}

	// Verify implementation-level env.files
	if parsed.Commands[0].Implementations[0].Env == nil {
		t.Fatalf("Expected non-nil impl.Env after roundtrip")
	}
	if len(parsed.Commands[0].Implementations[0].Env.Files) != 1 {
		t.Fatalf("Expected 1 implementation env.files, got %d", len(parsed.Commands[0].Implementations[0].Env.Files))
	}
	if parsed.Commands[0].Implementations[0].Env.Files[0] != "impl.env" {
		t.Errorf("Implementation Env.Files[0] = %q, want %q", parsed.Commands[0].Implementations[0].Env.Files[0], "impl.env")
	}
}

func TestParseEnv_RootLevelFiles(t *testing.T) {
	t.Parallel()

	cueContent := `
env: {
	files: ["global.env", "shared.env?"]
}

cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
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

	if inv.Env == nil {
		t.Fatalf("Expected inv.Env to be non-nil")
	}
	if len(inv.Env.Files) != 2 {
		t.Fatalf("Expected 2 env.files, got %d", len(inv.Env.Files))
	}

	expectedFiles := []string{"global.env", "shared.env?"}
	for i, expected := range expectedFiles {
		if inv.Env.Files[i] != expected {
			t.Errorf("Env.Files[%d] = %q, want %q", i, inv.Env.Files[i], expected)
		}
	}
}

func TestParseEnv_RootLevelVars(t *testing.T) {
	t.Parallel()

	cueContent := `
env: {
	vars: {
		GLOBAL_VAR: "global_value"
		APP_ENV: "production"
	}
}

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

	if inv.Env == nil {
		t.Fatalf("Expected inv.Env to be non-nil")
	}
	if len(inv.Env.Vars) != 2 {
		t.Fatalf("Expected 2 env.vars, got %d", len(inv.Env.Vars))
	}

	if inv.Env.Vars["GLOBAL_VAR"] != "global_value" {
		t.Errorf("GLOBAL_VAR = %q, want %q", inv.Env.Vars["GLOBAL_VAR"], "global_value")
	}
	if inv.Env.Vars["APP_ENV"] != "production" {
		t.Errorf("APP_ENV = %q, want %q", inv.Env.Vars["APP_ENV"], "production")
	}
}

func TestParseEnv_AllThreeLevels(t *testing.T) {
	t.Parallel()

	cueContent := `
env: {
	files: ["global.env"]
	vars: {
		LEVEL: "root"
	}
}

cmds: [
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
				env: {
					files: ["impl.env"]
					vars: {
						LEVEL: "implementation"
					}
				}
			}
		]
		env: {
			files: ["cmd.env"]
			vars: {
				LEVEL: "command"
			}
		}
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

	// Check root-level env
	if inv.Env == nil {
		t.Fatalf("Expected inv.Env to be non-nil")
	}
	if len(inv.Env.Files) != 1 || inv.Env.Files[0] != "global.env" {
		t.Errorf("Root Env.Files = %v, want [global.env]", inv.Env.Files)
	}
	if inv.Env.Vars["LEVEL"] != "root" {
		t.Errorf("Root LEVEL = %q, want %q", inv.Env.Vars["LEVEL"], "root")
	}

	// Check command-level env
	cmd := inv.Commands[0]
	if cmd.Env == nil {
		t.Fatalf("Expected cmd.Env to be non-nil")
	}
	if len(cmd.Env.Files) != 1 || cmd.Env.Files[0] != "cmd.env" {
		t.Errorf("Command Env.Files = %v, want [cmd.env]", cmd.Env.Files)
	}
	if cmd.Env.Vars["LEVEL"] != "command" {
		t.Errorf("Command LEVEL = %q, want %q", cmd.Env.Vars["LEVEL"], "command")
	}

	// Check implementation-level env
	impl := cmd.Implementations[0]
	if impl.Env == nil {
		t.Fatalf("Expected impl.Env to be non-nil")
	}
	if len(impl.Env.Files) != 1 || impl.Env.Files[0] != "impl.env" {
		t.Errorf("Implementation Env.Files = %v, want [impl.env]", impl.Env.Files)
	}
	if impl.Env.Vars["LEVEL"] != "implementation" {
		t.Errorf("Implementation LEVEL = %q, want %q", impl.Env.Vars["LEVEL"], "implementation")
	}
}

func TestGenerateCUE_WithRootLevelEnv(t *testing.T) {
	t.Parallel()

	inv := &Invkfile{
		Env: &EnvConfig{
			Files: []string{"global.env", "shared.env?"},
			Vars: map[string]string{
				"GLOBAL_VAR": "global_value",
			},
		},
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",

						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	cue := GenerateCUE(inv)

	// Check root-level env block
	if !strings.Contains(cue, `env: {`) {
		t.Errorf("GenerateCUE() should include env block, got:\n%s", cue)
	}
	if !strings.Contains(cue, `files: ["global.env", "shared.env?"]`) {
		t.Errorf("GenerateCUE() should include root-level env.files, got:\n%s", cue)
	}
	if !strings.Contains(cue, `GLOBAL_VAR: "global_value"`) {
		t.Errorf("GenerateCUE() should include root-level env.vars, got:\n%s", cue)
	}
}

func TestGenerateCUE_RootEnvRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Invkfile{
		Env: &EnvConfig{
			Files: []string{"global.env", "shared.env?"},
			Vars: map[string]string{
				"GLOBAL_VAR": "global_value",
			},
		},
		Commands: []Command{
			{
				Name:        "deploy",
				Description: "Deploy the application",
				Implementations: []Implementation{
					{
						Script: "echo deploying",

						Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	// Generate CUE
	cue := GenerateCUE(original)

	// Write to temp file
	tmpDir := t.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(cue), 0o644); err != nil {
		t.Fatalf("Failed to write invkfile: %v", err)
	}

	// Parse it back
	parsed, err := Parse(invkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Verify root-level env.files
	if parsed.Env == nil {
		t.Fatalf("Expected non-nil Env after roundtrip")
	}
	if len(parsed.Env.Files) != 2 {
		t.Fatalf("Expected 2 root env.files, got %d", len(parsed.Env.Files))
	}
	if parsed.Env.Files[0] != "global.env" {
		t.Errorf("Root Env.Files[0] = %q, want %q", parsed.Env.Files[0], "global.env")
	}
	if parsed.Env.Files[1] != "shared.env?" {
		t.Errorf("Root Env.Files[1] = %q, want %q", parsed.Env.Files[1], "shared.env?")
	}

	// Verify root-level env.vars
	if len(parsed.Env.Vars) != 1 {
		t.Fatalf("Expected 1 root env.vars, got %d", len(parsed.Env.Vars))
	}
	if parsed.Env.Vars["GLOBAL_VAR"] != "global_value" {
		t.Errorf("GLOBAL_VAR = %q, want %q", parsed.Env.Vars["GLOBAL_VAR"], "global_value")
	}
}
