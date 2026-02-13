// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"io"

	"invowk-cli/internal/issue"
)

// ServiceError is an error that carries optional rendering information for
// the CLI layer. When the CLI layer receives a ServiceError, it renders the
// styled error message (if present) before formatting the underlying error.
type ServiceError struct {
	// Err is the underlying error.
	Err error
	// IssueID is the optional issue catalog ID for rendering help text.
	IssueID issue.Id
	// StyledMessage is the optional pre-rendered styled error text.
	StyledMessage string
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
		rendered, _ := catalogEntry.Render("dark")
		fmt.Fprint(stderr, rendered)
	}
}
