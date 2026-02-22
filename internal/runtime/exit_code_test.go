// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"
)

func TestExitCodeIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     ExitCode
		wantValid bool
	}{
		{name: "zero is valid", value: 0, wantValid: true},
		{name: "one is valid", value: 1, wantValid: true},
		{name: "125 is valid", value: 125, wantValid: true},
		{name: "126 is valid", value: 126, wantValid: true},
		{name: "255 is valid", value: 255, wantValid: true},
		{name: "negative is invalid", value: -1, wantValid: false},
		{name: "256 is invalid", value: 256, wantValid: false},
		{name: "large positive is invalid", value: 1000, wantValid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			isValid, errs := tt.value.IsValid()
			if isValid != tt.wantValid {
				t.Errorf("ExitCode(%d).IsValid() = %v, want %v", tt.value, isValid, tt.wantValid)
			}
			if tt.wantValid {
				if len(errs) != 0 {
					t.Errorf("ExitCode(%d).IsValid() returned errors for valid value: %v", tt.value, errs)
				}
			} else {
				if len(errs) == 0 {
					t.Error("ExitCode.IsValid() returned no errors for invalid value")
				}
				if !errors.Is(errs[0], ErrInvalidExitCode) {
					t.Errorf("error does not wrap ErrInvalidExitCode: %v", errs[0])
				}
			}
		})
	}
}

func TestExitCodeIsSuccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code ExitCode
		want bool
	}{
		{0, true},
		{1, false},
		{125, false},
		{255, false},
	}

	for _, tt := range tests {
		if got := tt.code.IsSuccess(); got != tt.want {
			t.Errorf("ExitCode(%d).IsSuccess() = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestExitCodeIsTransient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code ExitCode
		want bool
	}{
		{0, false},
		{1, false},
		{124, false},
		{125, true},
		{126, true},
		{127, false},
		{255, false},
	}

	for _, tt := range tests {
		if got := tt.code.IsTransient(); got != tt.want {
			t.Errorf("ExitCode(%d).IsTransient() = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestExitCodeString(t *testing.T) {
	t.Parallel()

	if got := ExitCode(42).String(); got != "42" {
		t.Errorf("ExitCode(42).String() = %q, want %q", got, "42")
	}
}
