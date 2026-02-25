// SPDX-License-Identifier: MPL-2.0

package issue

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type (
	// ActionableError is an error with context for user-facing error messages.
	// It provides structured information about what operation failed, what resource
	// was involved, and suggestions for how to fix the issue.
	// Fields are unexported for immutability; use Operation(), Resource(),
	// Suggestions(), and Cause() accessors.
	//
	// Use the ErrorContext builder for convenient construction:
	//
	//	err := issue.NewErrorContext().
	//		WithOperation("load invowkfile").
	//		WithResource("./invowkfile.cue").
	//		WithSuggestion("Run 'invowk init' to create one").
	//		Wrap(originalErr).
	//		Build()
	ActionableError struct {
		operation   string
		resource    string
		suggestions []string
		cause       error
	}

	// ErrorContext is a builder for constructing ActionableError instances.
	// It provides a fluent API for setting error context incrementally.
	//
	// Example:
	//
	//	ctx := issue.NewErrorContext().
	//		WithOperation("parse config").
	//		WithResource("/etc/myapp/config.yaml")
	//
	//	// Later, when error occurs:
	//	return ctx.WithSuggestion("Check YAML syntax").Wrap(err).Build()
	ErrorContext struct {
		operation   string
		resource    string
		suggestions []string
		cause       error
	}
)

// --- Constructors ---

// NewActionableError creates an ActionableError with the given operation.
// Use this for simple errors; use ErrorContext for more complex cases.
func NewActionableError(operation string) *ActionableError {
	return &ActionableError{
		operation: operation,
	}
}

// --- Accessors ---

// Operation returns the operation that was being attempted.
func (e *ActionableError) Operation() string { return e.operation }

// Resource returns the file, path, or entity involved (may be empty).
func (e *ActionableError) Resource() string { return e.resource }

// Suggestions returns a copy of the fix suggestions (may be empty).
func (e *ActionableError) Suggestions() []string { return slices.Clone(e.suggestions) }

// Cause returns the underlying error (may be nil).
func (e *ActionableError) Cause() error { return e.cause }

// NewErrorContext creates a new ErrorContext builder.
func NewErrorContext() *ErrorContext {
	return &ErrorContext{}
}

// WrapWithOperation wraps an error with operation context.
// This is a shorthand for common wrapping patterns.
func WrapWithOperation(err error, operation string) *ActionableError {
	if err == nil {
		return nil
	}
	return &ActionableError{
		operation: operation,
		cause:     err,
	}
}

// WrapWithContext wraps an error with operation and resource context.
func WrapWithContext(err error, operation, resource string) *ActionableError {
	if err == nil {
		return nil
	}
	return &ActionableError{
		operation: operation,
		resource:  resource,
		cause:     err,
	}
}

// --- ActionableError Methods ---

// Error implements the error interface.
// Returns a concise error message suitable for default (non-verbose) output.
func (e *ActionableError) Error() string {
	var msg strings.Builder

	msg.WriteString("failed to ")
	msg.WriteString(e.operation)

	if e.resource != "" {
		msg.WriteString(": ")
		msg.WriteString(e.resource)
	}

	if e.cause != nil {
		msg.WriteString(": ")
		msg.WriteString(e.cause.Error())
	}

	return msg.String()
}

// Unwrap returns the underlying cause error for use with errors.Is/As.
func (e *ActionableError) Unwrap() error {
	return e.cause
}

// Format returns a formatted error message with optional verbosity.
//
// When verbose is false:
//
//	failed to <operation>: <resource>: <cause message>
//	  • <suggestion 1>
//	  • <suggestion 2>
//
// When verbose is true, additionally includes the full error chain.
func (e *ActionableError) Format(verbose bool) string {
	var msg strings.Builder

	// Write the main error message
	msg.WriteString(e.Error())

	// Add suggestions if present
	if len(e.suggestions) > 0 {
		msg.WriteString("\n")
		for _, suggestion := range e.suggestions {
			msg.WriteString("\n  • ")
			msg.WriteString(suggestion)
		}
	}

	// In verbose mode, include the full error chain
	if verbose && e.cause != nil {
		msg.WriteString("\n\nError chain:")
		err := e.cause
		depth := 1
		for err != nil {
			fmt.Fprintf(&msg, "\n  %d. %s", depth, err.Error())
			err = errors.Unwrap(err)
			depth++
		}
	}

	return msg.String()
}

// HasSuggestions returns true if the error has any suggestions.
func (e *ActionableError) HasSuggestions() bool {
	return len(e.suggestions) > 0
}

// --- ErrorContext Methods ---

// WithOperation sets the operation being performed.
// The operation should be a verb phrase like "load invowkfile" or "execute command".
func (c *ErrorContext) WithOperation(op string) *ErrorContext {
	c.operation = op
	return c
}

// WithResource sets the resource (file, path, entity) involved.
func (c *ErrorContext) WithResource(res string) *ErrorContext {
	c.resource = res
	return c
}

// WithSuggestion adds a suggestion for how to fix the issue.
// Can be called multiple times to add multiple suggestions.
func (c *ErrorContext) WithSuggestion(sug string) *ErrorContext {
	c.suggestions = append(c.suggestions, sug)
	return c
}

// WithSuggestions adds multiple suggestions at once.
func (c *ErrorContext) WithSuggestions(sugs ...string) *ErrorContext {
	c.suggestions = append(c.suggestions, sugs...)
	return c
}

// Wrap wraps an underlying error as the cause.
func (c *ErrorContext) Wrap(err error) *ErrorContext {
	c.cause = err
	return c
}

// Build creates an ActionableError from the context.
// Returns nil if no operation is set (operation is required).
func (c *ErrorContext) Build() *ActionableError {
	if c.operation == "" {
		return nil
	}

	return &ActionableError{
		operation:   c.operation,
		resource:    c.resource,
		suggestions: c.suggestions,
		cause:       c.cause,
	}
}

// BuildError creates an ActionableError and returns it as an error interface.
// This is a convenience method for direct use in return statements.
// Returns nil if no operation is set.
func (c *ErrorContext) BuildError() error {
	ae := c.Build()
	if ae == nil {
		return nil
	}
	return ae
}
