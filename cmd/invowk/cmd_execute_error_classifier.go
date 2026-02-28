// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/issue"
	"github.com/invowk/invowk/internal/runtime"
)

// classifyExecutionError maps execution/runtime failures to issue catalog IDs and
// returns a styled message for CLI rendering. It preserves actionable error details.
//
// Timeout and cancellation intentionally reuse ScriptExecutionFailedId rather than
// introducing dedicated issue IDs. The user-facing message already distinguishes
// these cases ("timed out" vs "was cancelled"), and the issue catalog entry provides
// generic guidance that applies to all script execution failures.
//
//plint:render
func classifyExecutionError(err error, verbose bool) (issueID issue.Id, styledMsg string) {
	issueID = issue.ScriptExecutionFailedId

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		// Include the full error chain so that timeout errors surface
		// which runtime operation was running when the deadline fired.
		return issueID, fmt.Sprintf("\n%s command timed out: %s\n", ErrorStyle.Render("Error:"), formatErrorForDisplay(err, verbose))
	case errors.Is(err, context.Canceled):
		return issueID, fmt.Sprintf("\n%s command was cancelled: %s\n", ErrorStyle.Render("Error:"), formatErrorForDisplay(err, verbose))
	case errors.Is(err, container.ErrNoEngineAvailable):
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

	return issueID, fmt.Sprintf("\n%s %s\n", ErrorStyle.Render("Error:"), formatErrorForDisplay(err, verbose))
}
