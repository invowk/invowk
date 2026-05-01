// SPDX-License-Identifier: MPL-2.0

package deps

import (
	"fmt"
	"strings"

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

// ShellEscapeSingleQuote escapes single quotes for safe use inside shell single-quoted arguments.
// Each embedded single-quote is replaced with the shell idiom that closes the current quoting,
// adds a backslash-escaped literal quote, and reopens single-quoting.
func ShellEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}
