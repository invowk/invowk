// SPDX-License-Identifier: MPL-2.0

package runtime

import "errors"

const (
	nativeNoImplErrMsg      = "no script selected for execution"
	nativeNoScriptErrMsg    = "script has no content to execute"
	virtualNoImplErrMsg     = "no script selected for execution"
	virtualNoScriptErrMsg   = "script has no content to execute"
	containerNoImplErrMsg   = "no implementation selected for execution"
	containerNoScriptErrMsg = "implementation has no script to execute"
)

var (
	errNativeNoImpl      = errors.New(nativeNoImplErrMsg)
	errNativeNoScript    = errors.New(nativeNoScriptErrMsg)
	errVirtualNoImpl     = errors.New(virtualNoImplErrMsg)
	errVirtualNoScript   = errors.New(virtualNoScriptErrMsg)
	errContainerNoImpl   = errors.New(containerNoImplErrMsg)
	errContainerNoScript = errors.New(containerNoScriptErrMsg)
)

// validateExecutionContextForRun performs lightweight precondition checks shared by
// runtime execute/prepare paths. It prevents nil-pointer panics when callers build
// invalid contexts in tests or benchmarks and returns actionable errors instead.
func validateExecutionContextForRun(ctx *ExecutionContext, noImplErr, noScriptErr error) error {
	if ctx == nil {
		return errors.New("execution context is required")
	}
	if ctx.Invowkfile == nil {
		return errors.New("execution context has no invowkfile")
	}
	if ctx.SelectedImpl == nil {
		return noImplErr
	}
	if ctx.SelectedImpl.Script == "" {
		return noScriptErr
	}
	return nil
}
