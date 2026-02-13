// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"

	"invowk-cli/internal/container"
	"invowk-cli/internal/issue"
	"invowk-cli/internal/runtime"
)

// classifyExecutionError maps execution/runtime failures to issue catalog IDs and
// returns a styled message for CLI rendering. It preserves actionable error details.
func classifyExecutionError(err error, verbose bool) (issueID issue.Id, styledMsg string) {
	issueID = issue.ScriptExecutionFailedId

	switch {
	case errors.Is(err, container.ErrNoEngineAvailable):
		issueID = issue.ContainerEngineNotFoundId
	case errors.Is(err, runtime.ErrRuntimeNotAvailable):
		issueID = issue.RuntimeNotAvailableId
	case errors.Is(err, os.ErrPermission):
		issueID = issue.PermissionDeniedId
	default:
		var ae *issue.ActionableError
		if errors.As(err, &ae) && ae.Operation == "find shell" {
			issueID = issue.ShellNotFoundId
		}
	}

	return issueID, fmt.Sprintf("\n%s %s\n", ErrorStyle.Render("Error:"), formatErrorForDisplay(err, verbose))
}
