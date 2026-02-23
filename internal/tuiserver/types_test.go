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

func TestAuthToken_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   AuthToken
		want    bool
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
			isValid, errs := tt.token.IsValid()
			if isValid != tt.want {
				t.Errorf("AuthToken(%q).IsValid() = %v, want %v", tt.token, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("AuthToken(%q).IsValid() returned no errors, want error", tt.token)
				}
				if !errors.Is(errs[0], ErrInvalidAuthToken) {
					t.Errorf("error should wrap ErrInvalidAuthToken, got: %v", errs[0])
				}
				var atErr *InvalidAuthTokenError
				if !errors.As(errs[0], &atErr) {
					t.Errorf("error should be *InvalidAuthTokenError, got: %T", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("AuthToken(%q).IsValid() returned unexpected errors: %v", tt.token, errs)
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
