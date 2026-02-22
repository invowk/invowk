// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
)

func TestClassifyExecutionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		verbose     bool
		wantIssueID issue.Id
		wantInStyle []string
	}{
		{
			name:        "container engine unavailable maps to container issue",
			err:         &container.EngineNotAvailableError{Engine: "podman", Reason: "not installed"},
			wantIssueID: issue.ContainerEngineNotFoundId,
			wantInStyle: []string{"Error:", "container engine 'podman' is not available"},
		},
		{
			name:        "runtime unavailable maps to runtime issue",
			err:         fmt.Errorf("wrapped: %w", runtime.ErrRuntimeNotAvailable),
			wantIssueID: issue.RuntimeNotAvailableId,
			wantInStyle: []string{"runtime not available"},
		},
		{
			name:        "permission denied maps to permission issue",
			err:         fmt.Errorf("wrapped: %w", os.ErrPermission),
			wantIssueID: issue.PermissionDeniedId,
			wantInStyle: []string{"permission denied"},
		},
		{
			name: "shell lookup actionable error maps to shell issue",
			err: issue.NewErrorContext().
				WithOperation("find shell").
				WithSuggestion("Install bash").
				Wrap(fmt.Errorf("no shell found in PATH")).
				BuildError(),
			wantIssueID: issue.ShellNotFoundId,
			wantInStyle: []string{"Install bash"},
		},
		{
			name:        "not-registered runtime maps to runtime issue via sentinel wrapping",
			err:         fmt.Errorf("failed to get runtime: %w", fmt.Errorf("runtime 'container' not registered: %w", runtime.ErrRuntimeNotAvailable)),
			wantIssueID: issue.RuntimeNotAvailableId,
			wantInStyle: []string{"not registered"},
		},
		{
			name:        "deadline exceeded maps to script execution with timeout message",
			err:         context.DeadlineExceeded,
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"timed out"},
		},
		{
			name:        "context cancelled maps to script execution with cancelled message",
			err:         context.Canceled,
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"cancelled"},
		},
		{
			name:        "wrapped deadline exceeded preserves error chain",
			err:         fmt.Errorf("dependency 'lint' failed: %w", context.DeadlineExceeded),
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"timed out", "lint"},
		},
		{
			name:        "unknown error falls back to script execution issue",
			err:         fmt.Errorf("unexpected boom"),
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"unexpected boom"},
		},
		{
			name: "verbose actionable error includes chain",
			err: issue.NewErrorContext().
				WithOperation("find shell").
				Wrap(fmt.Errorf("no shell found in PATH")).
				BuildError(),
			verbose:     true,
			wantIssueID: issue.ShellNotFoundId,
			wantInStyle: []string{"Error chain:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotIssueID, styled := classifyExecutionError(tt.err, tt.verbose)
			if gotIssueID != tt.wantIssueID {
				t.Fatalf("classifyExecutionError() issue ID = %v, want %v", gotIssueID, tt.wantIssueID)
			}

			for _, token := range tt.wantInStyle {
				if !strings.Contains(strings.ToLower(styled), strings.ToLower(token)) {
					t.Fatalf("styled message %q does not contain token %q", styled, token)
				}
			}
		})
	}
}

func TestCreateRuntimeRegistryWithDiagnostics(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.ContainerEngine = "not-a-real-engine"

	result := createRuntimeRegistry(cfg, nil)
	defer result.Cleanup()

	if result.Registry == nil {
		t.Fatal("createRuntimeRegistry() returned nil registry")
	}

	if result.ContainerInitErr == nil {
		t.Fatal("createRuntimeRegistry() should return container init error for invalid engine")
	}

	if len(result.Diagnostics) == 0 {
		t.Fatal("createRuntimeRegistry() should return diagnostics for invalid engine")
	}

	foundInitDiag := false
	for _, diag := range result.Diagnostics {
		if diag.Code == "container_runtime_init_failed" {
			foundInitDiag = true
			break
		}
	}
	if !foundInitDiag {
		t.Fatalf("expected container_runtime_init_failed diagnostic, got %#v", result.Diagnostics)
	}

	if _, err := result.Registry.Get(runtime.RuntimeTypeNative); err != nil {
		t.Fatalf("native runtime should be registered: %v", err)
	}
	if _, err := result.Registry.Get(runtime.RuntimeTypeVirtual); err != nil {
		t.Fatalf("virtual runtime should be registered: %v", err)
	}
	if _, err := result.Registry.Get(runtime.RuntimeTypeContainer); err == nil {
		t.Fatal("container runtime should not be registered when initialization fails")
	}
}
