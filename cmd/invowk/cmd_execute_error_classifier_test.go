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
	"github.com/invowk/invowk/internal/app/deps"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
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

// TestRenderAndWrapServiceError_ExitCodes verifies the usage classification:
// command-line rejections exit with ExitCodeUsage, every other service error
// exits with 1, and the ServiceError stays reachable for rendering.
func TestRenderAndWrapServiceError_ExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want types.ExitCode
	}{
		{name: "argument validation", err: &deps.ArgumentValidationError{Type: deps.ArgErrMissingRequired, CommandName: "build", MinArgs: 1}, want: ExitCodeUsage},
		{name: "flag validation", err: &deps.FlagValidationError{CommandName: "build", Failures: []deps.DependencyMessage{"bad"}}, want: ExitCodeUsage},
		{name: "ambiguous command", err: &commandsvc.AmbiguousCommandError{CommandName: "deploy", Sources: []discovery.SourceID{"invowkfile", "m"}}, want: ExitCodeUsage},
		{name: "source not found", err: &commandsvc.SourceNotFoundError{Source: "bogus"}, want: ExitCodeUsage},
		{name: "runtime not allowed", err: &commandsvc.RuntimeNotAllowedError{CommandName: "build", Runtime: invowkfile.RuntimeNative, Allowed: []invowkfile.RuntimeMode{invowkfile.RuntimeVirtualSh}}, want: ExitCodeUsage},
		{name: "classified command not found", err: &commandsvc.ClassifiedError{Err: errors.New("command 'nope' not found"), Kind: commandsvc.ErrorKindCommandNotFound}, want: ExitCodeUsage},
		{name: "classified command ambiguous", err: &commandsvc.ClassifiedError{Err: errors.New("ambiguous"), Kind: commandsvc.ErrorKindCommandAmbiguous}, want: ExitCodeUsage},
		{name: "dependency error", err: &deps.DependencyError{CommandName: "build"}, want: 1},
		{name: "unsupported platform", err: &commandsvc.UnsupportedPlatformError{CommandName: "build", Current: invowkfile.PlatformLinux, Supported: []invowkfile.Platform{invowkfile.PlatformMac}}, want: 1},
		{name: "classified script failure", err: &commandsvc.ClassifiedError{Err: errors.New("boom"), Kind: commandsvc.ErrorKindScriptExecutionFailed}, want: 1},
		{name: "classified engine not found", err: &commandsvc.ClassifiedError{Err: errors.New("no engine"), Kind: commandsvc.ErrorKindContainerEngineNotFound}, want: 1},
		{name: "classified runtime not available", err: &commandsvc.ClassifiedError{Err: errors.New("no runtime"), Kind: commandsvc.ErrorKindRuntimeNotAvailable}, want: 1},
		{name: "classified permission denied", err: &commandsvc.ClassifiedError{Err: os.ErrPermission, Kind: commandsvc.ErrorKindPermissionDenied}, want: 1},
		{name: "classified shell not found", err: &commandsvc.ClassifiedError{Err: errors.New("no shell"), Kind: commandsvc.ErrorKindShellNotFound}, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrapped := renderAndWrapServiceError(tt.err, ExecuteRequest{Name: "build"})
			if _, ok := errors.AsType[*ServiceError](wrapped); !ok {
				t.Fatalf("renderAndWrapServiceError() returned %T, want a *ServiceError in the chain", wrapped)
			}
			if got := exitCodeForError(wrapped); got != tt.want {
				t.Fatalf("exitCodeForError() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCreateRuntimeSessionWithDiagnostics(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.ContainerEngine = "not-a-real-engine"

	session := commandadapters.RuntimeRegistryFactory{}.Create(cfg, newTestHostAccess(t), invowkfile.RuntimeContainer)
	defer session.Close()

	if session.NewExecutionID() == "" {
		t.Fatal("Create() returned session without execution IDs")
	}

	if session.ContainerInitErr() == nil {
		t.Fatal("Create() should return container init error for invalid engine")
	}

	diagnostics := session.Diagnostics()
	if len(diagnostics) == 0 {
		t.Fatal("Create() should return diagnostics for invalid engine")
	}

	foundInitDiag := false
	for _, diag := range diagnostics {
		if diag.Code() == "container_runtime_init_failed" {
			foundInitDiag = true
			break
		}
	}
	if !foundInitDiag {
		t.Fatalf("expected container_runtime_init_failed diagnostic, got %#v", diagnostics)
	}

	nativeResult := session.Execute(runtimeContextForMode(t, invowkfile.RuntimeNative))
	if !nativeResult.Success() {
		t.Fatalf("native Execute() result = %#v, want success", nativeResult)
	}

	virtualResult := session.Execute(runtimeContextForMode(t, invowkfile.RuntimeVirtualSh))
	if !virtualResult.Success() {
		t.Fatalf("virtual Execute() result = %#v, want success", virtualResult)
	}

	containerResult := session.Execute(runtimeContextForMode(t, invowkfile.RuntimeContainer))
	if !errors.Is(containerResult.Error, runtime.ErrRuntimeNotAvailable) {
		t.Fatalf("container Execute() error = %v, want ErrRuntimeNotAvailable", containerResult.Error)
	}
}
