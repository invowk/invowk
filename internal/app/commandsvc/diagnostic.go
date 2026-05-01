// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// DiagnosticSeverityWarning indicates a recoverable command-service warning.
	DiagnosticSeverityWarning DiagnosticSeverity = "warning"
	// DiagnosticSeverityError indicates a non-fatal command-service error diagnostic.
	DiagnosticSeverityError DiagnosticSeverity = "error"

	// DiagnosticCodeConfigLoadFailed indicates the config file failed to load.
	DiagnosticCodeConfigLoadFailed DiagnosticCode = "config_load_failed"
	// DiagnosticCodeContainerRuntimeInitFailed indicates the container runtime could not be initialized.
	DiagnosticCodeContainerRuntimeInitFailed DiagnosticCode = "container_runtime_init_failed"
)

type (
	// DiagnosticSeverity is the command-service diagnostic level.
	DiagnosticSeverity string

	// DiagnosticCode is a stable command-service diagnostic identifier.
	DiagnosticCode string

	// DiagnosticMessage is the human-readable command-service diagnostic message.
	DiagnosticMessage string

	// Diagnostic is a command-service warning returned to driving adapters.
	Diagnostic struct {
		severity DiagnosticSeverity
		code     DiagnosticCode
		message  DiagnosticMessage
		path     types.FilesystemPath
		cause    error
	}
)

// String returns the string representation of the DiagnosticSeverity.
func (s DiagnosticSeverity) String() string { return string(s) }

// Validate returns nil if the severity is non-empty.
func (s DiagnosticSeverity) Validate() error {
	if strings.TrimSpace(string(s)) == "" {
		return errors.New("empty diagnostic severity")
	}
	return nil
}

// String returns the string representation of the DiagnosticCode.
func (c DiagnosticCode) String() string { return string(c) }

// Validate returns nil if the diagnostic code is non-empty.
func (c DiagnosticCode) Validate() error {
	if strings.TrimSpace(string(c)) == "" {
		return errors.New("empty diagnostic code")
	}
	return nil
}

// String returns the string representation of the DiagnosticMessage.
func (m DiagnosticMessage) String() string { return string(m) }

// Validate returns nil if the message is non-empty.
func (m DiagnosticMessage) Validate() error {
	if strings.TrimSpace(string(m)) == "" {
		return errors.New("empty diagnostic message")
	}
	return nil
}

// NewDiagnosticWithCause creates a command-service Diagnostic.
func NewDiagnosticWithCause(severity DiagnosticSeverity, code DiagnosticCode, message string, path types.FilesystemPath, cause error) (Diagnostic, error) {
	//goplint:ignore -- constructor validates the converted diagnostic message before returning it.
	diag := Diagnostic{severity: severity, code: code, message: DiagnosticMessage(message), path: path, cause: cause}
	if err := diag.Validate(); err != nil {
		return Diagnostic{}, err
	}
	return diag, nil
}

// DiagnosticFromDiscovery converts a discovery diagnostic at the service boundary.
func DiagnosticFromDiscovery(diag discovery.Diagnostic) (Diagnostic, error) {
	return NewDiagnosticWithCause(
		DiagnosticSeverity(diag.Severity()),
		DiagnosticCode(diag.Code()),
		diag.Message(),
		diag.Path(),
		diag.Cause(),
	)
}

// Severity returns the diagnostic level.
func (d Diagnostic) Severity() DiagnosticSeverity { return d.severity }

// Code returns the machine-readable diagnostic code.
func (d Diagnostic) Code() DiagnosticCode { return d.code }

// Message returns the human-readable diagnostic message.
func (d Diagnostic) Message() DiagnosticMessage { return d.message }

// Path returns the file path associated with this diagnostic, if any.
func (d Diagnostic) Path() types.FilesystemPath { return d.path }

// Cause returns the underlying error, if any.
func (d Diagnostic) Cause() error { return d.cause }

// Validate returns nil if the diagnostic has valid required fields.
func (d Diagnostic) Validate() error {
	var errs []error
	if err := d.severity.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.code.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := d.message.Validate(); err != nil {
		errs = append(errs, err)
	}
	if d.path != "" {
		if err := d.path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid command diagnostic: %w", errors.Join(errs...))
	}
	return nil
}
