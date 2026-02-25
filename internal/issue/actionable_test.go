// SPDX-License-Identifier: MPL-2.0

package issue

import (
	"errors"
	"strings"
	"testing"
)

// T035: ActionableError formatting tests
func TestActionableError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ActionableError
		expected string
	}{
		{
			name: "operation only",
			err: &ActionableError{
				operation: "load invowkfile",
			},
			expected: "failed to load invowkfile",
		},
		{
			name: "operation with resource",
			err: &ActionableError{
				operation: "load invowkfile",
				resource:  "./invowkfile.cue",
			},
			expected: "failed to load invowkfile: ./invowkfile.cue",
		},
		{
			name: "operation with cause",
			err: &ActionableError{
				operation: "parse config",
				cause:     errors.New("syntax error at line 5"),
			},
			expected: "failed to parse config: syntax error at line 5",
		},
		{
			name: "full context",
			err: &ActionableError{
				operation: "load invowkfile",
				resource:  "./invowkfile.cue",
				cause:     errors.New("file not found"),
			},
			expected: "failed to load invowkfile: ./invowkfile.cue: file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestActionableError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying error")
	err := &ActionableError{
		operation: "test",
		cause:     cause,
	}

	// Test that Unwrap returns the cause (use errors.Is for proper comparison)
	if !errors.Is(err.Unwrap(), cause) {
		t.Error("Unwrap() should return the cause error")
	}

	errNoCause := &ActionableError{operation: "test"}
	if errNoCause.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no cause")
	}
}

func TestActionableError_ErrorsIs(t *testing.T) {
	t.Parallel()

	cause := errors.New("specific error")
	wrapped := &ActionableError{
		operation: "test",
		cause:     cause,
	}

	if !errors.Is(wrapped, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
}

func TestActionableError_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *ActionableError
		verbose  bool
		contains []string
		excludes []string
	}{
		{
			name: "simple error non-verbose",
			err: &ActionableError{
				operation: "load config",
			},
			verbose:  false,
			contains: []string{"failed to load config"},
		},
		{
			name: "error with suggestions",
			err: &ActionableError{
				operation:   "load invowkfile",
				resource:    "./invowkfile.cue",
				suggestions: []string{"Run 'invowk init'", "Check file permissions"},
			},
			verbose: false,
			contains: []string{
				"failed to load invowkfile",
				"./invowkfile.cue",
				"• Run 'invowk init'",
				"• Check file permissions",
			},
		},
		{
			name: "error chain in verbose mode",
			err: &ActionableError{
				operation: "parse config",
				cause:     errors.New("syntax error"),
			},
			verbose: true,
			contains: []string{
				"failed to parse config",
				"Error chain:",
				"1. syntax error",
			},
		},
		{
			name: "no error chain in non-verbose",
			err: &ActionableError{
				operation: "parse config",
				cause:     errors.New("syntax error"),
			},
			verbose:  false,
			contains: []string{"failed to parse config: syntax error"},
			excludes: []string{"Error chain:"},
		},
		{
			name: "nested error chain verbose",
			err: &ActionableError{
				operation: "execute command",
				cause: &ActionableError{
					operation: "load script",
					cause:     errors.New("file not found"),
				},
			},
			verbose: true,
			contains: []string{
				"Error chain:",
				"1. failed to load script: file not found",
				"2. file not found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.err.Format(tt.verbose)

			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("Format() missing %q\ngot:\n%s", s, got)
				}
			}

			for _, s := range tt.excludes {
				if strings.Contains(got, s) {
					t.Errorf("Format() should not contain %q\ngot:\n%s", s, got)
				}
			}
		})
	}
}

func TestActionableError_HasSuggestions(t *testing.T) {
	t.Parallel()

	withSuggestions := &ActionableError{
		operation:   "test",
		suggestions: []string{"Try this"},
	}
	if !withSuggestions.HasSuggestions() {
		t.Error("HasSuggestions() should return true when suggestions present")
	}

	withoutSuggestions := &ActionableError{
		operation: "test",
	}
	if withoutSuggestions.HasSuggestions() {
		t.Error("HasSuggestions() should return false when no suggestions")
	}
}

