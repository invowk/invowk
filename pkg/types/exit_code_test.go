// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestExitCodeValidate(t *testing.T) {
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

			err := tt.value.Validate()
			if (err == nil) != tt.wantValid {
				t.Errorf("ExitCode(%d).Validate() error = %v, wantValid %v", tt.value, err, tt.wantValid)
			}
			if tt.wantValid {
				if err != nil {
					t.Errorf("ExitCode(%d).Validate() returned error for valid value: %v", tt.value, err)
				}
			} else {
				if err == nil {
					t.Error("ExitCode.Validate() returned nil for invalid value")
				}
				if !errors.Is(err, ErrInvalidExitCode) {
					t.Errorf("error does not wrap ErrInvalidExitCode: %v", err)
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
