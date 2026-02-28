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

func TestTerminalDimension_Validate(t *testing.T) {
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
			err := tt.d.Validate()
			if (err == nil) != tt.want {
				t.Errorf("TerminalDimension(%d).Validate() err = %v, wantValid %v", tt.d, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TerminalDimension(%d).Validate() returned nil, want error", tt.d)
				}
				if !errors.Is(err, ErrInvalidTerminalDimension) {
					t.Errorf("error should wrap ErrInvalidTerminalDimension, got: %v", err)
				}
				var tdErr *InvalidTerminalDimensionError
				if !errors.As(err, &tdErr) {
					t.Errorf("error should be *InvalidTerminalDimensionError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("TerminalDimension(%d).Validate() returned unexpected error: %v", tt.d, err)
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
