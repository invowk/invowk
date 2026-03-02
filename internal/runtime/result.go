// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidExitCode is the sentinel error for invalid exit codes.
var ErrInvalidExitCode = types.ErrInvalidExitCode

type (
	// ExitCode is a type alias for the cross-cutting DDD Value Type defined in pkg/types.
	ExitCode = types.ExitCode

	// InvalidExitCodeError is a type alias for the error type in pkg/types.
	InvalidExitCodeError = types.InvalidExitCodeError
)

// NewErrorResult creates a Result with the given exit code and error.
func NewErrorResult(code ExitCode, err error) *Result {
	return &Result{ExitCode: code, Error: err}
}

// NewSuccessResult creates a Result with exit code 0 and no error.
func NewSuccessResult() *Result {
	return &Result{}
}

// NewExitCodeResult creates a Result with the given exit code and no error.
// Use this for non-zero exits that represent normal process termination
// rather than infrastructure failures.
func NewExitCodeResult(code ExitCode) *Result {
	return &Result{ExitCode: code}
}
