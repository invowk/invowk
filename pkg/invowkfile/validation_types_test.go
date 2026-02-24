// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      ValidationError
		expected string
	}{
		{
			name: "error with field",
			err: ValidationError{
				Validator: "structure",
				Field:     "command 'build' implementation #1",
				Message:   "must have a script",
				Severity:  SeverityError,
			},
			expected: "command 'build' implementation #1: must have a script",
		},
		{
			name: "error without field",
			err: ValidationError{
				Validator: "structure",
				Field:     "",
				Message:   "invowkfile has no commands",
				Severity:  SeverityError,
			},
			expected: "invowkfile has no commands",
		},
		{
			name: "warning with field",
			err: ValidationError{
				Validator: "structure",
				Field:     "root",
				Message:   "deprecated field used",
				Severity:  SeverityWarning,
			},
			expected: "root: deprecated field used",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidationError_IsError(t *testing.T) {
	t.Parallel()

	errorErr := ValidationError{Severity: SeverityError}
	warningErr := ValidationError{Severity: SeverityWarning}

	if !errorErr.IsError() {
		t.Error("SeverityError.IsError() should be true")
	}
	if errorErr.IsWarning() {
		t.Error("SeverityError.IsWarning() should be false")
	}
	if warningErr.IsError() {
		t.Error("SeverityWarning.IsError() should be false")
	}
	if !warningErr.IsWarning() {
		t.Error("SeverityWarning.IsWarning() should be true")
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errs     ValidationErrors
		contains []string
	}{
		{
			name:     "empty errors",
			errs:     ValidationErrors{},
			contains: []string{""}, // empty string
		},
		{
			name: "single error",
			errs: ValidationErrors{
				{Field: "field1", Message: "error message", Severity: SeverityError},
			},
			contains: []string{"field1: error message"},
		},
		{
			name: "multiple errors",
			errs: ValidationErrors{
				{Field: "field1", Message: "first error", Severity: SeverityError},
				{Field: "field2", Message: "second error", Severity: SeverityError},
			},
			contains: []string{"2 errors", "first error", "second error"},
		},
		{
			name: "mixed errors and warnings",
			errs: ValidationErrors{
				{Field: "field1", Message: "error", Severity: SeverityError},
				{Field: "field2", Message: "warning", Severity: SeverityWarning},
			},
			contains: []string{"1 error", "1 warning"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.errs.Error()
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("Error() = %q, should contain %q", result, want)
				}
			}
		})
	}
}

