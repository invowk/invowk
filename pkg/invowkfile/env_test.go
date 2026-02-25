// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
)

func TestEnvVarName_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		n       EnvVarName
		want    bool
		wantErr bool
	}{
		{"valid simple", "HOME", true, false},
		{"valid with underscore", "MY_VAR", true, false},
		{"valid leading underscore", "_PRIVATE", true, false},
		{"valid alphanumeric", "VAR123", true, false},
		{"valid single char", "X", true, false},
		{"invalid empty", "", false, true},
		{"invalid whitespace only", "   ", false, true},
		{"invalid starts with number", "1VAR", false, true},
		{"invalid hyphen", "MY-VAR", false, true},
		{"invalid dot", "MY.VAR", false, true},
		{"invalid space in name", "MY VAR", false, true},
		{"invalid special char", "MY$VAR", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.n.IsValid()
			if isValid != tt.want {
				t.Errorf("EnvVarName(%q).IsValid() = %v, want %v", tt.n, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("EnvVarName(%q).IsValid() returned no errors, want error", tt.n)
				}
				if !errors.Is(errs[0], ErrInvalidEnvVarName) {
					t.Errorf("error should wrap ErrInvalidEnvVarName, got: %v", errs[0])
				}
				var typedErr *InvalidEnvVarNameError
				if !errors.As(errs[0], &typedErr) {
					t.Errorf("error should be *InvalidEnvVarNameError, got: %T", errs[0])
				} else if typedErr.Value != tt.n {
					t.Errorf("InvalidEnvVarNameError.Value = %q, want %q", typedErr.Value, tt.n)
				}
			} else if len(errs) > 0 {
				t.Errorf("EnvVarName(%q).IsValid() returned unexpected errors: %v", tt.n, errs)
			}
		})
	}
}

func TestEnvVarName_String(t *testing.T) {
	t.Parallel()

	if got := EnvVarName("HOME").String(); got != "HOME" {
		t.Errorf("EnvVarName(\"HOME\").String() = %q, want %q", got, "HOME")
	}
}

func TestValidateEnvVarName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "HOME", false},
		{"valid with underscore", "MY_VAR", false},
		{"valid leading underscore", "_PRIVATE", false},
		{"valid alphanumeric", "VAR123", false},
		{"valid single char", "X", false},
		{"invalid empty", "", true},
		{"invalid whitespace only", "   ", true},
		{"invalid starts with number", "1VAR", true},
		{"invalid hyphen", "MY-VAR", true},
		{"invalid dot", "MY.VAR", true},
		{"invalid space in name", "MY VAR", true},
		{"invalid special char", "MY$VAR", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEnvVarName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEnvVarName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
