// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

const (
	// SeverityError indicates a validation failure that prevents execution.
	SeverityError ValidationSeverity = iota
	// SeverityWarning indicates a potential issue that doesn't prevent execution.
	SeverityWarning
)

var (
	// ErrInvalidValidationSeverity is returned when a ValidationSeverity value is not one of the defined severities.
	ErrInvalidValidationSeverity = errors.New("invalid validation severity")
	// ErrInvalidValidatorName is returned when a ValidatorName is empty or whitespace-only.
	ErrInvalidValidatorName = errors.New("invalid validator name")
)

type (
	// ValidationSeverity indicates the severity level of a validation error.
	ValidationSeverity int

	// InvalidValidationSeverityError is returned when a ValidationSeverity value is not recognized.
	// It wraps ErrInvalidValidationSeverity for errors.Is() compatibility.
	InvalidValidationSeverityError struct {
		Value ValidationSeverity
	}

	// ValidatorName identifies a validation component (e.g., "structure", "shebang").
	// Must be non-empty and not whitespace-only.
	ValidatorName string

	// InvalidValidatorNameError is returned when a ValidatorName is empty or whitespace-only.
	InvalidValidatorNameError struct {
		Value ValidatorName
	}

	// ValidationError represents a single validation issue found during invowkfile validation.
	ValidationError struct {
		// Validator is the name of the validator that produced this error.
		Validator ValidatorName
		// Field is the field path where the error occurred (e.g., "command 'build' implementation #1").
		Field string
		// Message is the human-readable error message.
		Message string
		// Severity indicates whether this is an error or warning.
		Severity ValidationSeverity
	}

	// ValidationErrors is a collection of validation errors that implements the error interface.
	// This allows returning multiple validation issues from a single validation pass.
	ValidationErrors []ValidationError

	// ValidationContext provides context for validation operations.
	// It contains information about the environment and configuration
	// that validators may need to properly validate an invowkfile.
	//
	// Validators apply defaults when values are zero-valued: nil FS defaults to
	// os.DirFS(WorkDir), empty Platform defaults to the current host platform.
	ValidationContext struct {
		// WorkDir is the working directory for resolving relative paths.
		WorkDir WorkDir
		// FS is the filesystem to use for file existence checks.
		// Defaults to os.DirFS(WorkDir) if nil.
		FS fs.FS
		// Platform is the target platform for validation.
		// Zero value ("") means current platform.
		Platform PlatformType
		// StrictMode treats warnings as errors when true.
		StrictMode bool
		// FilePath is the path to the invowkfile being validated.
		FilePath FilesystemPath
	}

	// Validator defines the interface for invowkfile validators.
	// Validators check specific aspects of an invowkfile and return all errors found.
	// Callers should display all returned errors collectively (not stop at first).
	// Error order is unspecified; priority is encoded in ValidationSeverity.
	// No circuit breaker or max error count is enforced by the framework.
	Validator interface {
		// Name returns a unique identifier for this validator.
		Name() ValidatorName
		// Validate checks the invowkfile and returns all validation errors found.
		// Unlike traditional validation that stops on first error, this collects
		// ALL errors to provide comprehensive feedback to users.
		Validate(ctx *ValidationContext, inv *Invowkfile) []ValidationError
	}

	// FieldPath is a builder for constructing hierarchical field paths.
	// It provides a fluent API for building context strings like
	// "command 'build' implementation #1 runtime #2".
	FieldPath struct {
		parts []string
	}
)

// Error implements the error interface for InvalidValidationSeverityError.
func (e *InvalidValidationSeverityError) Error() string {
	return fmt.Sprintf("invalid validation severity %d (valid: 0=error, 1=warning)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidValidationSeverityError) Unwrap() error {
	return ErrInvalidValidationSeverity
}

// IsValid returns whether the ValidationSeverity is one of the defined severity levels,
// and a list of validation errors if it is not.
func (s ValidationSeverity) IsValid() (bool, []error) {
	switch s {
	case SeverityError, SeverityWarning:
		return true, nil
	default:
		return false, []error{&InvalidValidationSeverityError{Value: s}}
	}
}

// String returns a human-readable representation of the severity level.
func (s ValidationSeverity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "unknown"
	}
}

