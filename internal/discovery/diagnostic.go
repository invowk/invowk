// SPDX-License-Identifier: MPL-2.0

package discovery

// Severity represents discovery diagnostic severity.
type Severity string

const (
	// SeverityWarning indicates a recoverable discovery warning.
	SeverityWarning Severity = "warning"
	// SeverityError indicates a non-fatal discovery error diagnostic.
	SeverityError Severity = "error"
)

// Diagnostic represents a structured discovery diagnostic.
type Diagnostic struct {
	Severity Severity
	Code     string
	Message  string
	Path     string
	Cause    error
}

// CommandSetResult contains discovered commands and diagnostics.
type CommandSetResult struct {
	Set         *DiscoveredCommandSet
	Diagnostics []Diagnostic
}

// LookupResult contains a command lookup result and diagnostics.
type LookupResult struct {
	Command     *CommandInfo
	Diagnostics []Diagnostic
}
