// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"errors"
	"os"

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

	// ErrRuntimeNotAllowed is returned when a runtime override is incompatible with a command.
	ErrRuntimeNotAllowed = errors.New("runtime not allowed")
)

// classifyExecutionError maps execution/runtime failures to service-owned error
// kinds and returns a classification hint (e.g., HintTimedOut, HintCancelled).
// The CLI adapter maps the kind to presentation content.
//
// Timeout and cancellation intentionally reuse ScriptExecutionFailedId rather than
// introducing dedicated issue IDs. The user-facing message already distinguishes
// these cases, and the issue catalog entry provides generic guidance that applies
// to all script execution failures.
func classifyExecutionError(err error) (kind ErrorKind, hint string) {
	kind = ErrorKindScriptExecutionFailed

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return kind, HintTimedOut
	case errors.Is(err, context.Canceled):
		return kind, HintCancelled
	case errors.Is(err, runtime.ErrContainerEngineUnavailable):
		kind = ErrorKindContainerEngineNotFound
	case errors.Is(err, runtime.ErrRuntimeNotAvailable):
		kind = ErrorKindRuntimeNotAvailable
	case errors.Is(err, runtime.ErrShellNotFound):
		kind = ErrorKindShellNotFound
	case errors.Is(err, os.ErrPermission):
		kind = ErrorKindPermissionDenied
	}

	return kind, ""
}