func TestValidationErrors_HasErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		errs     ValidationErrors
		expected bool
	}{
		{
			name:     "empty",
			errs:     ValidationErrors{},
			expected: false,
		},
		{
			name: "only warnings",
			errs: ValidationErrors{
				{Severity: SeverityWarning},
			},
			expected: false,
		},
		{
			name: "has error",
			errs: ValidationErrors{
				{Severity: SeverityError},
			},
			expected: true,
		},
		{
			name: "mixed",
			errs: ValidationErrors{
				{Severity: SeverityWarning},
				{Severity: SeverityError},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.errs.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationErrors_ErrorCount(t *testing.T) {
	t.Parallel()

	errs := ValidationErrors{
		{Severity: SeverityError},
		{Severity: SeverityWarning},
		{Severity: SeverityError},
		{Severity: SeverityWarning},
		{Severity: SeverityError},
	}

	if got := errs.ErrorCount(); got != 3 {
		t.Errorf("ErrorCount() = %d, want 3", got)
	}
	if got := errs.WarningCount(); got != 2 {
		t.Errorf("WarningCount() = %d, want 2", got)
	}
}

func TestValidationErrors_Filter(t *testing.T) {
	t.Parallel()

	errs := ValidationErrors{
		{Message: "error1", Severity: SeverityError},
		{Message: "warning1", Severity: SeverityWarning},
		{Message: "error2", Severity: SeverityError},
	}

	filteredErrs := errs.Errors()
	if len(filteredErrs) != 2 {
		t.Errorf("Errors() returned %d items, want 2", len(filteredErrs))
	}
	for _, e := range filteredErrs {
		if e.Severity != SeverityError {
			t.Errorf("Errors() should only return errors, got warning")
		}
	}

	warnings := errs.Warnings()
	if len(warnings) != 1 {
		t.Errorf("Warnings() returned %d items, want 1", len(warnings))
	}
	for _, w := range warnings {
		if w.Severity != SeverityWarning {
			t.Errorf("Warnings() should only return warnings, got error")
		}
	}
}

func TestValidationSeverity_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity ValidationSeverity
		expected string
	}{
		{SeverityError, "error"},
		{SeverityWarning, "warning"},
		{ValidationSeverity(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			if got := tt.severity.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFieldPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		build    func() *FieldPath
		expected string
	}{
		{
			name:     "empty path",
			build:    NewFieldPath,
			expected: "",
		},
		{
			name:     "root",
			build:    func() *FieldPath { return NewFieldPath().Root() },
			expected: "root",
		},
		{
			name:     "command",
			build:    func() *FieldPath { return NewFieldPath().Command("build") },
			expected: "command 'build'",
		},
		{
			name: "command implementation",
			build: func() *FieldPath {
				return NewFieldPath().Command("build").Implementation(0)
			},
			expected: "command 'build' implementation #1",
		},
		{
			name: "command implementation runtime",
			build: func() *FieldPath {
				return NewFieldPath().Command("build").Implementation(0).Runtime(1)
			},
			expected: "command 'build' implementation #1 runtime #2",
		},
		{
			name: "command flag",
			build: func() *FieldPath {
				return NewFieldPath().Command("build").Flag("verbose")
			},
			expected: "command 'build' flag 'verbose'",
		},
		{
			name: "command arg",
			build: func() *FieldPath {
				return NewFieldPath().Command("build").Arg("target")
			},
			expected: "command 'build' argument 'target'",
		},
		{
			name: "root env file",
			build: func() *FieldPath {
				return NewFieldPath().Root().EnvFile(0)
			},
			expected: "root env.files[1]",
		},
		{
			name: "root env var",
			build: func() *FieldPath {
				return NewFieldPath().Root().EnvVar("PATH")
			},
			expected: "root env.vars['PATH']",
		},
		{
			name: "command volume",
			build: func() *FieldPath {
				return NewFieldPath().Command("build").Implementation(0).Runtime(0).Volume(2)
			},
			expected: "command 'build' implementation #1 runtime #1 volume #3",
		},
		{
			name: "custom check",
			build: func() *FieldPath {
				return NewFieldPath().Command("build").CustomCheck(0, 1)
			},
			expected: "command 'build' custom_check #1 alternative #2",
		},
		{
			name: "tools dependency",
			build: func() *FieldPath {
				return NewFieldPath().Root().Tools(0, 0)
			},
			expected: "root tools[1].alternatives[1]",
		},
		{
			name: "env vars dependency",
			build: func() *FieldPath {
				return NewFieldPath().Command("test").EnvVars(1, 0).Field("validation")
			},
			expected: "command 'test' env_vars[2].alternatives[1] validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.build().String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFieldPath_Copy(t *testing.T) {
	t.Parallel()

	original := NewFieldPath().Command("build").Implementation(0)
	copied := original.Copy()

	// Modify the copied path
	copied.Runtime(0)

	// Original should be unchanged
	if original.String() != "command 'build' implementation #1" {
		t.Errorf("original was modified: %q", original.String())
	}
	if copied.String() != "command 'build' implementation #1 runtime #1" {
		t.Errorf("copied has unexpected value: %q", copied.String())
	}
}

func TestValidatorName_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   ValidatorName
		want    bool
		wantErr bool
	}{
		{"valid name", "structure", true, false},
		{"empty string", "", false, true},
		{"space only", " ", false, true},
		{"tab only", "\t", false, true},
		{"mixed whitespace", " \t\n ", false, true},
		{"name with spaces", "my validator", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.value.IsValid()
			if isValid != tt.want {
				t.Errorf("ValidatorName(%q).IsValid() = %v, want %v", tt.value, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ValidatorName(%q).IsValid() returned no errors, want error", tt.value)
				}
				if !errors.Is(errs[0], ErrInvalidValidatorName) {
					t.Errorf("error should wrap ErrInvalidValidatorName, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ValidatorName(%q).IsValid() returned unexpected errors: %v", tt.value, errs)
			}
		})
	}
}

func TestValidatorName_String(t *testing.T) {
	t.Parallel()

	n := ValidatorName("structure")
	if n.String() != "structure" {
		t.Errorf("String() = %q, want %q", n.String(), "structure")
	}
}

func TestNewValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validator ValidatorName
		field     string
		message   string
		severity  ValidationSeverity
		wantOK    bool
		wantErrs  int
	}{
		{
			name:      "valid construction",
			validator: "structure",
			field:     "command 'build'",
			message:   "must have a script",
			severity:  SeverityError,
			wantOK:    true,
			wantErrs:  0,
		},
		{
			name:      "invalid validator name",
			validator: "",
			field:     "field",
			message:   "msg",
			severity:  SeverityWarning,
			wantOK:    false,
			wantErrs:  1,
		},
		{
			name:      "invalid severity",
			validator: "structure",
			field:     "field",
			message:   "msg",
			severity:  ValidationSeverity(99),
			wantOK:    false,
			wantErrs:  1,
		},
		{
			name:      "both invalid",
			validator: "  ",
			field:     "field",
			message:   "msg",
			severity:  ValidationSeverity(-1),
			wantOK:    false,
			wantErrs:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ve, errs := NewValidationError(tt.validator, tt.field, tt.message, tt.severity)
			if tt.wantOK {
				if len(errs) != 0 {
					t.Fatalf("NewValidationError() returned errors: %v", errs)
				}
				if ve.Validator != tt.validator {
					t.Errorf("Validator = %q, want %q", ve.Validator, tt.validator)
				}
				if ve.Field != tt.field {
					t.Errorf("Field = %q, want %q", ve.Field, tt.field)
				}
				if ve.Message != tt.message {
					t.Errorf("Message = %q, want %q", ve.Message, tt.message)
				}
				if ve.Severity != tt.severity {
					t.Errorf("Severity = %d, want %d", ve.Severity, tt.severity)
				}
			} else {
				if len(errs) != tt.wantErrs {
					t.Errorf("NewValidationError() returned %d errors, want %d: %v", len(errs), tt.wantErrs, errs)
				}
				if ve != (ValidationError{}) {
					t.Errorf("NewValidationError() returned non-zero ValidationError on failure: %+v", ve)
				}
			}
		})
	}
}

