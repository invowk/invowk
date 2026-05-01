// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// EvaluateAlternatives iterates over a list of alternatives with OR semantics:
// the first alternative that passes the check function satisfies the dependency.
// Returns (true, nil) if any alternative passed, or (false, lastErr) if all failed.
func EvaluateAlternatives[T any](alternatives []T, check func(T) error) (bool, error) {
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

// NewContainerValidationContext creates an ExecutionContext for running a validation
// script inside a container. This DRYs the 6+ identical struct constructions
// across the container dependency check functions.
func NewContainerValidationContext(parentCtx *runtime.ExecutionContext, script string) (execCtx *runtime.ExecutionContext, stdout, stderr *bytes.Buffer) {
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	selectedImpl := invowkfile.Implementation{
		Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}},
	}
	if parentCtx.SelectedImpl != nil {
		selectedImpl = *parentCtx.SelectedImpl
	}
	selectedImpl.Script = invowkfile.ScriptContent(script) //goplint:ignore -- inline validation script
	selectedRuntime := parentCtx.SelectedRuntime
	if selectedRuntime == "" {
		selectedRuntime = invowkfile.RuntimeContainer
	}
	execCtx = &runtime.ExecutionContext{
		Command:         parentCtx.Command,
		Invowkfile:      parentCtx.Invowkfile,
		SelectedImpl:    &selectedImpl,
		SelectedRuntime: selectedRuntime,
		Context:         parentCtx.Context,
		PositionalArgs:  parentCtx.PositionalArgs,
		WorkDir:         parentCtx.WorkDir,
		Verbose:         parentCtx.Verbose,
		ForceRebuild:    parentCtx.ForceRebuild,
		ExecutionID:     parentCtx.ExecutionID,
		IO:              runtime.IOContext{Stdout: stdout, Stderr: stderr},
		Env:             parentCtx.Env,
		TUI:             parentCtx.TUI,
	}
	return execCtx, stdout, stderr
}

// CollectToolErrors evaluates each tool dependency and collects error messages for
// tools that are not satisfied. Each tool has alternatives with OR semantics (any
// alternative found satisfies the dependency). The check function validates a single
// tool name; it's called for each alternative until one succeeds.
func CollectToolErrors(tools []invowkfile.ToolDependency, check func(invowkfile.BinaryName) error) []DependencyMessage {
	var toolErrors []DependencyMessage

	for _, tool := range tools {
		found, lastErr := EvaluateAlternatives(tool.Alternatives, check)
		if !found && lastErr != nil {
			if len(tool.Alternatives) == 1 {
				toolErrors = append(toolErrors, dependencyMessageFromDetail(lastErr.Error()))
			} else {
				names := make([]string, len(tool.Alternatives))
				for i, alt := range tool.Alternatives {
					names[i] = string(alt)
				}
				toolErrors = append(toolErrors, dependencyMessageFromDetail(fmt.Sprintf("none of [%s] found", strings.Join(names, ", "))))
			}
		}
	}

	return toolErrors
}

// CheckTransientExitCode returns a formatted error if the container execution result
// indicates a transient engine failure (exit codes 125/126). Returns nil otherwise.
// All container validation functions must call this after checking result.Error
// and before interpreting result.ExitCode for domain-specific failures.
func CheckTransientExitCode(result *runtime.Result, label string) error {
	if container.IsTransientEngineExitCode(result.ExitCode) {
		return fmt.Errorf("  • %s - %w (exit code %s)", label, ErrContainerEngineFailure, result.ExitCode)
	}
	return nil
}

// ShellEscapeSingleQuote escapes single quotes for safe use inside shell single-quoted arguments.
// Each embedded single-quote is replaced with the shell idiom that closes the current quoting,
// adds a backslash-escaped literal quote, and reopens single-quoting.
func ShellEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
