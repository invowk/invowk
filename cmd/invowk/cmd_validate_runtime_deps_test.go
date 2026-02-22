// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// T1: validateRuntimeDependencies() early-return guard tests
//
// These tests verify that validateRuntimeDependencies is a no-op for
// non-container runtimes and handles nil/empty DependsOn gracefully.
// No container runtime is needed — they exercise pure Go guard logic.
// ---------------------------------------------------------------------------

func TestValidateRuntimeDependencies_NativeRuntime_NoOp(t *testing.T) {
	t.Parallel()

	cmdInfo := &discovery.CommandInfo{
		Invowkfile: &invowkfile.Invowkfile{FilePath: "/fake/invowkfile.cue"},
		Command:    &invowkfile.Command{Name: "test"},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		SelectedImpl: &invowkfile.Implementation{
			Runtimes: []invowkfile.RuntimeConfig{{
				Name: invowkfile.RuntimeNative,
				DependsOn: &invowkfile.DependsOn{
					Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-xyz"}}},
				},
			}},
		},
		SelectedRuntime: invowkfile.RuntimeNative,
	}

	// Even though there are deps with a nonexistent tool, native runtime should be a no-op
	err := validateRuntimeDependencies(cmdInfo, nil, ctx)
	if err != nil {
		t.Errorf("validateRuntimeDependencies() should be no-op for native runtime, got: %v", err)
	}
}

func TestValidateRuntimeDependencies_VirtualRuntime_NoOp(t *testing.T) {
	t.Parallel()

	cmdInfo := &discovery.CommandInfo{
		Invowkfile: &invowkfile.Invowkfile{FilePath: "/fake/invowkfile.cue"},
		Command:    &invowkfile.Command{Name: "test"},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		SelectedImpl: &invowkfile.Implementation{
			Runtimes: []invowkfile.RuntimeConfig{{
				Name: invowkfile.RuntimeVirtual,
				DependsOn: &invowkfile.DependsOn{
					Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-xyz"}}},
				},
			}},
		},
		SelectedRuntime: invowkfile.RuntimeVirtual,
	}

	err := validateRuntimeDependencies(cmdInfo, nil, ctx)
	if err != nil {
		t.Errorf("validateRuntimeDependencies() should be no-op for virtual runtime, got: %v", err)
	}
}

func TestValidateRuntimeDependencies_ContainerRuntime_NilRuntimeConfig(t *testing.T) {
	t.Parallel()

	cmdInfo := &discovery.CommandInfo{
		Invowkfile: &invowkfile.Invowkfile{FilePath: "/fake/invowkfile.cue"},
		Command:    &invowkfile.Command{Name: "test"},
	}

	// SelectedImpl has no container runtime config at all
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		SelectedImpl: &invowkfile.Implementation{
			Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}},
		},
		SelectedRuntime: invowkfile.RuntimeContainer,
	}

	// FindRuntimeConfig returns nil → early return
	err := validateRuntimeDependencies(cmdInfo, nil, ctx)
	if err != nil {
		t.Errorf("validateRuntimeDependencies() should return nil when no container RuntimeConfig found, got: %v", err)
	}
}

func TestValidateRuntimeDependencies_ContainerRuntime_NilDependsOn(t *testing.T) {
	t.Parallel()

	cmdInfo := &discovery.CommandInfo{
		Invowkfile: &invowkfile.Invowkfile{FilePath: "/fake/invowkfile.cue"},
		Command:    &invowkfile.Command{Name: "test"},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		SelectedImpl: &invowkfile.Implementation{
			Runtimes: []invowkfile.RuntimeConfig{{
				Name:      invowkfile.RuntimeContainer,
				Image:     "debian:stable-slim",
				DependsOn: nil,
			}},
		},
		SelectedRuntime: invowkfile.RuntimeContainer,
	}

	err := validateRuntimeDependencies(cmdInfo, nil, ctx)
	if err != nil {
		t.Errorf("validateRuntimeDependencies() should return nil when DependsOn is nil, got: %v", err)
	}
}

