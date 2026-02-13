// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"strings"
	"testing"

	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// Capability dependency tests
// ---------------------------------------------------------------------------

func TestCheckCapabilityDependencies_NoCapabilities(t *testing.T) {
	deps := &invowkfile.DependsOn{
		Capabilities: []invowkfile.CapabilityDependency{},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkCapabilityDependencies() with empty capabilities returned error: %v", err)
	}
}

func TestCheckCapabilityDependencies_NilDeps(t *testing.T) {
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(nil, ctx)
	if err != nil {
		t.Errorf("checkCapabilityDependencies() with nil deps returned error: %v", err)
	}
}

func TestCheckCapabilityDependencies_DuplicateSkipped(t *testing.T) {
	// This test verifies that duplicate capabilities are silently skipped
	// The actual success/failure depends on network connectivity
	deps := &invowkfile.DependsOn{
		Capabilities: []invowkfile.CapabilityDependency{
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityLocalAreaNetwork}},
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityLocalAreaNetwork}}, // duplicate
			{Alternatives: []invowkfile.CapabilityName{invowkfile.CapabilityLocalAreaNetwork}}, // another duplicate
		},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkCapabilityDependencies(deps, ctx)
	// If there's an error, it should only report the capability once
	if err != nil {
		depErr, ok := errors.AsType[*DependencyError](err)
		if !ok {
			t.Fatalf("checkCapabilityDependencies() should return *DependencyError, got: %T", err)
		}
		// Even with 3 duplicate entries, we should only have 1 error
		if len(depErr.MissingCapabilities) > 1 {
			t.Errorf("Expected at most 1 capability error (duplicates should be skipped), got %d", len(depErr.MissingCapabilities))
		}
	}
	// If no error, that's fine too - machine has network
}

func TestDependencyError_WithCapabilities(t *testing.T) {
	err := &DependencyError{
		CommandName: "test",
		MissingCapabilities: []string{
			"  - capability \"internet\" not available: no connection",
		},
	}

	expected := "dependencies not satisfied for command 'test'"
	if err.Error() != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestRenderDependencyError_MissingCapabilities(t *testing.T) {
	err := &DependencyError{
		CommandName: "deploy",
		MissingCapabilities: []string{
			"  - capability \"internet\" not available: no connection",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Dependencies not satisfied") {
		t.Error("RenderDependencyError should contain header")
	}

	if !strings.Contains(output, "'deploy'") {
		t.Error("RenderDependencyError should contain command name")
	}

	if !strings.Contains(output, "Missing Capabilities") {
		t.Error("RenderDependencyError should contain 'Missing Capabilities' section")
	}

	if !strings.Contains(output, "internet") {
		t.Error("RenderDependencyError should contain capability name")
	}
}

func TestRenderDependencyError_AllDependencyTypes(t *testing.T) {
	err := &DependencyError{
		CommandName: "complex-deploy",
		MissingTools: []string{
			"  - kubectl - not found in PATH",
		},
		MissingCommands: []string{
			"  - build - command not found",
		},
		MissingFilepaths: []string{
			"  - config.yaml - file not found",
		},
		MissingCapabilities: []string{
			"  - capability \"internet\" not available: no connection",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Missing Tools") {
		t.Error("RenderDependencyError should contain 'Missing Tools' section")
	}

	if !strings.Contains(output, "Missing Commands") {
		t.Error("RenderDependencyError should contain 'Missing Commands' section")
	}

	if !strings.Contains(output, "Missing or Inaccessible Files") {
		t.Error("RenderDependencyError should contain 'Missing or Inaccessible Files' section")
	}

	if !strings.Contains(output, "Missing Capabilities") {
		t.Error("RenderDependencyError should contain 'Missing Capabilities' section")
	}
}

// ---------------------------------------------------------------------------
// Environment variable dependency tests
// ---------------------------------------------------------------------------

func TestCheckEnvVarDependencies_ExistingEnvVar(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{
			{
				Alternatives: []invowkfile.EnvVarCheck{
					{Name: "TEST_ENV_VAR"},
				},
			},
		},
	}

	userEnv := map[string]string{
		"TEST_ENV_VAR": "some_value",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when env var exists, got: %v", err)
	}
}

func TestCheckEnvVarDependencies_MissingEnvVar(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{
			{
				Alternatives: []invowkfile.EnvVarCheck{
					{Name: "NONEXISTENT_ENV_VAR"},
				},
			},
		},
	}

	userEnv := map[string]string{} // Empty environment

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail when env var is missing")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingEnvVars) != 1 {
		t.Errorf("Expected 1 missing env var error, got %d", len(depErr.MissingEnvVars))
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "NONEXISTENT_ENV_VAR") {
		t.Errorf("Error message should contain env var name, got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_ValidationRegexPass(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{
			{
				Alternatives: []invowkfile.EnvVarCheck{
					{Name: "GO_VERSION", Validation: `^[0-9]+\.[0-9]+\.[0-9]+$`},
				},
			},
		},
	}

	userEnv := map[string]string{
		"GO_VERSION": "1.26.0",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when regex matches, got: %v", err)
	}
}

