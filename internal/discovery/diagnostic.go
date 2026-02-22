// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"fmt"
)

const (
	// SeverityWarning indicates a recoverable discovery warning.
	SeverityWarning Severity = "warning"
	// SeverityError indicates a non-fatal discovery error diagnostic.
	SeverityError Severity = "error"

	// CodeWorkingDirUnavailable indicates the working directory is unavailable for discovery.
	CodeWorkingDirUnavailable DiagnosticCode = "working_dir_unavailable"
	// CodeCommandsDirUnavailable indicates the user commands directory is unavailable.
	CodeCommandsDirUnavailable DiagnosticCode = "commands_dir_unavailable"
	// CodeConfigLoadFailed indicates the config file failed to load.
	CodeConfigLoadFailed DiagnosticCode = "config_load_failed"
	// CodeCommandNotFound indicates a requested command was not found.
	CodeCommandNotFound DiagnosticCode = "command_not_found"
	// CodeInvowkfileParseSkipped indicates an invowkfile was skipped due to parse errors.
	CodeInvowkfileParseSkipped DiagnosticCode = "invowkfile_parse_skipped"
	// CodeModuleScanPathInvalid indicates a module scan path was invalid.
	CodeModuleScanPathInvalid DiagnosticCode = "module_scan_path_invalid"
	// CodeModuleScanFailed indicates a module directory scan failed.
	CodeModuleScanFailed DiagnosticCode = "module_scan_failed"
	// CodeReservedModuleNameSkipped indicates a module with a reserved name was skipped.
	CodeReservedModuleNameSkipped DiagnosticCode = "reserved_module_name_skipped"
	// CodeModuleLoadSkipped indicates a module was skipped due to load failure.
	CodeModuleLoadSkipped DiagnosticCode = "module_load_skipped"
	// CodeIncludeNotModule indicates an include path does not point to a module.
	CodeIncludeNotModule DiagnosticCode = "include_not_module"
	// CodeIncludeReservedSkipped indicates an included module with a reserved name was skipped.
	CodeIncludeReservedSkipped DiagnosticCode = "include_reserved_module_skipped"
	// CodeIncludeModuleLoadFailed indicates an included module failed to load.
	CodeIncludeModuleLoadFailed DiagnosticCode = "include_module_load_failed"
	// CodeVendoredScanFailed indicates a vendored modules scan failed.
	CodeVendoredScanFailed DiagnosticCode = "vendored_scan_failed"
	// CodeVendoredReservedSkipped indicates a vendored module with a reserved name was skipped.
	CodeVendoredReservedSkipped DiagnosticCode = "vendored_reserved_module_skipped"
	// CodeVendoredModuleLoadSkipped indicates a vendored module was skipped due to load failure.
	CodeVendoredModuleLoadSkipped DiagnosticCode = "vendored_module_load_skipped"
	// CodeVendoredNestedIgnored indicates nested vendored modules were ignored.
	CodeVendoredNestedIgnored DiagnosticCode = "vendored_nested_ignored"
)

var (
	// ErrInvalidSeverity is returned when a Severity value is not one of the defined severities.
	ErrInvalidSeverity = errors.New("invalid severity")
	// ErrInvalidDiagnosticCode is returned when a DiagnosticCode value is not one of the defined codes.
	ErrInvalidDiagnosticCode = errors.New("invalid diagnostic code")
	// ErrInvalidSource is returned when a Source value is not one of the defined source types.
	ErrInvalidSource = errors.New("invalid source")
	// ErrInvalidSourceID is returned when a SourceID value does not match the expected format.
	ErrInvalidSourceID = errors.New("invalid source id")
)

type (
	// Severity represents discovery diagnostic severity.
	Severity string

	// DiagnosticCode is a machine-readable identifier for a diagnostic.
	DiagnosticCode string

	// InvalidSeverityError is returned when a Severity value is not recognized.
	// It wraps ErrInvalidSeverity for errors.Is() compatibility.
	InvalidSeverityError struct {
		Value Severity
	}

	// InvalidDiagnosticCodeError is returned when a DiagnosticCode value is not recognized.
	// It wraps ErrInvalidDiagnosticCode for errors.Is() compatibility.
	InvalidDiagnosticCodeError struct {
		Value DiagnosticCode
	}

	// InvalidSourceError is returned when a Source value is not recognized.
	// It wraps ErrInvalidSource for errors.Is() compatibility.
	InvalidSourceError struct {
		Value Source
	}

	// InvalidSourceIDError is returned when a SourceID value does not match the expected format.
	// It wraps ErrInvalidSourceID for errors.Is() compatibility.
	InvalidSourceIDError struct {
		Value SourceID
	}

	// Diagnostic represents a structured discovery diagnostic that is returned
	// to callers (rather than written to stderr) for consistent rendering policy.
	Diagnostic struct {
		// Severity is the diagnostic level (warning or error).
		Severity Severity
		// Code is a machine-readable identifier (e.g., "invowkfile_parse_skipped").
		Code DiagnosticCode
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

// String returns the string representation of the severity level.
func (s Severity) String() string {
	return string(s)
}

// Error implements the error interface for InvalidSeverityError.
func (e *InvalidSeverityError) Error() string {
	return fmt.Sprintf("invalid severity %q (valid: warning, error)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidSeverityError) Unwrap() error {
	return ErrInvalidSeverity
}

// IsValid returns whether the Severity is one of the defined severity levels,
// and a list of validation errors if it is not.
func (s Severity) IsValid() (bool, []error) {
	switch s {
	case SeverityWarning, SeverityError:
		return true, nil
	default:
		return false, []error{&InvalidSeverityError{Value: s}}
	}
}

// Error implements the error interface for InvalidDiagnosticCodeError.
func (e *InvalidDiagnosticCodeError) Error() string {
	return fmt.Sprintf("invalid diagnostic code %q", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidDiagnosticCodeError) Unwrap() error {
	return ErrInvalidDiagnosticCode
}

// Error implements the error interface for InvalidSourceError.
func (e *InvalidSourceError) Error() string {
	return fmt.Sprintf("invalid source %d (valid: 0=current_directory, 1=module)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidSourceError) Unwrap() error {
	return ErrInvalidSource
}

// Error implements the error interface for InvalidSourceIDError.
func (e *InvalidSourceIDError) Error() string {
	return fmt.Sprintf("invalid source id %q (must start with a letter and contain only letters, digits, dots, underscores, or hyphens)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidSourceIDError) Unwrap() error {
	return ErrInvalidSourceID
}

// IsValid returns whether the DiagnosticCode is one of the defined codes,
// and a list of validation errors if it is not.
func (dc DiagnosticCode) IsValid() (bool, []error) {
	switch dc {
	case CodeWorkingDirUnavailable, CodeCommandsDirUnavailable, CodeConfigLoadFailed,
		CodeCommandNotFound, CodeInvowkfileParseSkipped, CodeModuleScanPathInvalid,
		CodeModuleScanFailed, CodeReservedModuleNameSkipped, CodeModuleLoadSkipped,
		CodeIncludeNotModule, CodeIncludeReservedSkipped, CodeIncludeModuleLoadFailed,
		CodeVendoredScanFailed, CodeVendoredReservedSkipped, CodeVendoredModuleLoadSkipped,
		CodeVendoredNestedIgnored:
		return true, nil
	default:
		return false, []error{&InvalidDiagnosticCodeError{Value: dc}}
	}
}