func TestNewValidationError_ErrorTypes(t *testing.T) {
	t.Parallel()

	// Verify invalid validator wraps correct sentinel
	_, errs := NewValidationError("", "field", "msg", SeverityError)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !errors.Is(errs[0], ErrInvalidValidatorName) {
		t.Errorf("error should wrap ErrInvalidValidatorName, got: %v", errs[0])
	}

	// Verify invalid severity wraps correct sentinel
	_, errs = NewValidationError("structure", "field", "msg", ValidationSeverity(99))
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !errors.Is(errs[0], ErrInvalidValidationSeverity) {
		t.Errorf("error should wrap ErrInvalidValidationSeverity, got: %v", errs[0])
	}
}

func TestValidationSeverity_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity ValidationSeverity
		want     bool
		wantErr  bool
	}{
		{SeverityError, true, false},
		{SeverityWarning, true, false},
		{ValidationSeverity(99), false, true},
		{ValidationSeverity(-1), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.severity.String(), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.severity.IsValid()
			if isValid != tt.want {
				t.Errorf("ValidationSeverity(%d).IsValid() = %v, want %v", tt.severity, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ValidationSeverity(%d).IsValid() returned no errors, want error", tt.severity)
				}
				if !errors.Is(errs[0], ErrInvalidValidationSeverity) {
					t.Errorf("error should wrap ErrInvalidValidationSeverity, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ValidationSeverity(%d).IsValid() returned unexpected errors: %v", tt.severity, errs)
			}
		})
	}
}