func TestErrorContext_Build(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setup      func() *ErrorContext
		wantNil    bool
		checkError func(t *testing.T, err *ActionableError)
	}{
		{
			name: "minimal with operation",
			setup: func() *ErrorContext {
				return NewErrorContext().WithOperation("test operation")
			},
			wantNil: false,
			checkError: func(t *testing.T, err *ActionableError) {
				t.Helper()
				if err.operation != "test operation" {
					t.Errorf("Operation = %q, want %q", err.operation, "test operation")
				}
			},
		},
		{
			name: "missing operation returns nil",
			setup: func() *ErrorContext {
				return NewErrorContext().WithResource("some/path")
			},
			wantNil: true,
		},
		{
			name: "full context",
			setup: func() *ErrorContext {
				return NewErrorContext().
					WithOperation("load config").
					WithResource("/etc/app/config.yaml").
					WithSuggestion("Check syntax").
					WithSuggestion("Verify permissions").
					Wrap(errors.New("parse error"))
			},
			wantNil: false,
			checkError: func(t *testing.T, err *ActionableError) {
				t.Helper()
				if err.operation != "load config" {
					t.Errorf("Operation = %q", err.operation)
				}
				if err.resource != "/etc/app/config.yaml" {
					t.Errorf("Resource = %q", err.resource)
				}
				if len(err.suggestions) != 2 {
					t.Errorf("Suggestions count = %d, want 2", len(err.suggestions))
				}
				if err.cause == nil || err.cause.Error() != "parse error" {
					t.Errorf("Cause = %v", err.cause)
				}
			},
		},
		{
			name: "with multiple suggestions",
			setup: func() *ErrorContext {
				return NewErrorContext().
					WithOperation("execute").
					WithSuggestions("Suggestion 1", "Suggestion 2", "Suggestion 3")
			},
			wantNil: false,
			checkError: func(t *testing.T, err *ActionableError) {
				t.Helper()
				if len(err.suggestions) != 3 {
					t.Errorf("Suggestions count = %d, want 3", len(err.suggestions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.setup()
			err := ctx.Build()

			if tt.wantNil {
				if err != nil {
					t.Errorf("Build() = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatal("Build() returned nil, want error")
			}

			if tt.checkError != nil {
				tt.checkError(t, err)
			}
		})
	}
}

func TestErrorContext_BuildError(t *testing.T) {
	t.Parallel()

	// Test that BuildError returns error interface
	ctx := NewErrorContext().WithOperation("test")
	err := ctx.BuildError()

	if err == nil {
		t.Fatal("BuildError() returned nil")
	}

	// Verify it's an *ActionableError
	if _, ok := errors.AsType[*ActionableError](err); !ok {
		t.Error("BuildError() should return *ActionableError")
	}

	// Test nil case
	ctxNil := NewErrorContext()
	errNil := ctxNil.BuildError()
	if errNil != nil {
		t.Error("BuildError() should return nil when operation missing")
	}
}

func TestNewActionableError(t *testing.T) {
	t.Parallel()

	err := NewActionableError("test operation")

	if err.operation != "test operation" {
		t.Errorf("Operation = %q", err.operation)
	}
	if err.resource != "" {
		t.Errorf("Resource should be empty, got %q", err.resource)
	}
	if err.cause != nil {
		t.Error("Cause should be nil")
	}
}

func TestWrapWithOperation(t *testing.T) {
	t.Parallel()

	cause := errors.New("original error")
	err := WrapWithOperation(cause, "process file")

	if err == nil {
		t.Fatal("WrapWithOperation returned nil")
	}

	if err.operation != "process file" {
		t.Errorf("Operation = %q", err.operation)
	}

	if !errors.Is(err.cause, cause) {
		t.Error("Cause should be the original error")
	}

	// Test nil error
	nilErr := WrapWithOperation(nil, "test")
	if nilErr != nil {
		t.Error("WrapWithOperation(nil) should return nil")
	}
}

func TestWrapWithContext(t *testing.T) {
	t.Parallel()

	cause := errors.New("original error")
	err := WrapWithContext(cause, "load file", "/path/to/file")

	if err == nil {
		t.Fatal("WrapWithContext returned nil")
	}

	if err.operation != "load file" {
		t.Errorf("Operation = %q", err.operation)
	}

	if err.resource != "/path/to/file" {
		t.Errorf("Resource = %q", err.resource)
	}

	if !errors.Is(err.cause, cause) {
		t.Error("Cause should be the original error")
	}

	// Test nil error
	nilErr := WrapWithContext(nil, "test", "resource")
	if nilErr != nil {
		t.Error("WrapWithContext(nil) should return nil")
	}
}

// Test error interface compliance
func TestActionableError_ErrorInterface(t *testing.T) {
	t.Parallel()

	var _ error = (*ActionableError)(nil)
}

// Test that ErrorContext can be reused with different causes
func TestErrorContext_Reuse(t *testing.T) {
	t.Parallel()

	ctx := NewErrorContext().
		WithOperation("process file").
		WithResource("/data/input.txt").
		WithSuggestion("Check file format")

	err1 := ctx.Wrap(errors.New("error 1")).Build()
	err2 := ctx.Wrap(errors.New("error 2")).Build()

	// Both should have the same operation/resource but different causes
	if err1.cause.Error() == err2.cause.Error() {
		t.Error("Reused context should allow different causes")
	}

	if err1.operation != err2.operation {
		t.Error("Reused context should preserve operation")
	}
}
