// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/invowk/invowk/internal/issue"
)

// ServiceError is an error that carries optional rendering information for
// the CLI layer. When the CLI layer receives a ServiceError, it renders the
// styled error message (if present) before formatting the underlying error.
// Always create via newServiceError to enforce the Err-must-be-non-nil invariant.
type ServiceError struct {
	// Err is the underlying error (must not be nil).
	Err error
	// IssueID is the optional issue catalog ID for rendering help text.
	IssueID issue.Id
	// StyledMessage is the optional pre-rendered styled error text.
	StyledMessage string
}

// newServiceError creates a ServiceError with a nil-Err panic guard.
// All construction sites must use this instead of struct literals.
func newServiceError(err error, issueID issue.Id, styledMessage string) *ServiceError {
	if err == nil {
		panic("ServiceError: Err must not be nil")
	}
	return &ServiceError{
		Err:           err,
		IssueID:       issueID,
		StyledMessage: styledMessage,
	}
}

// Error implements the error interface.
func (e *ServiceError) Error() string { return e.Err.Error() }

// Unwrap returns the underlying error for errors.Is/As chains.
func (e *ServiceError) Unwrap() error { return e.Err }

// renderServiceError renders a ServiceError in the CLI layer.
// It prints any styled message first, then the optional issue help section.
func renderServiceError(stderr io.Writer, svcErr *ServiceError) {
	if svcErr == nil {
		return
	}

	if svcErr.StyledMessage != "" {
		fmt.Fprint(stderr, svcErr.StyledMessage)
	}

	if svcErr.IssueID == 0 {
		return
	}

	if catalogEntry := issue.Get(svcErr.IssueID); catalogEntry != nil {
		rendered, renderErr := catalogEntry.Render("dark")
		if renderErr != nil {
			slog.Warn("failed to render issue catalog entry", "issueID", svcErr.IssueID, "error", renderErr)
		} else {
			fmt.Fprint(stderr, rendered)
		}
	}
}
