// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"errors"
	"testing"
)

func TestSeverity_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity Severity
		want     bool
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
			isValid, errs := tt.severity.IsValid()
			if isValid != tt.want {
				t.Errorf("Severity(%q).IsValid() = %v, want %v", tt.severity, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Severity(%q).IsValid() returned no errors, want error", tt.severity)
				}
				if !errors.Is(errs[0], ErrInvalidSeverity) {
					t.Errorf("error should wrap ErrInvalidSeverity, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("Severity(%q).IsValid() returned unexpected errors: %v", tt.severity, errs)
			}
		})
	}
}

func TestDiagnosticCode_IsValid(t *testing.T) {
	t.Parallel()

	validCodes := []DiagnosticCode{
		CodeWorkingDirUnavailable, CodeCommandsDirUnavailable, CodeConfigLoadFailed,
		CodeCommandNotFound, CodeInvowkfileParseSkipped, CodeModuleScanPathInvalid,
		CodeModuleScanFailed, CodeReservedModuleNameSkipped, CodeModuleLoadSkipped,
		CodeIncludeNotModule, CodeIncludeReservedSkipped, CodeIncludeModuleLoadFailed,
		CodeVendoredScanFailed, CodeVendoredReservedSkipped, CodeVendoredModuleLoadSkipped,
		CodeVendoredNestedIgnored,
	}

	for _, code := range validCodes {
		t.Run(string(code), func(t *testing.T) {
			t.Parallel()
			isValid, errs := code.IsValid()
			if !isValid {
				t.Errorf("DiagnosticCode(%q).IsValid() = false, want true", code)
			}
			if len(errs) > 0 {
				t.Errorf("DiagnosticCode(%q).IsValid() returned unexpected errors: %v", code, errs)
			}
		})
	}

	invalidCodes := []DiagnosticCode{"", "invalid", "WORKING_DIR_UNAVAILABLE"}
	for _, code := range invalidCodes {
		t.Run("invalid_"+string(code), func(t *testing.T) {
			t.Parallel()
			isValid, errs := code.IsValid()
			if isValid {
				t.Errorf("DiagnosticCode(%q).IsValid() = true, want false", code)
			}
			if len(errs) == 0 {
				t.Fatalf("DiagnosticCode(%q).IsValid() returned no errors, want error", code)
			}
			if !errors.Is(errs[0], ErrInvalidDiagnosticCode) {
				t.Errorf("error should wrap ErrInvalidDiagnosticCode, got: %v", errs[0])
			}
		})
	}
}

func TestNewDiagnostic(t *testing.T) {
	t.Parallel()

	d := NewDiagnostic(SeverityWarning, CodeConfigLoadFailed, "test message")

	if d.Severity != SeverityWarning {
		t.Errorf("Severity = %q, want %q", d.Severity, SeverityWarning)
	}
	if d.Code != CodeConfigLoadFailed {
		t.Errorf("Code = %q, want %q", d.Code, CodeConfigLoadFailed)
	}
	if d.Message != "test message" {
		t.Errorf("Message = %q, want %q", d.Message, "test message")
	}
	if d.Path != "" {
		t.Errorf("Path = %q, want empty string", d.Path)
	}
	if d.Cause != nil {
		t.Errorf("Cause = %v, want nil", d.Cause)
	}
}

func TestNewDiagnosticWithPath(t *testing.T) {
	t.Parallel()

	d := NewDiagnosticWithPath(SeverityError, CodeInvowkfileParseSkipped, "parse failed", "/some/path")

	if d.Severity != SeverityError {
		t.Errorf("Severity = %q, want %q", d.Severity, SeverityError)
	}
	if d.Code != CodeInvowkfileParseSkipped {
		t.Errorf("Code = %q, want %q", d.Code, CodeInvowkfileParseSkipped)
	}
	if d.Message != "parse failed" {
		t.Errorf("Message = %q, want %q", d.Message, "parse failed")
	}
	if d.Path != "/some/path" {
		t.Errorf("Path = %q, want %q", d.Path, "/some/path")
	}
	if d.Cause != nil {
		t.Errorf("Cause = %v, want nil", d.Cause)
	}
}

func TestNewDiagnosticWithCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("underlying error")
	d := NewDiagnosticWithCause(SeverityError, CodeModuleScanFailed, "scan failed", "/module/path", cause)

	if d.Severity != SeverityError {
		t.Errorf("Severity = %q, want %q", d.Severity, SeverityError)
	}
	if d.Code != CodeModuleScanFailed {
		t.Errorf("Code = %q, want %q", d.Code, CodeModuleScanFailed)
	}
	if d.Message != "scan failed" {
		t.Errorf("Message = %q, want %q", d.Message, "scan failed")
	}
	if d.Path != "/module/path" {
		t.Errorf("Path = %q, want %q", d.Path, "/module/path")
	}
	if !errors.Is(d.Cause, cause) {
		t.Errorf("Cause = %v, want %v", d.Cause, cause)
	}
}

func TestSource_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		source  Source
		want    bool
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

			isValid, errs := tt.source.IsValid()
			if isValid != tt.want {
				t.Errorf("Source(%d).IsValid() = %v, want %v", tt.source, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("Source(%d).IsValid() returned no errors, want error", tt.source)
				}
				if !errors.Is(errs[0], ErrInvalidSource) {
					t.Errorf("error should wrap ErrInvalidSource, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("Source(%d).IsValid() returned unexpected errors: %v", tt.source, errs)
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
