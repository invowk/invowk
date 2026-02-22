// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"bytes"
	"fmt"
	"strings"

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
		SelectedImpl:    &invowkfile.Implementation{Script: invowkfile.ScriptContent(script), Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}}},
		SelectedRuntime: invowkfile.RuntimeContainer,
		Context:         parentCtx.Context,
		IO:              runtime.IOContext{Stdout: stdout, Stderr: stderr},
		Env:             runtime.DefaultEnv(),
	}
	return execCtx, stdout, stderr
}

// collectToolErrors evaluates each tool dependency and collects error messages for
// tools that are not satisfied. Each tool has alternatives with OR semantics (any
// alternative found satisfies the dependency). The check function validates a single
// tool name; it's called for each alternative until one succeeds.
func collectToolErrors(tools []invowkfile.ToolDependency, check func(invowkfile.BinaryName) error) []string {
	var toolErrors []string

	for _, tool := range tools {
		found, lastErr := evaluateAlternatives(tool.Alternatives, check)
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, lastErr.Error())
			} else {
				names := make([]string, len(tool.Alternatives))
				for i, alt := range tool.Alternatives {
					names[i] = string(alt)
				}
				toolErrors = append(toolErrors, fmt.Sprintf("  • none of [%s] found", strings.Join(names, ", ")))
			}
		}
	}

	return toolErrors
}

// checkTransientExitCode returns a formatted error if the container execution result
// indicates a transient engine failure (exit codes 125/126). Returns nil otherwise.
// All container validation functions must call this after checking result.Error
// and before interpreting result.ExitCode for domain-specific failures.
func checkTransientExitCode(result *runtime.Result, label string) error {
	if result.ExitCode.IsTransient() {
		return fmt.Errorf("  • %s - container engine failure (exit code %s)", label, result.ExitCode)
	}
	return nil
}
