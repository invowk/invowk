// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
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
		{"http url", TUIServerURL("http://localhost:8080"), true, false},
		{"https url", TUIServerURL("https://example.com/tui"), true, false},
		{"empty is valid (zero value)", TUIServerURL(""), true, false},
		{"no scheme is invalid", TUIServerURL("localhost:8080"), false, true},
		{"ftp scheme is invalid", TUIServerURL("ftp://server"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.url.Validate()
			if (err == nil) != tt.want {
				t.Errorf("TUIServerURL(%q).Validate() valid = %v, want %v", tt.url, err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TUIServerURL(%q).Validate() returned nil, want error", tt.url)
				}
				if !errors.Is(err, ErrInvalidTUIServerURL) {
					t.Errorf("error should wrap ErrInvalidTUIServerURL, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("TUIServerURL(%q).Validate() returned unexpected error: %v", tt.url, err)
			}
		})
	}
}

func TestTUIServerToken_Validate(t *testing.T) {
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
			err := tt.token.Validate()
			if (err == nil) != tt.want {
				t.Errorf("TUIServerToken(%q).Validate() valid = %v, want %v", tt.token, err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TUIServerToken(%q).Validate() returned nil, want error", tt.token)
				}
				if !errors.Is(err, ErrInvalidTUIServerToken) {
					t.Errorf("error should wrap ErrInvalidTUIServerToken, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("TUIServerToken(%q).Validate() returned unexpected error: %v", tt.token, err)
			}
		})
	}
}

func TestTUIServerURL_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  TUIServerURL
		want string
	}{
		{"http url", TUIServerURL("http://localhost:8080"), "http://localhost:8080"},
		{"empty", TUIServerURL(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.url.String(); got != tt.want {
				t.Errorf("TUIServerURL(%q).String() = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestTUIServerToken_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token TUIServerToken
		want  string
	}{
		{"valid token", TUIServerToken("abc123"), "abc123"},
		{"empty", TUIServerToken(""), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.token.String(); got != tt.want {
				t.Errorf("TUIServerToken(%q).String() = %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

func TestTUIContext_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ctx       TUIContext
		want      bool
		wantErr   bool
		wantCount int // expected number of field errors
	}{
		{
			"all valid",
			TUIContext{
				ServerURL:   TUIServerURL("http://localhost:8080"),
				ServerToken: TUIServerToken("abc123"),
			},
			true, false, 0,
		},
		{
			"zero value is valid (both fields zero-valid)",
			TUIContext{},
			true, false, 0,
		},
		{
			"valid URL, empty token",
			TUIContext{
				ServerURL:   TUIServerURL("https://example.com"),
				ServerToken: TUIServerToken(""),
			},
			true, false, 0,
		},
		{
			"invalid URL (no scheme)",
			TUIContext{
				ServerURL:   TUIServerURL("localhost:8080"),
				ServerToken: TUIServerToken("abc123"),
			},
			false, true, 1,
		},
		{
			"invalid token (whitespace-only)",
			TUIContext{
				ServerURL:   TUIServerURL("http://localhost:8080"),
				ServerToken: TUIServerToken("   "),
			},
			false, true, 1,
		},
		{
			"both invalid",
			TUIContext{
				ServerURL:   TUIServerURL("ftp://server"),
				ServerToken: TUIServerToken("\t"),
			},
			false, true, 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.ctx.Validate()
			if (err == nil) != tt.want {
				t.Errorf("TUIContext.Validate() valid = %v, want %v", err == nil, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("TUIContext.Validate() returned nil, want error")
				}
				if !errors.Is(err, ErrInvalidTUIContext) {
					t.Errorf("error should wrap ErrInvalidTUIContext, got: %v", err)
				}
				var ctxErr *InvalidTUIContextError
				if !errors.As(err, &ctxErr) {
					t.Fatalf("error should be *InvalidTUIContextError, got: %T", err)
				}
				if len(ctxErr.FieldErrors) != tt.wantCount {
					t.Errorf("field errors count = %d, want %d", len(ctxErr.FieldErrors), tt.wantCount)
				}
			} else if err != nil {
				t.Errorf("TUIContext.Validate() returned unexpected error: %v", err)
			}
		})
	}
}