func TestCheckEnvVarDependencies_ValidationRegexFail(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{
			{
				Alternatives: []invowkfile.EnvVarCheck{
					{Name: "GO_VERSION", Validation: `^[0-9]+\.[0-9]+\.[0-9]+$`},
				},
			},
		},
	}

	userEnv := map[string]string{
		"GO_VERSION": "invalid-version",
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail when regex doesn't match")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if len(depErr.MissingEnvVars) != 1 {
		t.Errorf("Expected 1 env var error, got %d", len(depErr.MissingEnvVars))
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "does not match required pattern") {
		t.Errorf("Error message should mention pattern mismatch, got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_AlternativesORSemantics(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{
			{
				Alternatives: []invowkfile.EnvVarCheck{
					{Name: "AWS_ACCESS_KEY_ID"},
					{Name: "AWS_PROFILE"},
				},
			},
		},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	// Test 1: First alternative exists
	userEnv := map[string]string{
		"AWS_ACCESS_KEY_ID": "AKIAIOSFODNN7EXAMPLE",
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when first alternative exists, got: %v", err)
	}

	// Test 2: Second alternative exists
	userEnv = map[string]string{
		"AWS_PROFILE": "dev",
	}

	err = checkEnvVarDependencies(deps, userEnv, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should pass when second alternative exists, got: %v", err)
	}

	// Test 3: Neither alternative exists
	userEnv = map[string]string{}

	err = checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail when no alternatives exist")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "none of") {
		t.Errorf("Error message should mention 'none of' for multiple alternatives, got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_EmptyName(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{
			{
				Alternatives: []invowkfile.EnvVarCheck{
					{Name: "   "}, // Whitespace-only name
				},
			},
		},
	}

	userEnv := map[string]string{}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, userEnv, ctx)
	if err == nil {
		t.Error("checkEnvVarDependencies() should fail with empty name")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("checkEnvVarDependencies() should return *DependencyError, got: %T", err)
	}

	if !strings.Contains(depErr.MissingEnvVars[0], "empty") {
		t.Errorf("Error message should mention 'empty', got: %s", depErr.MissingEnvVars[0])
	}
}

func TestCheckEnvVarDependencies_NilDeps(t *testing.T) {
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(nil, map[string]string{}, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should handle nil deps gracefully, got: %v", err)
	}
}

func TestCheckEnvVarDependencies_EmptyEnvVars(t *testing.T) {
	deps := &invowkfile.DependsOn{
		EnvVars: []invowkfile.EnvVarDependency{}, // Empty list
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test-cmd"},
	}

	err := checkEnvVarDependencies(deps, map[string]string{}, ctx)
	if err != nil {
		t.Errorf("checkEnvVarDependencies() should handle empty env_vars list gracefully, got: %v", err)
	}
}

func TestDependencyError_WithEnvVars(t *testing.T) {
	err := &DependencyError{
		CommandName: "test",
		MissingEnvVars: []string{
			"  - AWS_ACCESS_KEY_ID - not set in environment",
		},
	}

	expected := "dependencies not satisfied for command 'test'"
	if err.Error() != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestRenderDependencyError_MissingEnvVars(t *testing.T) {
	err := &DependencyError{
		CommandName: "deploy",
		MissingEnvVars: []string{
			"  - AWS_ACCESS_KEY_ID - not set in environment",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Dependencies not satisfied") {
		t.Error("RenderDependencyError should contain header")
	}

	if !strings.Contains(output, "'deploy'") {
		t.Error("RenderDependencyError should contain command name")
	}

	if !strings.Contains(output, "Missing or Invalid Environment Variables") {
		t.Error("RenderDependencyError should contain 'Missing or Invalid Environment Variables' section")
	}

	if !strings.Contains(output, "AWS_ACCESS_KEY_ID") {
		t.Error("RenderDependencyError should contain env var name")
	}
}

func TestRenderDependencyError_AllDependencyTypesIncludingEnvVars(t *testing.T) {
	err := &DependencyError{
		CommandName: "complex-deploy",
		MissingTools: []string{
			"  - kubectl - not found in PATH",
		},
		MissingCommands: []string{
			"  - build - command not found",
		},
		MissingFilepaths: []string{
			"  - config.yaml - file not found",
		},
		MissingCapabilities: []string{
			"  - capability \"internet\" not available: no connection",
		},
		MissingEnvVars: []string{
			"  - AWS_ACCESS_KEY_ID - not set in environment",
		},
	}

	output := RenderDependencyError(err)

	if !strings.Contains(output, "Missing Tools") {
		t.Error("RenderDependencyError should contain 'Missing Tools' section")
	}

	if !strings.Contains(output, "Missing Commands") {
		t.Error("RenderDependencyError should contain 'Missing Commands' section")
	}

	if !strings.Contains(output, "Missing or Inaccessible Files") {
		t.Error("RenderDependencyError should contain 'Missing or Inaccessible Files' section")
	}

	if !strings.Contains(output, "Missing Capabilities") {
		t.Error("RenderDependencyError should contain 'Missing Capabilities' section")
	}

	if !strings.Contains(output, "Missing or Invalid Environment Variables") {
		t.Error("RenderDependencyError should contain 'Missing or Invalid Environment Variables' section")
	}
}
