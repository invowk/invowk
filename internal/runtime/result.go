// SPDX-License-Identifier: MPL-2.0

package runtime

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
