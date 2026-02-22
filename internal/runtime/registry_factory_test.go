// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"
)

func TestInitDiagnosticCode_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    InitDiagnosticCode
		want    bool
		wantErr bool
	}{
		{"container_runtime_init_failed", CodeContainerRuntimeInitFailed, true, false},
		{"empty", InitDiagnosticCode(""), false, true},
		{"unknown", InitDiagnosticCode("unknown_code"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.code.IsValid()
			if isValid != tt.want {
				t.Errorf("InitDiagnosticCode(%q).IsValid() = %v, want %v", tt.code, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("InitDiagnosticCode(%q).IsValid() returned no errors, want error", tt.code)
				}
				if !errors.Is(errs[0], ErrInvalidInitDiagnosticCode) {
					t.Errorf("error should wrap ErrInvalidInitDiagnosticCode, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("InitDiagnosticCode(%q).IsValid() returned unexpected errors: %v", tt.code, errs)
			}
		})
	}
}
