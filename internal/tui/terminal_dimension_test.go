// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"errors"
	"testing"
)

func TestTerminalDimension_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d    TerminalDimension
		want string
	}{
		{0, "0"},
		{80, "80"},
		{120, "120"},
		{-1, "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.d.String()
			if got != tt.want {
				t.Errorf("TerminalDimension(%d).String() = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestTerminalDimension_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		d       TerminalDimension
		want    bool
		wantErr bool
	}{
		{0, true, false},
		{1, true, false},
		{80, true, false},
		{65535, true, false},
		{-1, false, true},
		{-100, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.d.IsValid()
			if isValid != tt.want {
				t.Errorf("TerminalDimension(%d).IsValid() = %v, want %v", tt.d, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("TerminalDimension(%d).IsValid() returned no errors, want error", tt.d)
				}
				if !errors.Is(errs[0], ErrInvalidTerminalDimension) {
					t.Errorf("error should wrap ErrInvalidTerminalDimension, got: %v", errs[0])
				}
				var tdErr *InvalidTerminalDimensionError
				if !errors.As(errs[0], &tdErr) {
					t.Errorf("error should be *InvalidTerminalDimensionError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("TerminalDimension(%d).IsValid() returned unexpected errors: %v", tt.d, errs)
			}
		})
	}
}

func TestInvalidTerminalDimensionError(t *testing.T) {
	t.Parallel()

	err := &InvalidTerminalDimensionError{Value: -5}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidTerminalDimension) {
		t.Error("expected error to wrap ErrInvalidTerminalDimension")
	}
}