// Error implements the error interface for InvalidValidatorNameError.
func (e *InvalidValidatorNameError) Error() string {
	return fmt.Sprintf("invalid validator name: %q", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidValidatorNameError) Unwrap() error {
	return ErrInvalidValidatorName
}

// IsValid returns whether the ValidatorName is non-empty and not whitespace-only,
// and a list of validation errors if it is not.
func (n ValidatorName) IsValid() (bool, []error) {
	if strings.TrimSpace(string(n)) == "" {
		return false, []error{&InvalidValidatorNameError{Value: n}}
	}
	return true, nil
}

// String returns the string representation of the ValidatorName.
func (n ValidatorName) String() string {
	return string(n)
}

// Error implements the error interface for ValidationError.
func (e ValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// IsError returns true if this is an error-level validation issue.
func (e ValidationError) IsError() bool {
	return e.Severity == SeverityError
}

// IsWarning returns true if this is a warning-level validation issue.
func (e ValidationError) IsWarning() bool {
	return e.Severity == SeverityWarning
}

// Error implements the error interface by joining all error messages.
func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return ""
	}
	if len(errs) == 1 {
		return errs[0].Error()
	}

	var b strings.Builder
	b.WriteString("validation failed with ")
	errorCount := errs.ErrorCount()
	warningCount := errs.WarningCount()

	if errorCount > 0 {
		if errorCount == 1 {
			b.WriteString("1 error")
		} else {
			b.WriteString(itoa(errorCount))
			b.WriteString(" errors")
		}
	}
	if warningCount > 0 {
		if errorCount > 0 {
			b.WriteString(" and ")
		}
		if warningCount == 1 {
			b.WriteString("1 warning")
		} else {
			b.WriteString(itoa(warningCount))
			b.WriteString(" warnings")
		}
	}
	b.WriteString(":\n")

	for i, err := range errs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  - ")
		b.WriteString(err.Error())
	}

	return b.String()
}

