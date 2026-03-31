// SPDX-License-Identifier: MPL-2.0

package runtime

import "errors"

const (
	nativeNoImplErrMsg           = "no script selected for execution"
	nativeNoScriptErrMsg         = "script has no content to execute"
	virtualNoImplErrMsg          = "no script selected for execution"
	virtualNoScriptErrMsg        = "script has no content to execute"
	containerNoImplErrMsg        = "no implementation selected for execution"
	containerNoScriptErrMsg      = "implementation has no script to execute"
	nilExecutionContextErrMsg    = "execution context is required"
	noInvowkfileErrMsg           = "execution context has no invowkfile"
	sshServerNotConfiguredErrMsg = "enable_host_ssh is enabled but SSH server is not configured"
	sshServerNotRunningErrMsg    = "enable_host_ssh is enabled but SSH server is not running"
)

var (
	errNilExecutionContext    = errors.New(nilExecutionContextErrMsg)
	errNoInvowkfile           = errors.New(noInvowkfileErrMsg)
	errNativeNoImpl           = errors.New(nativeNoImplErrMsg)
	errNativeNoScript         = errors.New(nativeNoScriptErrMsg)
	errVirtualNoImpl          = errors.New(virtualNoImplErrMsg)
	errVirtualNoScript        = errors.New(virtualNoScriptErrMsg)
	errContainerNoImpl        = errors.New(containerNoImplErrMsg)
	errContainerNoScript      = errors.New(containerNoScriptErrMsg)
	errSSHServerNotConfigured = errors.New(sshServerNotConfiguredErrMsg)
	errSSHServerNotRunning    = errors.New(sshServerNotRunningErrMsg)
)

// validateExecutionContextForRun performs lightweight precondition checks shared by
// runtime execute/prepare paths. It prevents nil-pointer panics when callers build
// invalid contexts in tests or benchmarks and returns actionable errors instead.
func validateExecutionContextForRun(ctx *ExecutionContext, noImplErr, noScriptErr error) error {
	if ctx == nil {
		return errNilExecutionContext
	}
	if ctx.Invowkfile == nil {
		return errNoInvowkfile
	}
	if ctx.SelectedImpl == nil {
		return noImplErr
	}
	if ctx.SelectedImpl.Script == "" {
		return noScriptErr
	}
	return nil
}
