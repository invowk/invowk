// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"testing"
	"time"
)

func TestGlobPattern_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern GlobPattern
		want    bool
		wantErr bool
	}{
		{"simple pattern", GlobPattern("*.go"), true, false},
		{"recursive pattern", GlobPattern("**/*.go"), true, false},
		{"directory pattern", GlobPattern("src/**/*.ts"), true, false},
		{"single file", GlobPattern("Makefile"), true, false},
		{"question mark wildcard", GlobPattern("file?.txt"), true, false},
		{"character class", GlobPattern("[abc].txt"), true, false},
		{"empty is invalid", GlobPattern(""), false, true},
		{"unclosed bracket", GlobPattern("[invalid"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.pattern.Validate()
			if (err == nil) != tt.want {
				t.Errorf("GlobPattern(%q).Validate() error = %v, want valid=%v", tt.pattern, err, tt.want)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatalf("GlobPattern(%q).Validate() returned nil, want error", tt.pattern)
				}
				if !errors.Is(err, ErrInvalidGlobPattern) {
					t.Errorf("error should wrap ErrInvalidGlobPattern, got: %v", err)
				}
				var gpErr *InvalidGlobPatternError
				if !errors.As(err, &gpErr) {
					t.Errorf("error should be *InvalidGlobPatternError, got: %T", err)
				}
			} else if err != nil {
				t.Errorf("GlobPattern(%q).Validate() returned unexpected error: %v", tt.pattern, err)
			}
		})
	}
}

func TestGlobPattern_String(t *testing.T) {
	t.Parallel()
	g := GlobPattern("**/*.go")
	if g.String() != "**/*.go" {
		t.Errorf("GlobPattern.String() = %q, want %q", g.String(), "**/*.go")
	}
}

func TestParseDebounce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		debounce DurationString
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "empty string returns zero",
			debounce: "",
			want:     0,
			wantErr:  false,
		},
		{
			name:     "500 milliseconds",
			debounce: "500ms",
			want:     500 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "1 second",
			debounce: "1s",
			want:     1 * time.Second,
			wantErr:  false,
		},
		{
			name:     "invalid string returns error",
			debounce: "invalid",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "zero duration returns error",
			debounce: "0s",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "negative duration returns error",
			debounce: "-1s",
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := &WatchConfig{Debounce: tt.debounce}
			got, err := w.ParseDebounce()

			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDebounce() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseDebounce() = %v, want %v", got, tt.want)
			}
		})
	}
}