// HasErrors returns true if there are any error-level validation issues.
func (errs ValidationErrors) HasErrors() bool {
	for _, e := range errs {
		if e.IsError() {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warning-level validation issues.
func (errs ValidationErrors) HasWarnings() bool {
	for _, e := range errs {
		if e.IsWarning() {
			return true
		}
	}
	return false
}

// Errors returns only the error-level validation issues.
func (errs ValidationErrors) Errors() ValidationErrors {
	var result ValidationErrors
	for _, e := range errs {
		if e.IsError() {
			result = append(result, e)
		}
	}
	return result
}

// Warnings returns only the warning-level validation issues.
func (errs ValidationErrors) Warnings() ValidationErrors {
	var result ValidationErrors
	for _, e := range errs {
		if e.IsWarning() {
			result = append(result, e)
		}
	}
	return result
}

// ErrorCount returns the number of error-level validation issues.
func (errs ValidationErrors) ErrorCount() int {
	count := 0
	for _, e := range errs {
		if e.IsError() {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-level validation issues.
func (errs ValidationErrors) WarningCount() int {
	count := 0
	for _, e := range errs {
		if e.IsWarning() {
			count++
		}
	}
	return count
}

// NewFieldPath creates a new empty FieldPath builder.
func NewFieldPath() *FieldPath {
	return &FieldPath{}
}

// String returns the complete field path as a string.
func (p *FieldPath) String() string {
	return strings.Join(p.parts, " ")
}

// Root adds the "root" context to the path.
func (p *FieldPath) Root() *FieldPath {
	p.parts = append(p.parts, "root")
	return p
}

// Command adds a command context to the path.
func (p *FieldPath) Command(name string) *FieldPath {
	p.parts = append(p.parts, "command '"+name+"'")
	return p
}

// Implementation adds an implementation context to the path (1-indexed for user display).
func (p *FieldPath) Implementation(index int) *FieldPath {
	p.parts = append(p.parts, "implementation #"+itoa(index+1))
	return p
}

// Runtime adds a runtime context to the path (1-indexed for user display).
func (p *FieldPath) Runtime(index int) *FieldPath {
	p.parts = append(p.parts, "runtime #"+itoa(index+1))
	return p
}

// Flag adds a flag context to the path.
func (p *FieldPath) Flag(name string) *FieldPath {
	p.parts = append(p.parts, "flag '"+name+"'")
	return p
}

// FlagIndex adds a flag context by index to the path (1-indexed for user display).
func (p *FieldPath) FlagIndex(index int) *FieldPath {
	p.parts = append(p.parts, "flag #"+itoa(index+1))
	return p
}

// Arg adds an argument context to the path.
func (p *FieldPath) Arg(name string) *FieldPath {
	p.parts = append(p.parts, "argument '"+name+"'")
	return p
}

// ArgIndex adds an argument context by index to the path (1-indexed for user display).
func (p *FieldPath) ArgIndex(index int) *FieldPath {
	p.parts = append(p.parts, "argument #"+itoa(index+1))
	return p
}

// Volume adds a volume context to the path (1-indexed for user display).
func (p *FieldPath) Volume(index int) *FieldPath {
	p.parts = append(p.parts, "volume #"+itoa(index+1))
	return p
}

// Port adds a port context to the path (1-indexed for user display).
func (p *FieldPath) Port(index int) *FieldPath {
	p.parts = append(p.parts, "port #"+itoa(index+1))
	return p
}

// Env adds an env context to the path.
func (p *FieldPath) Env() *FieldPath {
	p.parts = append(p.parts, "env")
	return p
}

// EnvFile adds an env.files context to the path (1-indexed for user display).
func (p *FieldPath) EnvFile(index int) *FieldPath {
	p.parts = append(p.parts, "env.files["+itoa(index+1)+"]")
	return p
}

// EnvVar adds an env.vars context to the path.
func (p *FieldPath) EnvVar(key string) *FieldPath {
	p.parts = append(p.parts, "env.vars['"+key+"']")
	return p
}

// DependsOn adds a depends_on context to the path.
func (p *FieldPath) DependsOn() *FieldPath {
	p.parts = append(p.parts, "depends_on")
	return p
}

// Tools adds a tools context to the path with indices (1-indexed for user display).
func (p *FieldPath) Tools(depIndex, altIndex int) *FieldPath {
	p.parts = append(p.parts, "tools["+itoa(depIndex+1)+"].alternatives["+itoa(altIndex+1)+"]")
	return p
}

// Commands adds a cmds context to the path with indices (1-indexed for user display).
func (p *FieldPath) Commands(depIndex, altIndex int) *FieldPath {
	p.parts = append(p.parts, "cmds["+itoa(depIndex+1)+"].alternatives["+itoa(altIndex+1)+"]")
	return p
}

// Filepaths adds a filepaths context to the path (1-indexed for user display).
func (p *FieldPath) Filepaths(index int) *FieldPath {
	p.parts = append(p.parts, "filepaths["+itoa(index+1)+"]")
	return p
}

// EnvVars adds an env_vars context to the path with indices (1-indexed for user display).
func (p *FieldPath) EnvVars(depIndex, altIndex int) *FieldPath {
	p.parts = append(p.parts, "env_vars["+itoa(depIndex+1)+"].alternatives["+itoa(altIndex+1)+"]")
	return p
}

// CustomCheck adds a custom_check context to the path (1-indexed for user display).
func (p *FieldPath) CustomCheck(checkIndex, altIndex int) *FieldPath {
	p.parts = append(p.parts, "custom_check #"+itoa(checkIndex+1)+" alternative #"+itoa(altIndex+1))
	return p
}

// Field adds a generic field context to the path.
func (p *FieldPath) Field(name string) *FieldPath {
	p.parts = append(p.parts, name)
	return p
}

// Copy returns a shallow copy of the FieldPath.
// This is useful when branching into sub-contexts.
func (p *FieldPath) Copy() *FieldPath {
	parts := make([]string, len(p.parts))
	copy(parts, p.parts)
	return &FieldPath{parts: parts}
}

// itoa converts an integer to a string without importing strconv.
// This is a simple helper to avoid an extra import for basic number formatting.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + itoa(-i)
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
