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

func TestListenPort_Validate(t *testing.T) {
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
			err := tt.port.Validate()
			if (err == nil) != tt.want {
				t.Errorf("ListenPort(%d).Validate() error = %v, wantValid %v", tt.port, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ListenPort(%d).Validate() returned nil, want error", tt.port)
				}
				if !errors.Is(err, ErrInvalidListenPort) {
					t.Errorf("error should wrap ErrInvalidListenPort, got: %v", err)
				}
				var lpErr *InvalidListenPortError
				if !errors.As(err, &lpErr) {
					t.Errorf("error should be *InvalidListenPortError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("ListenPort(%d).Validate() returned unexpected error: %v", tt.port, err)
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