func TestValidateRuntimeDependencies_ContainerRuntime_EmptyDependsOn(t *testing.T) {
	t.Parallel()

	cmdInfo := &discovery.CommandInfo{
		Invowkfile: &invowkfile.Invowkfile{FilePath: "/fake/invowkfile.cue"},
		Command:    &invowkfile.Command{Name: "test"},
	}

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		SelectedImpl: &invowkfile.Implementation{
			Runtimes: []invowkfile.RuntimeConfig{{
				Name:      invowkfile.RuntimeContainer,
				Image:     "debian:stable-slim",
				DependsOn: &invowkfile.DependsOn{},
			}},
		},
		SelectedRuntime: invowkfile.RuntimeContainer,
	}

	err := validateRuntimeDependencies(cmdInfo, nil, ctx)
	if err != nil {
		t.Errorf("validateRuntimeDependencies() should return nil for empty DependsOn, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// T2: checkHostToolDependencies() tests
// ---------------------------------------------------------------------------

func TestCheckHostToolDependencies_NilDeps(t *testing.T) {
	t.Parallel()

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostToolDependencies(nil, ctx)
	if err != nil {
		t.Errorf("checkHostToolDependencies() should return nil for nil deps, got: %v", err)
	}
}

func TestCheckHostToolDependencies_EmptyTools(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostToolDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkHostToolDependencies() should return nil for empty tools, got: %v", err)
	}
}

func TestCheckHostToolDependencies_ExistingTool(t *testing.T) {
	t.Parallel()

	var existingTool string
	for _, tool := range []string{"sh", "bash", "echo", "cat"} {
		if _, err := exec.LookPath(tool); err == nil {
			existingTool = tool
			break
		}
	}
	if existingTool == "" {
		t.Skip("No common tools found in PATH")
	}

	deps := &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{invowkfile.BinaryName(existingTool)}}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostToolDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkHostToolDependencies() should return nil for existing tool %q, got: %v", existingTool, err)
	}
}

func TestCheckHostToolDependencies_MissingTool(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{Alternatives: []invowkfile.BinaryName{"nonexistent-tool-xyz-12345"}}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostToolDependencies(deps, ctx)
	if err == nil {
		t.Fatal("checkHostToolDependencies() should return error for missing tool")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("expected *DependencyError, got %T", err)
	}
	if depErr.CommandName != "test" {
		t.Errorf("CommandName = %q, want %q", depErr.CommandName, "test")
	}
	if len(depErr.MissingTools) != 1 {
		t.Errorf("MissingTools length = %d, want 1", len(depErr.MissingTools))
	}
}

func TestCheckHostToolDependencies_AlternativesOR(t *testing.T) {
	t.Parallel()

	var existingTool string
	for _, tool := range []string{"sh", "bash", "echo"} {
		if _, err := exec.LookPath(tool); err == nil {
			existingTool = tool
			break
		}
	}
	if existingTool == "" {
		t.Skip("No common tools found in PATH")
	}

	// One existing + one missing → should pass (OR semantics)
	deps := &invowkfile.DependsOn{
		Tools: []invowkfile.ToolDependency{{
			Alternatives: []invowkfile.BinaryName{"nonexistent-tool-xyz", invowkfile.BinaryName(existingTool)},
		}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostToolDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkHostToolDependencies() should pass with one existing alternative, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// T3: checkHostFilepathDependencies() tests
// ---------------------------------------------------------------------------

func TestCheckHostFilepathDependencies_NilDeps(t *testing.T) {
	t.Parallel()

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostFilepathDependencies(nil, "/fake/invowkfile.cue", ctx)
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for nil deps, got: %v", err)
	}
}

func TestCheckHostFilepathDependencies_ExistingPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	deps := &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{testFile}}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostFilepathDependencies(deps, filepath.Join(tmpDir, "invowkfile.cue"), ctx)
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should return nil for existing file, got: %v", err)
	}
}

func TestCheckHostFilepathDependencies_MissingPath(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"/nonexistent/path/xyz"}}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostFilepathDependencies(deps, "/fake/invowkfile.cue", ctx)
	if err == nil {
		t.Fatal("checkHostFilepathDependencies() should return error for missing path")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("expected *DependencyError, got %T", err)
	}
	if len(depErr.MissingFilepaths) != 1 {
		t.Errorf("MissingFilepaths length = %d, want 1", len(depErr.MissingFilepaths))
	}
}

