// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"
)

func TestListenPort_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		port ListenPort
		want string
	}{
		{0, "0"},
		{80, "80"},
		{443, "443"},
		{8080, "8080"},
		{65535, "65535"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.port.String()
			if got != tt.want {
				t.Errorf("ListenPort(%d).String() = %q, want %q", tt.port, got, tt.want)
			}
		})
	}
}

func TestListenPort_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		port    ListenPort
		want    bool
		wantErr bool
	}{
		{0, true, false},
		{1, true, false},
		{80, true, false},
		{443, true, false},
		{8080, true, false},
		{65535, true, false},
		{-1, false, true},
		{65536, false, true},
		{-100, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.port.String(), func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.port.IsValid()
			if isValid != tt.want {
				t.Errorf("ListenPort(%d).IsValid() = %v, want %v", tt.port, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("ListenPort(%d).IsValid() returned no errors, want error", tt.port)
				}
				if !errors.Is(errs[0], ErrInvalidListenPort) {
					t.Errorf("error should wrap ErrInvalidListenPort, got: %v", errs[0])
				}
				var lpErr *InvalidListenPortError
				if !errors.As(errs[0], &lpErr) {
					t.Errorf("error should be *InvalidListenPortError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("ListenPort(%d).IsValid() returned unexpected errors: %v", tt.port, errs)
			}
		})
	}
}

func TestInvalidListenPortError(t *testing.T) {
	t.Parallel()

	err := &InvalidListenPortError{Value: -5}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidListenPort) {
		t.Error("expected error to wrap ErrInvalidListenPort")
	}
}
