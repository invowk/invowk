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
