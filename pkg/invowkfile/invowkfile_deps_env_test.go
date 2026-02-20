// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDependsOn_WithEnvVars(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "deploy"
		implementations: [{
			script: "echo deploy"
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
		depends_on: {
			env_vars: [
				{alternatives: [{name: "AWS_ACCESS_KEY_ID"}]},
				{alternatives: [{name: "GO_VERSION", validation: "^[0-9]+\\.[0-9]+"}]},
				{alternatives: [{name: "CI_TOKEN"}, {name: "GH_TOKEN"}]},
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

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	if cmd.DependsOn == nil {
		t.Fatal("DependsOn should not be nil")
	}

	if len(cmd.DependsOn.EnvVars) != 3 {
		t.Fatalf("Expected 3 env_vars, got %d", len(cmd.DependsOn.EnvVars))
	}

	// First: single env var, no validation
	ev0 := cmd.DependsOn.EnvVars[0]
	if len(ev0.Alternatives) != 1 || ev0.Alternatives[0].Name != "AWS_ACCESS_KEY_ID" {
		t.Errorf("First env_var = %v, want [{Name: AWS_ACCESS_KEY_ID}]", ev0.Alternatives)
	}
	if ev0.Alternatives[0].Validation != "" {
		t.Errorf("First env_var should have no validation, got %q", ev0.Alternatives[0].Validation)
	}

	// Second: single env var with validation regex
	ev1 := cmd.DependsOn.EnvVars[1]
	if ev1.Alternatives[0].Name != "GO_VERSION" {
		t.Errorf("Second env_var name = %q, want GO_VERSION", ev1.Alternatives[0].Name)
	}
	if ev1.Alternatives[0].Validation == "" {
		t.Error("Second env_var should have validation pattern")
	}

	// Third: alternatives (OR semantics)
	ev2 := cmd.DependsOn.EnvVars[2]
	if len(ev2.Alternatives) != 2 {
		t.Fatalf("Third env_var should have 2 alternatives, got %d", len(ev2.Alternatives))
	}
	if ev2.Alternatives[0].Name != "CI_TOKEN" || ev2.Alternatives[1].Name != "GH_TOKEN" {
		t.Errorf("Third env_var alternatives = %v, want [CI_TOKEN, GH_TOKEN]", ev2.Alternatives)
	}
}

func TestGenerateCUE_WithEnvVars(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name:            "deploy",
				Implementations: []Implementation{{Script: "echo deploy", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: AllPlatformConfigs()}},
				DependsOn: &DependsOn{
					EnvVars: []EnvVarDependency{
						{Alternatives: []EnvVarCheck{{Name: "AWS_ACCESS_KEY_ID"}}},
						{Alternatives: []EnvVarCheck{{Name: "CI_TOKEN"}, {Name: "GH_TOKEN"}}},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, "env_vars:") {
		t.Error("GenerateCUE should contain 'env_vars:'")
	}

	if !strings.Contains(output, `"AWS_ACCESS_KEY_ID"`) {
		t.Error("GenerateCUE should contain AWS_ACCESS_KEY_ID")
	}

	if !strings.Contains(output, `"CI_TOKEN"`) {
		t.Error("GenerateCUE should contain CI_TOKEN")
	}

	if !strings.Contains(output, `"GH_TOKEN"`) {
		t.Error("GenerateCUE should contain GH_TOKEN")
	}

	// Verify generated CUE round-trips
	tmpDir := t.TempDir()

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(output), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	parsed, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse generated CUE: %v", err)
	}

	if parsed.Commands[0].DependsOn == nil {
		t.Fatal("Parsed DependsOn should not be nil")
	}
	if len(parsed.Commands[0].DependsOn.EnvVars) != 2 {
		t.Errorf("Expected 2 env_var dependencies after round-trip, got %d", len(parsed.Commands[0].DependsOn.EnvVars))
	}
}