func TestCheckHostFilepathDependencies_RelativePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "script.sh"), []byte("#!/bin/sh"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	deps := &invowkfile.DependsOn{
		Filepaths: []invowkfile.FilepathDependency{{Alternatives: []string{"script.sh"}}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
	}

	err := checkHostFilepathDependencies(deps, filepath.Join(tmpDir, "invowkfile.cue"), ctx)
	if err != nil {
		t.Errorf("checkHostFilepathDependencies() should resolve relative paths against invowkfile dir, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// T4: checkHostCustomCheckDependencies() tests
// ---------------------------------------------------------------------------

func TestCheckHostCustomCheckDependencies_NilDeps(t *testing.T) {
	t.Parallel()

	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		Context: context.Background(),
	}

	err := checkHostCustomCheckDependencies(nil, ctx)
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil for nil deps, got: %v", err)
	}
}

func TestCheckHostCustomCheckDependencies_EmptyChecks(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		Context: context.Background(),
	}

	err := checkHostCustomCheckDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil for empty checks, got: %v", err)
	}
}

func TestCheckHostCustomCheckDependencies_PassingCheck(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Name:        "echo-check",
			CheckScript: "echo hello",
		}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		Context: context.Background(),
	}

	err := checkHostCustomCheckDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should return nil for passing check, got: %v", err)
	}
}

func TestCheckHostCustomCheckDependencies_FailingCheck(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Name:         "fail-check",
			CheckScript:  "exit 1",
			ExpectedCode: new(0),
		}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		Context: context.Background(),
	}

	err := checkHostCustomCheckDependencies(deps, ctx)
	if err == nil {
		t.Fatal("checkHostCustomCheckDependencies() should return error for failing check")
	}

	depErr, ok := errors.AsType[*DependencyError](err)
	if !ok {
		t.Fatalf("expected *DependencyError, got %T", err)
	}
	if len(depErr.FailedCustomChecks) != 1 {
		t.Errorf("FailedCustomChecks length = %d, want 1", len(depErr.FailedCustomChecks))
	}
}

func TestCheckHostCustomCheckDependencies_AlternativesOR(t *testing.T) {
	t.Parallel()

	deps := &invowkfile.DependsOn{
		CustomChecks: []invowkfile.CustomCheckDependency{{
			Alternatives: []invowkfile.CustomCheck{
				{Name: "failing", CheckScript: "exit 1", ExpectedCode: new(0)},
				{Name: "passing", CheckScript: "echo ok", ExpectedCode: new(0)},
			},
		}},
	}
	ctx := &runtime.ExecutionContext{
		Command: &invowkfile.Command{Name: "test"},
		Context: context.Background(),
	}

	// Second alternative passes → dependency satisfied
	err := checkHostCustomCheckDependencies(deps, ctx)
	if err != nil {
		t.Errorf("checkHostCustomCheckDependencies() should pass when any alternative passes, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// T5: capabilityCheckScript() tests (pure function, table-driven)
// ---------------------------------------------------------------------------

func TestCapabilityCheckScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		capName   invowkfile.CapabilityName
		wantEmpty bool
		contains  string // substring expected in non-empty scripts
	}{
		{
			name:      "internet produces script",
			capName:   invowkfile.CapabilityInternet,
			wantEmpty: false,
			contains:  "ping",
		},
		{
			name:      "containers produces script",
			capName:   invowkfile.CapabilityContainers,
			wantEmpty: false,
			contains:  "command -v",
		},
		{
			name:      "local-area-network produces script",
			capName:   invowkfile.CapabilityLocalAreaNetwork,
			wantEmpty: false,
			contains:  "route",
		},
		{
			name:      "tty produces script",
			capName:   invowkfile.CapabilityTTY,
			wantEmpty: false,
			contains:  "test -t",
		},
		{
			name:      "unknown capability returns empty",
			capName:   invowkfile.CapabilityName("nonexistent"),
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := capabilityCheckScript(tt.capName)
			if tt.wantEmpty {
				if result != "" {
					t.Errorf("capabilityCheckScript(%q) = %q, want empty", tt.capName, result)
				}
				return
			}
			if result == "" {
				t.Fatalf("capabilityCheckScript(%q) returned empty, want non-empty script", tt.capName)
			}
			if !strings.Contains(result, tt.contains) {
				t.Errorf("capabilityCheckScript(%q) = %q, want substring %q", tt.capName, result, tt.contains)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// shellEscapeSingleQuote tests
// ---------------------------------------------------------------------------

func TestShellEscapeSingleQuote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no quotes", "hello", "hello"},
		{"single quote", "it's", `it'\''s`},
		{"multiple quotes", "a'b'c", `a'\''b'\''c`},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shellEscapeSingleQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellEscapeSingleQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
