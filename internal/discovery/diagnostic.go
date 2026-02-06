// SPDX-License-Identifier: MPL-2.0

package discovery

const (
	// SeverityWarning indicates a recoverable discovery warning.
	SeverityWarning Severity = "warning"
	// SeverityError indicates a non-fatal discovery error diagnostic.
	SeverityError Severity = "error"
)

type (
	// Severity represents discovery diagnostic severity.
	Severity string

	// Diagnostic represents a structured discovery diagnostic that is returned
	// to callers (rather than written to stderr) for consistent rendering policy.
	Diagnostic struct {
		// Severity is the diagnostic level (warning or error).
		Severity Severity
		// Code is a machine-readable identifier (e.g., "invkfile_parse_skipped").
		Code string
		// Message is the human-readable description.
		Message string
		// Path is the file path associated with this diagnostic (optional).
		Path string
		// Cause is the underlying error (optional, for programmatic inspection).
		Cause error
	}

	// CommandSetResult bundles a DiscoveredCommandSet with diagnostics produced
	// during discovery. Diagnostics include parse warnings, config load failures,
	// and other non-fatal issues that should be rendered by the CLI layer.
	CommandSetResult struct {
		Set         *DiscoveredCommandSet
		Diagnostics []Diagnostic
	}

	// LookupResult bundles a single command lookup result with diagnostics.
	// Command is nil when the requested command was not found (the diagnostic
	// list will contain a "command_not_found" entry).
	LookupResult struct {
		Command     *CommandInfo
		Diagnostics []Diagnostic
	}
)
