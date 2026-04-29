// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"os"

	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
)

const (
	// HintTimedOut is the classification hint for deadline-exceeded errors.
	HintTimedOut = "timed out"
	// HintCancelled is the classification hint for context-cancelled errors.
	HintCancelled = "cancelled"
)

var (
	// ErrInteractiveExecutorNotConfigured is returned when interactive mode is
	// requested without injecting a terminal adapter into the service.
	ErrInteractiveExecutorNotConfigured = errors.New("interactive executor not configured")

	// ErrUnsupportedPlatform is returned when a command does not support the current host platform.
	ErrUnsupportedPlatform = errors.New("unsupported platform")

	// ErrRuntimeResolution is returned when the runtime for a command cannot be resolved.
	ErrRuntimeResolution = errors.New("runtime resolution failed")
)

// classifyExecutionError maps execution/runtime failures to issue catalog IDs and
// returns a classification hint (e.g., HintTimedOut, HintCancelled). The CLI adapter
// combines this with styled error formatting.
//
// Timeout and cancellation intentionally reuse ScriptExecutionFailedId rather than
// introducing dedicated issue IDs. The user-facing message already distinguishes
// these cases, and the issue catalog entry provides generic guidance that applies
// to all script execution failures.
func classifyExecutionError(err error) (issueID issue.Id, hint string) {
	issueID = issue.ScriptExecutionFailedId

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return issueID, HintTimedOut
	case errors.Is(err, context.Canceled):
		return issueID, HintCancelled
	case errors.Is(err, runtime.ErrContainerEngineUnavailable):
		issueID = issue.ContainerEngineNotFoundId
	case errors.Is(err, runtime.ErrRuntimeNotAvailable):
		issueID = issue.RuntimeNotAvailableId
	case errors.Is(err, os.ErrPermission):
		issueID = issue.PermissionDeniedId
	default:
		if ae, ok := errors.AsType[*issue.ActionableError](err); ok && ae.Operation() == "find shell" {
			issueID = issue.ShellNotFoundId
		}
	}

	return issueID, ""
}
