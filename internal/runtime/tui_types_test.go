// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"testing"
)

func TestTUIServerURL_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     TUIServerURL
		want    bool
		wantErr bool
	}{
		{"http url", TUIServerURL("http://localhost:8080"), true, false},
		{"https url", TUIServerURL("https://example.com/tui"), true, false},
		{"empty is valid (zero value)", TUIServerURL(""), true, false},
		{"no scheme is invalid", TUIServerURL("localhost:8080"), false, true},
		{"ftp scheme is invalid", TUIServerURL("ftp://server"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.url.IsValid()
			if isValid != tt.want {
				t.Errorf("TUIServerURL(%q).IsValid() = %v, want %v", tt.url, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("TUIServerURL(%q).IsValid() returned no errors, want error", tt.url)
				}
				if !errors.Is(errs[0], ErrInvalidTUIServerURL) {
					t.Errorf("error should wrap ErrInvalidTUIServerURL, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("TUIServerURL(%q).IsValid() returned unexpected errors: %v", tt.url, errs)
			}
		})
	}
}

func TestTUIServerToken_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   TUIServerToken
		want    bool
		wantErr bool
	}{
		{"valid token", TUIServerToken("abc123"), true, false},
		{"uuid token", TUIServerToken("550e8400-e29b-41d4-a716-446655440000"), true, false},
		{"empty is valid (zero value)", TUIServerToken(""), true, false},
		{"whitespace only is invalid", TUIServerToken("   "), false, true},
		{"tab only is invalid", TUIServerToken("\t"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			isValid, errs := tt.token.IsValid()
			if isValid != tt.want {
				t.Errorf("TUIServerToken(%q).IsValid() = %v, want %v", tt.token, isValid, tt.want)
			}
			if tt.wantErr {
				if len(errs) == 0 {
					t.Fatalf("TUIServerToken(%q).IsValid() returned no errors, want error", tt.token)
				}
				if !errors.Is(errs[0], ErrInvalidTUIServerToken) {
					t.Errorf("error should wrap ErrInvalidTUIServerToken, got: %v", errs[0])
				}
			} else if len(errs) > 0 {
				t.Errorf("TUIServerToken(%q).IsValid() returned unexpected errors: %v", tt.token, errs)
			}
		})
	}
}
