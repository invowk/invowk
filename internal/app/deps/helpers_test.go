// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"errors"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

func TestCollectToolErrors(t *testing.T) {
	t.Parallel()

	alwaysFail := func(_ invowkfile.BinaryName) error {
		return errors.New("  • tool - not found")
	}
	alwaysPass := func(_ invowkfile.BinaryName) error {
		return nil
	}

	t.Run("no tools returns nil", func(t *testing.T) {
		t.Parallel()

		result := CollectToolErrors(nil, alwaysFail)
		if result != nil {
			t.Errorf("CollectToolErrors(nil) = %v, want nil", result)
		}
	})

	t.Run("single alternative found", func(t *testing.T) {
		t.Parallel()

		tools := []invowkfile.ToolDependency{
			{Alternatives: []invowkfile.BinaryName{"go"}},
		}
		result := CollectToolErrors(tools, alwaysPass)
		if len(result) != 0 {
			t.Errorf("expected no errors, got %v", result)
		}
	})

	t.Run("single alternative missing", func(t *testing.T) {
		t.Parallel()

		tools := []invowkfile.ToolDependency{
			{Alternatives: []invowkfile.BinaryName{"missing-tool"}},
		}
		result := CollectToolErrors(tools, alwaysFail)
		if len(result) != 1 {
			t.Fatalf("expected 1 error, got %d", len(result))
		}
		if !strings.Contains(result[0].String(), "not found") {
			t.Errorf("expected 'not found' in message, got %q", result[0])
		}
	})

	t.Run("multi-alternative all missing", func(t *testing.T) {
		t.Parallel()

		tools := []invowkfile.ToolDependency{
			{Alternatives: []invowkfile.BinaryName{"podman", "docker"}},
		}
		result := CollectToolErrors(tools, alwaysFail)
		if len(result) != 1 {
			t.Fatalf("expected 1 error, got %d", len(result))
		}
		msg := result[0].String()
		if !strings.Contains(msg, "none of [podman, docker] found") {
			t.Errorf("expected 'none of [podman, docker] found', got %q", msg)
		}
	})

	t.Run("multi-alternative first found", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		tools := []invowkfile.ToolDependency{
			{Alternatives: []invowkfile.BinaryName{"podman", "docker"}},
		}
		result := CollectToolErrors(tools, func(_ invowkfile.BinaryName) error {
			callCount++
			return nil
		})
		if len(result) != 0 {
			t.Errorf("expected no errors, got %v", result)
		}
		if callCount != 1 {
			t.Errorf("expected early return after first match, check called %d times", callCount)
		}
	})

	t.Run("multiple tools with mixed results", func(t *testing.T) {
		t.Parallel()

		tools := []invowkfile.ToolDependency{
			{Alternatives: []invowkfile.BinaryName{"go"}},
			{Alternatives: []invowkfile.BinaryName{"missing1", "missing2"}},
		}
		result := CollectToolErrors(tools, func(name invowkfile.BinaryName) error {
			if name == "go" {
				return nil
			}
			return errors.New("  • " + string(name) + " - not found")
		})
		if len(result) != 1 {
			t.Fatalf("expected 1 error (second tool), got %d", len(result))
		}
		if !strings.Contains(result[0].String(), "none of [missing1, missing2] found") {
			t.Errorf("expected multi-alt format, got %q", result[0])
		}
	})
}

func TestNewContainerValidationContextPreservesExecutionContract(t *testing.T) {
	t.Parallel()

	parentImpl := &invowkfile.Implementation{
		Script: "python3 app.py",
		Runtimes: []invowkfile.RuntimeConfig{{
			Name:           invowkfile.RuntimeContainer,
			Image:          "python:3-slim",
			Volumes:        []invowkfile.VolumeMountSpec{"/host:/container"},
			Ports:          []invowkfile.PortMappingSpec{"8080:80"},
			EnableHostSSH:  true,
			EnvInheritMode: invowkfile.EnvInheritAllow,
			EnvInheritAllow: []invowkfile.EnvVarName{
				"PATH",
			},
		}},
	}
	parentCtx := &runtime.ExecutionContext{
		Command:         &invowkfile.Command{Name: "build"},
		SelectedImpl:    parentImpl,
		SelectedRuntime: invowkfile.RuntimeContainer,
		Context:         t.Context(),
		Env: runtime.EnvContext{
			ExtraEnv: map[string]string{"INVOWK_FLAG_VERBOSE": "true"},
		},
		ForceRebuild: true,
	}

	execCtx, stdout, stderr := NewContainerValidationContext(parentCtx, "command -v python3")

	if execCtx.SelectedImpl == parentImpl {
		t.Fatal("validation context should clone the selected implementation")
	}
	if execCtx.SelectedImpl.Script != "command -v python3" {
		t.Fatalf("Script = %q, want validation script", execCtx.SelectedImpl.Script)
	}
	if got := execCtx.SelectedImpl.Runtimes[0]; got.Image != "python:3-slim" || len(got.Volumes) != 1 || len(got.Ports) != 1 || !got.EnableHostSSH {
		t.Fatalf("runtime config not preserved: %#v", got)
	}
	if execCtx.SelectedRuntime != invowkfile.RuntimeContainer {
		t.Fatalf("SelectedRuntime = %q, want container", execCtx.SelectedRuntime)
	}
	if execCtx.Env.ExtraEnv["INVOWK_FLAG_VERBOSE"] != "true" {
		t.Fatalf("Env.ExtraEnv not preserved: %#v", execCtx.Env.ExtraEnv)
	}
	if execCtx.ForceRebuild != parentCtx.ForceRebuild {
		t.Fatalf("ForceRebuild = %v, want %v", execCtx.ForceRebuild, parentCtx.ForceRebuild)
	}
	if execCtx.IO.Stdout != stdout || execCtx.IO.Stderr != stderr {
		t.Fatal("validation context should capture stdout and stderr")
	}
}

func TestCheckTransientExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exitCode types.ExitCode
		wantErr  bool
	}{
		{"zero is not transient", 0, false},
		{"one is not transient", 1, false},
		{"125 is transient", 125, true},
		{"126 is transient", 126, true},
		{"127 is not transient", 127, false},
		{"255 is not transient", 255, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := &runtime.Result{ExitCode: tt.exitCode}
			err := CheckTransientExitCode(result, "test-check")
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckTransientExitCode(exitCode=%d) error = %v, wantErr %v", tt.exitCode, err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrContainerEngineFailure) {
				t.Errorf("errors.Is(err, ErrContainerEngineFailure) = false for %v", err)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "test-check") {
				t.Errorf("error message should include label 'test-check', got %q", err.Error())
			}
		})
	}
}
