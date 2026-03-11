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

func TestCheckTransientExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		exitCode types.ExitCode
		wantErr  bool
		wantMsg  string
	}{
		{"zero is not transient", 0, false, ""},
		{"one is not transient", 1, false, ""},
		{"125 is transient", 125, true, "container engine failure"},
		{"126 is transient", 126, true, "container engine failure"},
		{"127 is not transient", 127, false, ""},
		{"255 is not transient", 255, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := &runtime.Result{ExitCode: tt.exitCode}
			err := CheckTransientExitCode(result, "test-check")
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckTransientExitCode(exitCode=%d) error = %v, wantErr %v", tt.exitCode, err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("error message = %q, want substring %q", err.Error(), tt.wantMsg)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "test-check") {
				t.Errorf("error message should include label 'test-check', got %q", err.Error())
			}
		})
	}
}
