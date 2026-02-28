// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"testing"
)

func TestSeverity_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity Severity
		wantOK   bool
		wantErr  bool
	}{
		{SeverityWarning, true, false},
		{SeverityError, true, false},
		{"", false, true},
		{"invalid", false, true},
		{"WARNING", false, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			t.Parallel()
			err := tt.severity.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("Severity(%q).Validate() error = %v, wantOK %v", tt.severity, err, tt.wantOK)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Severity(%q).Validate() returned nil, want error", tt.severity)
				}
				if !errors.Is(err, ErrInvalidSeverity) {
					t.Errorf("error should wrap ErrInvalidSeverity, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("Severity(%q).Validate() returned unexpected error: %v", tt.severity, err)
			}
		})
	}
}

func TestDiagnosticCode_Validate(t *testing.T) {
	t.Parallel()

	validCodes := []DiagnosticCode{
		CodeWorkingDirUnavailable, CodeCommandsDirUnavailable, CodeConfigLoadFailed,
		CodeCommandNotFound, CodeInvowkfileParseSkipped, CodeModuleScanPathInvalid,
		CodeModuleScanFailed, CodeReservedModuleNameSkipped, CodeModuleLoadSkipped,
		CodeIncludeNotModule, CodeIncludeReservedSkipped, CodeIncludeModuleLoadFailed,
		CodeVendoredScanFailed, CodeVendoredReservedSkipped, CodeVendoredModuleLoadSkipped,
		CodeVendoredNestedIgnored, CodeContainerRuntimeInitFailed,
	}

	for _, code := range validCodes {
		t.Run(string(code), func(t *testing.T) {
			t.Parallel()
			err := code.Validate()
			if err != nil {
				t.Errorf("DiagnosticCode(%q).Validate() returned unexpected error: %v", code, err)
			}
		})
	}

	invalidCodes := []DiagnosticCode{"", "invalid", "WORKING_DIR_UNAVAILABLE"}
	for _, code := range invalidCodes {
		t.Run("invalid_"+string(code), func(t *testing.T) {
			t.Parallel()
			err := code.Validate()
			if err == nil {
				t.Fatalf("DiagnosticCode(%q).Validate() returned nil, want error", code)
			}
			if !errors.Is(err, ErrInvalidDiagnosticCode) {
				t.Errorf("error should wrap ErrInvalidDiagnosticCode, got: %v", err)
			}
		})
	}
}

func TestNewDiagnostic(t *testing.T) {
	t.Parallel()

	d, err := NewDiagnostic(SeverityWarning, CodeConfigLoadFailed, "test message")
	if err != nil {
		t.Fatalf("NewDiagnostic() unexpected error: %v", err)
	}

	if d.severity != SeverityWarning {
		t.Errorf("Severity = %q, want %q", d.severity, SeverityWarning)
	}
	if d.code != CodeConfigLoadFailed {
		t.Errorf("Code = %q, want %q", d.code, CodeConfigLoadFailed)
	}
	if d.message != "test message" {
		t.Errorf("Message = %q, want %q", d.message, "test message")
	}
	if d.path != "" {
		t.Errorf("Path = %q, want empty string", d.path)
	}
	if d.cause != nil {
		t.Errorf("Cause = %v, want nil", d.cause)
	}
}

func TestNewDiagnostic_InvalidParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		severity Severity
		code     DiagnosticCode
	}{
		{"invalid severity", Severity("bogus"), CodeConfigLoadFailed},
		{"invalid code", SeverityError, DiagnosticCode("bogus")},
		{"both invalid", Severity("nope"), DiagnosticCode("also_nope")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewDiagnostic(tt.severity, tt.code, "msg")
			if err == nil {
				t.Fatal("NewDiagnostic() expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidDiagnostic) {
				t.Errorf("error should wrap ErrInvalidDiagnostic, got: %v", err)
			}
		})
	}
}

func TestNewDiagnosticWithPath(t *testing.T) {
	t.Parallel()

	d, err := NewDiagnosticWithPath(SeverityError, CodeInvowkfileParseSkipped, "parse failed", "/some/path")
	if err != nil {
		t.Fatalf("NewDiagnosticWithPath() unexpected error: %v", err)
	}

	if d.severity != SeverityError {
		t.Errorf("Severity = %q, want %q", d.severity, SeverityError)
	}
	if d.code != CodeInvowkfileParseSkipped {
		t.Errorf("Code = %q, want %q", d.code, CodeInvowkfileParseSkipped)
	}
	if d.message != "parse failed" {
		t.Errorf("Message = %q, want %q", d.message, "parse failed")
	}
	if d.path != "/some/path" {
		t.Errorf("Path = %q, want %q", d.path, "/some/path")
	}
	if d.cause != nil {
		t.Errorf("Cause = %v, want nil", d.cause)
	}
}

func TestNewDiagnosticWithCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying error")
	d, err := NewDiagnosticWithCause(SeverityError, CodeModuleScanFailed, "scan failed", "/module/path", cause)
	if err != nil {
		t.Fatalf("NewDiagnosticWithCause() unexpected error: %v", err)
	}

	if d.severity != SeverityError {
		t.Errorf("Severity = %q, want %q", d.severity, SeverityError)
	}
	if d.code != CodeModuleScanFailed {
		t.Errorf("Code = %q, want %q", d.code, CodeModuleScanFailed)
	}
	if d.message != "scan failed" {
		t.Errorf("Message = %q, want %q", d.message, "scan failed")
	}
	if d.path != "/module/path" {
		t.Errorf("Path = %q, want %q", d.path, "/module/path")
	}
	if !errors.Is(d.cause, cause) {
		t.Errorf("Cause = %v, want %v", d.cause, cause)
	}
}

func TestSource_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  Source
		wantOK  bool
		wantErr bool
	}{
		{"SourceCurrentDir", SourceCurrentDir, true, false},
		{"SourceModule", SourceModule, true, false},
		{"invalid negative", Source(-1), false, true},
		{"invalid large", Source(99), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.source.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("Source(%d).Validate() error = %v, wantOK %v", tt.source, err, tt.wantOK)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Source(%d).Validate() returned nil, want error", tt.source)
				}
				if !errors.Is(err, ErrInvalidSource) {
					t.Errorf("error should wrap ErrInvalidSource, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("Source(%d).Validate() returned unexpected error: %v", tt.source, err)
			}
		})
	}
}

func TestDiagnostic_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		diag          Diagnostic
		wantOK        bool
		wantErr       bool
		wantFieldErrs int
	}{
		{
			name: "valid diagnostic",
			diag: Diagnostic{
				severity: SeverityWarning,
				code:     CodeConfigLoadFailed,
				message:  "test message",
			},
			wantOK: true,
		},
		{
			name: "invalid severity",
			diag: Diagnostic{
				severity: Severity("bogus"),
				code:     CodeConfigLoadFailed,
				message:  "test message",
			},
			wantOK:        false,
			wantErr:       true,
			wantFieldErrs: 1,
		},
		{
			name: "invalid code",
			diag: Diagnostic{
				severity: SeverityError,
				code:     DiagnosticCode("bogus_code"),
				message:  "test message",
			},
			wantOK:        false,
			wantErr:       true,
			wantFieldErrs: 1,
		},
		{
			name: "both invalid",
			diag: Diagnostic{
				severity: Severity("nope"),
				code:     DiagnosticCode("also_nope"),
				message:  "test message",
			},
			wantOK:        false,
			wantErr:       true,
			wantFieldErrs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.diag.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("Diagnostic.Validate() error = %v, wantOK %v", err, tt.wantOK)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Diagnostic.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidDiagnostic) {
					t.Errorf("error should wrap ErrInvalidDiagnostic, got: %v", err)
				}
				var diagErr *InvalidDiagnosticError
				if !errors.As(err, &diagErr) {
					t.Fatalf("error should be *InvalidDiagnosticError, got: %T", err)
				}
				if len(diagErr.FieldErrors) != tt.wantFieldErrs {
					t.Errorf("InvalidDiagnosticError.FieldErrors = %d, want %d", len(diagErr.FieldErrors), tt.wantFieldErrs)
				}
			} else if err != nil {
				t.Errorf("Diagnostic.Validate() returned unexpected error: %v", err)
			}
		})
	}
}

func TestSeverity_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		severity Severity
		want     string
	}{
		{"warning", SeverityWarning, "warning"},
		{"error", SeverityError, "error"},
		{"empty", Severity(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.severity.String(); got != tt.want {
				t.Errorf("Severity(%q).String() = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

func TestSource_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source Source
		want   string
	}{
		{"current directory", SourceCurrentDir, "current directory"},
		{"module", SourceModule, "module"},
		{"unknown (out of range)", Source(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.source.String(); got != tt.want {
				t.Errorf("Source(%d).String() = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestDiagnosticCode_String(t *testing.T) {
	t.Parallel()

	if got := CodeCommandNotFound.String(); got != "command_not_found" {
		t.Errorf("CodeCommandNotFound.String() = %q, want %q", got, "command_not_found")
	}
	if got := DiagnosticCode("").String(); got != "" {
		t.Errorf("DiagnosticCode(\"\").String() = %q, want %q", got, "")
	}
}
