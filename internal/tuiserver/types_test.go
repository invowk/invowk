// SPDX-License-Identifier: MPL-2.0

package tuiserver

import (
	"errors"
	"testing"
)

func TestAuthToken_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		token AuthToken
		want  string
	}{
		{AuthToken("abc123"), "abc123"},
		{AuthToken("deadbeef"), "deadbeef"},
		{AuthToken(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.token.String()
			if got != tt.want {
				t.Errorf("AuthToken(%q).String() = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestAuthToken_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   AuthToken
		wantOK  bool
		wantErr bool
	}{
		{"valid_hex", AuthToken("abc123def456"), true, false},
		{"valid_long", AuthToken("0123456789abcdef0123456789abcdef"), true, false},
		{"empty", AuthToken(""), false, true},
		{"whitespace_only", AuthToken("   "), false, true},
		{"tab_only", AuthToken("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.token.Validate()
			if (err == nil) != tt.wantOK {
				t.Errorf("AuthToken(%q).Validate() error = %v, wantOK %v", tt.token, err, tt.wantOK)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("AuthToken(%q).Validate() returned nil, want error", tt.token)
				}
				if !errors.Is(err, ErrInvalidAuthToken) {
					t.Errorf("error should wrap ErrInvalidAuthToken, got: %v", err)
				}
				var atErr *InvalidAuthTokenError
				if !errors.As(err, &atErr) {
					t.Errorf("error should be *InvalidAuthTokenError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("AuthToken(%q).Validate() returned unexpected error: %v", tt.token, err)
			}
		})
	}
}

func TestInvalidAuthTokenError(t *testing.T) {
	t.Parallel()

	err := &InvalidAuthTokenError{Value: ""}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
	if !errors.Is(err, ErrInvalidAuthToken) {
		t.Error("expected error to wrap ErrInvalidAuthToken")
	}
}
