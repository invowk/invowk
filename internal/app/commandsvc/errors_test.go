// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"os"
	"testing"

	runtimepkg "github.com/invowk/invowk/internal/runtime"
)

func TestClassifyExecutionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantKind ErrorKind
		wantHint string
	}{
		{
			name:     "deadline exceeded",
			err:      context.DeadlineExceeded,
			wantKind: ErrorKindScriptExecutionFailed,
			wantHint: HintTimedOut,
		},
		{
			name:     "cancelled",
			err:      context.Canceled,
			wantKind: ErrorKindScriptExecutionFailed,
			wantHint: HintCancelled,
		},
		{
			name:     "no engine available",
			err:      runtimepkg.ErrContainerEngineUnavailable,
			wantKind: ErrorKindContainerEngineNotFound,
		},
		{
			name:     "runtime not available",
			err:      runtimepkg.ErrRuntimeNotAvailable,
			wantKind: ErrorKindRuntimeNotAvailable,
		},
		{
			name:     "permission denied",
			err:      os.ErrPermission,
			wantKind: ErrorKindPermissionDenied,
		},
		{
			name:     "shell not found",
			err:      &runtimepkg.ShellNotFoundError{Attempted: runtimepkg.ShellLookupAttempts{"bash", "sh"}},
			wantKind: ErrorKindShellNotFound,
		},
		{
			name:     "default fallback",
			err:      errors.New("boom"),
			wantKind: ErrorKindScriptExecutionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotKind, gotHint := classifyExecutionError(tt.err)
			if gotKind != tt.wantKind || gotHint != tt.wantHint {
				t.Fatalf("classifyExecutionError() = (%v, %q), want (%v, %q)", gotKind, gotHint, tt.wantKind, tt.wantHint)
			}
		})
	}
}
