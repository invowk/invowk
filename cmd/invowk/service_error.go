// SPDX-License-Identifier: MPL-2.0

package cmd

import "invowk-cli/internal/issue"

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
