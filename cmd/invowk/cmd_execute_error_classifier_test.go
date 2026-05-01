// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestRenderAndWrapServiceError_ClassifiedError verifies that the CLI adapter
// correctly renders ClassifiedError variants with appropriate styled messages
// and issue catalog IDs.
func TestRenderAndWrapServiceError_ClassifiedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		verbose     bool
		wantIssueID issue.Id
		wantInStyle []string
	}{
		{
			name: "container engine unavailable maps to container issue",
			err: &commandsvc.ClassifiedError{
				Err:  &container.EngineNotAvailableError{Engine: "podman", Reason: "not installed"},
				Kind: commandsvc.ErrorKindContainerEngineNotFound,
			},
			wantIssueID: issue.ContainerEngineNotFoundId,
			wantInStyle: []string{"Error:", "container engine 'podman' is not available"},
		},
		{
			name: "runtime unavailable maps to runtime issue",
			err: &commandsvc.ClassifiedError{
				Err:  fmt.Errorf("wrapped: %w", runtime.ErrRuntimeNotAvailable),
				Kind: commandsvc.ErrorKindRuntimeNotAvailable,
			},
			wantIssueID: issue.RuntimeNotAvailableId,
			wantInStyle: []string{"runtime not available"},
		},
		{
			name: "permission denied maps to permission issue",
			err: &commandsvc.ClassifiedError{
				Err:  fmt.Errorf("wrapped: %w", os.ErrPermission),
				Kind: commandsvc.ErrorKindPermissionDenied,
			},
			wantIssueID: issue.PermissionDeniedId,
			wantInStyle: []string{"permission denied"},
		},
		{
			name: "deadline exceeded uses timed out hint",
			err: &commandsvc.ClassifiedError{
				Err:     context.DeadlineExceeded,
				Kind:    commandsvc.ErrorKindScriptExecutionFailed,
				Message: "timed out",
			},
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"timed out"},
		},
		{
			name: "context cancelled uses cancelled hint",
			err: &commandsvc.ClassifiedError{
				Err:     context.Canceled,
				Kind:    commandsvc.ErrorKindScriptExecutionFailed,
				Message: "cancelled",
			},
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"cancelled"},
		},
		{
			name: "unknown error falls back to script execution issue",
			err: &commandsvc.ClassifiedError{
				Err:  errors.New("unexpected boom"),
				Kind: commandsvc.ErrorKindScriptExecutionFailed,
			},
			wantIssueID: issue.ScriptExecutionFailedId,
			wantInStyle: []string{"unexpected boom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := ExecuteRequest{Verbose: tt.verbose}
			wrapped := renderAndWrapServiceError(tt.err, req)

			svcErr, ok := errors.AsType[*ServiceError](wrapped)
			if !ok {
				t.Fatalf("renderAndWrapServiceError() returned %T, want *ServiceError", wrapped)
			}

			if svcErr.IssueID != tt.wantIssueID {
				t.Fatalf("ServiceError.IssueID = %v, want %v", svcErr.IssueID, tt.wantIssueID)
			}

			for _, token := range tt.wantInStyle {
				if !strings.Contains(strings.ToLower(svcErr.StyledMessage), strings.ToLower(token)) {
					t.Fatalf("styled message %q does not contain token %q", svcErr.StyledMessage, token)
				}
			}
		})
	}
}

func TestCreateRuntimeRegistryWithDiagnostics(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.ContainerEngine = "not-a-real-engine"

	result := commandadapters.RuntimeRegistryFactory{}.Create(cfg, newTestHostAccess(t), invowkfile.RuntimeContainer)
	defer result.Cleanup()

	if result.Registry == nil {
		t.Fatal("CreateRuntimeRegistry() returned nil registry")
	}

	if result.ContainerInitErr == nil {
		t.Fatal("CreateRuntimeRegistry() should return container init error for invalid engine")
	}

	if len(result.Diagnostics) == 0 {
		t.Fatal("CreateRuntimeRegistry() should return diagnostics for invalid engine")
	}

	foundInitDiag := false
	for _, diag := range result.Diagnostics {
		if diag.Code() == "container_runtime_init_failed" {
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
