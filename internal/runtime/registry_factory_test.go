// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"
)

func TestInitDiagnosticCode_Validate(t *testing.T) {
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
			err := tt.code.Validate()
			if (err == nil) != tt.want {
				t.Errorf("InitDiagnosticCode(%q).Validate() valid = %v, want %v", tt.code, err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("InitDiagnosticCode(%q).Validate() returned nil, want error", tt.code)
				}
				if !errors.Is(err, ErrInvalidInitDiagnosticCode) {
					t.Errorf("error should wrap ErrInvalidInitDiagnosticCode, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("InitDiagnosticCode(%q).Validate() returned unexpected error: %v", tt.code, err)
			}
		})
	}
}

func TestInitDiagnosticCode_String(t *testing.T) {
	t.Parallel()

	if got := CodeContainerRuntimeInitFailed.String(); got != "container_runtime_init_failed" {
		t.Errorf("CodeContainerRuntimeInitFailed.String() = %q, want %q", got, "container_runtime_init_failed")
	}
	if got := InitDiagnosticCode("").String(); got != "" {
		t.Errorf("InitDiagnosticCode(\"\").String() = %q, want %q", got, "")
	}
}
