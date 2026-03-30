// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"strings"
	"testing"
)

func TestTUIServerURL_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     TUIServerURL
		want    bool
		wantErr bool
	}{
		{"empty is valid (zero value)", "", true, false},
		{"http URL is valid", "http://localhost:8080", true, false},
		{"https URL is valid", "https://example.com", true, false},
		{"http with path is valid", "http://host:9000/api", true, false},
		{"ftp scheme is invalid", "ftp://example.com", false, true},
		{"no scheme is invalid", "localhost:8080", false, true},
		{"ws scheme is invalid", "ws://example.com", false, true},
		{"whitespace is invalid", " ", false, true},
		{"bare word is invalid", "notaurl", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.url.Validate()
			if (err == nil) != tt.want {
				t.Errorf("TUIServerURL(%q).Validate() error = %v, wantValid %v", tt.url, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TUIServerURL(%q).Validate() returned nil, want error", tt.url)
				}
				if !errors.Is(err, ErrInvalidTUIServerURL) {
					t.Errorf("error should wrap ErrInvalidTUIServerURL, got: %v", err)
				}
				var urlErr *InvalidTUIServerURLError
				if !errors.As(err, &urlErr) {
					t.Fatalf("error should be *InvalidTUIServerURLError, got: %T", err)
				}
				if urlErr.Value != tt.url {
					t.Errorf("InvalidTUIServerURLError.Value = %q, want %q", urlErr.Value, tt.url)
				}
			} else if err != nil {
				t.Errorf("TUIServerURL(%q).Validate() returned unexpected error: %v", tt.url, err)
			}
		})
	}
}

func TestTUIServerURL_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  TUIServerURL
		want string
	}{
		{"", ""},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://example.com", "https://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := tt.url.String()
			if got != tt.want {
				t.Errorf("TUIServerURL(%q).String() = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestInvalidTUIServerURLError(t *testing.T) {
	t.Parallel()

	err := &InvalidTUIServerURLError{Value: "ftp://bad"}
	if !strings.Contains(err.Error(), "ftp://bad") {
		t.Errorf("Error() = %q, want containing input value", err.Error())
	}
	var typedErr *InvalidTUIServerURLError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected *InvalidTUIServerURLError, got %T", err)
	}
	if typedErr.Value != "ftp://bad" {
		t.Errorf("InvalidTUIServerURLError.Value = %q, want %q", typedErr.Value, "ftp://bad")
	}
	if !errors.Is(err, ErrInvalidTUIServerURL) {
		t.Error("expected error to wrap ErrInvalidTUIServerURL")
	}
}
