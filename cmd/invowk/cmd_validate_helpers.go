// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"

	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// evaluateAlternatives iterates over a list of alternatives with OR semantics:
// the first alternative that passes the check function satisfies the dependency.
// Returns (true, nil) if any alternative passed, or (false, lastErr) if all failed.
func evaluateAlternatives[T any](alternatives []T, check func(T) error) (bool, error) {
	var lastErr error
	for _, alt := range alternatives {
		if err := check(alt); err == nil {
			return true, nil
		} else {
			lastErr = err
		}
	}
	return false, lastErr
}

// newContainerValidationContext creates an ExecutionContext for running a validation
// script inside a container. This DRYs the 6+ identical struct constructions
// across the container dependency check functions.
func newContainerValidationContext(parentCtx *runtime.ExecutionContext, script string) (execCtx *runtime.ExecutionContext, stdout, stderr *bytes.Buffer) {
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	execCtx = &runtime.ExecutionContext{
		Command:         parentCtx.Command,
		Invowkfile:      parentCtx.Invowkfile,
		SelectedImpl:    &invowkfile.Implementation{Script: script, Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}},
		SelectedRuntime: invowkfile.RuntimeContainer,
		Context:         parentCtx.Context,
		IO:              runtime.IOContext{Stdout: stdout, Stderr: stderr},
		Env:             runtime.DefaultEnv(),
	}
	return execCtx, stdout, stderr
}
