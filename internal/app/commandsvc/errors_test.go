// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/issue"
	runtimepkg "github.com/invowk/invowk/internal/runtime"
)

func TestClassifyExecutionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		wantID   issue.Id
		wantHint string
	}{
		{
			name:     "deadline exceeded",
			err:      context.DeadlineExceeded,
			wantID:   issue.ScriptExecutionFailedId,
			wantHint: HintTimedOut,
		},
		{
			name:     "cancelled",
			err:      context.Canceled,
			wantID:   issue.ScriptExecutionFailedId,
			wantHint: HintCancelled,
		},
		{
			name:   "no engine available",
			err:    container.ErrNoEngineAvailable,
			wantID: issue.ContainerEngineNotFoundId,
		},
		{
			name:   "runtime not available",
			err:    runtimepkg.ErrRuntimeNotAvailable,
			wantID: issue.RuntimeNotAvailableId,
		},
		{
			name:   "permission denied",
			err:    os.ErrPermission,
			wantID: issue.PermissionDeniedId,
		},
		{
			name:   "actionable find shell",
			err:    issue.NewErrorContext().WithOperation("find shell").Wrap(errors.New("missing")).Build(),
			wantID: issue.ShellNotFoundId,
		},
		{
			name:   "default fallback",
			err:    errors.New("boom"),
			wantID: issue.ScriptExecutionFailedId,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotID, gotHint := classifyExecutionError(tt.err)
			if gotID != tt.wantID || gotHint != tt.wantHint {
				t.Fatalf("classifyExecutionError() = (%v, %q), want (%v, %q)", gotID, gotHint, tt.wantID, tt.wantHint)
			}
		})
	}
}
