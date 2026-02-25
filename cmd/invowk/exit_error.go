// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"

	"github.com/invowk/invowk/pkg/types"
)

// ExitError signals a non-zero exit code without forcing os.Exit in RunE handlers.
type ExitError struct {
	Code types.ExitCode
	Err  error
}

// Error returns the error message for ExitError.
func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

// Unwrap returns the underlying error, if any.
func (e *ExitError) Unwrap() error {
	return e.Err
}
