// SPDX-License-Identifier: MPL-2.0

package discovery

const (
	// SeverityWarning indicates a recoverable discovery warning.
	SeverityWarning = "warning"
	// SeverityError indicates a non-fatal discovery error diagnostic.
	SeverityError = "error"
)

type (
	// Severity represents discovery diagnostic severity.
	Severity string

	// Diagnostic represents a structured discovery diagnostic.
	Diagnostic struct {
		Severity Severity
		Code     string
		Message  string
		Path     string
		Cause    error
	}

	// CommandSetResult contains discovered commands and diagnostics.
	CommandSetResult struct {
		Set         *DiscoveredCommandSet
		Diagnostics []Diagnostic
	}

	// LookupResult contains a command lookup result and diagnostics.
	LookupResult struct {
		Command     *CommandInfo
		Diagnostics []Diagnostic
	}
)
